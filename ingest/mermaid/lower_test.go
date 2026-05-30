package mermaid

import (
	"testing"

	"m31labs.dev/sirena"
)

// TestShapeFor covers B1: the shape-string table.
func TestShapeFor(t *testing.T) {
	cases := map[string]string{
		"flow_vertex_square":        "rect",
		"flow_vertex_rect":          "rect",
		"flow_vertex_round":         "round",
		"flow_vertex_stadium":       "stadium",
		"flow_vertex_subroutine":    "subroutine",
		"flow_vertex_cylinder":      "cylinder",
		"flow_vertex_circle":        "circle",
		"flow_vertex_ellipse":       "ellipse",
		"flow_vertex_diamond":       "diamond",
		"flow_vertex_hexagon":       "hexagon",
		"flow_vertex_odd":           "odd",
		"flow_vertex_trapezoid":     "trapezoid",
		"flow_vertex_inv_trapezoid": "inv_trapezoid",
		"flow_vertex_leanleft":      "leanleft",
		"flow_vertex_leanright":     "leanright",
		"flow_vertex_unknown_xyz":   "", // truly unknown → no shape key
	}
	for nodeType, want := range cases {
		if got := shapeFor(nodeType); got != want {
			t.Errorf("shapeFor(%q)=%q want %q", nodeType, got, want)
		}
	}
}

// TestLower_VerticesWithShapesAndLabels covers B2.
func TestLower_VerticesWithShapesAndLabels(t *testing.T) {
	doc, _, err := Parse([]byte("flowchart LR\n  A[Web] --> B[(DB)]\n"), Options{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(doc.Systems) != 1 {
		t.Fatalf("want 1 system, got %d", len(doc.Systems))
	}
	sys := doc.Systems[0]
	if got := len(sys.Elements); got != 2 {
		t.Fatalf("want 2 elements, got %d", got)
	}

	// Element A: name="A", kind=Node, label="Web", shape="rect"
	a := findElement(sys.Elements, "A")
	if a == nil {
		t.Fatal("element A not found")
	}
	if a.Kind != sirena.ElementKindNode {
		t.Errorf("A.Kind = %v, want ElementKindNode", a.Kind)
	}
	assertMetaString(t, a, "label", "Web")
	assertMetaIdent(t, a, "shape", "rect")

	// Element B: name="B", kind=Node, label="DB", shape="cylinder"
	b := findElement(sys.Elements, "B")
	if b == nil {
		t.Fatal("element B not found")
	}
	if b.Kind != sirena.ElementKindNode {
		t.Errorf("B.Kind = %v, want ElementKindNode", b.Kind)
	}
	assertMetaString(t, b, "label", "DB")
	assertMetaIdent(t, b, "shape", "cylinder")
}

// TestLower_EdgesWithLabel covers C1: edges (labeled and plain) + 3 elements.
func TestLower_EdgesWithLabel(t *testing.T) {
	doc, _, err := Parse([]byte("flowchart LR\n  A -->|reads| B\n  B --> C\n"), Options{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	sys := doc.Systems[0]

	if got := len(sys.Elements); got != 3 {
		t.Fatalf("want 3 elements, got %d", got)
	}
	if got := len(sys.Edges); got != 2 {
		t.Fatalf("want 2 edges, got %d", got)
	}

	// Edge A→B with label "reads"
	ab := findEdge(sys.Edges, "A", "B")
	if ab == nil {
		t.Fatal("edge A→B not found")
	}
	if ab.Kind != sirena.EdgeKindFlow {
		t.Errorf("A→B Kind = %v, want EdgeKindFlow", ab.Kind)
	}
	if ab.Direction != sirena.DirForward {
		t.Errorf("A→B Direction = %v, want DirForward", ab.Direction)
	}
	if ab.Label != "reads" {
		t.Errorf("A→B Label = %q, want %q", ab.Label, "reads")
	}
	if ab.FromRef == nil || ab.FromRef.Name != "A" {
		t.Errorf("A→B FromRef = %v, want &Ref{Name:A}", ab.FromRef)
	}
	if ab.ToRef == nil || ab.ToRef.Name != "B" {
		t.Errorf("A→B ToRef = %v, want &Ref{Name:B}", ab.ToRef)
	}

	// Edge B→C with no label
	bc := findEdge(sys.Edges, "B", "C")
	if bc == nil {
		t.Fatal("edge B→C not found")
	}
	if bc.Label != "" {
		t.Errorf("B→C Label = %q, want empty", bc.Label)
	}
}

// TestLower_ChainedStatement covers the chained a→b→c flat walk.
func TestLower_ChainedStatement(t *testing.T) {
	doc, _, err := Parse([]byte("flowchart LR\n  a --> b --> c\n"), Options{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	sys := doc.Systems[0]
	if got := len(sys.Elements); got != 3 {
		t.Fatalf("want 3 elements, got %d", got)
	}
	if got := len(sys.Edges); got != 2 {
		t.Fatalf("want 2 edges, got %d", got)
	}
	if findEdge(sys.Edges, "a", "b") == nil {
		t.Error("edge a→b not found")
	}
	if findEdge(sys.Edges, "b", "c") == nil {
		t.Error("edge b→c not found")
	}
}

// TestLower_DeduplicatesElements ensures repeated vertex references don't
// create duplicate Elements.
func TestLower_DeduplicatesElements(t *testing.T) {
	doc, _, err := Parse([]byte("flowchart LR\n  A --> B\n  B --> C\n"), Options{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	sys := doc.Systems[0]
	if got := len(sys.Elements); got != 3 {
		t.Fatalf("want 3 elements (no dups), got %d", got)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func findElement(els []*sirena.Element, name string) *sirena.Element {
	for _, e := range els {
		if e.Name == name {
			return e
		}
	}
	return nil
}

func findEdge(edges []*sirena.Edge, from, to string) *sirena.Edge {
	for _, e := range edges {
		if e.From == from && e.To == to {
			return e
		}
	}
	return nil
}

func assertMetaString(t *testing.T, el *sirena.Element, key, want string) {
	t.Helper()
	v, ok := el.Metadata[key]
	if !ok {
		t.Errorf("element %q: missing metadata key %q", el.Name, key)
		return
	}
	s, ok := v.(sirena.String)
	if !ok {
		t.Errorf("element %q: metadata[%q] type %T, want sirena.String", el.Name, key, v)
		return
	}
	if s.Value != want {
		t.Errorf("element %q: metadata[%q] = %q, want %q", el.Name, key, s.Value, want)
	}
}

func assertMetaIdent(t *testing.T, el *sirena.Element, key, want string) {
	t.Helper()
	v, ok := el.Metadata[key]
	if !ok {
		t.Errorf("element %q: missing metadata key %q", el.Name, key)
		return
	}
	id, ok := v.(sirena.Ident)
	if !ok {
		t.Errorf("element %q: metadata[%q] type %T, want sirena.Ident", el.Name, key, v)
		return
	}
	if id.Value != want {
		t.Errorf("element %q: metadata[%q] = %q, want %q", el.Name, key, id.Value, want)
	}
}
