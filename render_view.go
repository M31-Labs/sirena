package sirena

// AllElementsView builds a ResolvedView containing every element,
// boundary, and edge in a document — the implicit "render everything"
// view for a system file with no explicit view declaration. Nested
// elements and boundaries are flattened into the Elements/Boundaries
// slices (matching EvaluateView's shape) so the layout engine can
// reconstruct containment. Pointers come straight from the document and
// must be treated as read-only.
func AllElementsView(doc *Document) *ResolvedView {
	rv := &ResolvedView{Source: &ViewDecl{Name: "all"}}
	if doc == nil {
		return rv
	}
	var addBoundary func(b *Boundary)
	addBoundary = func(b *Boundary) {
		rv.Boundaries = append(rv.Boundaries, b)
		for _, c := range b.Children {
			switch n := c.(type) {
			case *Element:
				rv.Elements = append(rv.Elements, n)
			case *Boundary:
				addBoundary(n)
			case *Edge:
				rv.Edges = append(rv.Edges, n)
			}
		}
	}
	for _, sys := range doc.Systems {
		rv.Elements = append(rv.Elements, sys.Elements...)
		for _, b := range sys.Boundaries {
			addBoundary(b)
		}
		rv.Edges = append(rv.Edges, sys.Edges...)
	}
	return rv
}
