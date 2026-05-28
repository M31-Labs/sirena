// Package sirena: cross-file name resolution.
//
// resolve.go ships the workspace-level symbol table and the lookup API
// that downstream consumers (edge resolution, view evaluation, lint)
// share. The symbol table is built lazily on the first Resolve /
// ResolveDiagnostics / Touch call so workspaces opened only to enumerate
// files never pay the parse-and-index cost.
//
// The design separates two concerns:
//
//   - Per-file symbol maps (built by buildFileSymbolMap) hold the
//     declarations local to one .sir file, keyed by their workspace-
//     qualified name. A top-level `service api` lands at key "api"; a
//     `boundary trust "pci" { service worker }` lands at "worker" AND
//     "pci.worker" so a caller can resolve either form.
//   - The workspace-level symbol table merges per-file maps into a
//     single bare-name index for cheap O(1) lookups, plus a
//     namespace-aware path through Document.Imports for qualified
//     names like "platform.kafka".
//
// Incremental rebuilds work via fingerprints: a sha256 of file content
// (truncated for log brevity) is stored per file. Touch re-reads the
// file from disk; if the hash matches, it skips the reparse. If it
// differs, the file's symbols are evicted from the workspace-level index
// and the file is re-parsed, re-indexed, and re-merged in place.
package sirena

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Symbol is a workspace-resolvable reference target — an Element,
// Boundary, or Edge declaration. Edges are included because future tools
// (canopy-style "who reads from db?") may want to resolve to specific
// edges. For Phase 5, only Elements and Boundaries are actually used by
// the resolver.
type Symbol struct {
	// Node points at the IR node this symbol declares.
	Node Node
	// File is the workspace file that declares this symbol.
	File *WorkspaceFile
	// QualifiedName is the lookup key: "name" for top-level decls, or
	// "outer.name" for boundary-scoped decls.
	QualifiedName string
}

// symbolTable is the workspace-internal index. byName is the merged
// bare-name lookup table (cheap O(1) for the common case). perFile
// holds each file's qualified-name map so a single Touch can evict and
// rebuild just one file's entries without touching the rest of the
// workspace. fingerprints stores the sha256 hash of each file's source
// so Touch can no-op when the file hasn't changed.
//
// diagnostics accumulates SIR-IMPORT-UNRESOLVED entries gathered during
// indexing. Other resolution diagnostics (e.g. unresolved edge targets)
// land here too as later tasks build on this index.
type symbolTable struct {
	// byName is the top-level symbol lookup; keys are bare names. A
	// single key may have multiple entries when several files declare
	// the same name; ws.Resolve returns the first match. Workspace-wide
	// duplicate-name diagnostics land in Phase 5+ via a workspace-aware
	// Validate pass.
	byName map[string][]*Symbol

	// perFile is the per-file index keyed by WorkspaceFile pointer. The
	// inner map keys are the file's qualified names (bare for top-level,
	// "outer.name" for nested). Used by Touch to invalidate just one
	// file's entries.
	perFile map[*WorkspaceFile]map[string]*Symbol

	// namespaces maps an import namespace (the imported file's basename
	// without ".sir") to the workspace file backing it. Populated during
	// indexing so qualified lookups like "platform.kafka" can route to
	// the imported file's perFile map.
	namespaces map[string]*WorkspaceFile

	// fingerprints is the sha256 hex prefix of each file's source. Touch
	// compares the on-disk hash against this entry to decide whether a
	// reparse is needed.
	fingerprints map[*WorkspaceFile]string

	// diagnostics holds the resolution-time findings gathered during the
	// most recent index build.
	diagnostics []Diagnostic
}

// Resolve returns the Symbol with the given workspace-qualified name, or
// nil if unresolved. Three name shapes are accepted, evaluated in order:
//
//   - Bare name: "api" — matches a top-level element/boundary anywhere
//     in the workspace. Returns the first match found.
//   - Boundary-qualified: "pci.api" — name "api" inside boundary "pci";
//     searched against the per-file qualified-name index.
//   - Namespaced: "platform.kafka" — name "kafka" inside the namespace
//     "platform" (resolved through Document.Imports).
//
// Bare-name lookup takes precedence: a bare-name match short-circuits
// before trying the dotted forms. This is fine for v0.1 because the
// workspace error-checks duplicate names via Validate at the per-file
// level; workspace-wide duplicate detection lands in Phase 5+.
func (ws *Workspace) Resolve(qualifiedName string) *Symbol {
	if qualifiedName == "" {
		return nil
	}
	ws.ensureIndexed()

	// Bare lookup first — covers the common case.
	if matches, ok := ws.symbols.byName[qualifiedName]; ok && len(matches) > 0 {
		return matches[0]
	}

	// Dotted form: try boundary-qualified ("outer.inner") and namespaced
	// ("ns.name") forms. Boundary form is keyed inside each file's
	// perFile map; namespace form routes through ws.symbols.namespaces.
	dot := strings.IndexByte(qualifiedName, '.')
	if dot <= 0 || dot == len(qualifiedName)-1 {
		return nil
	}

	// Boundary-qualified: scan every file's perFile map for the full
	// "outer.inner" key. This is O(files) but only kicks in when the
	// bare lookup missed.
	for _, m := range ws.symbols.perFile {
		if s, ok := m[qualifiedName]; ok {
			return s
		}
	}

	// Namespaced: split on the first dot and look up the second half in
	// the namespaced file's per-file map. The bare name inside the
	// imported file is the lookup key, NOT the qualified form.
	ns := qualifiedName[:dot]
	rest := qualifiedName[dot+1:]
	if nsFile, ok := ws.symbols.namespaces[ns]; ok {
		if m, ok := ws.symbols.perFile[nsFile]; ok {
			if s, ok := m[rest]; ok {
				return s
			}
		}
	}
	return nil
}

