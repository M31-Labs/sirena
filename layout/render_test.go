package layout_test

import (
	"testing"

	"m31labs.dev/sirena"
	"m31labs.dev/sirena/layout"
)

// TestRender_AlwaysLinksLayout verifies the convenience entrypoint returns a
// positioned layout. Importing the layout package is enough to guarantee the
// engine is linked, so layout.Render never produces the engine-not-linked nil
// result that bare sirena.Render yields when layout is absent from the build.
func TestRender_AlwaysLinksLayout(t *testing.T) {
	doc, err := sirena.Parse([]byte("service frontend\nservice backend\nfrontend -> backend: calls\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	rv := sirena.AllElementsView(doc)

	lr, _, err := layout.Render(rv, sirena.RenderOptions{})
	if err != nil {
		t.Fatalf("layout.Render: %v", err)
	}
	if lr == nil {
		t.Fatal("layout.Render returned nil layout; the engine was not linked")
	}
	if len(lr.NodePlacements) == 0 {
		t.Errorf("layout.Render produced no node placements for a 2-node view")
	}
}

// TestLayoutLinked_TrueWhenImported verifies LayoutLinked reports true once the
// layout package is part of the build (this test binary imports it).
func TestLayoutLinked_TrueWhenImported(t *testing.T) {
	if !sirena.LayoutLinked() {
		t.Error("LayoutLinked() = false, want true (layout is imported into this test binary)")
	}
}
