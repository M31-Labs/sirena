package layout

import (
	"fmt"
	"math"
	"sort"

	"m31labs.dev/sirena"
)

func actorIdentity(e *sirena.Element) string {
	if sid, ok := e.Metadata["sid"].(sirena.String); ok && sid.Value != "" {
		return sid.Value
	}
	return e.Name
}

func stabilizeLayout(lr, previous *sirena.LayoutResult) error {
	kind := sirena.DiagramName(lr.View)
	if lr.Diagram != "" {
		kind = lr.Diagram
	}
	switch kind {
	case "architecture", "state", "class", "er", "mindmap":
	default:
		return fmt.Errorf("sirena: stable edits are unsupported for %s", kind)
	}
	oldKind := previous.Diagram
	if oldKind == "" {
		oldKind = sirena.DiagramName(previous.View)
	}
	if oldKind != kind {
		return fmt.Errorf("sirena: previous layout family %s differs from %s", oldKind, kind)
	}
	type box struct {
		id     string
		bounds *sirena.Rect
	}
	boxes := []box{}
	old := map[string]sirena.Rect{}
	for _, p := range previous.NodePlacements {
		if p != nil && p.Node != nil {
			old["node:"+actorIdentity(p.Node)] = p.Bounds
		}
	}
	for _, p := range previous.SummaryPlacements {
		if p != nil && p.Summary != nil && p.Summary.Boundary != nil {
			old["summary:"+p.Summary.Boundary.Name] = p.Bounds
		}
	}
	for _, p := range lr.NodePlacements {
		if p != nil && p.Node != nil {
			boxes = append(boxes, box{"node:" + actorIdentity(p.Node), &p.Bounds})
		}
	}
	for _, p := range lr.SummaryPlacements {
		if p != nil && p.Summary != nil && p.Summary.Boundary != nil {
			boxes = append(boxes, box{"summary:" + p.Summary.Boundary.Name, &p.Bounds})
		}
	}
	sort.SliceStable(boxes, func(i, j int) bool {
		_, a := old[boxes[i].id]
		_, b := old[boxes[j].id]
		if a != b {
			return a
		}
		return boxes[i].id < boxes[j].id
	})
	occupied := []sirena.Rect{}
	seen := map[string]bool{}
	for _, b := range boxes {
		if seen[b.id] {
			return fmt.Errorf("sirena: duplicate stable identity %q", b.id)
		}
		seen[b.id] = true
		r := *b.bounds
		if oldRect, ok := old[b.id]; ok {
			center := oldRect.Center()
			r = sirena.Rect{Min: sirena.Point{X: center.X - r.Width()/2, Y: center.Y - r.Height()/2}, Max: sirena.Point{X: center.X + r.Width()/2, Y: center.Y + r.Height()/2}}
		}
		*b.bounds = nearestFreeBox(r, occupied)
		occupied = append(occupied, *b.bounds)
	}
	// Refit nested frames around their stable children, reserving title space.
	nodes := map[string]*sirena.NodePlacement{}
	for _, p := range lr.NodePlacements {
		nodes[p.Node.Name] = p
	}
	var refit func(*sirena.BoundaryPlacement)
	refit = func(bp *sirena.BoundaryPlacement) {
		var contents []sirena.Rect
		for _, child := range bp.Children {
			refit(child)
			contents = append(contents, child.Bounds)
		}
		if bp.Boundary != nil {
			for _, child := range bp.Boundary.Children {
				if e, ok := child.(*sirena.Element); ok {
					if p, ok := nodes[e.Name]; ok {
						contents = append(contents, p.Bounds)
					}
				}
			}
		}
		if len(contents) == 0 {
			return
		}
		bounds := contents[0]
		for _, r := range contents[1:] {
			bounds = unionRect(bounds, r)
		}
		bp.ChildrenBounds = bounds
		bp.Bounds = sirena.Rect{Min: sirena.Point{X: bounds.Min.X - boundaryPadding, Y: bounds.Min.Y - boundaryPadding - 24}, Max: sirena.Point{X: bounds.Max.X + boundaryPadding, Y: bounds.Max.Y + boundaryPadding}}
		if bp.Boundary != nil {
			bp.Bounds.Max.X = max(bp.Bounds.Max.X, bp.Bounds.Min.X+DefaultMetrics().TextWidth(bp.Boundary.DisplayLabel())+2*boundaryPadding)
		}
	}
	for _, bp := range lr.BoundaryPlacements {
		refit(bp)
	}
	var shiftFrame func(*sirena.BoundaryPlacement, float64, float64)
	shiftFrame = func(bp *sirena.BoundaryPlacement, dx, dy float64) {
		shiftBoundaryTree(bp, dx, dy)
		var shiftActors func(*sirena.BoundaryPlacement)
		shiftActors = func(b *sirena.BoundaryPlacement) {
			if b.Boundary != nil {
				for _, child := range b.Boundary.Children {
					if e, ok := child.(*sirena.Element); ok {
						if p := nodes[e.Name]; p != nil {
							p.Bounds = shiftRect(p.Bounds, dx, dy)
						}
					}
				}
			}
			for _, child := range b.Children {
				shiftActors(child)
			}
		}
		shiftActors(bp)
	}
	pack := func(frames []*sirena.BoundaryPlacement, direct []*sirena.NodePlacement) {
		reserved := []sirena.Rect{}
		for _, p := range direct {
			reserved = append(reserved, p.Bounds)
		}
		for _, bp := range frames {
			position := nearestFreeBox(bp.Bounds, reserved)
			shiftFrame(bp, position.Min.X-bp.Bounds.Min.X, position.Min.Y-bp.Bounds.Min.Y)
			reserved = append(reserved, bp.Bounds)
		}
	}
	contained := map[string]bool{}
	var settle func(*sirena.BoundaryPlacement)
	settle = func(bp *sirena.BoundaryPlacement) {
		for _, child := range bp.Children {
			settle(child)
		}
		var direct []*sirena.NodePlacement
		if bp.Boundary != nil {
			for _, child := range bp.Boundary.Children {
				if e, ok := child.(*sirena.Element); ok {
					if p := nodes[e.Name]; p != nil {
						direct = append(direct, p)
						contained[e.Name] = true
					}
				}
			}
		}
		pack(bp.Children, direct)
		refit(bp)
	}
	for _, bp := range lr.BoundaryPlacements {
		settle(bp)
	}
	var loose []*sirena.NodePlacement
	for _, p := range lr.NodePlacements {
		if !contained[p.Node.Name] {
			loose = append(loose, p)
		}
	}
	pack(lr.BoundaryPlacements, loose)
	connectDiagram(lr, DefaultMetrics())
	return nil
}

