package layout

import (
	"testing"

	"m31labs.dev/sirena"
)

func TestCompute_ForcePresetDispatches(t *testing.T) {
	rv := &sirena.ResolvedView{
		Source:   &sirena.ViewDecl{Name: "v"},
		Elements: []*sirena.Element{elem("A"), elem("B"), elem("C"), elem("D")},
		Edges:    []*sirena.Edge{fwd("A", "B"), fwd("B", "C"), fwd("C", "D")},
	}
	layered, _ := Compute(rv, LayoutOptions{})
	forced, _ := Compute(rv, LayoutOptions{Preset: LayoutPresetForce})

	lp := map[string]sirena.Rect{}
	for _, np := range layered.NodePlacements {
		lp[np.Node.Name] = np.Bounds
	}
	if len(forced.NodePlacements) != 4 {
		t.Fatalf("force preset: got %d placements, want 4", len(forced.NodePlacements))
	}
	diff := false
	for _, np := range forced.NodePlacements {
		if lp[np.Node.Name] != np.Bounds {
			diff = true
		}
	}
	if !diff {
		t.Errorf("force preset produced layout identical to layered")
	}
}

func TestCompute_RespectsTopLevelHints(t *testing.T) {
	a := &sirena.Element{Kind: sirena.ElementKindService, Name: "a"}
	b := &sirena.Element{Kind: sirena.ElementKindService, Name: "b"}
	bx := &sirena.Boundary{Kind: sirena.BoundaryKindTrust, Name: "X", Children: []sirena.Node{a}}
	by := &sirena.Boundary{Kind: sirena.BoundaryKindTrust, Name: "Y", Children: []sirena.Node{b}}
	rv := &sirena.ResolvedView{
		Source:     &sirena.ViewDecl{Name: "v", Layout: &sirena.LayoutHints{Direction: "left-right"}},
		Elements:   []*sirena.Element{a, b},
		Boundaries: []*sirena.Boundary{bx, by},
	}

	lr, err := Compute(rv, LayoutOptions{})
	if err != nil {
		t.Fatalf("Compute error: %v", err)
	}
	if len(lr.BoundaryPlacements) != 2 {
		t.Fatalf("BoundaryPlacements = %d, want 2", len(lr.BoundaryPlacements))
	}
	var bxR, byR sirena.Rect
	for _, bp := range lr.BoundaryPlacements {
		switch bp.Boundary.Name {
		case "X":
			bxR = bp.Bounds
		case "Y":
			byR = bp.Bounds
		}
	}
	if bxR.Max.X > byR.Min.X {
		t.Errorf("left-right: X should be entirely left of Y; X=%v Y=%v", bxR, byR)
	}
}

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
