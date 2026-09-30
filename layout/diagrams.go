package layout

import (
	"fmt"
	"math"

	"m31labs.dev/sirena"
	"m31labs.dev/sirena/render/svg/font"
)

func flatActors(rv *sirena.ResolvedView) (map[string]int, error) {
	if len(rv.Boundaries) > 0 || len(rv.Summaries) > 0 {
		return nil, fmt.Errorf("sirena: %s diagrams require a flat view; select participants without boundaries or summaries", sirena.DiagramName(rv))
	}
	if len(rv.Elements) > 1000 || len(rv.Edges) > 2000 {
		return nil, fmt.Errorf("sirena: diagram exceeds 1000 participants or 2000 relationships")
	}
	index := map[string]int{}
	for i, actor := range rv.Elements {
		if actor == nil || actor.Name == "" {
			return nil, fmt.Errorf("sirena: diagram has an unnamed participant")
		}
		if _, ok := index[actor.Name]; ok {
			return nil, fmt.Errorf("sirena: duplicate participant %q", actor.Name)
		}
		index[actor.Name] = i
	}
	for _, edge := range rv.Edges {
		if edge == nil {
			return nil, fmt.Errorf("sirena: diagram has a nil relationship")
		}
		for _, name := range []string{edge.From, edge.To} {
			if _, ok := index[name]; !ok {
				return nil, fmt.Errorf("sirena: relationship refers to participant %q outside this view", name)
			}
		}
	}
	return index, nil
}

func labelWidth(label string) float64 { return font.Measure(label) * 14 / font.EmSize }
func actorWidth(actor *sirena.Element) float64 {
	return math.Max(60, labelWidth(actor.DisplayLabel())+24)
}

// computeSequence preserves declaration and message order. Each relationship
// occupies its own time row, including repeated requests and self messages.
func computeSequence(rv *sirena.ResolvedView, seed [32]byte, metrics Metrics) (*sirena.LayoutResult, error) {
	index, err := flatActors(rv)
	if err != nil {
		return nil, err
	}
	lr := &sirena.LayoutResult{View: rv, Seed: seed, Diagram: "sequence"}
	if len(rv.Elements) == 0 {
		return lr, nil
	}
	spacing := 180.0
	for _, actor := range rv.Elements {
		spacing = math.Max(spacing, actorWidth(actor)+48)
	}
	for _, edge := range rv.Edges {
		distance := math.Abs(float64(index[edge.To] - index[edge.From]))
		if distance > 0 {
			spacing = math.Max(spacing, (labelWidth(edge.Label)+48)/distance)
		} else {
			spacing = math.Max(spacing, labelWidth(edge.Label)+96)
		}
	}
	bottom := 100 + float64(len(rv.Edges))*64
	xs := map[string]float64{}
	for i, actor := range rv.Elements {
		x := 96 + float64(i)*spacing
		xs[actor.Name] = x
		width := actorWidth(actor)
		lr.NodePlacements = append(lr.NodePlacements, &sirena.NodePlacement{Node: actor, Bounds: sirena.Rect{Min: sirena.Point{X: x - width/2}, Max: sirena.Point{X: x + width/2, Y: 40}}})
		lr.Lifelines = append(lr.Lifelines, sirena.LifelinePlacement{Actor: actor, From: sirena.Point{X: x, Y: 40}, To: sirena.Point{X: x, Y: bottom}})
	}
	for i, edge := range rv.Edges {
		x1, x2 := xs[edge.From], xs[edge.To]
		y := 80 + float64(i)*64
		route := &sirena.EdgeRoute{Edge: edge, Points: []sirena.Point{{X: x1, Y: y}, {X: x2, Y: y}}}
		anchor := sirena.Point{X: (x1 + x2) / 2, Y: y - 16}
		if x1 == x2 {
			extent := math.Max(48, labelWidth(edge.Label)/2+24)
			route.Points = []sirena.Point{{X: x1, Y: y}, {X: x1 + extent, Y: y}, {X: x1 + extent, Y: y + 24}, {X: x1, Y: y + 24}}
			anchor.X = x1 + extent/2
		}
		if edge.Label != "" {
			route.Label = &sirena.EdgeLabel{Text: edge.Label, Anchor: anchor}
		}
		lr.EdgeRoutes = append(lr.EdgeRoutes, route)
	}
	lr.Bounds = diagramBounds(lr)
	return lr, nil
}

// computeRadial places the first participant at the center and the remaining
// participants around a ring sized to keep their label boxes separate.
func computeRadial(rv *sirena.ResolvedView, seed [32]byte, metrics Metrics) (*sirena.LayoutResult, error) {
	_, err := flatActors(rv)
	if err != nil {
		return nil, err
	}
	lr := &sirena.LayoutResult{View: rv, Seed: seed, Diagram: "radial"}
	if len(rv.Elements) == 0 {
		return lr, nil
	}
	maxWidth := 60.0
	for _, actor := range rv.Elements {
		maxWidth = math.Max(maxWidth, actorWidth(actor))
	}
	n := len(rv.Elements) - 1
	radius := maxWidth + 96
	if n > 1 {
		radius = math.Max(radius, (maxWidth+48)/(2*math.Sin(math.Pi/float64(n))))
	}
	for i, actor := range rv.Elements {
		x, y := 0.0, 0.0
		if i > 0 {
			angle := -math.Pi/2 + 2*math.Pi*float64(i-1)/float64(n)
			x, y = radius*math.Cos(angle), radius*math.Sin(angle)
		}
		width := actorWidth(actor)
		lr.NodePlacements = append(lr.NodePlacements, &sirena.NodePlacement{Node: actor, Bounds: sirena.Rect{Min: sirena.Point{X: x - width/2, Y: y - 20}, Max: sirena.Point{X: x + width/2, Y: y + 20}}})
	}
	ports := assignPorts(lr.NodePlacements, rv.Edges)
	lr.EdgeRoutes = routeEdges(lr.NodePlacements, ports, rv.Edges)
	placeLabels(lr.EdgeRoutes, lr.NodePlacements, metrics)
	lr.Bounds = diagramBounds(lr)
	return lr, nil
}

func diagramBounds(lr *sirena.LayoutResult) sirena.Rect {
	var bounds sirena.Rect
	have := false
	include := func(rect sirena.Rect) {
		if !have {
			bounds = rect
			have = true
		} else {
			bounds = unionRect(bounds, rect)
		}
	}
	for _, node := range lr.NodePlacements {
		include(node.Bounds)
	}
	for _, line := range lr.Lifelines {
		include(sirena.Rect{Min: line.From, Max: line.To})
	}
	for _, route := range lr.EdgeRoutes {
		for _, point := range route.Points {
			include(sirena.Rect{Min: point, Max: point})
		}
		if route.Label != nil {
			p := route.Label.Anchor
			width := labelWidth(route.Label.Text)
			include(sirena.Rect{Min: sirena.Point{X: p.X - width/2, Y: p.Y - 10}, Max: sirena.Point{X: p.X + width/2, Y: p.Y + 10}})
		}
	}
	return bounds
}
