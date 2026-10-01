package layout

import (
	"fmt"
	"math"
	"sort"
	"strconv"

	"m31labs.dev/sirena"
)

func finiteMetadata(e *sirena.Element, key string) (float64, error) {
	v, ok := e.Metadata[key].(sirena.Number)
	if !ok || math.IsNaN(v.Value) || math.IsInf(v.Value, 0) || math.Abs(v.Value) > 1e9 {
		return 0, fmt.Errorf("sirena: %s.%s must be a finite number between -1e9 and 1e9", e.Name, key)
	}
	return v.Value, nil
}
func seriesName(e *sirena.Element) string {
	if v, ok := e.Metadata["series"].(sirena.String); ok {
		return v.Value
	}
	return "Data"
}
func computePlot(rv *sirena.ResolvedView, seed [32]byte, kind string) (*sirena.LayoutResult, error) {
	if _, err := flatActors(rv); err != nil {
		return nil, err
	}
	if len(rv.Edges) > 0 {
		return nil, fmt.Errorf("sirena: %s plots use series metadata, not relationships", kind)
	}
	lr := &sirena.LayoutResult{View: rv, Seed: seed, Diagram: kind}
	p := &sirena.PlotPlacement{Bounds: sirena.Rect{Min: sirena.Point{X: 96, Y: 32}, Max: sirena.Point{X: 736, Y: 432}}, XMin: math.Inf(1), XMax: math.Inf(-1), YMin: math.Inf(1), YMax: math.Inf(-1)}
	values := make([]sirena.Point, len(rv.Elements))
	for i, e := range rv.Elements {
		x, err := finiteMetadata(e, "x")
		if err != nil {
			return nil, err
		}
		y, err := finiteMetadata(e, "y")
		if err != nil {
			return nil, err
		}
		values[i] = sirena.Point{X: x, Y: y}
		p.XMin = math.Min(p.XMin, x)
		p.XMax = math.Max(p.XMax, x)
		p.YMin = math.Min(p.YMin, y)
		p.YMax = math.Max(p.YMax, y)
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("sirena: %s needs at least one point", kind)
	}
	if p.XMin == p.XMax {
		p.XMin -= .5
		p.XMax += .5
	}
	if p.YMin == p.YMax {
		p.YMin -= .5
		p.YMax += .5
	}
	series := map[string]int{}
	for i, e := range rv.Elements {
		value := values[i]
		point := sirena.Point{X: p.Bounds.Min.X + (value.X-p.XMin)/(p.XMax-p.XMin)*p.Bounds.Width(), Y: p.Bounds.Max.Y - (value.Y-p.YMin)/(p.YMax-p.YMin)*p.Bounds.Height()}
		lr.NodePlacements = append(lr.NodePlacements, &sirena.NodePlacement{Node: e, Bounds: sirena.Rect{Min: sirena.Point{X: point.X - 5, Y: point.Y - 5}, Max: sirena.Point{X: point.X + 5, Y: point.Y + 5}}})
		name := seriesName(e)
		index, ok := series[name]
		if !ok {
			index = len(p.Series)
			series[name] = index
			p.Series = append(p.Series, sirena.ChartSeries{Name: name})
		}
		p.Series[index].Points = append(p.Series[index].Points, point)
	}
	for i := range p.Series {
		sort.SliceStable(p.Series[i].Points, func(a, b int) bool { return p.Series[i].Points[a].X < p.Series[i].Points[b].X })
	}
	lr.Plot = p
	lr.Bounds = sirena.Rect{Max: sirena.Point{X: 800, Y: 500 + float64(len(p.Series))*26}}
	return lr, nil
}

func computeRadar(rv *sirena.ResolvedView, seed [32]byte) (*sirena.LayoutResult, error) {
	if _, err := flatActors(rv); err != nil {
		return nil, err
	}
	if len(rv.Edges) > 0 {
		return nil, fmt.Errorf("sirena: radar does not accept relationships")
	}
	r := &sirena.RadarPlacement{Center: sirena.Point{X: 320, Y: 272}, Radius: 192}
	axisIndex, seriesIndex := map[string]int{}, map[string]int{}
	type datum struct {
		axis, name string
		value      float64
		element    *sirena.Element
	}
	var data []datum
	for _, e := range rv.Elements {
		v, err := chartValue(e)
		if err != nil {
			return nil, err
		}
		if v < 0 {
			return nil, fmt.Errorf("sirena: radar %s.value must be non-negative", e.Name)
		}
		label := e.DisplayLabel()
		if a, ok := e.Metadata["axis"].(sirena.String); ok {
			label = a.Value
		}
		if _, ok := axisIndex[label]; !ok {
			axisIndex[label] = len(r.Axes)
			r.Axes = append(r.Axes, label)
		}
		name := seriesName(e)
		if _, ok := seriesIndex[name]; !ok {
			seriesIndex[name] = len(r.Series)
			r.Series = append(r.Series, sirena.ChartSeries{Name: name})
		}
		r.Maximum = math.Max(r.Maximum, v)
		data = append(data, datum{label, name, v, e})
	}
	if len(r.Axes) < 3 || len(r.Axes) > 32 {
		return nil, fmt.Errorf("sirena: radar requires 3–32 axes")
	}
	if r.Maximum == 0 {
		r.Maximum = 1
	}
	for i := range r.Series {
		r.Series[i].Points = make([]sirena.Point, len(r.Axes))
	}
	seen := map[string]bool{}
	lr := &sirena.LayoutResult{View: rv, Seed: seed, Diagram: "radar", Radar: r}
	labelSpace := 0.0
	for _, d := range data {
		key := d.name + "\x00" + d.axis
		if seen[key] {
			return nil, fmt.Errorf("sirena: duplicate radar axis %q in series %q", d.axis, d.name)
		}
		seen[key] = true
		index := axisIndex[d.axis]
		angle := -math.Pi/2 + float64(index)*2*math.Pi/float64(len(r.Axes))
		radius := d.value / r.Maximum * r.Radius
		point := sirena.Point{X: r.Center.X + radius*math.Cos(angle), Y: r.Center.Y + radius*math.Sin(angle)}
		r.Series[seriesIndex[d.name]].Points[index] = point
		lr.NodePlacements = append(lr.NodePlacements, &sirena.NodePlacement{Node: d.element, Bounds: sirena.Rect{Min: sirena.Point{X: point.X - 4, Y: point.Y - 4}, Max: sirena.Point{X: point.X + 4, Y: point.Y + 4}}})
		labelSpace = math.Max(labelSpace, labelWidth(d.axis))
	}
	for _, s := range r.Series {
		for _, axis := range r.Axes {
			if !seen[s.Name+"\x00"+axis] {
				return nil, fmt.Errorf("sirena: radar series %q is missing axis %q", s.Name, axis)
			}
		}
	}
	lr.Bounds = sirena.Rect{Min: sirena.Point{X: math.Min(0, 320-224-labelSpace/2)}, Max: sirena.Point{X: math.Max(640, 320+224+labelSpace/2), Y: 560 + float64(len(r.Series))*26}}
	return lr, nil
}

