// Package emit builds sirena documents from external sources. It is the
// home of the "projection target" story: a code-understanding front end
// reads source with gotreesitter and emits a typed sirena.Document, which
// the layout engine and renderer turn into a regenerable view of the code.
//
// GoPackageGraph is the first such emitter: it models a Go module's
// package structure as a diagram (one element per package, one depends_on
// edge per intra-module import). It deliberately uses gotreesitter rather
// than go/ast so the same approach generalizes to the other languages
// gotreesitter parses.
package emit

import (
	_ "embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	gt "github.com/odvcencio/gotreesitter"

	"m31labs.dev/sirena"
)

//go:embed go.bin
var goGrammarBlob []byte

var (
	goLangOnce sync.Once
	goLang     *gt.Language
)

func goLanguage() *gt.Language {
	goLangOnce.Do(func() {
		lang, err := gt.LoadLanguage(goGrammarBlob)
		if err != nil {
			panic("sirena/emit: failed to load embedded Go grammar: " + err.Error())
		}
		goLang = lang
	})
	return goLang
}

// GoPackageGraph reads the Go module rooted at dir and returns a
// sirena.Document whose elements are the module's packages and whose edges
// are intra-module import dependencies (EdgeKindDependsOn). Package main
// becomes a gateway element; every other package becomes a service. The
// document has no boundaries — it is a flat package-dependency graph,
// suitable for AllElementsView + Render.
//
// dir must contain (or be under a tree containing) a go.mod; the module
// path from go.mod determines which imports count as intra-module. Files
// that fail to parse are skipped rather than aborting the whole walk.
func GoPackageGraph(dir string) (*sirena.Document, error) {
	module, err := modulePath(dir)
	if err != nil {
		return nil, err
	}

	lang := goLanguage()
	parser := gt.NewParser(lang)

	pkgs := map[string]*goPkg{} // keyed by import path
	getPkg := func(importPath, relDir string) *goPkg {
		p := pkgs[importPath]
		if p == nil {
			label := "./" + relDir
			if relDir == "." {
				label = "./" + filepath.Base(module)
			}
			p = &goPkg{id: identFor(module, importPath), label: label, imports: map[string]bool{}}
			pkgs[importPath] = p
		}
		return p
	}

	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if path != dir && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "vendor" || name == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		src, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		tree, perr := parser.Parse(src)
		if perr != nil || tree == nil || tree.RootNode() == nil {
			return nil
		}
		defer tree.Release()

		pkgName, imports := extractGo(tree.RootNode(), lang, src)
		relDir, rerr := filepath.Rel(dir, filepath.Dir(path))
		if rerr != nil {
			return nil
		}
		relDir = filepath.ToSlash(relDir)
		importPath := module
		if relDir != "." {
			importPath = module + "/" + relDir
		}
		p := getPkg(importPath, relDir)
		if pkgName == "main" {
			p.isMain = true
		}
		for _, imp := range imports {
			if strings.HasPrefix(imp, module) && imp != importPath {
				p.imports[imp] = true
			}
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no Go packages found under %s", dir)
	}

	return buildDoc(module, pkgs), nil
}

// goPkg is the per-package accumulator built during the walk.
type goPkg struct {
	id      string          // sirena identifier, e.g. render_svg
	label   string          // display, e.g. ./render/svg
	isMain  bool            // package main -> gateway
	imports map[string]bool // intra-module import paths
}

