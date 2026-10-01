package sirena

// LifelinePlacement is an actor's vertical time axis in a sequence diagram.
type LifelinePlacement struct {
	Actor    *Element
	From, To Point
}

func ValidDiagram(name string) bool {
	switch name {
	case "architecture", "sequence", "radial", "state", "class", "er", "swimlane", "timeline", "mindmap", "bar", "pie", "gantt", "line", "scatter", "sankey", "radar":
		return true
	}
	return false
}

// DiagramName reads the view's diagram hint without mutating source IR.
func DiagramName(rv *ResolvedView) string {
	if rv == nil || rv.Source == nil || rv.Source.Layout == nil {
		return "architecture"
	}
	switch v := rv.Source.Layout.Metadata["diagram"].(type) {
	case String:
		return v.Value
	case Ident:
		return v.Value
	}
	return "architecture"
}

// WithDiagram returns a view with an independent layout hint; source nodes
// and the caller's view declaration remain unchanged.
func WithDiagram(rv *ResolvedView, name string) *ResolvedView {
	copy := ResolvedView{}
	if rv != nil {
		copy = *rv
	}
	source := ViewDecl{}
	if copy.Source != nil {
		source = *copy.Source
	}
	layout := LayoutHints{}
	if source.Layout != nil {
		layout = *source.Layout
	}
	layout.Metadata = map[string]Value{}
	if source.Layout != nil {
		for k, v := range source.Layout.Metadata {
			layout.Metadata[k] = v
		}
	}
	layout.Metadata["diagram"] = String{Value: name}
	source.Layout = &layout
	copy.Source = &source
	return &copy
}

// PieSlicePlacement carries data geometry independently of its legend row.
type PieSlicePlacement struct {
	Node                                *Element
	Center                              Point
	Radius, Start, End, Value, Fraction float64
}

// PlotPlacement carries numerical axes separately from data point boxes.
type PlotPlacement struct {
	Bounds                 Rect
	XMin, XMax, YMin, YMax float64
	Series                 []ChartSeries
}
type ChartSeries struct {
	Name   string
	Points []Point
}
type RadarPlacement struct {
	Center          Point
	Radius, Maximum float64
	Axes            []string
	Series          []ChartSeries
}
type FlowPlacement struct {
	Edge         *Edge
	From, To     Point
	Width, Value float64
}
