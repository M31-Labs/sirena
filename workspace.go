// Package sirena: workspace discovery.
//
// A Workspace is the unit of cross-file resolution: a directory tree
// containing .sir, .view.sir, and .gen.sir files, rooted at the directory
// holding sirena.toml. Most operations on a Workspace target the symbol
// table or per-file diagnostics; the workspace itself is the registry of
// what files exist and what mode the workspace is in.
//
// Phase 5 only ships discovery + lazy per-file parsing. Resolver / symbol
// table work lands in Tasks 17-19.
package sirena

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// manifestFileName is the conventional name of the workspace manifest at
// the workspace root. The `.toml` extension is a historical misnomer — the
// file is parsed as YAML in v0.1 (see manifest.go for the carry-forward
// note). The rename is deferred to Plan 6.
const manifestFileName = "sirena.toml"

// WorkspaceMode discriminates how a workspace was constructed.
//
// The Standalone mode is the canonical disk-walked workspace. The other
// two modes exist for the mdpp embedding story: SelfContained is a
// synthesized single-file workspace (a fence with no host workspace) and
// WorkspaceResolving is a synthesized workspace whose resolver chains
// through a host workspace (a fence inside a document inside a real
// sirena workspace). Phase 5 only constructs Standalone workspaces;
// the other two are populated by mdpp in Phase 8+.
type WorkspaceMode int

const (
	// ModeUnknown is the zero value. It exists so a forgotten Mode
	// assignment surfaces as "unknown" in diagnostics rather than as
	// any meaningful mode.
	ModeUnknown WorkspaceMode = iota
	// ModeStandalone is a directory walked from disk; the canonical mode
	// for `sirena render` / `sirena lint` on a real workspace.
	ModeStandalone
	// ModeSelfContained is a synthesized single-file workspace — the
	// mdpp fence case with no host workspace. Imports are rejected.
	ModeSelfContained
	// ModeWorkspaceResolving is a synthesized workspace whose resolver
	// chains through a host workspace. Used when mdpp embeds a fence
	// inside a document that lives within a sirena workspace.
	ModeWorkspaceResolving
)

// String returns the short label for a WorkspaceMode. ModeUnknown maps to
// "unknown" so diagnostics flag misuse rather than crashing.
func (m WorkspaceMode) String() string {
	switch m {
	case ModeStandalone:
		return "standalone"
	case ModeSelfContained:
		return "self-contained"
	case ModeWorkspaceResolving:
		return "workspace-resolving"
	default:
		return "unknown"
	}
}

// FileKind discriminates a workspace file's role.
//
// The three recognized kinds correspond to the three file extensions
// understood by sirena v0.1: hand-authored systems (.sir),
// generator-owned systems (.gen.sir), and views (.view.sir). The
// generator-owned variant is split out so the round-trip mechanism can
// treat it as machine-replaceable without disturbing the hand-authored
// declarations the user owns.
type FileKind int

const (
	// FileKindUnknown is the zero value. It exists so workspace consumers
	// can distinguish "not enumerated as a sirena file" from "intentionally
	// a system file."
	FileKindUnknown FileKind = iota
	// FileKindSystem corresponds to a .sir hand-authored system file.
	FileKindSystem
	// FileKindGenSystem corresponds to a .gen.sir generator-owned system
	// file. The round-trip mechanism is allowed to replace these in full;
	// hand-authored .sir files are merged field-by-field.
	FileKindGenSystem
	// FileKindView corresponds to a .view.sir view declaration file.
	FileKindView
)

// String returns the short label for a FileKind. FileKindUnknown
// stringifies to "unknown" so misclassified files surface in diagnostics
// rather than as a real kind.
func (k FileKind) String() string {
	switch k {
	case FileKindSystem:
		return "system"
	case FileKindGenSystem:
		return "gen-system"
	case FileKindView:
		return "view"
	default:
		return "unknown"
	}
}

// fileKindForName classifies a file by its name suffix. Order matters
// because `.view.sir` and `.gen.sir` are both suffixes of `.sir`; the
// most-specific suffix must win.
func fileKindForName(name string) FileKind {
	switch {
	case strings.HasSuffix(name, ".view.sir"):
		return FileKindView
	case strings.HasSuffix(name, ".gen.sir"):
		return FileKindGenSystem
	case strings.HasSuffix(name, ".sir"):
		return FileKindSystem
	default:
		return FileKindUnknown
	}
}