func computeSankey(rv *sirena.ResolvedView, seed [32]byte, metrics Metrics) (*sirena.LayoutResult, error) {
	index, err := flatActors(rv)
	if err != nil {
		return nil, err
	}
	n := len(rv.Elements)
	if n == 0 || len(rv.Edges) == 0 {
		return nil, fmt.Errorf("sirena: sankey needs nodes and positive weighted relationships")
	}
	in, out := make([]float64, n), make([]float64, n)
	degree, rank := make([]int, n), make([]int, n)
	adj := make([][]int, n)
	weights := make([]float64, len(rv.Edges))
	for i, e := range rv.Edges {
		v, err := strconv.ParseFloat(e.Label, 64)
		if metadata, ok := e.Metadata["value"].(sirena.Number); ok {
			v = metadata.Value
			err = nil
		}
		if err != nil || v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) || v > 1e9 {
			return nil, fmt.Errorf("sirena: sankey %s -> %s needs a positive numeric label or value", e.From, e.To)
		}
		if e.Direction != sirena.DirForward {
			return nil, fmt.Errorf("sirena: sankey requires forward relationships")
		}
		a, b := index[e.From], index[e.To]
		weights[i] = v
		out[a] += v
		in[b] += v
		degree[b]++
		adj[a] = append(adj[a], b)
	}
	queue := make([]int, 0, n)
	for i, d := range degree {
		if d == 0 {
			queue = append(queue, i)
		}
	}
	for k := 0; k < len(queue); k++ {
		a := queue[k]
		for _, b := range adj[a] {
			rank[b] = max(rank[b], rank[a]+1)
			degree[b]--
			if degree[b] == 0 {
				queue = append(queue, b)
			}
		}
	}
	if len(queue) != n {
		return nil, fmt.Errorf("sirena: sankey requires an acyclic flow graph")
	}
	levels := make([][]int, 1)
	width := 100.0
	for i, e := range rv.Elements {
		for len(levels) <= rank[i] {
			levels = append(levels, nil)
		}
		levels[rank[i]] = append(levels[rank[i]], i)
		width = math.Max(width, actorWidth(e))
	}
	maxTotal := 0.0
	for _, level := range levels {
		sum := 0.0
		for _, i := range level {
			sum += math.Max(in[i], out[i])
		}
		maxTotal = math.Max(maxTotal, sum)
	}
	unit := 360 / maxTotal
	lr := &sirena.LayoutResult{View: rv, Seed: seed, Diagram: "sankey"}
	placements := make([]*sirena.NodePlacement, n)
	for column, level := range levels {
		y := 24.0
		for _, i := range level {
			height := math.Max(32, math.Max(in[i], out[i])*unit)
			x := float64(column) * (width + 160)
			placements[i] = &sirena.NodePlacement{Node: rv.Elements[i], Bounds: sirena.Rect{Min: sirena.Point{X: x, Y: y}, Max: sirena.Point{X: x + width, Y: y + height}}}
			y += height + 32
		}
	}
	lr.NodePlacements = placements
	usedIn, usedOut := make([]float64, n), make([]float64, n)
	for i, e := range rv.Edges {
		a, b := index[e.From], index[e.To]
		ra, rb := placements[a].Bounds, placements[b].Bounds
		w := weights[i] * unit
		from := sirena.Point{X: ra.Max.X, Y: ra.Center().Y - out[a]*unit/2 + usedOut[a] + w/2}
		to := sirena.Point{X: rb.Min.X, Y: rb.Center().Y - in[b]*unit/2 + usedIn[b] + w/2}
		usedOut[a] += w
		usedIn[b] += w
		lr.Flows = append(lr.Flows, sirena.FlowPlacement{Edge: e, From: from, To: to, Width: w, Value: weights[i]})
		lr.EdgeRoutes = append(lr.EdgeRoutes, &sirena.EdgeRoute{Edge: e, Points: []sirena.Point{from, to}})
	}
	lr.Bounds = diagramBounds(lr)
	return lr, nil
}
