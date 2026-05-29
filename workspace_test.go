package sirena_test

import (
	"os"
	"path/filepath"
	"testing"

	"m31labs.dev/sirena"
)

// fixtureWorkspace builds a tiny on-disk workspace in t.TempDir() and
// returns its root. Nested directories implied by the keys of files are
// created automatically; values are written verbatim as the file body.
func fixtureWorkspace(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		full := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// TestOpenWorkspace_EnumeratesAllKinds drives the headline workspace
// behavior: walk a directory tree, identify .sir / .view.sir / .gen.sir
// files by their suffix, classify each with the matching FileKind, parse
// the manifest, and tag the workspace ModeStandalone. Non-sirena files and
// files under dot-prefixed directories are silently skipped.
func TestOpenWorkspace_EnumeratesAllKinds(t *testing.T) {
	root := fixtureWorkspace(t, map[string]string{
		"sirena.toml":                  "default_preset: default\ntheme: earth-default\n",
		"services/api.sir":             "service api",
		"services/db.sir":              "database db",
		"diagrams/data-plane.view.sir": `view "data-plane" { preset: default }`,
		"generated/inventory.gen.sir":  "service inventory",
		"README.md":                    "ignored",
		".hidden/should-skip.sir":      "service ghost",
	})
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	if ws.Mode != sirena.ModeStandalone {
		t.Errorf("Mode = %v, want ModeStandalone", ws.Mode)
	}
	if ws.Manifest == nil {
		t.Fatal("Manifest nil")
	}
	if ws.Manifest.DefaultPreset != "default" {
		t.Errorf("DefaultPreset = %q", ws.Manifest.DefaultPreset)
	}
	if ws.Manifest.Theme != "earth-default" {
		t.Errorf("Theme = %q", ws.Manifest.Theme)
	}

	kinds := map[sirena.FileKind]int{}
	relpaths := map[string]sirena.FileKind{}
	for _, f := range ws.Files {
		kinds[f.Kind]++
		relpaths[f.RelPath] = f.Kind
	}
	if kinds[sirena.FileKindSystem] != 2 {
		t.Errorf("system files: %d (want 2)", kinds[sirena.FileKindSystem])
	}
	if kinds[sirena.FileKindView] != 1 {
		t.Errorf("view files: %d (want 1)", kinds[sirena.FileKindView])
	}
	if kinds[sirena.FileKindGenSystem] != 1 {
		t.Errorf("gen files: %d (want 1)", kinds[sirena.FileKindGenSystem])
	}
	if kinds[sirena.FileKindUnknown] != 0 {
		t.Errorf("unknown files: %d (want 0)", kinds[sirena.FileKindUnknown])
	}
	if _, ok := relpaths[".hidden/should-skip.sir"]; ok {
		t.Errorf("hidden dir not skipped")
	}
}

// TestOpenWorkspace_NoManifest confirms a workspace without sirena.toml
// still opens cleanly: Manifest is nil (not an error) and files enumerate
// normally. This is the bare-directory case for `sirena lint` against a
// scratch tree.
func TestOpenWorkspace_NoManifest(t *testing.T) {
	root := fixtureWorkspace(t, map[string]string{
		"single.sir": "service api",
	})
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	if ws.Manifest != nil {
		t.Errorf("Manifest should be nil: %+v", ws.Manifest)
	}
	if len(ws.Files) != 1 {
		t.Errorf("Files: %d", len(ws.Files))
	}
}

// TestOpenWorkspace_MalformedManifest asserts a syntactically broken
// sirena.toml fails the whole workspace open. The manifest is
// workspace-scoped so the failure must be loud — silently opening with a
// nil manifest would mask theme / preset / output_dir mistakes.
func TestOpenWorkspace_MalformedManifest(t *testing.T) {
	root := fixtureWorkspace(t, map[string]string{
		"sirena.toml": `theme: "unterminated`,
	})
	_, err := sirena.OpenWorkspace(root)
	if err == nil {
		t.Fatal("expected error for malformed manifest")
	}
}

// TestOpenWorkspace_LoadDocument exercises the lazy per-file parse path.
// OpenWorkspace enumerates but does not parse; LoadDocument parses (and
// validates) on demand and caches the result. Errors — including
// error-severity validation diagnostics — surface through both the return
// value and the WorkspaceFile.LoadErr field so callers can either react
// per-file or batch.
func TestOpenWorkspace_LoadDocument(t *testing.T) {
	root := fixtureWorkspace(t, map[string]string{
		"api.sir": `service api { label: "API" }`,
		// Duplicate name within one document — caught by Validate, which
		// LoadDocument promotes to a LoadErr so workspace consumers can
		// treat any per-file failure uniformly.
		"bad.sir": "service dup\nservice dup\n",
	})
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}

	var apiFile, badFile *sirena.WorkspaceFile
	for _, f := range ws.Files {
		switch f.RelPath {
		case "api.sir":
			apiFile = f
		case "bad.sir":
			badFile = f
		}
	}
	if apiFile == nil || badFile == nil {
		t.Fatal("missing files")
	}

	doc, err := ws.LoadDocument(apiFile)
	if err != nil {
		t.Fatal(err)
	}
	if doc == nil {
		t.Fatal("doc nil")
	}
	if len(doc.Systems) == 0 || len(doc.Systems[0].Elements) != 1 {
		t.Errorf("expected 1 element, got systems=%d", len(doc.Systems))
	}
	// Caching: a second LoadDocument returns the same pointer.
	doc2, _ := ws.LoadDocument(apiFile)
	if doc != doc2 {
		t.Error("LoadDocument should cache")
	}

	// bad.sir: confirm LoadErr is populated and a nil document is returned.
	badDoc, err := ws.LoadDocument(badFile)
	if err == nil {
		t.Error("expected error for malformed sirena")
	}
	if badDoc != nil {
		t.Errorf("expected nil document on error, got %+v", badDoc)
	}
	if badFile.LoadErr == nil {
		t.Error("LoadErr should be populated")
	}
}

