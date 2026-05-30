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

// TestLower_SubgraphToBoundary covers D2: subgraph → Boundary with children +
// top-level element + edges in sys.Edges regardless of containment.
func TestLower_SubgraphToBoundary(t *testing.T) {
	src := "flowchart TB\n  subgraph net[Network]\n    api[API] --> db[(DB)]\n  end\n  client --> api\n"
	doc, _, err := Parse([]byte(src), Options{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(doc.Systems) != 1 {
		t.Fatalf("want 1 system, got %d", len(doc.Systems))
	}
	sys := doc.Systems[0]

	// Exactly 1 boundary.
	if got := len(sys.Boundaries); got != 1 {
		t.Fatalf("want 1 boundary, got %d", got)
	}
	b := sys.Boundaries[0]
	if b.Name != "net" {
		t.Errorf("boundary Name = %q, want %q", b.Name, "net")
	}
	if b.Kind != sirena.BoundaryKindGroup {
		t.Errorf("boundary Kind = %v, want BoundaryKindGroup", b.Kind)
	}
	// Label metadata.
	labelVal, ok := b.Metadata["label"]
	if !ok {
		t.Error("boundary missing metadata key \"label\"")
	} else {
		s, ok := labelVal.(sirena.String)
		if !ok {
			t.Errorf("boundary metadata[label] type %T, want sirena.String", labelVal)
		} else if s.Value != "Network" {
			t.Errorf("boundary metadata[label] = %q, want %q", s.Value, "Network")
		}
	}
	// 2 child Elements: api and db.
	if got := len(b.Children); got != 2 {
		t.Fatalf("want 2 boundary children, got %d", got)
	}
	var childNames []string
	for _, c := range b.Children {
		el, ok := c.(*sirena.Element)
		if !ok {
			t.Errorf("boundary child type %T, want *sirena.Element", c)
			continue
		}
		childNames = append(childNames, el.Name)
	}
	if !contains(childNames, "api") || !contains(childNames, "db") {
		t.Errorf("boundary children = %v, want [api db] (any order)", childNames)
	}

	// 1 top-level element: client.
	if got := len(sys.Elements); got != 1 {
		t.Fatalf("want 1 top-level element, got %d", got)
	}
	if sys.Elements[0].Name != "client" {
		t.Errorf("top-level element Name = %q, want %q", sys.Elements[0].Name, "client")
	}

	// 2 edges: api→db and client→api, both in sys.Edges.
	if got := len(sys.Edges); got != 2 {
		t.Fatalf("want 2 edges, got %d", got)
	}
	if findEdge(sys.Edges, "api", "db") == nil {
		t.Error("edge api→db not found in sys.Edges")
	}
	if findEdge(sys.Edges, "client", "api") == nil {
		t.Error("edge client→api not found in sys.Edges")
	}
}

// TestLower_SubgraphUnlabeled covers D-bug: subgraph without brackets.
// subgraph net → id="net", no label metadata, 2 children.
func TestLower_SubgraphUnlabeled(t *testing.T) {
	src := "flowchart TB\n  subgraph net\n    api[API] --> db[(DB)]\n  end\n"
	doc, _, err := Parse([]byte(src), Options{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	sys := doc.Systems[0]

	if got := len(sys.Boundaries); got != 1 {
		t.Fatalf("want 1 boundary, got %d", got)
	}
	b := sys.Boundaries[0]
	if b.Name != "net" {
		t.Errorf("boundary Name = %q, want %q", b.Name, "net")
	}
	if b.Kind != sirena.BoundaryKindGroup {
		t.Errorf("boundary Kind = %v, want BoundaryKindGroup", b.Kind)
	}
	// No label metadata for unlabeled subgraph.
	if _, ok := b.Metadata["label"]; ok {
		t.Error("unlabeled boundary should have no \"label\" metadata key")
	}
	// 2 child Elements: api and db.
	if got := len(b.Children); got != 2 {
		t.Fatalf("want 2 boundary children, got %d", got)
	}
	var childNames []string
	for _, c := range b.Children {
		el, ok := c.(*sirena.Element)
		if !ok {
			t.Errorf("child type %T, want *sirena.Element", c)
			continue
		}
		childNames = append(childNames, el.Name)
	}
	if !contains(childNames, "api") || !contains(childNames, "db") {
		t.Errorf("boundary children = %v, want [api db]", childNames)
	}
}

// TestLower_SiblingSubgraphs covers D: two top-level subgraphs → 2 boundaries,
// elements routed to the correct boundary.
func TestLower_SiblingSubgraphs(t *testing.T) {
	src := "flowchart LR\n  subgraph frontend\n    ui[UI]\n  end\n  subgraph backend\n    api[API]\n  end\n  ui --> api\n"
	doc, _, err := Parse([]byte(src), Options{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	sys := doc.Systems[0]

	if got := len(sys.Boundaries); got != 2 {
		t.Fatalf("want 2 boundaries, got %d", got)
	}
	fe := findBoundary(sys.Boundaries, "frontend")
	be := findBoundary(sys.Boundaries, "backend")
	if fe == nil {
		t.Fatal("boundary \"frontend\" not found")
	}
	if be == nil {
		t.Fatal("boundary \"backend\" not found")
	}

	// frontend has child ui; backend has child api.
	if got := len(fe.Children); got != 1 {
		t.Fatalf("frontend: want 1 child, got %d", got)
	}
	if el, ok := fe.Children[0].(*sirena.Element); !ok || el.Name != "ui" {
		t.Errorf("frontend child = %v, want Element{ui}", fe.Children[0])
	}
	if got := len(be.Children); got != 1 {
		t.Fatalf("backend: want 1 child, got %d", got)
	}
	if el, ok := be.Children[0].(*sirena.Element); !ok || el.Name != "api" {
		t.Errorf("backend child = %v, want Element{api}", be.Children[0])
	}

	// Edge ui→api in sys.Edges.
	if findEdge(sys.Edges, "ui", "api") == nil {
		t.Error("edge ui→api not found in sys.Edges")
	}

	// No top-level elements (both ui and api are in boundaries).
	if got := len(sys.Elements); got != 0 {
		t.Errorf("want 0 top-level elements, got %d", got)
	}
}

// TestLower_NestedSubgraphs covers D: 2-level nesting.
// outer has inner boundary as child; inner elements in inner.Children.
func TestLower_NestedSubgraphs(t *testing.T) {
	src := "flowchart TB\n  subgraph outer\n    subgraph inner\n      svc[Svc]\n    end\n  end\n"
	doc, _, err := Parse([]byte(src), Options{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	sys := doc.Systems[0]

	if got := len(sys.Boundaries); got != 1 {
		t.Fatalf("want 1 top-level boundary, got %d", got)
	}
	outerB := sys.Boundaries[0]
	if outerB.Name != "outer" {
		t.Errorf("outer boundary Name = %q, want %q", outerB.Name, "outer")
	}
	if got := len(outerB.Children); got != 1 {
		t.Fatalf("outer: want 1 child (the inner boundary), got %d", got)
	}
	innerB, ok := outerB.Children[0].(*sirena.Boundary)
	if !ok {
		t.Fatalf("outer child type %T, want *sirena.Boundary", outerB.Children[0])
	}
	if innerB.Name != "inner" {
		t.Errorf("inner boundary Name = %q, want %q", innerB.Name, "inner")
	}
	if got := len(innerB.Children); got != 1 {
		t.Fatalf("inner: want 1 child element, got %d", got)
	}
	el, ok := innerB.Children[0].(*sirena.Element)
	if !ok {
		t.Fatalf("inner child type %T, want *sirena.Element", innerB.Children[0])
	}
	if el.Name != "svc" {
		t.Errorf("inner child Name = %q, want %q", el.Name, "svc")
	}
}

// TestLower_CrossReferencedElement covers D: element declared inside a subgraph,
// referenced by a top-level edge. api stays in boundary.Children (not duplicated
// into sys.Elements); edge is still emitted.
func TestLower_CrossReferencedElement(t *testing.T) {
	src := "flowchart TB\n  subgraph net\n    api[API]\n  end\n  client --> api\n"
	doc, _, err := Parse([]byte(src), Options{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	sys := doc.Systems[0]

	// 1 boundary with api inside it.
	if got := len(sys.Boundaries); got != 1 {
		t.Fatalf("want 1 boundary, got %d", got)
	}
	b := sys.Boundaries[0]
	if got := len(b.Children); got != 1 {
		t.Fatalf("boundary: want 1 child, got %d", got)
	}
	el, ok := b.Children[0].(*sirena.Element)
	if !ok || el.Name != "api" {
		t.Errorf("boundary child = %v, want Element{api}", b.Children[0])
	}

	// api must NOT appear in top-level sys.Elements.
	if findElement(sys.Elements, "api") != nil {
		t.Error("api should not appear in sys.Elements (it lives in boundary)")
	}

	// client IS a top-level element.
	if findElement(sys.Elements, "client") == nil {
		t.Error("client not found in sys.Elements")
	}

	// Edge client→api in sys.Edges.
	if findEdge(sys.Edges, "client", "api") == nil {
		t.Error("edge client→api not found in sys.Edges")
	}
}

// TestLower_EmptySubgraph covers D: empty subgraph → 1 boundary, 0 children, no crash.
func TestLower_EmptySubgraph(t *testing.T) {
	src := "flowchart TB\n  subgraph net\n  end\n"
	doc, _, err := Parse([]byte(src), Options{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	sys := doc.Systems[0]

	if got := len(sys.Boundaries); got != 1 {
		t.Fatalf("want 1 boundary, got %d", got)
	}
	b := sys.Boundaries[0]
	if b.Name != "net" {
		t.Errorf("boundary Name = %q, want %q", b.Name, "net")
	}
	if got := len(b.Children); got != 0 {
		t.Errorf("want 0 children, got %d", got)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func findBoundary(bounds []*sirena.Boundary, name string) *sirena.Boundary {
	for _, b := range bounds {
		if b.Name == name {
			return b
		}
	}
	return nil
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

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
