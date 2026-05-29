package layout

import (
	"math"
	"testing"

	"m31labs.dev/sirena"
)

func seedFrom(b byte) [32]byte {
	var s [32]byte
	for i := range s {
		s[i] = b
	}
	return s
}

func TestForce_SameSeed_SameOutput(t *testing.T) {
	els := []*sirena.Element{elem("A"), elem("B"), elem("C")}
	edges := []*sirena.Edge{fwd("A", "B"), fwd("B", "C")}
	s := seedFrom(7)

	a := forceDirectedLayout(els, edges, s, DefaultMetrics())
	b := forceDirectedLayout(els, edges, s, DefaultMetrics())
	for _, e := range els {
		if a[e.Name] != b[e.Name] {
			t.Errorf("same seed differs for %s: %v vs %v", e.Name, a[e.Name], b[e.Name])
		}
	}
}

func TestForce_DifferentSeeds_DifferentOutput(t *testing.T) {
	els := []*sirena.Element{elem("A"), elem("B"), elem("C")}
	edges := []*sirena.Edge{fwd("A", "B"), fwd("B", "C")}

	a := forceDirectedLayout(els, edges, seedFrom(1), DefaultMetrics())
	b := forceDirectedLayout(els, edges, seedFrom(2), DefaultMetrics())
	sum := func(m map[string]sirena.Rect) float64 {
		s := 0.0
		for _, r := range m {
			s += r.Min.X
		}
		return s
	}
	if sum(a) == sum(b) {
		t.Errorf("different seeds produced identical x-sum %v", sum(a))
	}
}

func TestForce_Converges(t *testing.T) {
	els := []*sirena.Element{elem("C"), elem("P1"), elem("P2"), elem("P3"), elem("P4")}
	edges := []*sirena.Edge{fwd("C", "P1"), fwd("C", "P2"), fwd("C", "P3"), fwd("C", "P4")}

	out := forceDirectedLayout(els, edges, seedFrom(3), DefaultMetrics())
	cc := out["C"].Center()

	perim := []string{"P1", "P2", "P3", "P4"}
	var sx, sy float64
	for _, p := range perim {
		c := out[p].Center()
		sx, sy = sx+c.X, sy+c.Y
	}
	cx, cy := sx/4, sy/4
	centerDist := math.Hypot(cc.X-cx, cc.Y-cy)

	var avgPerim float64
	for _, p := range perim {
		c := out[p].Center()
		avgPerim += math.Hypot(c.X-cx, c.Y-cy)
	}
	avgPerim /= 4
	if centerDist > 0.6*avgPerim {
		t.Errorf("center not near perimeter centroid: centerDist=%v avgPerimRadius=%v", centerDist, avgPerim)
	}
}

func TestForce_NoNaN(t *testing.T) {
	els := []*sirena.Element{elem("A"), elem("B"), elem("C")} // disconnected
	out := forceDirectedLayout(els, nil, seedFrom(9), DefaultMetrics())
	for _, e := range els {
		r := out[e.Name]
		for _, v := range []float64{r.Min.X, r.Min.Y, r.Max.X, r.Max.Y} {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Errorf("non-finite coordinate for %s: %v", e.Name, r)
			}
		}
	}
}