func nearestFreeBox(preferred sirena.Rect, occupied []sirena.Rect) sirena.Rect {
	clear := func(r sirena.Rect) bool {
		for _, o := range occupied {
			if r.Intersects(o) {
				return false
			}
		}
		return true
	}
	if clear(preferred) {
		return preferred
	}
	candidates := []sirena.Rect{}
	for _, o := range occupied {
		for _, p := range []sirena.Point{
			{X: o.Max.X + 16, Y: preferred.Min.Y}, {X: o.Min.X - preferred.Width() - 16, Y: preferred.Min.Y},
			{X: preferred.Min.X, Y: o.Max.Y + 16}, {X: preferred.Min.X, Y: o.Min.Y - preferred.Height() - 16},
		} {
			candidates = append(candidates, sirena.Rect{Min: p, Max: sirena.Point{X: p.X + preferred.Width(), Y: p.Y + preferred.Height()}})
		}
	}
	center := preferred.Center()
	score := func(r sirena.Rect) float64 { c := r.Center(); return math.Abs(c.X-center.X) + math.Abs(c.Y-center.Y) }
	sort.SliceStable(candidates, func(i, j int) bool { return score(candidates[i]) < score(candidates[j]) })
	for _, r := range candidates {
		if clear(r) {
			return r
		}
	}
	// The rightmost candidate is outside every occupied actor.
	x := preferred.Min.X
	for _, o := range occupied {
		x = max(x, o.Max.X+16)
	}
	return shiftRect(preferred, x-preferred.Min.X, 0)
}
