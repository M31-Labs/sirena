package layout

import (
	"testing"

	"m31labs.dev/sirena"
)

func TestCell_FiveNodeGraph(t *testing.T) {
	a, b, c, d, e := elem("A"), elem("B"), elem("C"), elem("D"), elem("E")
	items := []cellItem{{element: a}, {element: b}, {element: c}, {element: d}, {element: e}}
	edges := []*sirena.Edge{fwd("A", "B"), fwd("A", "C"), fwd("B", "D"), fwd("C", "D"), fwd("D", "E")}
	m := DefaultMetrics()

	nps, sps, bounds := layoutCell(items, edges, m)
	if len(sps) != 0 {
		t.Fatalf("got %d summaries, want 0", len(sps))
	}
	if len(nps) != 5 {
		t.Fatalf("got %d node placements, want 5", len(nps))
	}

	byName := map[string]sirena.Rect{}
	for _, np := range nps {
		byName[np.Node.Name] = np.Bounds
	}
	step := m.NodeHeight() + m.LayerSpacing
	if byName["D"].Min.Y != 2*step {
		t.Errorf("D at y=%v, want %v (rank 2)", byName["D"].Min.Y, 2*step)
	}
	if byName["E"].Min.Y != 3*step {
		t.Errorf("E at y=%v, want %v (rank 3)", byName["E"].Min.Y, 3*step)
	}

	for _, np := range nps {
		if np.Bounds.Width() <= 0 || np.Bounds.Height() <= 0 {
			t.Errorf("node %s has zero-area bounds %v", np.Node.Name, np.Bounds)
		}
	}
	for i := 0; i < len(nps); i++ {
		for j := i + 1; j < len(nps); j++ {
			if nps[i].Bounds.Intersects(nps[j].Bounds) {
				t.Errorf("nodes %s and %s overlap: %v %v",
					nps[i].Node.Name, nps[j].Node.Name, nps[i].Bounds, nps[j].Bounds)
			}
		}
	}
	if bounds.Width() <= 0 {
		t.Errorf("cell bounds width = %v, want > 0", bounds.Width())
	}
}

func TestCell_RespectsSummaries(t *testing.T) {
	a, b := elem("A"), elem("B")
	sum := &sirena.BoundarySummary{Boundary: &sirena.Boundary{Name: "grp"}, Label: "grp", HiddenChildren: 3}
	items := []cellItem{{element: a}, {summary: sum}, {element: b}}
	edges := []*sirena.Edge{fwd("A", "B")}

	nps, sps, _ := layoutCell(items, edges, DefaultMetrics())
	if len(nps) != 2 {
		t.Fatalf("got %d node placements, want 2", len(nps))
	}
	if len(sps) != 1 {
		t.Fatalf("got %d summary placements, want 1", len(sps))
	}

	var aY float64
	for _, np := range nps {
		if np.Node.Name == "A" {
			aY = np.Bounds.Min.Y
		}
	}
	if sps[0].Bounds.Min.Y != aY {
		t.Errorf("summary Y=%v, element A Y=%v; both at rank 0 should share a row",
			sps[0].Bounds.Min.Y, aY)
	}
}

func TestCell_EmptyInput(t *testing.T) {
	nps, sps, bounds := layoutCell(nil, nil, DefaultMetrics())
	if len(nps) != 0 || len(sps) != 0 {
		t.Errorf("empty input should yield no placements; got %d nodes, %d summaries", len(nps), len(sps))
	}
	if bounds != (sirena.Rect{}) {
		t.Errorf("empty bounds = %v, want zero rect", bounds)
	}
}