// ResolveDiagnostics returns the set of resolution diagnostics gathered
// during the most recent workspace-wide index build. v0.1 emits
// SIR-IMPORT-UNRESOLVED entries; later tasks add SIR-REF-UNRESOLVED for
// unresolved edge targets and similar.
func (ws *Workspace) ResolveDiagnostics() []Diagnostic {
	ws.ensureIndexed()
	out := make([]Diagnostic, len(ws.symbols.diagnostics))
	copy(out, ws.symbols.diagnostics)
	return out
}

// ResolveWorkspace performs full workspace-wide resolution:
//   - Ensures every file is loaded and indexed (calls ensureIndexed).
//   - Walks every edge in every file and binds Edge.FromRef.Definition
//     and Edge.ToRef.Definition to the Symbol.Node they point at.
//   - Emits SIR-REF-UNRESOLVED diagnostics for any From/To that can't
//     be resolved, anchored at the edge's Range.
//
// Returns the cumulative diagnostics (import-resolution diagnostics from
// the index build, plus the SIR-REF-UNRESOLVED entries gathered here).
// Calling ResolveWorkspace again re-resolves edges using the current
// index (useful after Touch).
func (ws *Workspace) ResolveWorkspace() []Diagnostic {
	ws.ensureIndexed()

	// Copy the index-build diagnostics so callers see the full picture in
	// one slice, then layer the per-edge resolution diagnostics on top.
	out := make([]Diagnostic, len(ws.symbols.diagnostics))
	copy(out, ws.symbols.diagnostics)

	for _, f := range ws.Files {
		if f.Kind == FileKindView {
			continue
		}
		doc, err := ws.LoadDocument(f)
		if err != nil || doc == nil {
			// Per-file load failures surface through f.LoadErr; don't
			// double-report them here.
			continue
		}
		for _, e := range allEdges(doc) {
			if e == nil {
				continue
			}
			if e.FromRef != nil {
				if sym := ws.Resolve(e.From); sym != nil {
					e.FromRef.Definition = sym.Node
				} else {
					out = append(out, Diagnostic{
						Code:     "SIR-REF-UNRESOLVED",
						Severity: SeverityError,
						Message:  fmt.Sprintf("unresolved edge source: %q", e.From),
						Range:    e.Range,
					})
				}
			}
			if e.ToRef != nil {
				if sym := ws.Resolve(e.To); sym != nil {
					e.ToRef.Definition = sym.Node
				} else {
					out = append(out, Diagnostic{
						Code:     "SIR-REF-UNRESOLVED",
						Severity: SeverityError,
						Message:  fmt.Sprintf("unresolved edge target: %q", e.To),
						Range:    e.Range,
					})
				}
			}
		}
	}
	return out
}

// allEdges returns every edge in a document, including those nested inside
// boundaries. Reused by Phase 6 view evaluation.
func allEdges(doc *Document) []*Edge {
	var out []*Edge
	if doc == nil {
		return out
	}
	for _, sys := range doc.Systems {
		if sys == nil {
			continue
		}
		out = append(out, sys.Edges...)
		for _, b := range sys.Boundaries {
			out = append(out, collectBoundaryEdges(b)...)
		}
	}
	return out
}

// collectBoundaryEdges recurses into a boundary's children and returns
// every edge declared inside (at any depth). Mirrors the boundary walk
// shape used by collectBoundaryChildren so future passes share one mental
// model.
func collectBoundaryEdges(b *Boundary) []*Edge {
	var out []*Edge
	if b == nil {
		return out
	}
	for _, c := range b.Children {
		switch n := c.(type) {
		case *Edge:
			out = append(out, n)
		case *Boundary:
			out = append(out, collectBoundaryEdges(n)...)
		}
	}
	return out
}

