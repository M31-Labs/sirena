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
	defaultNodeHeight  = 40.0
	defaultNodeMinW    = 60.0
	defaultGlyphWidth  = 8.0
	defaultLayerGap    = 24.0
	defaultNodeSpacing = 48.0

	// boundaryPadding is the inset between a boundary's content and its
	// outer rectangle.
	boundaryPadding = 16.0
	// intraGap is the vertical gap between a boundary's own cell and its
	// nested child boundaries.
	intraGap = 24.0
	// skeletonPadding is the gap between top-level regions (loose cell
	// and each top-level boundary). Task 11 replaces the naive stacking
	// that uses it with the constraint-solved skeleton.
	skeletonPadding = 24.0
)

// Compute is the public layout entrypoint. It turns a resolved view into
// a positioned *sirena.LayoutResult. The seed is taken from opts.Seed
// when set, otherwise derived from sirena.ViewHash(rv) so identical
// views lay out identically across runs.
//
// The layered preset reconstructs the boundary containment tree from the
// view's flat element/boundary/summary lists, lays out each top-level
// boundary's subtree recursively (layoutCell per scope), lays out the
// loose top-level elements and summaries as one cell, then arranges the
// top-level regions. v0.1 stacks the top-level regions vertically; Task
// 11 replaces that with the constraint-solved skeleton honoring the
// view's layout hints. Edge routing (Phases C) and the force preset
// (Phase E) graft into this entrypoint in later tasks.
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

	metrics := DefaultMetrics()

	// Reconstruct containment: which elements / boundaries are children
	// of an included boundary.
	included := make(map[*sirena.Boundary]bool, len(rv.Boundaries))
	for _, b := range rv.Boundaries {
		included[b] = true
	}
	childElem := map[*sirena.Element]bool{}
	childBound := map[*sirena.Boundary]bool{}
	for _, b := range rv.Boundaries {
		for _, c := range b.Children {
			switch n := c.(type) {
			case *sirena.Element:
				childElem[n] = true
			case *sirena.Boundary:
				if included[n] {
					childBound[n] = true
				}
			}
		}
	}

	// Loose top-level elements + every summary form one cell.
	var items []cellItem
	for _, e := range rv.Elements {
		if !childElem[e] {
			items = append(items, cellItem{element: e})
		}
	}
	for _, s := range rv.Summaries {
		items = append(items, cellItem{summary: s})
	}
	nps, sps, cellBounds := layoutCell(items, rv.Edges, metrics)

	var bps []*sirena.BoundaryPlacement
	overall := cellBounds
	overallHas := len(items) > 0
	yCursor := 0.0
	if overallHas {
		yCursor = cellBounds.Max.Y + skeletonPadding
	}

	// Each top-level boundary becomes a stacked region.
	for _, b := range rv.Boundaries {
		if childBound[b] {
			continue
		}
		bp, bnps, bsps := layoutBoundary(b, rv, included, metrics)
		shiftBoundaryTree(bp, 0, yCursor)
		shiftPlacements(bnps, bsps, 0, yCursor)
		bps = append(bps, bp)
		nps = append(nps, bnps...)
		sps = append(sps, bsps...)
		if overallHas {
			overall = unionRect(overall, bp.Bounds)
		} else {
			overall = bp.Bounds
			overallHas = true
		}
		yCursor = bp.Bounds.Max.Y + skeletonPadding
	}

	lr.NodePlacements = nps
	lr.SummaryPlacements = sps
	lr.BoundaryPlacements = bps
	if overallHas {
		lr.Bounds = overall
	}

	// Route edges between placed nodes. v0.1 routes node-to-node edges;
	// edges whose endpoint is a collapsed-boundary summary have no port
	// and are skipped.
	ports := assignPorts(lr.NodePlacements, rv.Edges)
	lr.EdgeRoutes = routeEdges(lr.NodePlacements, ports, rv.Edges)
	placeLabels(lr.EdgeRoutes, lr.NodePlacements, metrics)

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

// unionRect returns the smallest rectangle containing both a and b.
func unionRect(a, b sirena.Rect) sirena.Rect {
	return sirena.Rect{
		Min: sirena.Point{X: min(a.Min.X, b.Min.X), Y: min(a.Min.Y, b.Min.Y)},
		Max: sirena.Point{X: max(a.Max.X, b.Max.X), Y: max(a.Max.Y, b.Max.Y)},
	}
}
