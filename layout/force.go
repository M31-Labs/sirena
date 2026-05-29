package layout

import (
	"math"
	"math/rand/v2"
	"sort"

	"m31labs.dev/sirena"
)

// computeForce builds a LayoutResult using the force-directed preset. It
// flattens every element and summary (ignoring containment), positions
// them with forceDirectedLayout, then gives each boundary a bounding box
// that wraps its contained elements' force positions. Edges are routed
// the same way as the layered preset.
func computeForce(rv *sirena.ResolvedView, seed [32]byte, metrics Metrics) *sirena.LayoutResult {
	lr := &sirena.LayoutResult{View: rv, Seed: seed}

	var els []*sirena.Element
	labelByName := map[string]string{}
	for _, e := range rv.Elements {
		els = append(els, e)
		labelByName[e.Name] = e.Name
	}
	for _, s := range rv.Summaries {
		if s.Boundary == nil {
			continue
		}
		els = append(els, &sirena.Element{Name: s.Boundary.Name})
		labelByName[s.Boundary.Name] = s.Label
	}

	m := metrics
	base := m.WidthOf
	m.WidthOf = func(nameKey string) float64 {
		lbl := labelByName[nameKey]
		if lbl == "" {
			lbl = nameKey
		}
		if base != nil {
			return base(lbl)
		}
		return nodeWidth(lbl)
	}

	coords := forceDirectedLayout(els, rv.Edges, seed, m)

	for _, e := range rv.Elements {
		lr.NodePlacements = append(lr.NodePlacements, &sirena.NodePlacement{Node: e, Bounds: coords[e.Name]})
	}
	for _, s := range rv.Summaries {
		if s.Boundary == nil {
			continue
		}
		lr.SummaryPlacements = append(lr.SummaryPlacements, &sirena.SummaryPlacement{Summary: s, Bounds: coords[s.Boundary.Name]})
	}
	for _, b := range rv.Boundaries {
		if bb, ok := boundaryUnion(b, coords); ok {
			lr.BoundaryPlacements = append(lr.BoundaryPlacements, &sirena.BoundaryPlacement{
				Boundary:       b,
				Bounds:         bb,
				ChildrenBounds: bb,
			})
		}
	}

	if bounds, ok := unionAll(lr.NodePlacements, lr.SummaryPlacements, lr.BoundaryPlacements); ok {
		lr.Bounds = bounds
	}

	ports := assignPorts(lr.NodePlacements, rv.Edges)
	lr.EdgeRoutes = routeEdges(lr.NodePlacements, ports, rv.Edges)
	placeLabels(lr.EdgeRoutes, lr.NodePlacements, m)
	return lr
}

// boundaryUnion returns the bounding box of every element (recursively)
// inside a boundary, using force-assigned coordinates.
func boundaryUnion(b *sirena.Boundary, coords map[string]sirena.Rect) (sirena.Rect, bool) {
	var out sirena.Rect
	has := false
	var walk func(*sirena.Boundary)
	walk = func(bb *sirena.Boundary) {
		for _, c := range bb.Children {
			switch n := c.(type) {
			case *sirena.Element:
				if r, ok := coords[n.Name]; ok {
					if !has {
						out, has = r, true
					} else {
						out = unionRect(out, r)
					}
				}
			case *sirena.Boundary:
				walk(n)
			}
		}
	}
	walk(b)
	return out, has
}

