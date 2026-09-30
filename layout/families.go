package layout

import (
	"fmt"
	"math"
	"strings"

	"m31labs.dev/sirena"
)

func metadataText(e *sirena.Element, key string) string {
	switch value := e.Metadata[key].(type) {
	case sirena.String:
		return value.Value
	case sirena.Ident:
		return value.Value
	}
	return ""
}
func cardRows(e *sirena.Element) []string {
	var rows []string
	for _, key := range []string{"fields", "methods"} {
		for _, row := range strings.Split(metadataText(e, key), ";") {
			if row = strings.TrimSpace(row); row != "" {
				rows = append(rows, row)
			}
		}
	}
	return rows
}
func connectDiagram(lr *sirena.LayoutResult, metrics Metrics) {
	ports := assignPorts(lr.NodePlacements, lr.View.Edges)
	lr.EdgeRoutes = routeEdges(lr.NodePlacements, ports, lr.View.Edges)
	placeLabels(lr.EdgeRoutes, lr.NodePlacements, metrics)
	lr.Bounds = diagramBounds(lr)
	for _, boundary := range lr.BoundaryPlacements {
		lr.Bounds = unionRect(lr.Bounds, boundary.Bounds)
	}
}

// State machines and record diagrams retain declared identities and typed
// relationships. A deterministic grid handles cyclic graphs without force
// simulation; measured compartments keep fields and methods legible.
func computeCards(rv *sirena.ResolvedView, seed [32]byte, metrics Metrics, kind string) (*sirena.LayoutResult, error) {
	if _, err := flatActors(rv); err != nil {
		return nil, err
	}
	lr := &sirena.LayoutResult{View: rv, Seed: seed, Diagram: kind}
	cols := max(1, int(math.Ceil(math.Sqrt(float64(len(rv.Elements))))))
	width, height := 140.0, 48.0
	for _, e := range rv.Elements {
		width = math.Max(width, actorWidth(e)+32)
		if kind != "state" {
			rows := cardRows(e)
			height = math.Max(height, 40+float64(len(rows))*24)
			for _, row := range rows {
				width = math.Max(width, labelWidth(row)+32)
			}
		}
	}
	// Edge captions must fit the gutters, including unusually long labels.
	gap := 72.0
	for _, edge := range rv.Edges {
		gap = math.Max(gap, labelWidth(edge.Label)+32)
	}
	for i, e := range rv.Elements {
		x, y := float64(i%cols)*(width+gap), float64(i/cols)*(height+gap)
		lr.NodePlacements = append(lr.NodePlacements, &sirena.NodePlacement{Node: e, Bounds: sirena.Rect{Min: sirena.Point{X: x, Y: y}, Max: sirena.Point{X: x + width, Y: y + height}}})
	}
	connectDiagram(lr, metrics)
	return lr, nil
}

// Lane membership is explicit metadata, not inferred from declaration types.
// Declaration order is the progression order, with one task per global row.
func computeSwimlane(rv *sirena.ResolvedView, seed [32]byte, metrics Metrics) (*sirena.LayoutResult, error) {
	if _, err := flatActors(rv); err != nil {
		return nil, err
	}
	lr := &sirena.LayoutResult{View: rv, Seed: seed, Diagram: "swimlane"}
	lanes, index := []string{}, map[string]int{}
	width := 160.0
	for _, e := range rv.Elements {
		name := metadataText(e, "lane")
		if name == "" {
			name = "General"
		}
		if _, ok := index[name]; !ok {
			index[name] = len(lanes)
			lanes = append(lanes, name)
		}
		width = math.Max(width, math.Max(actorWidth(e)+48, labelWidth(name)+48))
	}
	for i, e := range rv.Elements {
		name := metadataText(e, "lane")
		if name == "" {
			name = "General"
		}
		x, y := float64(index[name])*(width+32)+24, 56+float64(i)*88
		lr.NodePlacements = append(lr.NodePlacements, &sirena.NodePlacement{Node: e, Bounds: sirena.Rect{Min: sirena.Point{X: x, Y: y}, Max: sirena.Point{X: x + width - 48, Y: y + 40}}})
	}
	for i, name := range lanes {
		x := float64(i) * (width + 32)
		lr.BoundaryPlacements = append(lr.BoundaryPlacements, &sirena.BoundaryPlacement{Boundary: &sirena.Boundary{Name: name, Kind: sirena.BoundaryKindGroup}, Bounds: sirena.Rect{Min: sirena.Point{X: x}, Max: sirena.Point{X: x + width, Y: float64(len(rv.Elements))*88 + 80}}})
	}
	connectDiagram(lr, metrics)
	return lr, nil
}

// Timeline uses numeric start/duration units, preserving actual duration ratios.
// Names live beside bars so a short duration never shrinks the task's label.
func computeTimeline(rv *sirena.ResolvedView, seed [32]byte, metrics Metrics) (*sirena.LayoutResult, error) {
	if _, err := flatActors(rv); err != nil {
		return nil, err
	}
	lr := &sirena.LayoutResult{View: rv, Seed: seed, Diagram: "timeline"}
	maxEnd, labelSpace := 1.0, 120.0
	read := func(e *sirena.Element, key string, def float64) (float64, error) {
		v, exists := e.Metadata[key]
		if !exists {
			return def, nil
		}
		number, ok := v.(sirena.Number)
		if !ok || math.IsNaN(number.Value) || math.IsInf(number.Value, 0) || number.Value < 0 || number.Value > 1e9 {
			return 0, fmt.Errorf("sirena: timeline %s.%s must be a finite non-negative number <= 1e9", e.Name, key)
		}
		return number.Value, nil
	}
	starts, durations := make([]float64, len(rv.Elements)), make([]float64, len(rv.Elements))
	for i, e := range rv.Elements {
		var err error
		starts[i], err = read(e, "start", float64(i))
		if err != nil {
			return nil, err
		}
		durations[i], err = read(e, "duration", 1)
		if err != nil {
			return nil, err
		}
		if durations[i] == 0 {
			return nil, fmt.Errorf("sirena: timeline %s.duration must be positive", e.Name)
		}
		maxEnd = math.Max(maxEnd, starts[i]+durations[i])
		labelSpace = math.Max(labelSpace, labelWidth(e.DisplayLabel())+32)
	}
	unit := 640 / maxEnd
	for i, e := range rv.Elements {
		x, y := labelSpace+starts[i]*unit, 48+float64(i)*56
		lr.NodePlacements = append(lr.NodePlacements, &sirena.NodePlacement{Node: e, Bounds: sirena.Rect{Min: sirena.Point{X: x, Y: y}, Max: sirena.Point{X: x + durations[i]*unit, Y: y + 28}}})
	}
	connectDiagram(lr, metrics)
	lr.Bounds.Min.X = 0
	lr.Bounds.Max.X = math.Max(lr.Bounds.Max.X, labelSpace+640)
	return lr, nil
}