// Touch invalidates the symbol-table entries derived from f's previous
// state and reindexes f if the on-disk content has changed. Other files'
// symbol entries are preserved. If the content fingerprint is unchanged
// Touch is a no-op (the cached Document survives), making Touch safe to
// call from a filesystem-watching client without forcing a reparse on
// every event.
func (ws *Workspace) Touch(f *WorkspaceFile) error {
	if f == nil {
		return fmt.Errorf("sirena: Touch: nil file")
	}
	ws.ensureIndexed()

	src, err := os.ReadFile(f.Path)
	if err != nil {
		return fmt.Errorf("sirena: Touch read %s: %w", f.RelPath, err)
	}
	hash := fingerprint(src)
	if old, ok := ws.symbols.fingerprints[f]; ok && old == hash {
		return nil
	}

	// Content changed (or never indexed): evict, reparse, re-merge.
	ws.evictFile(f)
	f.Document = nil
	f.LoadErr = nil
	doc, err := ws.LoadDocument(f)
	if err != nil {
		// Even if LoadDocument failed the fingerprint is updated so a
		// later Touch with the same broken content also no-ops; the
		// per-file map stays empty so nothing resolves through f.
		ws.symbols.fingerprints[f] = hash
		return err
	}
	ws.indexFile(f, doc)
	ws.symbols.fingerprints[f] = hash
	return nil
}

// ensureIndexed lazily builds the workspace symbol table on the first
// call. Subsequent calls are O(1) no-ops; Touch is the only path that
// mutates the table after the initial build.
func (ws *Workspace) ensureIndexed() {
	if ws.symbols != nil {
		return
	}
	ws.symbols = &symbolTable{
		byName:       map[string][]*Symbol{},
		perFile:      map[*WorkspaceFile]map[string]*Symbol{},
		namespaces:   map[string]*WorkspaceFile{},
		fingerprints: map[*WorkspaceFile]string{},
	}

	// First pass: parse every file and build per-file symbol maps. Files
	// that fail to load are skipped (LoadErr is already populated on the
	// WorkspaceFile so callers walking ws.Files can still observe the
	// failure); they contribute nothing to byName and namespaces stay
	// unaware of them.
	for _, f := range ws.Files {
		if f.Kind == FileKindView {
			// Views declare presentation, not symbols. Skip indexing so a
			// view file with the same stem as a system file doesn't
			// shadow it through the namespace map.
			continue
		}
		src, err := os.ReadFile(f.Path)
		if err != nil {
			continue
		}
		ws.symbols.fingerprints[f] = fingerprint(src)

		doc, loadErr := ws.LoadDocument(f)
		if loadErr != nil || doc == nil {
			continue
		}
		ws.indexFile(f, doc)
	}

	// Second pass: resolve imports. We do this after the first pass so
	// import targets can themselves be already-indexed files regardless
	// of walk order.
	for _, f := range ws.Files {
		if f.Document == nil {
			continue
		}
		for _, imp := range f.Document.Imports {
			target := ws.resolveImportPath(f, imp.Path)
			if target == nil {
				ws.symbols.diagnostics = append(ws.symbols.diagnostics, Diagnostic{
					Code:     "SIR-IMPORT-UNRESOLVED",
					Severity: SeverityError,
					Message:  fmt.Sprintf("import %q from %s does not resolve to a workspace file", imp.Path, f.RelPath),
					Range:    imp.Range,
				})
				continue
			}
			ns := namespaceForPath(imp.Path)
			if ns != "" {
				ws.symbols.namespaces[ns] = target
			}
		}
	}
}

// indexFile merges f's per-file symbol map into the workspace-level
// byName index. The per-file map itself is stored verbatim on
// symbols.perFile[f] so Touch can evict it cleanly later.
func (ws *Workspace) indexFile(f *WorkspaceFile, doc *Document) {
	m := buildFileSymbolMap(f, doc)
	ws.symbols.perFile[f] = m
	for key, sym := range m {
		// Only top-level (bare) names go into byName. Nested keys like
		// "pci.worker" stay in perFile so they're addressable via the
		// boundary-qualified Resolve path without polluting byName.
		if !strings.Contains(key, ".") {
			ws.symbols.byName[key] = append(ws.symbols.byName[key], sym)
		}
	}
}

// evictFile removes f's entries from byName and clears its perFile
// slot. Namespaces are left in place; if the basename mapping needs to
// move it is restored by the subsequent indexFile + import pass. For
// v0.1 Touch does not re-walk imports — that's deferred until a client
// needs it (Phase 6).
func (ws *Workspace) evictFile(f *WorkspaceFile) {
	old, ok := ws.symbols.perFile[f]
	if !ok {
		return
	}
	for key, sym := range old {
		if strings.Contains(key, ".") {
			continue
		}
		bucket := ws.symbols.byName[key]
		for i, existing := range bucket {
			if existing == sym {
				bucket = append(bucket[:i], bucket[i+1:]...)
				break
			}
		}
		if len(bucket) == 0 {
			delete(ws.symbols.byName, key)
		} else {
			ws.symbols.byName[key] = bucket
		}
	}
	delete(ws.symbols.perFile, f)
}