// forceDirectedLayout positions elements with a deterministic
// Fruchterman-Reingold relaxation: repulsion between every node pair,
// attraction along every in-scope edge, and a linearly cooling
// temperature that caps per-step displacement. The RNG is seeded from
// the supplied seed (the view hash on the production path) via a ChaCha8
// stream, so the same seed reproduces the same layout byte-for-byte and
// a different seed produces a different one. The result maps element name
// to an absolute Rect, recentered so the top-left of the bounding box
// sits at the origin.
//
// This is the LayoutPresetForce fallback; it never runs by default.
func forceDirectedLayout(elements []*sirena.Element, edges []*sirena.Edge, seed [32]byte, metrics Metrics) map[string]sirena.Rect {
	n := len(elements)
	if n == 0 {
		return map[string]sirena.Rect{}
	}

	names := make([]string, 0, n)
	width := make(map[string]float64, n)
	seen := map[string]bool{}
	for _, e := range elements {
		if seen[e.Name] {
			continue
		}
		seen[e.Name] = true
		names = append(names, e.Name)
		width[e.Name] = metrics.NodeWidth(e.Name)
	}
	sort.Strings(names)
	n = len(names)

	rng := rand.New(rand.NewChaCha8(seed))
	frame := 100 * math.Sqrt(float64(n))
	k := frame / math.Sqrt(float64(n)) // == sqrt(area/N) with area = frame²

	pos := make(map[string]sirena.Point, n)
	for _, nm := range names {
		pos[nm] = sirena.Point{X: rng.Float64() * frame, Y: rng.Float64() * frame}
	}

	inScope := make(map[string]bool, n)
	for _, nm := range names {
		inScope[nm] = true
	}
	var pairs []edgePair
	for _, e := range edges {
		s, d := edgeEndpoints(e)
		if s != d && inScope[s] && inScope[d] {
			pairs = append(pairs, edgePair{s, d})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].src != pairs[j].src {
			return pairs[i].src < pairs[j].src
		}
		return pairs[i].dst < pairs[j].dst
	})

	const maxSteps = 100
	t0 := frame * 0.1
	disp := make(map[string]sirena.Point, n)
	for step := 0; step < maxSteps; step++ {
		temp := t0 * (1 - float64(step)/float64(maxSteps))
		for _, nm := range names {
			disp[nm] = sirena.Point{}
		}

		// Repulsion between every pair (sorted index order for stability).
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				a, b := names[i], names[j]
				dx, dy := pos[a].X-pos[b].X, pos[a].Y-pos[b].Y
				d := math.Hypot(dx, dy)
				if d < 1e-9 {
					dx, dy, d = 0.01, 0.01, math.Hypot(0.01, 0.01)
				}
				f := k * k / d
				ux, uy := dx/d, dy/d
				da, db := disp[a], disp[b]
				da.X, da.Y = da.X+ux*f, da.Y+uy*f
				db.X, db.Y = db.X-ux*f, db.Y-uy*f
				disp[a], disp[b] = da, db
			}
		}

		// Attraction along edges.
		for _, p := range pairs {
			dx, dy := pos[p.src].X-pos[p.dst].X, pos[p.src].Y-pos[p.dst].Y
			d := math.Hypot(dx, dy)
			if d < 1e-9 {
				continue
			}
			f := d * d / k
			ux, uy := dx/d, dy/d
			da, db := disp[p.src], disp[p.dst]
			da.X, da.Y = da.X-ux*f, da.Y-uy*f
			db.X, db.Y = db.X+ux*f, db.Y+uy*f
			disp[p.src], disp[p.dst] = da, db
		}

		// Apply displacement capped by the current temperature.
		for _, nm := range names {
			dp := disp[nm]
			dl := math.Hypot(dp.X, dp.Y)
			if dl < 1e-9 {
				continue
			}
			capped := math.Min(dl, temp)
			pt := pos[nm]
			pt.X += dp.X / dl * capped
			pt.Y += dp.Y / dl * capped
			pos[nm] = pt
		}
	}

	h := metrics.NodeHeight()
	minX, minY := math.Inf(1), math.Inf(1)
	for _, nm := range names {
		minX = min(minX, pos[nm].X-width[nm]/2)
		minY = min(minY, pos[nm].Y-h/2)
	}

	out := make(map[string]sirena.Rect, n)
	for _, nm := range names {
		cx, cy := pos[nm].X-minX, pos[nm].Y-minY
		out[nm] = sirena.Rect{
			Min: sirena.Point{X: cx - width[nm]/2, Y: cy - h/2},
			Max: sirena.Point{X: cx + width[nm]/2, Y: cy + h/2},
		}
	}
	return out
}
