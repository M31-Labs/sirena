package layout

import (
	"fmt"
	"m31labs.dev/sirena"
	"math"
	"slices"
)

// Storyboard reserves stable graph slots and chart domains across 2–32 flat
// states. Names identify actors. Caller-owned views and metadata are read-only.
func Storyboard(views []*sirena.ResolvedView, opts sirena.RenderOptions) ([]*sirena.LayoutResult, error) {
	if len(views) < 2 || len(views) > 32 {
		return nil, fmt.Errorf("sirena: storyboard needs 2–32 states")
	}
	kind := opts.Diagram
	if kind == "" {
		kind = sirena.DiagramName(views[0])
	}
	switch kind {
	case "architecture", "state", "class", "er", "mindmap", "bar", "line", "scatter", "radar":
	default:
		return nil, fmt.Errorf("sirena: %s storyboards are not supported", kind)
	}
	var frames []*sirena.LayoutResult
	union := &sirena.ResolvedView{}
	byName := map[string]int{}
	edges := map[string]bool{}
	sizes := map[string]float64{}
	for i, v := range views {
		if v == nil {
			return nil, fmt.Errorf("sirena: state %d is nil", i)
		}
		if _, err := flatActors(v); err != nil {
			return nil, err
		}
		if opts.Diagram == "" && sirena.DiagramName(v) != kind {
			return nil, fmt.Errorf("sirena: storyboard families must match")
		}
		lr, _, err := Render(v, sirena.RenderOptions{Diagram: kind, StrictBudget: opts.StrictBudget})
		if err != nil {
			return nil, fmt.Errorf("state %d: %w", i, err)
		}
		frames = append(frames, lr)
		for _, np := range lr.NodePlacements {
			e := np.Node
			size := np.Bounds.Width() * np.Bounds.Height()
			index, ok := byName[e.Name]
			if !ok {
				index = len(union.Elements)
				byName[e.Name] = index
				union.Elements = append(union.Elements, e)
			}
			if size > sizes[e.Name] {
				union.Elements[index] = e
				sizes[e.Name] = size
			}
		}
		for _, e := range v.Edges {
			key := fmt.Sprintf("%s\x00%s\x00%d\x00%d", e.From, e.To, e.Kind, e.Direction)
			if !edges[key] {
				edges[key] = true
				union.Edges = append(union.Edges, e)
			}
		}
	}
	if len(union.Elements) > 1000 || len(union.Edges) > 2000 {
		return nil, fmt.Errorf("sirena: storyboard union exceeds 1000 nodes or 2000 edges")
	}
	bounds := sirena.Rect{}
	switch kind {
	case "line", "scatter":
		domain := *frames[0].Plot
		for _, lr := range frames[1:] {
			p := lr.Plot
			domain.XMin = math.Min(domain.XMin, p.XMin)
			domain.XMax = math.Max(domain.XMax, p.XMax)
			domain.YMin = math.Min(domain.YMin, p.YMin)
			domain.YMax = math.Max(domain.YMax, p.YMax)
		}
		for _, lr := range frames {
			p := lr.Plot
			old := *p
			p.XMin, p.XMax, p.YMin, p.YMax = domain.XMin, domain.XMax, domain.YMin, domain.YMax
			project := func(point sirena.Point) sirena.Point {
				x := old.XMin + (point.X-old.Bounds.Min.X)/old.Bounds.Width()*(old.XMax-old.XMin)
				y := old.YMin + (old.Bounds.Max.Y-point.Y)/old.Bounds.Height()*(old.YMax-old.YMin)
				return sirena.Point{X: p.Bounds.Min.X + (x-p.XMin)/(p.XMax-p.XMin)*p.Bounds.Width(), Y: p.Bounds.Max.Y - (y-p.YMin)/(p.YMax-p.YMin)*p.Bounds.Height()}
			}
			for _, np := range lr.NodePlacements {
				np.Bounds = pointBounds(project(np.Bounds.Center()), 5)
			}
			for i := range p.Series {
				for j, point := range p.Series[i].Points {
					p.Series[i].Points[j] = project(point)
				}
			}
		}
	case "radar":
		maximum := frames[0].Radar.Maximum
		for _, lr := range frames[1:] {
			maximum = math.Max(maximum, lr.Radar.Maximum)
			if !slices.Equal(lr.Radar.Axes, frames[0].Radar.Axes) {
				return nil, fmt.Errorf("sirena: radar storyboard axis order must match")
			}
		}
		for _, lr := range frames {
			r := lr.Radar
			ratio := r.Maximum / maximum
			r.Maximum = maximum
			project := func(p sirena.Point) sirena.Point {
				return sirena.Point{X: r.Center.X + (p.X-r.Center.X)*ratio, Y: r.Center.Y + (p.Y-r.Center.Y)*ratio}
			}
			for _, np := range lr.NodePlacements {
				np.Bounds = pointBounds(project(np.Bounds.Center()), 4)
			}
			for i := range r.Series {
				for j, p := range r.Series[i].Points {
					r.Series[i].Points[j] = project(p)
				}
			}
		}
	case "bar":
		lo, hi, labelSpace := 0.0, 0.0, 0.0
		for _, lr := range frames {
			labelSpace = math.Max(labelSpace, lr.ChartLabelX+12)
			for _, np := range lr.NodePlacements {
				value, _ := chartValue(np.Node)
				lo = math.Min(lo, value)
				hi = math.Max(hi, value)
			}
		}
		span := hi - lo
		if span == 0 {
			span = 1
		}
		unit := 600 / span
		zero := labelSpace - lo*unit
		for _, lr := range frames {
			lr.ChartBaseline = zero
			lr.ChartLabelX = labelSpace - 12
			for _, np := range lr.NodePlacements {
				value, _ := chartValue(np.Node)
				x := zero + math.Min(0, value)*unit
				y := 40 + float64(byName[np.Node.Name])*48
				np.Bounds = sirena.Rect{Min: sirena.Point{X: x, Y: y}, Max: sirena.Point{X: x + math.Max(1, math.Abs(value)*unit), Y: y + 28}}
			}
			lr.Bounds = sirena.Rect{Max: sirena.Point{X: labelSpace + 720, Y: math.Max(80, 40+float64(len(union.Elements))*48)}}
		}
	default:
		reference, _, err := Render(union, sirena.RenderOptions{Diagram: kind})
		if err != nil {
			return nil, fmt.Errorf("storyboard union: %w", err)
		}
		slots := map[string]sirena.Rect{}
		for _, np := range reference.NodePlacements {
			slots[np.Node.Name] = np.Bounds
		}
		for _, lr := range frames {
			for _, np := range lr.NodePlacements {
				np.Bounds = slots[np.Node.Name]
			}
			lr.EdgeRoutes = routeEdges(lr.NodePlacements, assignPorts(lr.NodePlacements, lr.View.Edges), lr.View.Edges)
			placeLabels(lr.EdgeRoutes, lr.NodePlacements, DefaultMetrics())
			lr.Bounds = diagramBounds(lr)
		}
		bounds = reference.Bounds
	}
	for _, lr := range frames {
		bounds.Min.X = math.Min(bounds.Min.X, lr.Bounds.Min.X)
		bounds.Min.Y = math.Min(bounds.Min.Y, lr.Bounds.Min.Y)
		bounds.Max.X = math.Max(bounds.Max.X, lr.Bounds.Max.X)
		bounds.Max.Y = math.Max(bounds.Max.Y, lr.Bounds.Max.Y)
	}
	for _, lr := range frames {
		lr.Bounds = bounds
	}
	return frames, nil
}
func pointBounds(p sirena.Point, r float64) sirena.Rect {
	return sirena.Rect{Min: sirena.Point{X: p.X - r, Y: p.Y - r}, Max: sirena.Point{X: p.X + r, Y: p.Y + r}}
}
