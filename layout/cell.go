package layout

import "m31labs.dev/sirena"

// cellItem is one node in a cell scope: either an element or a collapsed-
// boundary summary. Both lay out as uniform-height nodes; a summary's
// width derives from its label.
type cellItem struct {
	element *sirena.Element
	summary *sirena.BoundarySummary
}

func (c cellItem) name() string {
	if c.element != nil {
		return c.element.Name
	}
	if c.summary != nil && c.summary.Boundary != nil {
		return c.summary.Boundary.Name
	}
	return ""
}

func (c cellItem) label() string {
	if c.element != nil {
		return c.element.Name
	}
	if c.summary != nil {
		return c.summary.Label
	}
	return ""
}

// layoutCell lays out one boundary scope. It builds a synthetic element
// set (summaries get a placeholder element carrying their name), runs
// rank → order → coord, then maps the resulting coordinates back onto
// node and summary placements. Edges whose endpoints are both in scope
// drive the ranking; out-of-scope edges are ignored by rank/order/coord.
// The third result is the bounding box enclosing every placement.
func layoutCell(items []cellItem, edges []*sirena.Edge, metrics Metrics) ([]*sirena.NodePlacement, []*sirena.SummaryPlacement, sirena.Rect) {
	if len(items) == 0 {
		return nil, nil, sirena.Rect{}
	}

	els := make([]*sirena.Element, 0, len(items))
	byName := make(map[string]*sirena.Element, len(items))
	labelByName := make(map[string]string, len(items))
	for _, it := range items {
		nm := it.name()
		var e *sirena.Element
		if it.element != nil {
			e = it.element
		} else {
			e = &sirena.Element{Name: nm} // placeholder for the summary
		}
		els = append(els, e)
		byName[nm] = e
		labelByName[nm] = it.label()
	}

	ranks, _ := rank(els, edges)
	layers := order(ranks, edges, byName)

	// Width resolves through the item's label (a summary's label can
	// differ from the boundary name we key on).
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

	coords := assignCoords(layers, edges, m)

	var (
		nps    []*sirena.NodePlacement
		sps    []*sirena.SummaryPlacement
		bounds sirena.Rect
		has    bool
	)
	for _, it := range items {
		r := coords[it.name()]
		if !has {
			bounds = r
			has = true
		} else {
			bounds = unionRect(bounds, r)
		}
		if it.element != nil {
			nps = append(nps, &sirena.NodePlacement{Node: it.element, Bounds: r})
		} else {
			sps = append(sps, &sirena.SummaryPlacement{Summary: it.summary, Bounds: r})
		}
	}
	return nps, sps, bounds
}

// layoutBoundary lays out a boundary's subtree in a local coordinate
// frame: its direct child elements via layoutCell, then each included
// nested boundary recursively, stacked below. It returns the boundary
// placement (outer + children bounds) plus the node and summary
// placements for everything inside, all in local coordinates the caller
// translates into place. The returned placements do NOT include the
// boundary rectangle itself — that is the BoundaryPlacement.
func layoutBoundary(b *sirena.Boundary, rv *sirena.ResolvedView, included map[*sirena.Boundary]bool, metrics Metrics) (*sirena.BoundaryPlacement, []*sirena.NodePlacement, []*sirena.SummaryPlacement) {
	var items []cellItem
	for _, c := range b.Children {
		if e, ok := c.(*sirena.Element); ok {
			items = append(items, cellItem{element: e})
		}
	}
	nps, sps, cellBounds := layoutCell(items, rv.Edges, metrics)

	var childBPs []*sirena.BoundaryPlacement
	yCursor := 0.0
	if len(items) > 0 {
		yCursor = cellBounds.Max.Y + intraGap
	}
	for _, c := range b.Children {
		nb, ok := c.(*sirena.Boundary)
		if !ok || !included[nb] {
			continue
		}
		cbp, cnps, csps := layoutBoundary(nb, rv, included, metrics)
		shiftBoundaryTree(cbp, 0, yCursor)
		shiftPlacements(cnps, csps, 0, yCursor)
		childBPs = append(childBPs, cbp)
		nps = append(nps, cnps...)
		sps = append(sps, csps...)
		yCursor = cbp.Bounds.Max.Y + intraGap
	}

	childrenBounds, has := unionAll(nps, sps, childBPs)
	if !has {
		childrenBounds = sirena.Rect{}
	}

	// Inset the content by padding so it sits inside the boundary box.
	pad := boundaryPadding
	shiftPlacements(nps, sps, pad, pad)
	for _, c := range childBPs {
		shiftBoundaryTree(c, pad, pad)
	}
	childrenBounds = shiftRect(childrenBounds, pad, pad)

	outer := sirena.Rect{
		Min: sirena.Point{X: 0, Y: 0},
		Max: sirena.Point{X: childrenBounds.Max.X + pad, Y: childrenBounds.Max.Y + pad},
	}
	bp := &sirena.BoundaryPlacement{
		Boundary:       b,
		Bounds:         outer,
		ChildrenBounds: childrenBounds,
		Children:       childBPs,
	}
	return bp, nps, sps
}

// unionAll returns the bounding box of every node, summary, and boundary
// placement, plus whether any were present.
func unionAll(nps []*sirena.NodePlacement, sps []*sirena.SummaryPlacement, bps []*sirena.BoundaryPlacement) (sirena.Rect, bool) {
	var out sirena.Rect
	has := false
	add := func(r sirena.Rect) {
		if !has {
			out = r
			has = true
			return
		}
		out = unionRect(out, r)
	}
	for _, n := range nps {
		add(n.Bounds)
	}
	for _, s := range sps {
		add(s.Bounds)
	}
	for _, b := range bps {
		add(b.Bounds)
	}
	return out, has
}

// shiftRect translates a rectangle by (dx, dy).
func shiftRect(r sirena.Rect, dx, dy float64) sirena.Rect {
	return sirena.Rect{
		Min: sirena.Point{X: r.Min.X + dx, Y: r.Min.Y + dy},
		Max: sirena.Point{X: r.Max.X + dx, Y: r.Max.Y + dy},
	}
}

// shiftBoundaryTree translates a boundary placement and all nested
// boundary placements by (dx, dy).
func shiftBoundaryTree(bp *sirena.BoundaryPlacement, dx, dy float64) {
	bp.Bounds = shiftRect(bp.Bounds, dx, dy)
	bp.ChildrenBounds = shiftRect(bp.ChildrenBounds, dx, dy)
	for _, c := range bp.Children {
		shiftBoundaryTree(c, dx, dy)
	}
}

// shiftPlacements translates node and summary placements by (dx, dy).
func shiftPlacements(nps []*sirena.NodePlacement, sps []*sirena.SummaryPlacement, dx, dy float64) {
	for _, np := range nps {
		np.Bounds = shiftRect(np.Bounds, dx, dy)
	}
	for _, sp := range sps {
		sp.Bounds = shiftRect(sp.Bounds, dx, dy)
	}
}
