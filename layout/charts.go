package layout

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"m31labs.dev/sirena"
)

// Tree layout is linear in nodes and relationships. A forest is allowed;
// cycles and multiple parents get actionable diagnostics instead of bad geometry.
func computeMindmap(rv *sirena.ResolvedView, seed [32]byte, metrics Metrics) (*sirena.LayoutResult, error) {
	index, err := flatActors(rv)
	if err != nil {
		return nil, err
	}
	n := len(rv.Elements)
	children := make([][]int, n)
	indegree := make([]int, n)
	for _, edge := range rv.Edges {
		a, b := index[edge.From], index[edge.To]
		children[a] = append(children[a], b)
		indegree[b]++
		if indegree[b] > 1 {
			return nil, fmt.Errorf("sirena: mindmap %s has multiple parents", edge.To)
		}
	}
	lr := &sirena.LayoutResult{View: rv, Seed: seed, Diagram: "mindmap"}
	levels := make([]int, n)
	y := make([]float64, n)
	visited := 0
	row := 0.0
	width := 100.0
	gap := 100.0
	for _, edge := range rv.Edges {
		gap = math.Max(gap, labelWidth(edge.Label)+32)
	}
	for _, e := range rv.Elements {
		width = math.Max(width, actorWidth(e))
	}
	var walk func(int, int) float64
	walk = func(i, depth int) float64 {
		visited++
		levels[i] = depth
		if len(children[i]) == 0 {
			y[i] = row
			row += 72
		} else {
			first := walk(children[i][0], depth+1)
			last := first
			for _, child := range children[i][1:] {
				last = walk(child, depth+1)
			}
			y[i] = (first + last) / 2
		}
		return y[i]
	}
	for i := range rv.Elements {
		if indegree[i] == 0 {
			walk(i, 0)
			row += 32
		}
	}
	if visited != n {
		return nil, fmt.Errorf("sirena: mindmap requires an acyclic parent-to-child forest")
	}
	for i, e := range rv.Elements {
		x := float64(levels[i]) * (width + gap)
		lr.NodePlacements = append(lr.NodePlacements, &sirena.NodePlacement{Node: e, Bounds: sirena.Rect{Min: sirena.Point{X: x, Y: y[i]}, Max: sirena.Point{X: x + width, Y: y[i] + 40}}})
	}
	for _, edge := range rv.Edges {
		a, b := lr.NodePlacements[index[edge.From]].Bounds, lr.NodePlacements[index[edge.To]].Bounds
		from, to := sirena.Point{X: a.Max.X, Y: a.Center().Y}, sirena.Point{X: b.Min.X, Y: b.Center().Y}
		mid := (from.X + to.X) / 2
		route := &sirena.EdgeRoute{Edge: edge, Points: []sirena.Point{from, {X: mid, Y: from.Y}, {X: mid, Y: to.Y}, to}}
		if edge.Label != "" {
			anchor := sirena.Point{X: mid, Y: to.Y - 12}
			half := labelWidth(edge.Label) / 2
			route.Label = &sirena.EdgeLabel{Text: edge.Label, Anchor: anchor, Bounds: sirena.Rect{Min: sirena.Point{X: mid - half, Y: anchor.Y - 7}, Max: sirena.Point{X: mid + half, Y: anchor.Y + 7}}}
		}
		lr.EdgeRoutes = append(lr.EdgeRoutes, route)
	}
	lr.Bounds = diagramBounds(lr)
	return lr, nil
}

func chartValue(e *sirena.Element) (float64, error) {
	v, ok := e.Metadata["value"].(sirena.Number)
	if !ok || math.IsNaN(v.Value) || math.IsInf(v.Value, 0) || math.Abs(v.Value) > 1e9 {
		return 0, fmt.Errorf("sirena: %s.value must be a finite number between -1e9 and 1e9", e.Name)
	}
	return v.Value, nil
}

