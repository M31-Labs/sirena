package layout

import (
	"testing"

	"m31labs.dev/sirena"
)

func np(name string, minX, minY, maxX, maxY float64) *sirena.NodePlacement {
	return &sirena.NodePlacement{
		Node:   elem(name),
		Bounds: sirena.Rect{Min: sirena.Point{X: minX, Y: minY}, Max: sirena.Point{X: maxX, Y: maxY}},
	}
}

func TestPort_PickSideByDirection(t *testing.T) {
	src := np("src", 0, 0, 80, 40)
	tgt := np("tgt", 200, 100, 280, 140)
	e := fwd("src", "tgt")

	ports := assignPorts([]*sirena.NodePlacement{src, tgt}, []*sirena.Edge{e})
	p, ok := ports[e]
	if !ok {
		t.Fatal("no ports assigned for edge")
	}
	if p.Source.Side != sirena.PortSideRight {
		t.Errorf("source side = %v, want right", p.Source.Side)
	}
	if p.Target.Side != sirena.PortSideLeft {
		t.Errorf("target side = %v, want left", p.Target.Side)
	}
}

func TestPort_StackedTargets_DistinctOffsets(t *testing.T) {
	// Three sources to the left of one tall target; all incoming on Left.
	s1 := np("s1", 0, 0, 80, 40)
	s2 := np("s2", 0, 80, 80, 120)
	s3 := np("s3", 0, 160, 80, 200)
	tgt := np("tgt", 200, 0, 280, 200)
	edges := []*sirena.Edge{fwd("s1", "tgt"), fwd("s2", "tgt"), fwd("s3", "tgt")}

	ports := assignPorts([]*sirena.NodePlacement{s1, s2, s3, tgt}, edges)
	got := map[float64]bool{}
	for _, e := range edges {
		p := ports[e]
		if p.Target.Side != sirena.PortSideLeft {
			t.Errorf("edge to tgt: target side = %v, want left", p.Target.Side)
		}
		got[p.Target.Offset] = true
	}
	for _, want := range []float64{0.25, 0.5, 0.75} {
		if !got[want] {
			t.Errorf("missing target offset %v; got offsets %v", want, got)
		}
	}
	if len(got) != 3 {
		t.Errorf("expected 3 distinct target offsets, got %d: %v", len(got), got)
	}
}

func TestPort_Deterministic(t *testing.T) {
	s1 := np("s1", 0, 0, 80, 40)
	s2 := np("s2", 0, 80, 80, 120)
	tgt := np("tgt", 200, 0, 280, 200)
	edges := []*sirena.Edge{fwd("s1", "tgt"), fwd("s2", "tgt")}
	pl := []*sirena.NodePlacement{s1, s2, tgt}

	a := assignPorts(pl, edges)
	b := assignPorts(pl, edges)
	for _, e := range edges {
		if a[e] != b[e] {
			t.Errorf("ports not deterministic for edge %s->%s: %+v vs %+v", e.From, e.To, a[e], b[e])
		}
	}
}

func routeOne(t *testing.T, pl []*sirena.NodePlacement, e *sirena.Edge) *sirena.EdgeRoute {
	t.Helper()
	ports := assignPorts(pl, []*sirena.Edge{e})
	routes := routeEdges(pl, ports, []*sirena.Edge{e})
	if len(routes) != 1 {
		t.Fatalf("got %d routes, want 1", len(routes))
	}
	return routes[0]
}

func TestRoute_StraightSameLayer(t *testing.T) {
	a := np("A", 0, 0, 80, 40)
	b := np("B", 160, 0, 240, 40)
	pl := []*sirena.NodePlacement{a, b}

	r := routeOne(t, pl, fwd("A", "B"))
	if len(r.Points) != 2 {
		t.Errorf("aligned same-row edge should be a 2-point line; got %d points: %v", len(r.Points), r.Points)
	}
	if !r.IsOrthogonal() {
		t.Errorf("route not orthogonal: %v", r.Points)
	}
}

func TestRoute_ZShapedInterLayer(t *testing.T) {
	a := np("A", 0, 0, 80, 40)
	mid := np("B", 0, 60, 80, 100)
	c := np("C", 0, 120, 80, 160)
	pl := []*sirena.NodePlacement{a, mid, c}

	r := routeOne(t, pl, fwd("A", "C"))
	if len(r.Points) < 4 {
		t.Errorf("A→C around B should bend (≥4 points); got %d: %v", len(r.Points), r.Points)
	}
	if !r.IsOrthogonal() {
		t.Errorf("route not orthogonal: %v", r.Points)
	}
}

func TestRoute_AvoidsNodeRectangles(t *testing.T) {
	a := np("A", 0, 0, 80, 40)
	mid := np("B", 120, 0, 200, 40)
	c := np("C", 240, 0, 320, 40)
	pl := []*sirena.NodePlacement{a, mid, c}

	r := routeOne(t, pl, fwd("A", "C"))
	for i := 1; i < len(r.Points); i++ {
		if segCrossesRect(r.Points[i-1], r.Points[i], mid.Bounds) {
			t.Errorf("segment %v→%v passes through B %v", r.Points[i-1], r.Points[i], mid.Bounds)
		}
	}
	if !r.IsOrthogonal() {
		t.Errorf("route not orthogonal: %v", r.Points)
	}
}

func TestRoute_Deterministic(t *testing.T) {
	a := np("A", 0, 0, 80, 40)
	mid := np("B", 120, 0, 200, 40)
	c := np("C", 240, 0, 320, 40)
	pl := []*sirena.NodePlacement{a, mid, c}
	e := fwd("A", "C")
	ports := assignPorts(pl, []*sirena.Edge{e})

	r1 := routeEdges(pl, ports, []*sirena.Edge{e})
	r2 := routeEdges(pl, ports, []*sirena.Edge{e})
	if len(r1[0].Points) != len(r2[0].Points) {
		t.Fatalf("route length differs across runs")
	}
	for i := range r1[0].Points {
		if r1[0].Points[i] != r2[0].Points[i] {
			t.Errorf("route point %d differs: %v vs %v", i, r1[0].Points[i], r2[0].Points[i])
		}
	}
}