// WorkspaceFile is one source file enumerated under the workspace root.
//
// Document is lazy: nil until Workspace.LoadDocument(f) is called. This
// keeps OpenWorkspace cheap when callers only need the file listing
// (e.g. `sirena list`, file-tree UIs). LoadErr is the cached error from a
// prior LoadDocument call; once set, it short-circuits subsequent loads
// so a malformed file does not get re-parsed on every access.
type WorkspaceFile struct {
	// Path is the absolute on-disk path. Used for I/O and for diagnostic
	// rendering.
	Path string
	// RelPath is the path relative to Workspace.Root, with forward
	// slashes regardless of OS. Used for stable comparisons across
	// platforms and as the key in workspace-internal indexes.
	RelPath string
	// Kind tags the file's role; see FileKind.
	Kind FileKind
	// Document is the cached parse result, populated by LoadDocument.
	Document *Document
	// LoadErr is the cached error from a prior LoadDocument call.
	LoadErr error
}

// Workspace is the unit of cross-file resolution. See the package doc
// comment above for the role of each Mode.
//
// Root is the absolute directory the workspace was opened from; it is
// empty for synthesized SelfContained / WorkspaceResolving workspaces.
// Manifest is non-nil only when a sirena.toml was present at Root.
// Files is the sorted, deterministic list of enumerated source files.
type Workspace struct {
	// Root is the absolute path to the workspace root, or empty for
	// synthesized fence workspaces.
	Root string
	// Mode discriminates how the workspace was constructed.
	Mode WorkspaceMode
	// Manifest is the parsed sirena.toml manifest, or nil if absent.
	Manifest *Manifest
	// Files is the sorted list of enumerated .sir / .view.sir / .gen.sir
	// files under Root.
	Files []*WorkspaceFile
	// symbols is the workspace-level resolver index, populated lazily on
	// the first Resolve / ResolveDiagnostics / Touch call. See resolve.go
	// for the indexing strategy.
	symbols *symbolTable
	// host is the parent workspace consulted on resolver misses, populated
	// for ModeWorkspaceResolving fences synthesized by NewFenceWorkspace.
	// Nil for ModeStandalone and ModeSelfContained workspaces.
	host *Workspace
	// synthDiagnostics holds construction-time findings for synthesized
	// workspaces — currently SIR-IMPORT-IN-FENCE entries gathered when a
	// self-contained fence body declares an import. Surfaced through
	// ResolveDiagnostics alongside the lazy index-build diagnostics.
	synthDiagnostics []Diagnostic
}

// FenceOptions configures a synthesized workspace constructed by
// NewFenceWorkspace. The fields mirror the spec's FenceOptions surface so
// Plan 5's mdpp integration can pass them through unchanged.
type FenceOptions struct {
	// WorkspaceRoot, if non-empty, names a sirena workspace whose symbol
	// table the fence's resolver chains into. When empty, the fence is
	// self-contained and imports are rejected.
	WorkspaceRoot string
	// ViewRef, if non-empty, indicates the fence body is empty and the
	// synthesized workspace should expose the named view from the host
	// workspace. ViewRef is a path relative to WorkspaceRoot (or absolute);
	// NewFenceWorkspace walks the host workspace to find a matching file.
	ViewRef string
}

