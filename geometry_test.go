package sirena

import "testing"

func TestPoint_Zero(t *testing.T) {
	var p Point
	if p.X != 0 || p.Y != 0 {
		t.Fatalf("zero Point = %+v, want {0 0}", p)
	}
}

func TestRect_ContainsPoint(t *testing.T) {
	// 100×40 rect anchored at (10, 10): [10,110) × [10,50).
	r := Rect{Min: Point{X: 10, Y: 10}, Max: Point{X: 110, Y: 50}}
	if !r.Contains(Point{X: 50, Y: 30}) {
		t.Errorf("Contains(50,30) = false, want true")
	}
	if r.Contains(Point{X: 200, Y: 200}) {
		t.Errorf("Contains(200,200) = true, want false")
	}
	// Half-open: the Max corner is excluded.
	if r.Contains(r.Max) {
		t.Errorf("Contains(Max) = true, want false (half-open [Min,Max))")
	}
}

func TestRect_Intersects(t *testing.T) {
	a := Rect{Min: Point{X: 0, Y: 0}, Max: Point{X: 100, Y: 100}}
	overlap := Rect{Min: Point{X: 50, Y: 50}, Max: Point{X: 150, Y: 150}}
	disjoint := Rect{Min: Point{X: 200, Y: 200}, Max: Point{X: 300, Y: 300}}
	edgeTouch := Rect{Min: Point{X: 100, Y: 0}, Max: Point{X: 200, Y: 100}}

	if !a.Intersects(overlap) {
		t.Errorf("a.Intersects(overlap) = false, want true")
	}
	if a.Intersects(disjoint) {
		t.Errorf("a.Intersects(disjoint) = true, want false")
	}
	// Half-open semantics: rectangles that merely share an edge do not overlap.
	if a.Intersects(edgeTouch) {
		t.Errorf("a.Intersects(edgeTouch) = true, want false (half-open)")
	}
}

func TestEdgeRoute_OrthogonalOnly(t *testing.T) {
	ortho := EdgeRoute{Points: []Point{{X: 0, Y: 0}, {X: 0, Y: 10}, {X: 10, Y: 10}}}
	if !ortho.IsOrthogonal() {
		t.Errorf("orthogonal route reported non-orthogonal")
	}
	diag := EdgeRoute{Points: []Point{{X: 0, Y: 0}, {X: 10, Y: 10}}}
	if diag.IsOrthogonal() {
		t.Errorf("route with a diagonal segment reported orthogonal")
	}
}

func TestNodePlacement_PortsSorted(t *testing.T) {
	np := &NodePlacement{Ports: []Port{
		{Side: PortSideLeft, Offset: 0.5},
		{Side: PortSideTop, Offset: 0.9},
		{Side: PortSideTop, Offset: 0.1},
		{Side: PortSideBottom, Offset: 0.5},
		{Side: PortSideRight, Offset: 0.5},
	}}
	np.SortPorts()

	wantSides := []PortSide{PortSideTop, PortSideTop, PortSideRight, PortSideBottom, PortSideLeft}
	for i, p := range np.Ports {
		if p.Side != wantSides[i] {
			t.Fatalf("port %d side = %v, want %v (got order %+v)", i, p.Side, wantSides[i], np.Ports)
		}
	}
	// Stable within a side: the two Top ports stay ordered by ascending offset.
	if np.Ports[0].Offset != 0.1 || np.Ports[1].Offset != 0.9 {
		t.Errorf("Top ports not ordered by offset: got %v then %v", np.Ports[0].Offset, np.Ports[1].Offset)
	}
}
