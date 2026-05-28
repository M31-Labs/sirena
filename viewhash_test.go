package sirena_test

import (
	"strings"
	"testing"

	"m31labs.dev/sirena"
)

// buildResolvedView is a test helper: it materialises a small workspace
// from inline files, opens it, loads every view file, and returns the
// ResolvedView for the named view. Used by ViewHash tests where the
// scenario interest is the hash inputs, not the workspace plumbing.
func buildResolvedView(t *testing.T, files map[string]string, viewName string) *sirena.ResolvedView {
	t.Helper()
	root := fixtureWorkspace(t, files)
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range ws.Files {
		if !strings.HasSuffix(f.RelPath, ".view.sir") {
			continue
		}
		doc, err := ws.LoadDocument(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range doc.Views {
			if v.Name == viewName {
				rv, err := sirena.EvaluateView(ws, v)
				if err != nil {
					t.Fatal(err)
				}
				return rv
			}
		}
	}
	t.Fatalf("view %q not found", viewName)
	return nil
}

// TestViewHash_Deterministic confirms the headline contract: hashing the
// same resolved view twice yields the same 32 bytes. The hash function
// must not depend on map iteration order, pointer identity, or any other
// run-to-run state.
func TestViewHash_Deterministic(t *testing.T) {
	files := map[string]string{
		"sys.sir": `service api`,
		"v.view.sir": `view "v" {
  include: [service "api"]
  preset: default
}`,
	}
	rv := buildResolvedView(t, files, "v")
	h1 := sirena.ViewHash(rv)
	h2 := sirena.ViewHash(rv)
	if h1 != h2 {
		t.Errorf("hash not deterministic: %x vs %x", h1, h2)
	}
}

// TestViewHash_NilSafe asserts the nil-receiver fast path: a nil
// ResolvedView hashes to a stable value (SHA-256 of the empty byte
// slice, which is well-known and nonzero).
func TestViewHash_NilSafe(t *testing.T) {
	h := sirena.ViewHash(nil)
	var zero [32]byte
	if h == zero {
		t.Error("hash of nil should not equal zero array (sha256 of empty is nonzero)")
	}
	h2 := sirena.ViewHash(nil)
	if h != h2 {
		t.Error("nil hash not deterministic")
	}
}

// TestViewHash_ChangesOnElementSet drives the headline differentiation:
// adding a second element to the view's include list MUST yield a
// different hash so the layout cache sees the projection has changed.
func TestViewHash_ChangesOnElementSet(t *testing.T) {
	filesA := map[string]string{
		"sys.sir": `service api
service worker`,
		"v.view.sir": `view "v" {
  include: [service "api"]
}`,
	}
	filesB := map[string]string{
		"sys.sir": `service api
service worker`,
		"v.view.sir": `view "v" {
  include: [
    service "api"
    service "worker"
  ]
}`,
	}
	h1 := sirena.ViewHash(buildResolvedView(t, filesA, "v"))
	h2 := sirena.ViewHash(buildResolvedView(t, filesB, "v"))
	if h1 == h2 {
		t.Error("hash should differ when element set differs")
	}
}

// TestViewHash_ChangesOnEdgeSet asserts edge-set differences also flip
// the hash. We compare a view with no edges against a view that pulls
// every edge outgoing from "api"; only the second hits any edges.
func TestViewHash_ChangesOnEdgeSet(t *testing.T) {
	filesA := map[string]string{
		"sys.sir": `service api
database db`,
		"v.view.sir": `view "v" {
  include: [
    service "api"
    database "db"
  ]
}`,
	}
	filesB := map[string]string{
		"sys.sir": `service api
database db
api -> db: reads`,
		"v.view.sir": `view "v" {
  include: [
    service "api"
    database "db"
    edges: outgoing from "api"
  ]
}`,
	}
	h1 := sirena.ViewHash(buildResolvedView(t, filesA, "v"))
	h2 := sirena.ViewHash(buildResolvedView(t, filesB, "v"))
	if h1 == h2 {
		t.Error("hash should differ when edge set differs")
	}
}

// TestViewHash_ChangesOnPreset asserts the preset name is part of the
// hash input. Two views that differ only in `preset: default` must hash
// differently because preset choice rotates the rendering defaults.
func TestViewHash_ChangesOnPreset(t *testing.T) {
	base := map[string]string{
		"sys.sir": `service api`,
		"v.view.sir": `view "v" {
  include: [service "api"]
}`,
	}
	withPreset := map[string]string{
		"sys.sir": `service api`,
		"v.view.sir": `view "v" {
  include: [service "api"]
  preset: default
}`,
	}
	h1 := sirena.ViewHash(buildResolvedView(t, base, "v"))
	h2 := sirena.ViewHash(buildResolvedView(t, withPreset, "v"))
	if h1 == h2 {
		t.Error("hash should differ on preset change")
	}
}

// TestViewHash_ChangesOnThemeLayoutBudget drives the remaining three
// hash-input axes — theme, layout hints, budget — in a single
// table-driven block so adding a new axis later is a one-line patch.
func TestViewHash_ChangesOnThemeLayoutBudget(t *testing.T) {
	base := map[string]string{
		"sys.sir": `service api`,
		"v.view.sir": `view "v" {
  include: [service "api"]
}`,
	}
	cases := map[string]string{
		"theme": `view "v" {
  include: [service "api"]
  theme: earth-default
}`,
		"layout": `view "v" {
  include: [service "api"]
  layout {
    direction: top-down
  }
}`,
		"budget": `view "v" {
  include: [service "api"]
  budget {
    nodes: 10
  }
}`,
	}
	h0 := sirena.ViewHash(buildResolvedView(t, base, "v"))
	for name, viewSrc := range cases {
		files := map[string]string{
			"sys.sir":    `service api`,
			"v.view.sir": viewSrc,
		}
		h := sirena.ViewHash(buildResolvedView(t, files, "v"))
		if h == h0 {
			t.Errorf("%s: hash should differ", name)
		}
	}
}

// TestViewHash_StableAcrossLabelChange enforces the negative half of the
// contract: render-time metadata (labels) is deliberately NOT part of
// the hash. A label edit should never invalidate the layout cache.
func TestViewHash_StableAcrossLabelChange(t *testing.T) {
	filesA := map[string]string{
		"sys.sir": `service api {
  label: "first"
}`,
		"v.view.sir": `view "v" { include: [service "api"] }`,
	}
	filesB := map[string]string{
		"sys.sir": `service api {
  label: "different"
}`,
		"v.view.sir": `view "v" { include: [service "api"] }`,
	}
	h1 := sirena.ViewHash(buildResolvedView(t, filesA, "v"))
	h2 := sirena.ViewHash(buildResolvedView(t, filesB, "v"))
	if h1 != h2 {
		t.Errorf("hash should be stable across label change: %x vs %x", h1, h2)
	}
}

// TestViewHash_ChangesOnExpandCollapse asserts an explicit `collapse:`
// directive changes the hash even when the included boundary is the
// same — the collapse list shapes which children land in the
// projection.
func TestViewHash_ChangesOnExpandCollapse(t *testing.T) {
	sys := `boundary trust "pci" {
  service api
  database db
}`
	base := map[string]string{
		"sys.sir":    sys,
		"v.view.sir": `view "v" { include: [boundary "pci"] }`,
	}
	withCollapse := map[string]string{
		"sys.sir": sys,
		"v.view.sir": `view "v" {
  include: [boundary "pci"]
  collapse: ["pci"]
}`,
	}
	h1 := sirena.ViewHash(buildResolvedView(t, base, "v"))
	h2 := sirena.ViewHash(buildResolvedView(t, withCollapse, "v"))
	if h1 == h2 {
		t.Error("hash should differ when collapse changes")
	}
}
