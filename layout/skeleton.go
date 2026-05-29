package layout

import "m31labs.dev/sirena"

// solveSkeleton arranges top-level regions (boundary placements, and the
// implicit nil-boundary region for loose elements) along a primary axis
// chosen by the view's layout hints, then returns the overall canvas
// bounding box. It is a small iterative relaxation rather than a full
// constraint solver:
//
//   - Direction picks the primary axis and sweep order: top-down /
//     bottom-up stack along Y; left-right / right-left along X.
//   - Pins to the leading side (top for vertical, left for horizontal)
//     move those regions to the front of the order; pins to the trailing
//     side move them to the back. Declaration order is otherwise
//     preserved (stable).
//   - Regions are packed in that order with skeletonPadding between them,
//     aligned to 0 on the cross axis.
//
// Each region's whole placement tree is translated into place; the caller
// is responsible for translating the node/summary placements that live
// inside each region by the same delta (see Compute).
func solveSkeleton(regions []*sirena.BoundaryPlacement, hints *sirena.LayoutHints, metrics Metrics) sirena.Rect {
	_ = metrics // padding is the fixed skeletonPadding for v0.1
	if len(regions) == 0 {
		return sirena.Rect{}
	}

	dir := "top-down"
	var pin map[string]string
	if hints != nil {
		if hints.Direction != "" {
			dir = hints.Direction
		}
		pin = hints.Pin
	}
	horizontal := dir == "left-right" || dir == "right-left"
	reverse := dir == "bottom-up" || dir == "right-left"

	leadSide, trailSide := "top", "bottom"
	if horizontal {
		leadSide, trailSide = "left", "right"
	}

	var lead, mid, trail []*sirena.BoundaryPlacement
	for _, r := range regions {
		side := ""
		if r.Boundary != nil && pin != nil {
			side = pin[r.Boundary.Name]
		}
		switch side {
		case leadSide:
			lead = append(lead, r)
		case trailSide:
			trail = append(trail, r)
		default:
			mid = append(mid, r)
		}
	}
	ordered := make([]*sirena.BoundaryPlacement, 0, len(regions))
	ordered = append(ordered, lead...)
	ordered = append(ordered, mid...)
	ordered = append(ordered, trail...)
	if reverse {
		for i, j := 0, len(ordered)-1; i < j; i, j = i+1, j-1 {
			ordered[i], ordered[j] = ordered[j], ordered[i]
		}
	}

	cursor := 0.0
	for _, r := range ordered {
		var dx, dy float64
		if horizontal {
			dx, dy = cursor-r.Bounds.Min.X, -r.Bounds.Min.Y
		} else {
			dx, dy = -r.Bounds.Min.X, cursor-r.Bounds.Min.Y
		}
		shiftBoundaryTree(r, dx, dy)
		if horizontal {
			cursor = r.Bounds.Max.X + skeletonPadding
		} else {
			cursor = r.Bounds.Max.Y + skeletonPadding
		}
	}

	canvas := regions[0].Bounds
	for _, r := range regions[1:] {
		canvas = unionRect(canvas, r.Bounds)
	}
	return canvas
}
