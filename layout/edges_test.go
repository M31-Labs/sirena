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
