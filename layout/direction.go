package layout

import (
	"m31labs.dev/sirena"
	"math"
)

// Apply the view's direction within cells as well as between boundary regions.
// Text boxes keep their original dimensions when rank and order axes swap.
func layoutDirectedCell(items []cellItem, edges []*sirena.Edge, metrics Metrics, rv *sirena.ResolvedView) ([]*sirena.NodePlacement, []*sirena.SummaryPlacement, sirena.Rect) {
	direction := ""
	if rv != nil && rv.Source != nil && rv.Source.Layout != nil {
		direction = rv.Source.Layout.Direction
	}
	horizontal := direction == "left-right" || direction == "right-left"
	m := metrics
	if horizontal {
		width := metrics.NodeHeight()
		for _, item := range items {
			width = math.Max(width, metrics.NodeWidth(item.label()))
		}
		m.Height = width
		m.WidthOf = func(string) float64 { return metrics.NodeHeight() }
	}
	nodes, summaries, bounds := layoutCell(items, edges, m)
	transform := func(rect sirena.Rect, label string) sirena.Rect {
		if horizontal {
			center := rect.Center()
			width, height := metrics.NodeWidth(label), metrics.NodeHeight()
			rect = sirena.Rect{Min: sirena.Point{X: center.Y - width/2, Y: center.X - height/2}, Max: sirena.Point{X: center.Y + width/2, Y: center.X + height/2}}
			if direction == "right-left" {
				rect.Min.X, rect.Max.X = bounds.Min.Y+bounds.Max.Y-rect.Max.X, bounds.Min.Y+bounds.Max.Y-rect.Min.X
			}
		} else if direction == "bottom-up" {
			rect.Min.Y, rect.Max.Y = bounds.Min.Y+bounds.Max.Y-rect.Max.Y, bounds.Min.Y+bounds.Max.Y-rect.Min.Y
		}
		return rect
	}
	var result sirena.Rect
	has := false
	include := func(rect sirena.Rect) {
		if !has {
			result = rect
			has = true
		} else {
			result = unionRect(result, rect)
		}
	}
	for _, node := range nodes {
		node.Bounds = transform(node.Bounds, node.Node.Name)
		include(node.Bounds)
	}
	for _, summary := range summaries {
		summary.Bounds = transform(summary.Bounds, summary.Summary.Label)
		include(summary.Bounds)
	}
	return nodes, summaries, result
}
