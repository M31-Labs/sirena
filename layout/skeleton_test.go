package layout

import (
	"testing"

	"m31labs.dev/sirena"
)

func region(name string, w, h float64) *sirena.BoundaryPlacement {
	r := sirena.Rect{Min: sirena.Point{X: 0, Y: 0}, Max: sirena.Point{X: w, Y: h}}
	return &sirena.BoundaryPlacement{
		Boundary:       &sirena.Boundary{Name: name},
		Bounds:         r,
		ChildrenBounds: r,
	}
}

func TestSkeleton_TwoBoundariesTopDown(t *testing.T) {
	auth := region("auth", 100, 50)
	pay := region("payments", 120, 60)
	solveSkeleton([]*sirena.BoundaryPlacement{auth, pay}, &sirena.LayoutHints{Direction: "top-down"}, DefaultMetrics())

	if auth.Bounds.Max.Y+skeletonPadding > pay.Bounds.Min.Y {
		t.Errorf("payments should sit below auth + padding: auth.Max.Y=%v pay.Min.Y=%v",
			auth.Bounds.Max.Y, pay.Bounds.Min.Y)
	}
}

func TestSkeleton_PinTopWins(t *testing.T) {
	a := region("auth", 100, 50)
	g := region("gateway", 100, 50)
	p := region("payments", 100, 50)
	// payments declared last, but pinned to top.
	solveSkeleton([]*sirena.BoundaryPlacement{a, g, p},
		&sirena.LayoutHints{Direction: "top-down", Pin: map[string]string{"payments": "top"}},
		DefaultMetrics())

	if p.Bounds.Min.Y != 0 {
		t.Errorf("pinned-top payments Min.Y = %v, want 0", p.Bounds.Min.Y)
	}
}

func TestSkeleton_LeftRight(t *testing.T) {
	r0 := region("a", 100, 50)
	r1 := region("b", 100, 50)
	r2 := region("c", 100, 50)
	solveSkeleton([]*sirena.BoundaryPlacement{r0, r1, r2}, &sirena.LayoutHints{Direction: "left-right"}, DefaultMetrics())

	if !(r0.Bounds.Max.X <= r1.Bounds.Min.X && r1.Bounds.Max.X <= r2.Bounds.Min.X) {
		t.Errorf("left-right not x-monotone: %v %v %v", r0.Bounds, r1.Bounds, r2.Bounds)
	}
}

func TestSkeleton_NoHints_Stacks(t *testing.T) {
	r0 := region("a", 100, 50)
	r1 := region("b", 100, 50)
	solveSkeleton([]*sirena.BoundaryPlacement{r0, r1}, nil, DefaultMetrics())

	if r0.Bounds.Min.Y != 0 {
		t.Errorf("first region Min.Y = %v, want 0", r0.Bounds.Min.Y)
	}
	if r1.Bounds.Min.Y < r0.Bounds.Max.Y {
		t.Errorf("second region should stack below first: r0.Max.Y=%v r1.Min.Y=%v",
			r0.Bounds.Max.Y, r1.Bounds.Min.Y)
	}
}

func TestSkeleton_Deterministic(t *testing.T) {
	mk := func() []*sirena.BoundaryPlacement {
		return []*sirena.BoundaryPlacement{region("a", 100, 50), region("b", 80, 70), region("c", 60, 40)}
	}
	hints := &sirena.LayoutHints{Direction: "top-down"}
	r1 := mk()
	r2 := mk()
	solveSkeleton(r1, hints, DefaultMetrics())
	solveSkeleton(r2, hints, DefaultMetrics())
	for i := range r1 {
		if r1[i].Bounds != r2[i].Bounds {
			t.Errorf("region %d nondeterministic: %v vs %v", i, r1[i].Bounds, r2[i].Bounds)
		}
	}
}
