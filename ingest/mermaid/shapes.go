package mermaid

// shapeFor maps a flow_vertex_* CST node type to the canonical shape string
// stored in Element.Metadata["shape"]. Returns "" for unrecognized types,
// which means the shape key is omitted from Metadata entirely.
//
// The set of recognized node types is enumerated from the Mermaid grammar
// embedded in gotreesitter (grammars package):
//
//	flow_vertex_circle, cylinder, diamond, ellipse, hexagon, id,
//	inv_trapezoid, leanleft, leanright, odd, rect, round, square,
//	stadium, subroutine, trapezoid
//
// flow_vertex_id is the plain-identifier node (no shape decoration); it is
// intentionally not listed here — shape is only set when a shape node child
// is present.
func shapeFor(nodeType string) string {
	switch nodeType {
	case "flow_vertex_square":
		return "rect"
	case "flow_vertex_rect":
		return "rect"
	case "flow_vertex_round":
		return "round"
	case "flow_vertex_stadium":
		return "stadium"
	case "flow_vertex_subroutine":
		return "subroutine"
	case "flow_vertex_cylinder":
		return "cylinder"
	case "flow_vertex_circle":
		return "circle"
	case "flow_vertex_ellipse":
		return "ellipse"
	case "flow_vertex_diamond":
		return "diamond"
	case "flow_vertex_hexagon":
		return "hexagon"
	case "flow_vertex_odd":
		return "odd"
	case "flow_vertex_trapezoid":
		return "trapezoid"
	case "flow_vertex_inv_trapezoid":
		return "inv_trapezoid"
	case "flow_vertex_leanleft":
		return "leanleft"
	case "flow_vertex_leanright":
		return "leanright"
	default:
		return ""
	}
}