// NewFenceWorkspace synthesizes a workspace from a single source body
// (typically the body of an mdpp ```sirena fence). The mode is determined
// by opts.WorkspaceRoot:
//
//   - empty: ModeSelfContained — no imports allowed.
//   - non-empty: ModeWorkspaceResolving — resolution chains through the
//     workspace rooted at opts.WorkspaceRoot.
//
// The returned Workspace's Files slice contains a single synthetic
// WorkspaceFile with Path == "<fence>", RelPath == "<fence>", and Kind ==
// FileKindView when opts.ViewRef != "", FileKindSystem otherwise.
//
// Imports declared in the body of a self-contained fence surface as
// SIR-IMPORT-IN-FENCE diagnostics (severity Error). For workspace-resolving
// fences, imports flow through the host workspace's normal resolver path.
//
// If opts.ViewRef is set, src may be empty; the fence delegates to the
// host workspace to load the referenced view file. A missing ViewRef
// target surfaces as an error from NewFenceWorkspace itself rather than as
// a deferred load failure.
func NewFenceWorkspace(src []byte, opts FenceOptions) (*Workspace, error) {
	// ViewRef path: open host, locate the referenced file, attach as the
	// single synthetic WorkspaceFile. The fence body itself (src) is
	// ignored — the view file's parsed content drives downstream work.
	if opts.ViewRef != "" {
		if opts.WorkspaceRoot == "" {
			return nil, fmt.Errorf("sirena: NewFenceWorkspace: ViewRef requires WorkspaceRoot")
		}
		host, err := OpenWorkspace(opts.WorkspaceRoot)
		if err != nil {
			return nil, fmt.Errorf("sirena: NewFenceWorkspace: open host: %w", err)
		}
		viewFile := findHostFile(host, opts.ViewRef)
		if viewFile == nil {
			return nil, fmt.Errorf("sirena: NewFenceWorkspace: view %q not found in workspace %s", opts.ViewRef, host.Root)
		}
		ws := &Workspace{
			Mode:     ModeWorkspaceResolving,
			Manifest: host.Manifest,
			Files: []*WorkspaceFile{
				{
					Path:    viewFile.Path,
					RelPath: viewFile.RelPath,
					Kind:    FileKindView,
				},
			},
			host: host,
		}
		return ws, nil
	}

	// No ViewRef: parse the fence body and treat it as a synthetic system
	// file. WorkspaceRoot decides whether the fence is self-contained or
	// chains into a host workspace.
	doc, err := Parse(src)
	if err != nil {
		return nil, fmt.Errorf("sirena: NewFenceWorkspace: parse fence: %w", err)
	}

	file := &WorkspaceFile{
		Path:     "<fence>",
		RelPath:  "<fence>",
		Kind:     FileKindSystem,
		Document: doc,
	}
	ws := &Workspace{
		Files: []*WorkspaceFile{file},
	}

	if opts.WorkspaceRoot == "" {
		ws.Mode = ModeSelfContained
		// Self-contained: imports are an authoring mistake. Surface each
		// one as a SIR-IMPORT-IN-FENCE error so the fence author sees it
		// in the same channel as other resolution diagnostics.
		for _, imp := range doc.Imports {
			ws.synthDiagnostics = append(ws.synthDiagnostics, Diagnostic{
				Code:     "SIR-IMPORT-IN-FENCE",
				Severity: SeverityError,
				Message:  fmt.Sprintf("imports are not allowed in self-contained fences: %q", imp.Path),
				Range:    imp.Range,
			})
		}
		return ws, nil
	}

	host, err := OpenWorkspace(opts.WorkspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("sirena: NewFenceWorkspace: open host: %w", err)
	}
	ws.Mode = ModeWorkspaceResolving
	ws.Manifest = host.Manifest
	ws.host = host
	// Propagate the host's manifest to the fence's parsed document so view
	// evaluation downstream sees consistent defaults (mirrors the work
	// LoadDocument does for disk-walked workspaces).
	doc.Manifest = host.Manifest
	return ws, nil
}

// findHostFile returns the WorkspaceFile inside host matching the given
// reference, which may be a host-relative path (preferred form) or an
// absolute path under host.Root. Returns nil if no file matches.
func findHostFile(host *Workspace, ref string) *WorkspaceFile {
	if host == nil || ref == "" {
		return nil
	}
	candidate := ref
	if filepath.IsAbs(ref) {
		rel, err := filepath.Rel(host.Root, ref)
		if err != nil || strings.HasPrefix(rel, "..") {
			return nil
		}
		candidate = filepath.ToSlash(rel)
	}
	candidate = filepath.ToSlash(candidate)
	for _, f := range host.Files {
		if f.RelPath == candidate {
			return f
		}
	}
	return nil
}

