package layout

import (
	"testing"

	"m31labs.dev/sirena"
)

func TestCompute_RoutesEdges(t *testing.T) {
	rv := &sirena.ResolvedView{
		Source:   &sirena.ViewDecl{Name: "two"},
		Elements: []*sirena.Element{{Kind: sirena.ElementKindService, Name: "api"}, {Kind: sirena.ElementKindDatabase, Name: "db"}},
		Edges:    []*sirena.Edge{{From: "api", To: "db", Kind: sirena.EdgeKindReads, Direction: sirena.DirForward}},
	}
	lr, err := Compute(rv, LayoutOptions{})
	if err != nil {
		t.Fatalf("Compute error: %v", err)
	}
	if len(lr.EdgeRoutes) != 1 {
		t.Fatalf("EdgeRoutes = %d, want 1", len(lr.EdgeRoutes))
	}
	r := lr.EdgeRoutes[0]
	if len(r.Points) < 2 {
		t.Errorf("edge route has %d points, want >= 2", len(r.Points))
	}
	if !r.IsOrthogonal() {
		t.Errorf("edge route not orthogonal: %v", r.Points)
	}
}

func TestCompute_OneElement(t *testing.T) {
	rv := &sirena.ResolvedView{
		Source:   &sirena.ViewDecl{Name: "tiny"},
		Elements: []*sirena.Element{{Kind: sirena.ElementKindService, Name: "api"}},
	}
	lr, err := Compute(rv, LayoutOptions{})
	if err != nil {
		t.Fatalf("Compute error: %v", err)
	}
	if len(lr.NodePlacements) != len(rv.Elements) {
		t.Fatalf("NodePlacements = %d, want %d", len(lr.NodePlacements), len(rv.Elements))
	}
	for _, np := range lr.NodePlacements {
		if np.Bounds.Min.X < lr.Bounds.Min.X || np.Bounds.Max.X > lr.Bounds.Max.X ||
			np.Bounds.Min.Y < lr.Bounds.Min.Y || np.Bounds.Max.Y > lr.Bounds.Max.Y {
			t.Errorf("placement bounds %+v not enclosed by canvas bounds %+v", np.Bounds, lr.Bounds)
		}
	}
	if lr.Seed != sirena.ViewHash(rv) {
		t.Errorf("Seed = %x, want ViewHash = %x", lr.Seed, sirena.ViewHash(rv))
	}
}
