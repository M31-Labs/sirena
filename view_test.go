package sirena_test

import (
	"fmt"
	"strings"
	"testing"

	"m31labs.dev/sirena"
)

// loadView is a small helper that opens a workspace at root, finds the
// view file matching matchSuffix, loads its document, and returns the
// first view declaration. Keeps the per-test boilerplate minimal so each
// case reads like a focused scenario rather than a workspace tour.
func loadView(t *testing.T, root, matchSuffix string) (*sirena.Workspace, *sirena.ViewDecl) {
	t.Helper()
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	var viewFile *sirena.WorkspaceFile
	for _, f := range ws.Files {
		if strings.HasSuffix(f.RelPath, matchSuffix) {
			viewFile = f
		}
	}
	if viewFile == nil {
		t.Fatalf("no view file matching %q in workspace; files=%+v", matchSuffix, ws.Files)
	}
	doc, err := ws.LoadDocument(viewFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Views) == 0 {
		t.Fatalf("no views in %s", viewFile.RelPath)
	}
	return ws, doc.Views[0]
}

// TestEvaluateView_BasicSelection drives Task 20: include directives
// project a subset of the workspace into a ResolvedView. The selected
// boundary brings its children with it, and the named element resolves
// via the workspace symbol table. Unselected names stay excluded.
func TestEvaluateView_BasicSelection(t *testing.T) {
	root := fixtureWorkspace(t, map[string]string{
		"system.sir": `boundary trust "pci" {
  service api
  database db
}
service auth_svc
service unused`,
		"view.view.sir": `view "data" {
  include: [
    boundary "pci"
    service "auth_svc"
  ]
}`,
	})
	ws, vd := loadView(t, root, ".view.sir")
	rv, err := sirena.EvaluateView(ws, vd)
	if err != nil {
		t.Fatal(err)
	}
	if rv.Source != vd {
		t.Errorf("ResolvedView.Source should point at the input ViewDecl")
	}
	if len(rv.Boundaries) != 1 || rv.Boundaries[0].Name != "pci" {
		t.Errorf("Boundaries: %+v", rv.Boundaries)
	}
	names := map[string]bool{}
	for _, e := range rv.Elements {
		names[e.Name] = true
	}
	if !names["auth_svc"] {
		t.Error("auth_svc missing")
	}
	if !names["api"] || !names["db"] {
		t.Errorf("boundary children missing: %v", names)
	}
	if names["unused"] {
		t.Error("unused should be excluded")
	}
}

// TestEvaluateView_IncomingEdges asserts that an `edges: incoming to`
// selector pulls in every Edge whose target resolves to the named
// element, even when those edges live outside the named element's
// boundary.
func TestEvaluateView_IncomingEdges(t *testing.T) {
	root := fixtureWorkspace(t, map[string]string{
		"system.sir": `service api
service worker
database db
api -> db
worker -> db
worker -> api`,
		"view.view.sir": `view "v" {
  include: [
    database "db"
    edges: incoming to "db"
  ]
}`,
	})
	ws, vd := loadView(t, root, ".view.sir")
	rv, err := sirena.EvaluateView(ws, vd)
	if err != nil {
		t.Fatal(err)
	}
	if len(rv.Edges) != 2 {
		t.Errorf("expected 2 edges into db, got %d: %+v", len(rv.Edges), rv.Edges)
	}
	for _, e := range rv.Edges {
		if e.To != "db" {
			t.Errorf("unexpected edge: %+v", e)
		}
	}
}

