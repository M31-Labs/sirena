package emit

import (
	"os"
	"path/filepath"
	"testing"

	"m31labs.dev/sirena"
)

// writeFixtureModule lays down a tiny three-package Go module under a temp
// dir: a root `main` package importing ./a, and a importing ./b. The shape
// exercises the gateway/service distinction and intra-module edge tracking.
func writeFixtureModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":  "module example.com/m\n\ngo 1.26\n",
		"main.go": "package main\n\nimport (\n\t_ \"example.com/m/a\"\n\t\"fmt\"\n)\n\nfunc main() { fmt.Println(\"x\") }\n",
		"a/a.go":  "package a\n\nimport \"example.com/m/b\"\n\nfunc A() { b.B() }\n",
		"b/b.go":  "package b\n\nfunc B() {}\n",
	}
	for rel, body := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestGoPackageGraph_ShapeAndEdges checks the core contract: every package
// becomes an element, package main becomes a gateway, and each intra-module
// import becomes exactly one depends_on edge. Imports of stdlib ("fmt") are
// excluded; blank imports ("_ example.com/m/a") still count.
func TestGoPackageGraph_ShapeAndEdges(t *testing.T) {
	dir := writeFixtureModule(t)
	doc, err := GoPackageGraph(dir)
	if err != nil {
		t.Fatalf("GoPackageGraph: %v", err)
	}
	sys := doc.Systems[0]
	if got := len(sys.Elements); got != 3 {
		t.Fatalf("elements = %d, want 3 (m, a, b)", got)
	}

	kind := map[string]sirena.ElementKind{}
	for _, e := range sys.Elements {
		kind[e.Name] = e.Kind
	}
	if kind["m"] != sirena.ElementKindGateway {
		t.Errorf("root package kind = %v, want gateway (package main)", kind["m"])
	}
	if kind["a"] != sirena.ElementKindService {
		t.Errorf("package a kind = %v, want service", kind["a"])
	}

	type pair struct{ from, to string }
	got := map[pair]bool{}
	for _, e := range sys.Edges {
		if e.Kind != sirena.EdgeKindDependsOn {
			t.Errorf("edge %s->%s kind = %v, want depends_on", e.From, e.To, e.Kind)
		}
		got[pair{e.From, e.To}] = true
	}
	for _, want := range []pair{{"m", "a"}, {"a", "b"}} {
		if !got[want] {
			t.Errorf("missing edge %s -> %s; got %v", want.from, want.to, sys.Edges)
		}
	}
	if len(sys.Edges) != 2 {
		t.Errorf("edges = %d, want 2; got %v", len(sys.Edges), sys.Edges)
	}
}

// TestGoPackageGraph_IdentityMetadata confirms each emitted package element
// carries the round-trip identity metadata: a symbol_kind of "package" and a
// source_ref URI in the code://go/<module>#<package> scheme. This is what the
// agent-emission round trip (sid > source_ref > fuzzy) reconciles against.
func TestGoPackageGraph_IdentityMetadata(t *testing.T) {
	dir := writeFixtureModule(t)
	doc, err := GoPackageGraph(dir)
	if err != nil {
		t.Fatalf("GoPackageGraph: %v", err)
	}
	want := map[string]string{
		"m": "code://go/example.com/m#.",
		"a": "code://go/example.com/m#a",
		"b": "code://go/example.com/m#b",
	}
	for _, e := range doc.Systems[0].Elements {
		sk, _ := e.Metadata["symbol_kind"].(sirena.String)
		if sk.Value != "package" {
			t.Errorf("%s: symbol_kind = %q, want \"package\"", e.Name, sk.Value)
		}
		sr, _ := e.Metadata["source_ref"].(sirena.String)
		if sr.Value != want[e.Name] {
			t.Errorf("%s: source_ref = %q, want %q", e.Name, sr.Value, want[e.Name])
		}
	}
}

// TestGoPackageGraph_Printable confirms the emitted document round-trips
// through the public printer — i.e. the IR an emitter builds is the same IR
// a hand author writes.
func TestGoPackageGraph_Printable(t *testing.T) {
	dir := writeFixtureModule(t)
	doc, err := GoPackageGraph(dir)
	if err != nil {
		t.Fatal(err)
	}
	out, err := sirena.Print(doc)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("printed empty document")
	}
}

// TestGoPackageGraph_NoModule surfaces a clear error when there is no go.mod
// anywhere above the target directory.
func TestGoPackageGraph_NoModule(t *testing.T) {
	if _, err := GoPackageGraph(t.TempDir()); err == nil {
		t.Fatal("expected error for directory with no go.mod")
	}
}
