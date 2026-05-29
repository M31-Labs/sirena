package sirena_test

// render_test.go is an external test package on purpose: it blank-imports
// the layout sub-package to trigger its init()-time registration of the
// layout computer into the root package. The root package itself must
// never import layout (layout imports root for the IR + geometry types,
// so a root→layout import would cycle); the registration indirection is
// what lets sirena.Render delegate geometry without that cycle.

import (
	"testing"

	"m31labs.dev/sirena"
	_ "m31labs.dev/sirena/layout"
)

func oneElementView() *sirena.ResolvedView {
	return &sirena.ResolvedView{
		Source:   &sirena.ViewDecl{Name: "one"},
		Elements: []*sirena.Element{{Kind: sirena.ElementKindService, Name: "api"}},
	}
}

func TestRender_LayoutPopulated(t *testing.T) {
	rv := oneElementView()
	lr, _, err := sirena.Render(rv, sirena.RenderOptions{})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if lr == nil {
		t.Fatal("Render returned nil LayoutResult; layout engine not wired in")
	}
	if len(lr.NodePlacements) == 0 {
		t.Errorf("lr.NodePlacements is empty, want one placement for the single element")
	}
}

func TestRender_DeterministicSeed(t *testing.T) {
	rv := oneElementView()
	lr, _, err := sirena.Render(rv, sirena.RenderOptions{})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if lr.Seed != sirena.ViewHash(rv) {
		t.Errorf("lr.Seed = %x, want ViewHash(rv) = %x", lr.Seed, sirena.ViewHash(rv))
	}
}
