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

func TestParse_BoundaryNested(t *testing.T) {
	src := []byte(`boundary trust "pci" {
  service api
  database db
}`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(doc.Systems) != 1 {
		t.Fatalf("Systems: %d", len(doc.Systems))
	}
	sys := doc.Systems[0]
	if len(sys.Boundaries) != 1 {
		t.Fatalf("Boundaries: %d", len(sys.Boundaries))
	}
	b := sys.Boundaries[0]
	if b.Kind != sirena.BoundaryKindTrust {
		t.Fatalf("Kind: %v", b.Kind)
	}
	if b.Name != "pci" {
		t.Fatalf("Name: %q", b.Name)
	}
	if len(b.Children) != 2 {
		t.Fatalf("Children: %d", len(b.Children))
	}
	e0, ok := b.Children[0].(*sirena.Element)
	if !ok {
		t.Fatalf("Children[0] not Element: %T", b.Children[0])
	}
	if e0.Kind != sirena.ElementKindService || e0.Name != "api" {
		t.Errorf("Children[0]: %+v", e0)
	}
	e1, ok := b.Children[1].(*sirena.Element)
	if !ok {
		t.Fatalf("Children[1] not Element: %T", b.Children[1])
	}
	if e1.Kind != sirena.ElementKindDatabase || e1.Name != "db" {
		t.Errorf("Children[1]: %+v", e1)
	}
}

func TestParse_BoundaryNested_TwoLevels(t *testing.T) {
	src := []byte(`boundary network "vpc" {
  service edge
  boundary trust "pci" {
    database db
  }
}`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(doc.Systems) != 1 {
		t.Fatalf("Systems: %d", len(doc.Systems))
	}
	sys := doc.Systems[0]
	if len(sys.Boundaries) != 1 {
		t.Fatalf("Boundaries: %d", len(sys.Boundaries))
	}
	outer := sys.Boundaries[0]
	if outer.Kind != sirena.BoundaryKindNetwork {
		t.Fatalf("outer.Kind: %v", outer.Kind)
	}
	if outer.Name != "vpc" {
		t.Fatalf("outer.Name: %q", outer.Name)
	}
	if len(outer.Children) != 2 {
		t.Fatalf("outer.Children: %d", len(outer.Children))
	}
	edge, ok := outer.Children[0].(*sirena.Element)
	if !ok {
		t.Fatalf("outer.Children[0] not Element: %T", outer.Children[0])
	}
	if edge.Kind != sirena.ElementKindService || edge.Name != "edge" {
		t.Errorf("outer.Children[0]: %+v", edge)
	}
	inner, ok := outer.Children[1].(*sirena.Boundary)
	if !ok {
		t.Fatalf("outer.Children[1] not Boundary: %T", outer.Children[1])
	}
	if inner.Kind != sirena.BoundaryKindTrust {
		t.Fatalf("inner.Kind: %v", inner.Kind)
	}
	if inner.Name != "pci" {
		t.Fatalf("inner.Name: %q", inner.Name)
	}
	if len(inner.Children) != 1 {
		t.Fatalf("inner.Children: %d", len(inner.Children))
	}
	db, ok := inner.Children[0].(*sirena.Element)
	if !ok {
		t.Fatalf("inner.Children[0] not Element: %T", inner.Children[0])
	}
	if db.Kind != sirena.ElementKindDatabase || db.Name != "db" {
		t.Errorf("inner.Children[0]: %+v", db)
	}
}
