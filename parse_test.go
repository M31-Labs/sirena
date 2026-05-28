package sirena_test

import (
	"testing"

	"m31labs.dev/sirena"
)

func TestParse_ServiceDeclaration(t *testing.T) {
	doc, err := sirena.Parse([]byte(`service api`))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(doc.Systems) != 1 {
		t.Fatalf("want 1 SystemDecl, got %d", len(doc.Systems))
	}
	sys := doc.Systems[0]
	if len(sys.Elements) != 1 {
		t.Fatalf("want 1 Element, got %d", len(sys.Elements))
	}
	e := sys.Elements[0]
	if e.Kind != sirena.ElementKindService {
		t.Fatalf("Kind: got %v, want ElementKindService", e.Kind)
	}
	if e.Name != "api" {
		t.Fatalf("Name: got %q, want \"api\"", e.Name)
	}
	if e.Range.Start != 0 || e.Range.End != len("service api") {
		t.Errorf("Range: got [%d, %d), want [0, %d)", e.Range.Start, e.Range.End, len("service api"))
	}
}