// buildDoc turns the accumulated packages into a single-system Document
// with deterministic element and edge ordering. Each element carries
// round-trip identity metadata — a "package" symbol_kind and a source_ref in
// the code://go/<module>#<package> scheme — so emit --update can reconcile a
// regenerated graph against a prior .gen.sir (sid > source_ref > fuzzy).
func buildDoc(module string, pkgs map[string]*goPkg) *sirena.Document {
	sys := &sirena.SystemDecl{}

	paths := make([]string, 0, len(pkgs))
	for p := range pkgs {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, path := range paths {
		p := pkgs[path]
		kind := sirena.ElementKindService
		if p.isMain {
			kind = sirena.ElementKindGateway
		}
		sys.Elements = append(sys.Elements, &sirena.Element{
			Kind: kind,
			Name: p.id,
			Metadata: map[string]sirena.Value{
				"label":       sirena.String{Value: p.label},
				"symbol_kind": sirena.String{Value: "package"},
				"source_ref":  sirena.String{Value: sourceRefForPackage(module, path)},
			},
		})
	}

	type pair struct{ from, to string }
	seen := map[pair]bool{}
	for _, path := range paths {
		p := pkgs[path]
		imps := make([]string, 0, len(p.imports))
		for imp := range p.imports {
			imps = append(imps, imp)
		}
		sort.Strings(imps)
		for _, imp := range imps {
			dst := pkgs[imp]
			if dst == nil {
				continue
			}
			key := pair{p.id, dst.id}
			if seen[key] {
				continue
			}
			seen[key] = true
			sys.Edges = append(sys.Edges, &sirena.Edge{
				From:      p.id,
				To:        dst.id,
				FromRef:   &sirena.Ref{Name: p.id},
				ToRef:     &sirena.Ref{Name: dst.id},
				Kind:      sirena.EdgeKindDependsOn,
				Direction: sirena.DirForward,
			})
		}
	}
	return &sirena.Document{Systems: []*sirena.SystemDecl{sys}}
}

// extractGo returns the package name and import paths from a parsed Go
// source_file CST. It relies on the DFA parse backend (parser.Parse),
// which is the canonical backend for Go; the token-source backend yields a
// degenerate tree for Go and must not be used here.
func extractGo(root *gt.Node, lang *gt.Language, src []byte) (pkgName string, imports []string) {
	for i := 0; i < root.NamedChildCount(); i++ {
		c := root.NamedChild(i)
		switch c.Type(lang) {
		case "package_clause":
			for j := 0; j < c.NamedChildCount(); j++ {
				if gc := c.NamedChild(j); gc.Type(lang) == "package_identifier" {
					pkgName = string(src[gc.StartByte():gc.EndByte()])
				}
			}
		case "import_declaration":
			collectImportStrings(c, lang, src, &imports)
		}
	}
	return pkgName, imports
}

// collectImportStrings gathers every interpreted_string_literal_content
// (the unquoted import path) anywhere under an import_declaration, covering
// both `import "x"` and grouped `import ( ... )` forms, including aliased
// and blank (`_`) imports.
func collectImportStrings(n *gt.Node, lang *gt.Language, src []byte, out *[]string) {
	if n.Type(lang) == "interpreted_string_literal_content" {
		*out = append(*out, string(src[n.StartByte():n.EndByte()]))
		return
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		collectImportStrings(n.NamedChild(i), lang, src, out)
	}
}

// sourceRefForPackage builds the round-trip source_ref URI for a package in
// the code://go/<module>#<package> scheme. The fragment is the package path
// relative to the module ("." for the module root). Because the fuzzy
// rename heuristic keys on the post-# suffix, a module rename keeps the
// suffix stable and the package's identity (sid) carries forward.
func sourceRefForPackage(module, importPath string) string {
	rel := strings.TrimPrefix(strings.TrimPrefix(importPath, module), "/")
	if rel == "" {
		rel = "."
	}
	return "code://go/" + module + "#" + rel
}

// identFor maps a package import path to a sirena identifier by stripping
// the module prefix and replacing path separators and dots.
func identFor(module, importPath string) string {
	rel := strings.TrimPrefix(strings.TrimPrefix(importPath, module), "/")
	if rel == "" {
		rel = filepath.Base(module)
	}
	return strings.NewReplacer("/", "_", "-", "_", ".", "_").Replace(rel)
}

// modulePath reads the module line from the nearest go.mod at or above dir.
func modulePath(dir string) (string, error) {
	cur, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		data, rerr := os.ReadFile(filepath.Join(cur, "go.mod"))
		if rerr == nil {
			for line := range strings.SplitSeq(string(data), "\n") {
				if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
					return strings.TrimSpace(rest), nil
				}
			}
			return "", fmt.Errorf("go.mod at %s has no module line", cur)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", fmt.Errorf("no go.mod found at or above %s", dir)
		}
		cur = parent
	}
}