func computeChart(rv *sirena.ResolvedView, seed [32]byte, kind string) (*sirena.LayoutResult, error) {
	if _, err := flatActors(rv); err != nil {
		return nil, err
	}
	if len(rv.Edges) > 0 {
		return nil, fmt.Errorf("sirena: %s data charts do not accept relationships", kind)
	}
	lr := &sirena.LayoutResult{View: rv, Seed: seed, Diagram: kind}
	values := make([]float64, len(rv.Elements))
	lo, hi, total, labelSpace := 0.0, 0.0, 0.0, 100.0
	for i, e := range rv.Elements {
		v, err := chartValue(e)
		if err != nil {
			return nil, err
		}
		if kind == "pie" && v < 0 {
			return nil, fmt.Errorf("sirena: pie %s.value must be non-negative", e.Name)
		}
		values[i] = v
		lo = math.Min(lo, v)
		hi = math.Max(hi, v)
		total += v
		labelSpace = math.Max(labelSpace, labelWidth(e.DisplayLabel())+24)
	}
	if kind == "pie" {
		if len(rv.Elements) > 100 {
			return nil, fmt.Errorf("sirena: pie supports at most 100 slices; use bar for larger series")
		}
		if total <= 0 {
			return nil, fmt.Errorf("sirena: pie needs at least one positive value")
		}
		for i, e := range rv.Elements {
			label := fmt.Sprintf("%s — %s (%.1f%%)", e.DisplayLabel(), strconv.FormatFloat(values[i], 'g', -1, 64), values[i]/total*100)
			labelSpace = math.Max(labelSpace, labelWidth(label)+32)
		}
		angle := -math.Pi / 2
		for i, e := range rv.Elements {
			end := angle + values[i]/total*2*math.Pi
			lr.PieSlices = append(lr.PieSlices, sirena.PieSlicePlacement{Node: e, Center: sirena.Point{X: 160, Y: 160}, Radius: 144, Start: angle, End: end, Value: values[i], Fraction: values[i] / total})
			angle = end
			y := float64(i) * 40
			lr.NodePlacements = append(lr.NodePlacements, &sirena.NodePlacement{Node: e, Bounds: sirena.Rect{Min: sirena.Point{X: 344, Y: y}, Max: sirena.Point{X: 344 + labelSpace, Y: y + 32}}})
		}
		lr.Bounds = sirena.Rect{Max: sirena.Point{X: 344 + labelSpace, Y: math.Max(320, float64(len(rv.Elements))*40)}}
		return lr, nil
	}
	span := hi - lo
	if span == 0 {
		span = 1
	}
	unit := 600 / span
	zero := labelSpace - lo*unit
	lr.ChartBaseline = zero
	for i, e := range rv.Elements {
		x := zero + math.Min(0, values[i])*unit
		w := math.Max(1, math.Abs(values[i])*unit)
		y := 40 + float64(i)*48
		lr.NodePlacements = append(lr.NodePlacements, &sirena.NodePlacement{Node: e, Bounds: sirena.Rect{Min: sirena.Point{X: x, Y: y}, Max: sirena.Point{X: x + w, Y: y + 28}}})
	}
	lr.Bounds = sirena.Rect{Max: sirena.Point{X: labelSpace + 720, Y: math.Max(80, 40+float64(len(rv.Elements))*48)}}
	lr.ChartLabelX = labelSpace - 12
	return lr, nil
}

// Calendar dates map to actual UTC day distances; dependencies keep their
// original identities. Copies protect caller-owned metadata.
func computeGantt(rv *sirena.ResolvedView, seed [32]byte, metrics Metrics) (*sirena.LayoutResult, error) {
	if _, err := flatActors(rv); err != nil {
		return nil, err
	}
	copyView := *rv
	copyView.Elements = make([]*sirena.Element, len(rv.Elements))
	starts := make([]time.Time, len(rv.Elements))
	ends := make([]time.Time, len(rv.Elements))
	var first, last time.Time
	for i, e := range rv.Elements {
		start, err := time.Parse("2006-01-02", metadataText(e, "start"))
		if err != nil {
			return nil, fmt.Errorf("sirena: gantt %s.start requires YYYY-MM-DD", e.Name)
		}
		end, err := time.Parse("2006-01-02", metadataText(e, "end"))
		if err != nil || !end.After(start) {
			return nil, fmt.Errorf("sirena: gantt %s.end must be a YYYY-MM-DD after start (exclusive)", e.Name)
		}
		starts[i], ends[i] = start, end
		if i == 0 || start.Before(first) {
			first = start
		}
		if i == 0 || end.After(last) {
			last = end
		}
	}
	if len(rv.Elements) > 0 && last.Sub(first).Hours()/24 > 36525 {
		return nil, fmt.Errorf("sirena: gantt date range must fit within 100 years")
	}
	for i, e := range rv.Elements {
		copyElement := *e
		copyElement.Metadata = map[string]sirena.Value{}
		for k, v := range e.Metadata {
			copyElement.Metadata[k] = v
		}
		copyElement.Metadata["start"] = sirena.Number{Value: starts[i].Sub(first).Hours() / 24}
		copyElement.Metadata["duration"] = sirena.Number{Value: ends[i].Sub(starts[i]).Hours() / 24}
		copyView.Elements[i] = &copyElement
	}
	lr, err := computeTimeline(&copyView, seed, metrics)
	if err != nil {
		return nil, err
	}
	lr.View = rv
	lr.Diagram = "gantt"
	for i, np := range lr.NodePlacements {
		np.Node = rv.Elements[i]
	}
	if len(rv.Elements) > 0 {
		lr.ChartStart = first.Format("2006-01-02")
		lr.ChartEnd = last.Format("2006-01-02")
	}
	return lr, nil
}