// resolveImportPath maps an import path (as written) to the
// WorkspaceFile it refers to. Resolution rules:
//
//   - Absolute path: matched against ws.Root + RelPath; rejected if it
//     escapes Root.
//   - Relative path: resolved against the importing file's directory,
//     then matched against the workspace file list by RelPath.
//
// Returns nil if no workspace file matches.
func (ws *Workspace) resolveImportPath(from *WorkspaceFile, importPath string) *WorkspaceFile {
	if importPath == "" {
		return nil
	}

	var candidate string
	if filepath.IsAbs(importPath) {
		rel, err := filepath.Rel(ws.Root, importPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			return nil
		}
		candidate = filepath.ToSlash(rel)
	} else {
		fromDir := path.Dir(from.RelPath)
		// path.Join collapses "." and ".." segments in the relative
		// composition so a sibling import "shared/x.sir" from "a.sir"
		// resolves to "shared/x.sir" rather than "./shared/x.sir".
		candidate = path.Join(fromDir, importPath)
		if strings.HasPrefix(candidate, "..") {
			return nil
		}
	}

	for _, f := range ws.Files {
		if f.RelPath == candidate {
			return f
		}
	}
	return nil
}

// namespaceForPath returns the basename of an import path without the
// ".sir" extension. The ".view.sir" and ".gen.sir" suffixes are not
// special-cased — they're not valid import targets in v0.1, and the
// import resolver above will reject the path before this is called.
func namespaceForPath(importPath string) string {
	base := path.Base(importPath)
	base = strings.TrimSuffix(base, ".sir")
	return base
}

// fingerprint returns the 16-char hex prefix of the sha256 of src. The
// truncated form is enough for change detection (the full hash would be
// overkill for a per-file content cache) and keeps debug output and
// future diagnostic surfaces short.
func fingerprint(src []byte) string {
	sum := sha256.Sum256(src)
	return hex.EncodeToString(sum[:])[:16]
}

// buildFileSymbolMap walks doc and returns a map of qualified names to
// the Symbols they declare. Top-level elements and boundaries land at
// their bare name. Nested children (inside a boundary) land at BOTH
// their bare name (so cross-file lookups can find them workspace-wide)
// AND at "outer.inner" (so boundary-qualified lookups work). Edges are
// not symbols — they have no name to resolve against.
//
// Boundary nesting is recursive: a boundary inside a boundary contributes
// keys "innerBoundary" and "outerBoundary.innerBoundary"; its children
// land at "innerBoundary.child" (one level of qualification, matching
// the way authors write "pci.worker" rather than "edge.pci.worker").
func buildFileSymbolMap(f *WorkspaceFile, doc *Document) map[string]*Symbol {
	out := map[string]*Symbol{}
	if doc == nil {
		return out
	}
	for _, sys := range doc.Systems {
		if sys == nil {
			continue
		}
		for _, e := range sys.Elements {
			if e == nil || e.Name == "" {
				continue
			}
			out[e.Name] = &Symbol{Node: e, File: f, QualifiedName: e.Name}
		}
		for _, b := range sys.Boundaries {
			if b == nil {
				continue
			}
			if b.Name != "" {
				out[b.Name] = &Symbol{Node: b, File: f, QualifiedName: b.Name}
			}
			collectBoundaryChildren(f, b, out)
		}
	}
	return out
}

// collectBoundaryChildren recurses into b's children, registering each
// element / boundary under its bare name and its "boundary.child"
// qualified form. The qualifier is always the immediate parent's name;
// deeper paths ("outer.inner.leaf") are intentionally not generated —
// authors write at most one level of qualification in v0.1 and the
// resolver mirrors that surface.
func collectBoundaryChildren(f *WorkspaceFile, parent *Boundary, out map[string]*Symbol) {
	for _, child := range parent.Children {
		switch c := child.(type) {
		case *Element:
			if c == nil || c.Name == "" {
				continue
			}
			sym := &Symbol{Node: c, File: f, QualifiedName: parent.Name + "." + c.Name}
			out[c.Name] = &Symbol{Node: c, File: f, QualifiedName: c.Name}
			out[parent.Name+"."+c.Name] = sym
		case *Boundary:
			if c == nil {
				continue
			}
			if c.Name != "" {
				out[c.Name] = &Symbol{Node: c, File: f, QualifiedName: c.Name}
				out[parent.Name+"."+c.Name] = &Symbol{Node: c, File: f, QualifiedName: parent.Name + "." + c.Name}
			}
			collectBoundaryChildren(f, c, out)
		}
	}
}