// TestEvaluateView_OutgoingEdges asserts the symmetric `edges: outgoing
// from` selector picks up every edge whose source resolves to the named
// element.
func TestEvaluateView_OutgoingEdges(t *testing.T) {
	root := fixtureWorkspace(t, map[string]string{
		"system.sir": `service api
service worker
database db
api -> db
api -> worker
worker -> db`,
		"view.view.sir": `view "v" {
  include: [
    service "api"
    edges: outgoing from "api"
  ]
}`,
	})
	ws, vd := loadView(t, root, ".view.sir")
	rv, err := sirena.EvaluateView(ws, vd)
	if err != nil {
		t.Fatal(err)
	}
	if len(rv.Edges) != 2 {
		t.Errorf("expected 2 edges from api, got %d: %+v", len(rv.Edges), rv.Edges)
	}
	for _, e := range rv.Edges {
		if e.From != "api" {
			t.Errorf("unexpected edge: %+v", e)
		}
	}
}

// TestEvaluateView_Collapse drives Task 21a: a name listed in collapse
// is removed from Boundaries and added as a BoundarySummary carrying the
// hidden-child count and the boundary's label (or name, when no label
// metadata is present).
func TestEvaluateView_Collapse(t *testing.T) {
	root := fixtureWorkspace(t, map[string]string{
		"system.sir": `boundary trust "legacy_batch" {
  service old1
  service old2
  service old3
}`,
		"view.view.sir": `view "v" {
  include: [boundary "legacy_batch"]
  collapse: ["legacy_batch"]
}`,
	})
	ws, vd := loadView(t, root, ".view.sir")
	rv, err := sirena.EvaluateView(ws, vd)
	if err != nil {
		t.Fatal(err)
	}
	if len(rv.Boundaries) != 0 {
		t.Errorf("Boundaries should be empty after collapse: %+v", rv.Boundaries)
	}
	if len(rv.Summaries) != 1 {
		t.Fatalf("Summaries: %d", len(rv.Summaries))
	}
	s := rv.Summaries[0]
	if s.Boundary.Name != "legacy_batch" {
		t.Errorf("Summary boundary: %q", s.Boundary.Name)
	}
	if s.HiddenChildren != 3 {
		t.Errorf("HiddenChildren: %d", s.HiddenChildren)
	}
	if s.Label != "legacy_batch" {
		t.Errorf("Label: %q", s.Label)
	}
	// Collapsed boundary's child elements are pulled out of the view too.
	for _, e := range rv.Elements {
		if e.Name == "old1" || e.Name == "old2" || e.Name == "old3" {
			t.Errorf("collapsed child %q should not appear in Elements", e.Name)
		}
	}
}

// TestEvaluateView_AutoCollapseAndExpand drives Task 21b: the default
// preset auto-collapses boundaries with > 12 children. Listing the
// boundary in `expand:` overrides that auto-collapse and keeps the
// boundary visible.
func TestEvaluateView_AutoCollapseAndExpand(t *testing.T) {
	var children []string
	for i := 0; i < 15; i++ {
		children = append(children, fmt.Sprintf("  service svc%d", i))
	}
	src := fmt.Sprintf("boundary trust \"big\" {\n%s\n}", strings.Join(children, "\n"))

	// Auto-collapse path: preset default + no explicit expand.
	root := fixtureWorkspace(t, map[string]string{
		"system.sir": src,
		"view.view.sir": `view "v" {
  include: [boundary "big"]
  preset: default
}`,
	})
	ws, vd := loadView(t, root, ".view.sir")
	rv, err := sirena.EvaluateView(ws, vd)
	if err != nil {
		t.Fatal(err)
	}
	if len(rv.Boundaries) != 0 {
		t.Errorf("expected auto-collapse, got Boundaries: %+v", rv.Boundaries)
	}
	if len(rv.Summaries) != 1 {
		t.Errorf("Summaries: %d", len(rv.Summaries))
	}

	// Expand override path: same big boundary, but listed in expand.
	root2 := fixtureWorkspace(t, map[string]string{
		"system.sir": src,
		"view2.view.sir": `view "v2" {
  include: [boundary "big"]
  preset: default
  expand: ["big"]
}`,
	})
	ws2, vd2 := loadView(t, root2, "view2.view.sir")
	rv2, err := sirena.EvaluateView(ws2, vd2)
	if err != nil {
		t.Fatal(err)
	}
	if len(rv2.Boundaries) != 1 {
		t.Errorf("expand should override auto-collapse: Boundaries=%+v Summaries=%+v", rv2.Boundaries, rv2.Summaries)
	}
	if len(rv2.Summaries) != 0 {
		t.Errorf("expand should suppress summary: %+v", rv2.Summaries)
	}
}

