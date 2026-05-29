package layout

import (
	"math"
	"testing"

	"m31labs.dev/sirena"
)

func TestCoord_TwoNodes_SameLayer(t *testing.T) {
	a, b := elem("aa"), elem("bb") // equal-width labels
	layers := [][]*sirena.Element{{a, b}}
	m := DefaultMetrics()

	coords := assignCoords(layers, nil, m)
	ra, rb := coords["aa"], coords["bb"]
	if ra.Min.Y != 0 || rb.Min.Y != 0 {
		t.Errorf("layer-0 nodes should sit at Min.Y=0; got %v, %v", ra.Min.Y, rb.Min.Y)
	}
	w := m.NodeWidth("aa")
	if math.Abs(ra.Min.X-0) > 1e-9 {
		t.Errorf("first node Min.X = %v, want 0", ra.Min.X)
	}
	if math.Abs(rb.Min.X-(w+m.NodeSpacing)) > 1e-9 {
		t.Errorf("second node Min.X = %v, want %v (W + NodeSpacing)", rb.Min.X, w+m.NodeSpacing)
	}
}

func TestCoord_Chain_StraightLine(t *testing.T) {
	a, b, c := elem("A"), elem("B"), elem("C")
	layers := [][]*sirena.Element{{a}, {b}, {c}}
	edges := []*sirena.Edge{fwd("A", "B"), fwd("B", "C")}

	coords := assignCoords(layers, edges, DefaultMetrics())
	ax, bx, cx := coords["A"].Min.X, coords["B"].Min.X, coords["C"].Min.X
	if math.Abs(bx-ax) > 1 || math.Abs(bx-cx) > 1 {
		t.Errorf("single chain not vertically aligned: A.x=%v B.x=%v C.x=%v", ax, bx, cx)
	}
}

func TestCoord_NodeSize_RespectsLabel(t *testing.T) {
	short, long := elem("abc"), elem("abcdefghijkl") // 3 vs 12 chars
	layers := [][]*sirena.Element{{short}, {long}}

	coords := assignCoords(layers, nil, DefaultMetrics())
	if coords["abcdefghijkl"].Width() <= coords["abc"].Width() {
		t.Errorf("12-char node width %v should exceed 3-char width %v",
			coords["abcdefghijkl"].Width(), coords["abc"].Width())
	}
}

func TestCoord_Deterministic(t *testing.T) {
	a, b, c, d := elem("A"), elem("B"), elem("C"), elem("D")
	layers := [][]*sirena.Element{{a, b}, {c, d}}
	edges := []*sirena.Edge{fwd("A", "C"), fwd("B", "D"), fwd("A", "D")}

	c1 := assignCoords(layers, edges, DefaultMetrics())
	c2 := assignCoords(layers, edges, DefaultMetrics())
	for name, r := range c1 {
		if c2[name] != r {
			t.Errorf("coord nondeterministic for %s: %v vs %v", name, r, c2[name])
		}
	}
}
