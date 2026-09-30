package sirena

// LifelinePlacement is an actor's vertical time axis in a sequence diagram.
type LifelinePlacement struct {
	Actor    *Element
	From, To Point
}

func ValidDiagram(name string) bool {
	return name == "architecture" || name == "sequence" || name == "radial"
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
