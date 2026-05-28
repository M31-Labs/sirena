package sirena_test

import (
	"os"
	"testing"

	"m31labs.dev/sirena"
)

// TestResolve_PerFileSymbolMap drives Task 17a: a single file with a
// top-level service, top-level database, and a nested boundary with a
// child service produces symbol entries keyed by qualified name. The
// nested entry's key is "outer.inner" form so callers can resolve
// boundary-scoped names without re-walking the IR.
func TestResolve_PerFileSymbolMap(t *testing.T) {
	root := fixtureWorkspace(t, map[string]string{
		"all.sir": `service api
database db
boundary trust "pci" {
  service worker
}`,
	})
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	if s := ws.Resolve("api"); s == nil {
		t.Error("api unresolved")
	}
	if s := ws.Resolve("db"); s == nil {
		t.Error("db unresolved")
	}
	if s := ws.Resolve("pci.worker"); s == nil {
		t.Error("pci.worker unresolved")
	}
}

// TestResolve_AcrossFiles drives Task 17b: bare-name lookup merges the
// per-file symbol maps so a symbol declared in one file resolves from a
// workspace-wide lookup. Unknown names return nil cleanly.
func TestResolve_AcrossFiles(t *testing.T) {
	root := fixtureWorkspace(t, map[string]string{
		"a.sir": `service api`,
		"b.sir": `database db`,
	})
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	if ws.Resolve("api") == nil {
		t.Error("api unresolved")
	}
	if ws.Resolve("db") == nil {
		t.Error("db unresolved")
	}
	if ws.Resolve("ghost") != nil {
		t.Error("ghost resolved unexpectedly")
	}
}

// TestResolve_NamespacedViaImport drives Task 17c: an `import "x/y.sir"`
// makes y.sir's symbols resolvable as `y.<name>` from the importing file.
// Bare-name lookup still works workspace-wide since every file is also
// merged into byName.
func TestResolve_NamespacedViaImport(t *testing.T) {
	root := fixtureWorkspace(t, map[string]string{
		"shared/platform.sir": `service kafka`,
		"a.sir": `import "shared/platform.sir"
service api
api -> platform.kafka: publishes`,
	})
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	if s := ws.Resolve("platform.kafka"); s == nil {
		t.Error("platform.kafka unresolved")
	}
	if s := ws.Resolve("kafka"); s == nil {
		t.Error("bare kafka should still resolve workspace-wide")
	}
}

// TestResolve_BadImport asserts an unresolvable import path produces a
// SIR-IMPORT-UNRESOLVED diagnostic during indexing. Diagnostics surface
// via ResolveDiagnostics after the first Resolve call triggers indexing.
func TestResolve_BadImport(t *testing.T) {
	root := fixtureWorkspace(t, map[string]string{
		"a.sir": `import "shared/missing.sir"
service api`,
	})
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	ws.Resolve("anything") // triggers indexing
	diags := ws.ResolveDiagnostics()
	var found bool
	for _, d := range diags {
		if d.Code == "SIR-IMPORT-UNRESOLVED" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected SIR-IMPORT-UNRESOLVED, got %+v", diags)
	}
}

// TestResolve_TouchInvalidates drives Task 17d: after disk content
// changes, Touch reindexes the file in place and previously-unknown
// symbols become resolvable. Other files' entries remain untouched.
func TestResolve_TouchInvalidates(t *testing.T) {
	root := fixtureWorkspace(t, map[string]string{
		"a.sir": `service api`,
		"b.sir": `database db`,
	})
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	_ = ws.Resolve("api") // index
	_ = ws.Resolve("db")

	var aFile *sirena.WorkspaceFile
	for _, f := range ws.Files {
		if f.RelPath == "a.sir" {
			aFile = f
		}
	}
	if aFile == nil {
		t.Fatal("a.sir missing")
	}
	if err := os.WriteFile(aFile.Path, []byte("service api\nservice worker"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ws.Touch(aFile); err != nil {
		t.Fatal(err)
	}

	if ws.Resolve("worker") == nil {
		t.Error("worker should resolve after Touch")
	}
	if ws.Resolve("db") == nil {
		t.Error("db (from b.sir) should still resolve")
	}
}

// TestResolve_TouchSkipsUnchanged confirms Touch is a no-op when the
// file content hash matches the recorded fingerprint: the cached
// Document pointer survives and a second LoadDocument returns the same
// pointer. This is the cheap-incremental-rebuild promise.
func TestResolve_TouchSkipsUnchanged(t *testing.T) {
	root := fixtureWorkspace(t, map[string]string{
		"a.sir": `service api`,
	})
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	_ = ws.Resolve("api")

	var aFile *sirena.WorkspaceFile
	for _, f := range ws.Files {
		if f.RelPath == "a.sir" {
			aFile = f
		}
	}
	if aFile == nil {
		t.Fatal("a.sir missing")
	}
	doc1, err := ws.LoadDocument(aFile)
	if err != nil {
		t.Fatal(err)
	}

	if err := ws.Touch(aFile); err != nil {
		t.Fatal(err)
	}
	doc2, err := ws.LoadDocument(aFile)
	if err != nil {
		t.Fatal(err)
	}
	if doc1 != doc2 {
		t.Errorf("Touch should be a no-op for unchanged file")
	}
}