// TestEvaluateView_CollapseBeatsExpand confirms an explicit collapse:
// directive wins over expand: when both name the same boundary. Collapse
// is a stronger directive (the author is explicitly saying "summarize
// this") and must not be silently overridden.
func TestEvaluateView_CollapseBeatsExpand(t *testing.T) {
	root := fixtureWorkspace(t, map[string]string{
		"system.sir": `boundary trust "thing" {
  service a
  service b
}`,
		"view.view.sir": `view "v" {
  include: [boundary "thing"]
  collapse: ["thing"]
  expand: ["thing"]
}`,
	})
	ws, vd := loadView(t, root, ".view.sir")
	rv, err := sirena.EvaluateView(ws, vd)
	if err != nil {
		t.Fatal(err)
	}
	if len(rv.Boundaries) != 0 {
		t.Errorf("collapse must beat expand: Boundaries=%+v", rv.Boundaries)
	}
	if len(rv.Summaries) != 1 {
		t.Errorf("collapse should produce summary: %+v", rv.Summaries)
	}
}

// TestEvaluateView_PresetRenderDefaults drives Task 21c: a recognized
// preset populates RenderDefaults keyed by element name with the
// preset's per-kind defaults.
func TestEvaluateView_PresetRenderDefaults(t *testing.T) {
	root := fixtureWorkspace(t, map[string]string{
		"system.sir": `service api
database db`,
		"view.view.sir": `view "v" {
  include: [service "api", database "db"]
  preset: default
}`,
	})
	ws, vd := loadView(t, root, ".view.sir")
	rv, err := sirena.EvaluateView(ws, vd)
	if err != nil {
		t.Fatal(err)
	}
	if rv.RenderDefaults == nil {
		t.Fatal("RenderDefaults nil")
	}
	api := rv.RenderDefaults["api"]
	if api == nil || api.Shape != "rectangle" {
		t.Errorf("api defaults: %+v", api)
	}
	db := rv.RenderDefaults["db"]
	if db == nil || db.Shape != "cylinder" {
		t.Errorf("db defaults: %+v", db)
	}
}

// TestEvaluateView_UnknownPreset asserts an unrecognized preset name is
// not an error for v0.1 — it simply produces no RenderDefaults entries.
// Per-element render defaults are advisory and the diagnostic story for
// unknown presets is deferred to Phase 6 polish.
func TestEvaluateView_UnknownPreset(t *testing.T) {
	root := fixtureWorkspace(t, map[string]string{
		"system.sir": `service api`,
		"view.view.sir": `view "v" {
  include: [service "api"]
  preset: nonexistent
}`,
	})
	ws, vd := loadView(t, root, ".view.sir")
	rv, err := sirena.EvaluateView(ws, vd)
	if err != nil {
		t.Fatal(err)
	}
	if len(rv.RenderDefaults) != 0 {
		t.Errorf("unknown preset should not populate RenderDefaults: %+v", rv.RenderDefaults)
	}
}

// TestEvaluateView_PresetForName_KnownAndUnknown documents the helper's
// contract: every shipped preset is reachable by name, unknown names map
// to nil so callers can branch on the lookup result.
func TestEvaluateView_PresetForName_KnownAndUnknown(t *testing.T) {
	if p := sirena.PresetForName("default"); p == nil {
		t.Error("default preset must exist")
	}
	if p := sirena.PresetForName("nonexistent"); p != nil {
		t.Errorf("unknown preset should be nil, got %+v", p)
	}
}