// TestNewFenceWorkspace_SelfContained drives the headline self-contained
// fence path: a ````sirena` block with no host workspace turns
// into a single-file Workspace tagged ModeSelfContained, with one synthetic
// WorkspaceFile of FileKindSystem and a working bare-name resolver.
func TestNewFenceWorkspace_SelfContained(t *testing.T) {
	src := []byte(`service api
database db`)
	ws, err := sirena.NewFenceWorkspace(src, sirena.FenceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if ws.Mode != sirena.ModeSelfContained {
		t.Errorf("Mode: %v, want ModeSelfContained", ws.Mode)
	}
	if len(ws.Files) != 1 {
		t.Fatalf("Files: %d", len(ws.Files))
	}
	if ws.Files[0].Kind != sirena.FileKindSystem {
		t.Errorf("Kind: %v", ws.Files[0].Kind)
	}
	if ws.Resolve("api") == nil {
		t.Error("api unresolved")
	}
}

// TestNewFenceWorkspace_SelfContained_RejectsImports confirms a fence with
// no host workspace surfaces SIR-IMPORT-IN-FENCE diagnostics rather than
// silently treating the imports as no-ops. Imports inside a self-contained
// fence have no destination to resolve against — they're an authoring
// mistake and must be loud.
func TestNewFenceWorkspace_SelfContained_RejectsImports(t *testing.T) {
	src := []byte(`import "../shared/platform.sir"
service api`)
	ws, err := sirena.NewFenceWorkspace(src, sirena.FenceOptions{})
	if err != nil {
		t.Fatal(err)
	}

	var found bool
	for _, d := range ws.ResolveDiagnostics() {
		if d.Code == "SIR-IMPORT-IN-FENCE" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected SIR-IMPORT-IN-FENCE; got %+v", ws.ResolveDiagnostics())
	}
}

// TestNewFenceWorkspace_WorkspaceResolving_ChainsToHost drives the
// workspace-resolving fence path: when opts.WorkspaceRoot is non-empty the
// fence's resolver chains through the host workspace's symbol table.
// Bare names declared locally in the fence resolve to the fence; bare and
// namespaced names declared in the host resolve through the chained
// lookup.
func TestNewFenceWorkspace_WorkspaceResolving_ChainsToHost(t *testing.T) {
	hostRoot := fixtureWorkspace(t, map[string]string{
		"shared/platform.sir": `service kafka`,
	})
	src := []byte(`import "shared/platform.sir"
service api
api -> platform.kafka: publishes`)
	ws, err := sirena.NewFenceWorkspace(src, sirena.FenceOptions{
		WorkspaceRoot: hostRoot,
	})
	if err != nil {
		t.Fatal(err)
	}
	if ws.Mode != sirena.ModeWorkspaceResolving {
		t.Errorf("Mode: %v", ws.Mode)
	}
	if ws.Resolve("api") == nil {
		t.Error("api (local) unresolved")
	}
	if ws.Resolve("kafka") == nil {
		t.Error("kafka (host) unresolved")
	}
	if ws.Resolve("platform.kafka") == nil {
		t.Error("platform.kafka (namespaced via host) unresolved")
	}
}

// TestNewFenceWorkspace_ViewRef drives the ViewRef path: when opts.ViewRef
// is set the synthesized workspace's single file is the named view file
// from the host workspace. The fence body itself may be empty; the view's
// content drives rendering.
func TestNewFenceWorkspace_ViewRef(t *testing.T) {
	hostRoot := fixtureWorkspace(t, map[string]string{
		"diagrams/main.view.sir": `view "main" {
  title: "Main"
  preset: default
}`,
	})
	ws, err := sirena.NewFenceWorkspace(nil, sirena.FenceOptions{
		WorkspaceRoot: hostRoot,
		ViewRef:       "diagrams/main.view.sir",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ws.Mode != sirena.ModeWorkspaceResolving {
		t.Errorf("Mode: %v", ws.Mode)
	}
	if len(ws.Files) != 1 {
		t.Fatalf("Files: %d", len(ws.Files))
	}
	if ws.Files[0].Kind != sirena.FileKindView {
		t.Errorf("Kind: %v", ws.Files[0].Kind)
	}

	// Load the view doc and confirm the view is parsed.
	doc, err := ws.LoadDocument(ws.Files[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Views) != 1 {
		t.Errorf("Views: %d", len(doc.Views))
	}
	if doc.Views[0].Name != "main" {
		t.Errorf("View name: %q", doc.Views[0].Name)
	}
}

// TestNewFenceWorkspace_ViewRef_NotFound asserts an unresolved ViewRef
// surfaces as an error from NewFenceWorkspace. A missing view file is a
// configuration mistake and must fail loudly at construction time rather
// than producing a workspace whose only file silently fails to load.
func TestNewFenceWorkspace_ViewRef_NotFound(t *testing.T) {
	hostRoot := fixtureWorkspace(t, map[string]string{
		"other.sir": `service x`,
	})
	_, err := sirena.NewFenceWorkspace(nil, sirena.FenceOptions{
		WorkspaceRoot: hostRoot,
		ViewRef:       "diagrams/missing.view.sir",
	})
	if err == nil {
		t.Error("expected error for missing ViewRef")
	}
}

// TestOpenWorkspace_DeterministicFileOrder pins the Files slice to
// lexicographic RelPath order. Stable ordering matters for diagnostic
// output and for tests that assert a Workspace shape.
func TestOpenWorkspace_DeterministicFileOrder(t *testing.T) {
	root := fixtureWorkspace(t, map[string]string{
		"z.sir": "service z",
		"a.sir": "service a",
		"m.sir": "service m",
	})
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	var rels []string
	for _, f := range ws.Files {
		rels = append(rels, f.RelPath)
	}
	want := []string{"a.sir", "m.sir", "z.sir"}
	if len(rels) != len(want) {
		t.Fatalf("file count: %d", len(rels))
	}
	for i := range want {
		if rels[i] != want[i] {
			t.Errorf("rels[%d] = %q, want %q", i, rels[i], want[i])
		}
	}
}

// TestOpenWorkspace_SkipsUnreadableDir confirms a permission-denied
// subdirectory under the root is skipped rather than aborting the whole
// walk, so a readable file alongside it still loads. (When run as root,
// chmod 000 does not restrict access; the test then simply confirms the
// happy path still finds the file.)
func TestOpenWorkspace_SkipsUnreadableDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ok.sir"), []byte("service api"), 0o644); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Skipf("cannot chmod to exercise the skip: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	ws, err := sirena.OpenWorkspace(dir)
	if err != nil {
		t.Fatalf("OpenWorkspace should skip the unreadable dir, got: %v", err)
	}
	found := false
	for _, f := range ws.Files {
		if filepath.Base(f.Path) == "ok.sir" {
			found = true
		}
	}
	if !found {
		t.Error("readable ok.sir not enumerated")
	}
}
