package layout

import "m31labs.dev/sirena"

// init wires this package's Compute into the root sirena.Render
// entrypoint. The root package cannot import layout (cycle), so it
// exposes a registration hook and delegates through the function
// variable we set here. Linking layout into a build (a real or blank
// import) is what activates layout inside sirena.Render.
func init() {
	sirena.RegisterLayoutComputer(func(rv *sirena.ResolvedView) (*sirena.LayoutResult, error) {
		return Compute(rv, LayoutOptions{})
	})
}

// Default layout metrics. These are the spacing constants the cell
// pipeline and skeleton solver use; later phases route edges and pack
// boundaries against them. They live here (not in a config surface)
// because v0.1 ships one tuned set.
const (
	defaultNodeHeight = 40.0
	defaultNodeMinW   = 60.0
	defaultGlyphWidth = 8.0
	defaultLayerGap   = 24.0
)

// Compute is the public layout entrypoint. It turns a resolved view into
// a positioned *sirena.LayoutResult. The seed is taken from opts.Seed
// when set, otherwise derived from sirena.ViewHash(rv) so identical
// views lay out identically across runs.
//
// This is the Phase-A skeleton: it stacks every element vertically and
// gives boundaries the union bounding box. Phases B–E replace the body
// with the real ranking, ordering, coordinate, routing, skeleton, and
// force-directed algorithms; the signature and seeding contract are
// already final.
func Compute(rv *sirena.ResolvedView, opts LayoutOptions) (*sirena.LayoutResult, error) {
	var seed [32]byte
	if opts.Seed != nil {
		seed = *opts.Seed
	} else {
		seed = sirena.ViewHash(rv)
	}

	lr := &sirena.LayoutResult{View: rv, Seed: seed}
	if rv == nil {
		return lr, nil
	}

	// Stub placement: stack elements vertically, left-aligned at x=0.
	y := 0.0
	for _, e := range rv.Elements {
		w := nodeWidth(e.Name)
		np := &sirena.NodePlacement{
			Node: e,
			Bounds: sirena.Rect{
				Min: sirena.Point{X: 0, Y: y},
				Max: sirena.Point{X: w, Y: y + defaultNodeHeight},
			},
		}
		lr.NodePlacements = append(lr.NodePlacements, np)
		y += defaultNodeHeight + defaultLayerGap
	}

	bounds := unionPlacements(lr.NodePlacements)
	lr.Bounds = bounds

	// Stub: every boundary takes the overall union box. Phase C/D give
	// each boundary its own packed region.
	for _, b := range rv.Boundaries {
		lr.BoundaryPlacements = append(lr.BoundaryPlacements, &sirena.BoundaryPlacement{
			Boundary:       b,
			Bounds:         bounds,
			ChildrenBounds: bounds,
		})
	}

	return lr, nil
}

// nodeWidth is the placeholder text-measurement function: it estimates a
// node's width from its label length. Phase F replaces it with the
// bundled font's real advance widths.
func nodeWidth(label string) float64 {
	w := defaultGlyphWidth * float64(len([]rune(label)))
	if w < defaultNodeMinW {
		return defaultNodeMinW
	}
	return w
}

// unionPlacements returns the bounding box enclosing every node
// placement. An empty input yields the zero rectangle.
func unionPlacements(nps []*sirena.NodePlacement) sirena.Rect {
	if len(nps) == 0 {
		return sirena.Rect{}
	}
	out := nps[0].Bounds
	for _, np := range nps[1:] {
		out = unionRect(out, np.Bounds)
	}
	return out
}

// unionRect returns the smallest rectangle containing both a and b.
func unionRect(a, b sirena.Rect) sirena.Rect {
	return sirena.Rect{
		Min: sirena.Point{X: min(a.Min.X, b.Min.X), Y: min(a.Min.Y, b.Min.Y)},
		Max: sirena.Point{X: max(a.Max.X, b.Max.X), Y: max(a.Max.Y, b.Max.Y)},
	}
}
