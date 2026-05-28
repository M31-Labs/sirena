package sirena_test

import (
	"reflect"
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

func TestParse_Edge_Forward_Typed(t *testing.T) {
	src := []byte(`api -> db: reads "user records"`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(doc.Systems) != 1 {
		t.Fatalf("Systems: %d", len(doc.Systems))
	}
	sys := doc.Systems[0]
	if len(sys.Edges) != 1 {
		t.Fatalf("Edges: %d", len(sys.Edges))
	}
	e := sys.Edges[0]
	if e.From != "api" {
		t.Errorf("From: %q", e.From)
	}
	if e.To != "db" {
		t.Errorf("To: %q", e.To)
	}
	if e.Kind != sirena.EdgeKindReads {
		t.Errorf("Kind: %v", e.Kind)
	}
	if e.Direction != sirena.DirForward {
		t.Errorf("Direction: %v", e.Direction)
	}
	if e.Label != "user records" {
		t.Errorf("Label: %q", e.Label)
	}
}

func TestParse_Edge_Reverse(t *testing.T) {
	src := []byte(`api <- worker`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(doc.Systems) != 1 || len(doc.Systems[0].Edges) != 1 {
		t.Fatalf("expected 1 edge, doc=%+v", doc)
	}
	e := doc.Systems[0].Edges[0]
	if e.From != "api" || e.To != "worker" {
		t.Errorf("From/To: %q/%q (want api/worker as written)", e.From, e.To)
	}
	if e.Direction != sirena.DirReverse {
		t.Errorf("Direction: %v", e.Direction)
	}
	if e.Kind != sirena.EdgeKindFlow {
		t.Errorf("Kind (untyped): %v", e.Kind)
	}
}

func TestParse_Edge_Bidirectional(t *testing.T) {
	src := []byte(`api <-> peer`)
	e := sirena.MustParse(src).Systems[0].Edges[0]
	if e.Direction != sirena.DirBidirectional {
		t.Errorf("Direction: %v", e.Direction)
	}
}

func TestParse_Edge_Untyped_Forward(t *testing.T) {
	src := []byte(`api -> db`)
	e := sirena.MustParse(src).Systems[0].Edges[0]
	if e.Kind != sirena.EdgeKindFlow {
		t.Errorf("Kind (untyped fallback): %v", e.Kind)
	}
	if e.Label != "" {
		t.Errorf("Label should be empty: %q", e.Label)
	}
}

func TestParse_Edge_InsideNestedBoundary(t *testing.T) {
	// Exercises edge_decl_nested through TWO levels of boundary nesting,
	// confirming the LALR-state-merge workaround applies cleanly when
	// edge_decl appears in Repeat at multiple depths.
	src := []byte(`boundary network "vpc" {
  service edge
  boundary trust "pci" {
    service api
    database db
    api -> db: reads "rows"
  }
}`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	outer := doc.Systems[0].Boundaries[0]
	inner, ok := outer.Children[1].(*sirena.Boundary)
	if !ok {
		t.Fatalf("outer.Children[1] not Boundary: %T", outer.Children[1])
	}
	if len(inner.Children) != 3 {
		t.Fatalf("inner.Children: %d (want 2 elements + 1 edge)", len(inner.Children))
	}
	e, ok := inner.Children[2].(*sirena.Edge)
	if !ok {
		t.Fatalf("inner.Children[2] should be *Edge, got %T", inner.Children[2])
	}
	if e.Kind != sirena.EdgeKindReads || e.Label != "rows" {
		t.Errorf("Edge: %+v", e)
	}
}

func TestParse_Edge_InsideBoundary(t *testing.T) {
	src := []byte(`boundary trust "pci" {
  service api
  database db
  api -> db: writes
}`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	b := doc.Systems[0].Boundaries[0]
	if len(b.Children) != 3 {
		t.Fatalf("Children: %d (want 2 elements + 1 edge)", len(b.Children))
	}
	e, ok := b.Children[2].(*sirena.Edge)
	if !ok {
		t.Fatalf("Children[2] should be *Edge, got %T", b.Children[2])
	}
	if e.Kind != sirena.EdgeKindWrites {
		t.Errorf("Kind: %v", e.Kind)
	}
	if e.From != "api" || e.To != "db" {
		t.Errorf("From/To: %q/%q", e.From, e.To)
	}
}

func TestParse_ElementMetadata_FullCoverage(t *testing.T) {
	src := []byte(`service api {
  label: "API Service"
  owner: "team:platform"
  weight: 42
  enabled: yes
  tags: [stateless, edge, "v2"]
}`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(doc.Systems) != 1 || len(doc.Systems[0].Elements) != 1 {
		t.Fatalf("expected 1 element, doc=%+v", doc)
	}
	md := doc.Systems[0].Elements[0].Metadata
	if md == nil {
		t.Fatal("Metadata nil")
	}
	if s, ok := md["label"].(sirena.String); !ok || s.Value != "API Service" {
		t.Errorf("label: %#v", md["label"])
	}
	if s, ok := md["owner"].(sirena.String); !ok || s.Value != "team:platform" {
		t.Errorf("owner: %#v", md["owner"])
	}
	if n, ok := md["weight"].(sirena.Number); !ok || n.Value != 42 {
		t.Errorf("weight: %#v", md["weight"])
	}
	if id, ok := md["enabled"].(sirena.Ident); !ok || id.Value != "yes" {
		t.Errorf("enabled: %#v", md["enabled"])
	}
	list, ok := md["tags"].(sirena.List)
	if !ok || len(list.Values) != 3 {
		t.Fatalf("tags: %#v", md["tags"])
	}
	if id, ok := list.Values[0].(sirena.Ident); !ok || id.Value != "stateless" {
		t.Errorf("tags[0]: %#v", list.Values[0])
	}
	if id, ok := list.Values[1].(sirena.Ident); !ok || id.Value != "edge" {
		t.Errorf("tags[1]: %#v", list.Values[1])
	}
	if s, ok := list.Values[2].(sirena.String); !ok || s.Value != "v2" {
		t.Errorf("tags[2]: %#v", list.Values[2])
	}
}

func TestParse_BoundaryMetadata(t *testing.T) {
	src := []byte(`boundary trust "pci" {
  owner: "team:platform"
  tier: 1
} {
  service api
}`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(doc.Systems) != 1 || len(doc.Systems[0].Boundaries) != 1 {
		t.Fatalf("expected 1 boundary, doc=%+v", doc)
	}
	b := doc.Systems[0].Boundaries[0]
	if b.Metadata == nil {
		t.Fatal("boundary.Metadata nil")
	}
	if s, ok := b.Metadata["owner"].(sirena.String); !ok || s.Value != "team:platform" {
		t.Errorf("boundary owner: %#v", b.Metadata["owner"])
	}
	if n, ok := b.Metadata["tier"].(sirena.Number); !ok || n.Value != 1 {
		t.Errorf("boundary tier: %#v", b.Metadata["tier"])
	}
	if len(b.Children) != 1 {
		t.Errorf("boundary children: %d", len(b.Children))
	}
	if e, ok := b.Children[0].(*sirena.Element); !ok || e.Name != "api" {
		t.Errorf("Children[0]: %#v", b.Children[0])
	}
}

func TestParse_BoundaryMetadata_OmittedStillParses(t *testing.T) {
	// Sanity check that a boundary with no metadata block still works after
	// the optional metadata block is added to the grammar.
	src := []byte(`boundary trust "pci" {
  service api
}`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	b := doc.Systems[0].Boundaries[0]
	if b.Metadata != nil {
		t.Errorf("boundary.Metadata should be nil, got %#v", b.Metadata)
	}
	if len(b.Children) != 1 {
		t.Errorf("boundary children: %d", len(b.Children))
	}
}

func TestParse_EdgeMetadata(t *testing.T) {
	src := []byte(`api -> db: writes "user records" {
  rate: 100
  retry: yes
}`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(doc.Systems) != 1 || len(doc.Systems[0].Edges) != 1 {
		t.Fatalf("expected 1 edge, doc=%+v", doc)
	}
	e := doc.Systems[0].Edges[0]
	if e.Metadata == nil {
		t.Fatal("edge.Metadata nil")
	}
	if n, ok := e.Metadata["rate"].(sirena.Number); !ok || n.Value != 100 {
		t.Errorf("edge rate: %#v", e.Metadata["rate"])
	}
	if id, ok := e.Metadata["retry"].(sirena.Ident); !ok || id.Value != "yes" {
		t.Errorf("edge retry: %#v", e.Metadata["retry"])
	}
	if e.Label != "user records" {
		t.Errorf("Label: %q", e.Label)
	}
	if e.Kind != sirena.EdgeKindWrites {
		t.Errorf("Kind: %v", e.Kind)
	}
}

func TestParse_EdgeMetadata_NoLabel(t *testing.T) {
	// Edge metadata is allowed without a label/kind suffix too.
	src := []byte(`api -> db {
  rate: 50
}`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	e := doc.Systems[0].Edges[0]
	if e.Metadata == nil {
		t.Fatal("edge.Metadata nil")
	}
	if n, ok := e.Metadata["rate"].(sirena.Number); !ok || n.Value != 50 {
		t.Errorf("edge rate: %#v", e.Metadata["rate"])
	}
	if e.Kind != sirena.EdgeKindFlow {
		t.Errorf("Kind (untyped): %v", e.Kind)
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

func TestParse_Import(t *testing.T) {
	src := []byte(`import "../shared/platform.sir"`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(doc.Imports) != 1 {
		t.Fatalf("Imports: %d", len(doc.Imports))
	}
	if doc.Imports[0].Path != "../shared/platform.sir" {
		t.Errorf("Path: %q", doc.Imports[0].Path)
	}
}

func TestParse_Import_Multiple(t *testing.T) {
	src := []byte(`import "../a.sir"
import "../b.sir"
service api`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(doc.Imports) != 2 {
		t.Errorf("Imports: %d", len(doc.Imports))
	}
	if len(doc.Systems) != 1 || len(doc.Systems[0].Elements) != 1 {
		t.Errorf("Elements: %d", len(doc.Systems[0].Elements))
	}
}

func TestParse_View_Full(t *testing.T) {
	src := []byte(`view "payments-data-plane" {
  title: "Payments — data plane"
  preset: sociotechnical
  include: [
    boundary "pci"
    service "auth-svc"
    edges: incoming to "api"
  ]
  expand: ["pci"]
  collapse: ["legacy-batch"]
  theme: earth-default
  layout {
    direction: top-down
    pin: { "api": top }
  }
  budget {
    nodes: 30
    edges_per_node: 6
    depth: 4
    boundary_fanout: 12
    label_chars: 64
  }
}`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(doc.Views) != 1 {
		t.Fatalf("Views: %d", len(doc.Views))
	}
	v := doc.Views[0]

	if v.Name != "payments-data-plane" {
		t.Errorf("Name: %q", v.Name)
	}
	if v.Title != "Payments — data plane" {
		t.Errorf("Title: %q", v.Title)
	}
	if v.Preset != "sociotechnical" {
		t.Errorf("Preset: %q", v.Preset)
	}
	if v.Theme != "earth-default" {
		t.Errorf("Theme: %q", v.Theme)
	}

	if len(v.Include) != 3 {
		t.Fatalf("Include: %d", len(v.Include))
	}
	if v.Include[0].Kind != sirena.SelectorBoundary || v.Include[0].Target != "pci" {
		t.Errorf("Include[0]: %+v", v.Include[0])
	}
	if v.Include[1].Kind != sirena.SelectorElement || v.Include[1].ElementKind != sirena.ElementKindService || v.Include[1].Target != "auth-svc" {
		t.Errorf("Include[1]: %+v", v.Include[1])
	}
	if v.Include[2].Kind != sirena.SelectorEdgesIncoming || v.Include[2].Target != "api" {
		t.Errorf("Include[2]: %+v", v.Include[2])
	}

	if !reflect.DeepEqual(v.Expand, []string{"pci"}) {
		t.Errorf("Expand: %v", v.Expand)
	}
	if !reflect.DeepEqual(v.Collapse, []string{"legacy-batch"}) {
		t.Errorf("Collapse: %v", v.Collapse)
	}

	if v.Layout == nil {
		t.Fatal("Layout nil")
	}
	if v.Layout.Direction != "top-down" {
		t.Errorf("Layout.Direction: %q", v.Layout.Direction)
	}
	if v.Layout.Pin["api"] != "top" {
		t.Errorf("Layout.Pin[api]: %q", v.Layout.Pin["api"])
	}

	if v.Budget == nil {
		t.Fatal("Budget nil")
	}
	if v.Budget.Nodes != 30 {
		t.Errorf("Budget.Nodes: %d", v.Budget.Nodes)
	}
	if v.Budget.EdgesPerNode != 6 {
		t.Errorf("Budget.EdgesPerNode: %d", v.Budget.EdgesPerNode)
	}
	if v.Budget.Depth != 4 {
		t.Errorf("Budget.Depth: %d", v.Budget.Depth)
	}
	if v.Budget.BoundaryFanout != 12 {
		t.Errorf("Budget.BoundaryFanout: %d", v.Budget.BoundaryFanout)
	}
	if v.Budget.LabelChars != 64 {
		t.Errorf("Budget.LabelChars: %d", v.Budget.LabelChars)
	}
}

func TestParse_View_Minimal(t *testing.T) {
	src := []byte(`view "small" {}`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(doc.Views) != 1 {
		t.Fatalf("Views: %d", len(doc.Views))
	}
	v := doc.Views[0]
	if v.Name != "small" {
		t.Errorf("Name: %q", v.Name)
	}
	if v.Layout != nil {
		t.Errorf("Layout not nil: %+v", v.Layout)
	}
	if v.Budget != nil {
		t.Errorf("Budget not nil: %+v", v.Budget)
	}
}

func TestParse_QualifiedEdgeTarget(t *testing.T) {
	src := []byte(`import "../shared/platform.sir"
service api
api -> platform.kafka: publishes`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(doc.Systems) != 1 || len(doc.Systems[0].Edges) != 1 {
		t.Fatalf("expected 1 edge, doc=%+v", doc)
	}
	e := doc.Systems[0].Edges[0]
	if e.From != "api" {
		t.Errorf("From: %q", e.From)
	}
	if e.To != "platform.kafka" {
		t.Errorf("To: %q", e.To)
	}
	if e.ToRef == nil || e.ToRef.Namespace != "platform" || e.ToRef.Name != "kafka" {
		t.Errorf("ToRef: %#v", e.ToRef)
	}
	if e.FromRef == nil || e.FromRef.Namespace != "" || e.FromRef.Name != "api" {
		t.Errorf("FromRef: %#v", e.FromRef)
	}
}