// OpenWorkspace walks root for `.sir`, `.view.sir`, `.gen.sir` files and
// parses the workspace manifest from sirena.toml if present. The returned
// Workspace has Mode == ModeStandalone. Each file is enumerated but NOT
// parsed; callers load documents on demand via LoadDocument so a
// workspace with hundreds of files opens in O(file-count) syscalls
// instead of O(parse-cost).
//
// OpenWorkspace returns an error only for I/O failures (root unreadable)
// or a malformed manifest. Per-file parse errors surface later through
// LoadDocument's LoadErr; this split lets a CLI report manifest errors
// loudly at startup while still rendering / linting the files that did
// parse cleanly.
//
// Files of unknown kind are silently ignored. Directories whose name
// starts with `.` are skipped wholesale, which excludes .git, .hidden,
// editor caches, and similar infrastructure trees.
func OpenWorkspace(root string) (*Workspace, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("sirena: workspace root: %w", err)
	}

	ws := &Workspace{
		Root: abs,
		Mode: ModeStandalone,
	}

	manifestPath := filepath.Join(abs, manifestFileName)
	manifestBytes, err := os.ReadFile(manifestPath)
	switch {
	case err == nil:
		m, mErr := ParseManifest(manifestBytes)
		if mErr != nil {
			return nil, fmt.Errorf("sirena: parse %s: %w", manifestFileName, mErr)
		}
		ws.Manifest = m
		// Version-skew check: refuse newer minors loudly so a partial
		// read can't silently drop IR fields the build doesn't
		// understand. Older minors are accepted with an upgrade-hint
		// diagnostic surfaced through ResolveDiagnostics.
		if vDiag, ok := CheckVersion(m.SirenaVersion); !ok {
			return nil, fmt.Errorf("sirena: %s: %s: %s", manifestFileName, vDiag.Code, vDiag.Message)
		} else if vDiag.Code != "" {
			ws.synthDiagnostics = append(ws.synthDiagnostics, vDiag)
		}
	case errors.Is(err, fs.ErrNotExist):
		// Missing manifest is OK; Manifest stays nil.
	default:
		return nil, fmt.Errorf("sirena: read %s: %w", manifestFileName, err)
	}

	walkErr := filepath.WalkDir(abs, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// The root must be walkable, but an unreadable sibling
			// directory under the chosen root should not abort enumeration
			// of the rest of the workspace. Skip the offending subtree and
			// continue.
			if path == abs {
				return walkErr
			}
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// Skip dot-prefixed directories wholesale (but never the root
		// itself, which may legitimately live under a dot-prefixed
		// parent the caller chose).
		if d.IsDir() {
			if path != abs && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		kind := fileKindForName(d.Name())
		if kind == FileKindUnknown {
			return nil
		}
		rel, relErr := filepath.Rel(abs, path)
		if relErr != nil {
			return relErr
		}
		ws.Files = append(ws.Files, &WorkspaceFile{
			Path:    path,
			RelPath: filepath.ToSlash(rel),
			Kind:    kind,
		})
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("sirena: walk workspace %s: %w", abs, walkErr)
	}

	sort.Slice(ws.Files, func(i, j int) bool {
		return ws.Files[i].RelPath < ws.Files[j].RelPath
	})
	return ws, nil
}

// LoadDocument parses (or returns the cached parse of) file f's source.
// It is the only path through which a Workspace materializes Documents;
// callers walking ws.Files should never call Parse directly so the cache
// stays authoritative.
//
// On success, f.Document is populated and returned. On failure,
// f.LoadErr is populated and a nil document is returned. Error sources,
// in order of how a caller is most likely to encounter them:
//
//   - I/O error reading f.Path (file deleted between Open and Load,
//     permissions changed, etc).
//   - Parse error from Parse (currently rare — the tree-sitter grammar
//     is lenient — but reserved for future strict modes).
//   - Any error-severity diagnostic from Validate. Validation is run
//     here so workspace consumers can treat any per-file failure
//     uniformly; warning- and info-severity diagnostics do not block
//     the load.
//
// LoadDocument is safe to call repeatedly on the same file. Once
// LoadErr is set, subsequent calls short-circuit and return the same
// error so a malformed file does not get re-parsed on every access.
func (ws *Workspace) LoadDocument(f *WorkspaceFile) (*Document, error) {
	if f == nil {
		return nil, errors.New("sirena: LoadDocument: nil file")
	}
	if f.LoadErr != nil {
		return nil, f.LoadErr
	}
	if f.Document != nil {
		return f.Document, nil
	}

	src, err := os.ReadFile(f.Path)
	if err != nil {
		f.LoadErr = fmt.Errorf("sirena: read %s: %w", f.RelPath, err)
		return nil, f.LoadErr
	}

	doc, err := Parse(src)
	if err != nil {
		f.LoadErr = fmt.Errorf("sirena: parse %s: %w", f.RelPath, err)
		return nil, f.LoadErr
	}

	// Workspace-scoped manifest propagates to per-file documents so
	// downstream consumers (view evaluation, lint) have a single
	// document handle that carries the workspace defaults.
	doc.Manifest = ws.Manifest

	for _, diag := range Validate(doc) {
		if diag.Severity == SeverityError {
			f.LoadErr = fmt.Errorf("sirena: validate %s: %s: %s", f.RelPath, diag.Code, diag.Message)
			return nil, f.LoadErr
		}
	}

	f.Document = doc
	return doc, nil
}
