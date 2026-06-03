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

// Render renders a resolved view to a positioned layout with the layout
// engine guaranteed to be linked. It is the convenience entrypoint for
// library consumers: because calling it requires importing this package, the
// init above has already registered the engine into sirena.Render, so
// Render never returns the engine-not-linked nil *LayoutResult that bare
// sirena.Render yields when the layout package is absent from the build.
//
// The options and error contract are identical to sirena.Render (see its
// doc comment) — Render simply forwards. Reach for sirena.Render directly
// only on the budget-only path where no geometry is needed.
func Render(rv *sirena.ResolvedView, opts sirena.RenderOptions) (*sirena.LayoutResult, *sirena.BudgetReport, error) {
	return sirena.Render(rv, opts)
}

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

	// The force preset skips the cell/skeleton pipeline entirely.
	if opts.Preset == LayoutPresetForce {
		return computeForce(rv, seed, metrics), nil
	}

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
	looseNps, looseSps, looseBounds := layoutCell(items, rv.Edges, metrics)

	// Each top-level region — the implicit loose region (nil boundary)
	// plus every top-level boundary — is laid out at its own local origin,
	// then arranged by the skeleton solver. Node/summary placements live
	// outside the BoundaryPlacement tree, so we track them per region and
	// translate them by the same delta the solver applies.
	type regionContent struct {
		region  *sirena.BoundaryPlacement
		nps     []*sirena.NodePlacement
		sps     []*sirena.SummaryPlacement
		isLoose bool
	}
	var contents []regionContent
	if len(items) > 0 {
		contents = append(contents, regionContent{
			region:  &sirena.BoundaryPlacement{Bounds: looseBounds, ChildrenBounds: looseBounds},
			nps:     looseNps,
			sps:     looseSps,
			isLoose: true,
		})
	}
	for _, b := range rv.Boundaries {
		if childBound[b] {
			continue
		}
		bp, bnps, bsps := layoutBoundary(b, rv, included, metrics)
		contents = append(contents, regionContent{region: bp, nps: bnps, sps: bsps})
	}

	if len(contents) == 0 {
		return lr, nil
	}

	regions := make([]*sirena.BoundaryPlacement, len(contents))
	preMin := make([]sirena.Point, len(contents))
	for i, c := range contents {
		regions[i] = c.region
		preMin[i] = c.region.Bounds.Min
	}

	var hints *sirena.LayoutHints
	if rv.Source != nil {
		hints = rv.Source.Layout
	}
	canvas := solveSkeleton(regions, hints, metrics)

	var (
		allNps []*sirena.NodePlacement
		allSps []*sirena.SummaryPlacement
		bps    []*sirena.BoundaryPlacement
	)
	for i, c := range contents {
		dx := c.region.Bounds.Min.X - preMin[i].X
		dy := c.region.Bounds.Min.Y - preMin[i].Y
		shiftPlacements(c.nps, c.sps, dx, dy)
		allNps = append(allNps, c.nps...)
		allSps = append(allSps, c.sps...)
		if !c.isLoose {
			bps = append(bps, c.region)
		}
	}

	lr.NodePlacements = allNps
	lr.SummaryPlacements = allSps
	lr.BoundaryPlacements = bps
	lr.Bounds = canvas

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
