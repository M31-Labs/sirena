package mermaid

import (
	gt "github.com/odvcencio/gotreesitter"
	"m31labs.dev/sirena"
)

// lowerer carries all state needed to walk the Mermaid CST and produce a
// *sirena.Document. It is constructed by Parse and used for a single Parse
// call; it is not safe for concurrent use.
type lowerer struct {
	lang  *gt.Language
	src   []byte // normalized source the CST was parsed from
	smap  srcMap // clean-offset → original-offset remap (Task A2)
	opts  Options
	diags []sirena.Diagnostic
	fatal error

	// element dedup: source-order slice + fast-lookup map
	elemOrder []string
	elemMap   map[string]*sirena.Element
}

// lowerRoot walks the CST root and returns a *sirena.Document.
// Phase B/C populate sys.Elements / sys.Edges.
// Phase D adds sys.Boundaries for subgraphs.
// Phase E adds diagnostic emission for NOT-A-GRAPH, PARSE, and UNSUPPORTED.
func (l *lowerer) lowerRoot(root *gt.Node) *sirena.Document {
	l.elemOrder = nil
	l.elemMap = map[string]*sirena.Element{}

	// Phase E: detect non-flowchart diagram types.
	// Scan ALL named children of root for diagram_flow; a leading directive or
	// comment can occupy earlier slots so we must not key on just the first child.
	df := checkForDiagramFlow(root, l.lang)
	if df == nil {
		d := l.mkDiag(
			"SIR-MERMAID-NOT-A-GRAPH",
			sirena.SeverityError,
			"Input is not a Mermaid flowchart (no diagram_flow node found)",
			root,
		)
		l.diags = append(l.diags, d)
		l.fatal = notAGraphError
		return nil
	}

	// Phase E: check for root-level parse errors (broken statements).
	l.checkParseErrors(root)

	sys := &sirena.SystemDecl{}

	// Walk children of diagram_flow: vertices, subgraphs, and unsupported stmts.
	for j := 0; j < df.NamedChildCount(); j++ {
		stmt := df.NamedChild(j)
		switch stmt.Type(l.lang) {
		case "flow_stmt_vertice":
			l.lowerStmt(stmt, sys)
		case "flow_stmt_subgraph":
			b := l.lowerSubgraph(stmt, sys)
			if b != nil {
				sys.Boundaries = append(sys.Boundaries, b)
			}
		default:
			// Phase E: emit UNSUPPORTED for any other flow_stmt_* type.
			l.checkUnsupportedStmt(stmt)
		}
	}

	// Append top-level elements in source declaration order.
	// Elements that were registered as part of a boundary are tracked in
	// elemMap for dedup (cross-boundary edge references) but are NOT
	// appended to sys.Elements — they live in the boundary's Children.
	for _, name := range l.elemOrder {
		sys.Elements = append(sys.Elements, l.elemMap[name])
	}

	return &sirena.Document{Systems: []*sirena.SystemDecl{sys}}
}

// lowerSubgraph lowers a flow_stmt_subgraph node to a *sirena.Boundary.
// CST shape:
//
//	flow_stmt_subgraph
//	  subgraph            (keyword, not named)
//	  flow_vertex_id      (boundary id)
//	  [                   (optional label bracket, not named)
//	  flow_vertex_text    (optional label text)
//	  ]
//	  flow_stmt_subgraph_inner
//	    flow_stmt_vertice*
//	    flow_stmt_subgraph*  (nested subgraphs)
//	  end
//
// Inner vertices become boundary Children (as *sirena.Element).
// Inner edges go to sys.Edges regardless of containment.
// Nested subgraphs recurse and become child *sirena.Boundary entries.
func (l *lowerer) lowerSubgraph(node *gt.Node, sys *sirena.SystemDecl) *sirena.Boundary {
	// Extract boundary id and optional label.
	// labeled:   subgraph id[Label]  → flow_vertex_id is the id; flow_vertex_text is the label
	// unlabeled: subgraph id         → no flow_vertex_id; flow_vertex_text IS the id (no label)
	idNode := namedChildOfType(node, l.lang, "flow_vertex_id")
	var id, label string
	if idNode != nil {
		id = string(l.src[idNode.StartByte():idNode.EndByte()])
		if labelNode := namedChildOfType(node, l.lang, "flow_vertex_text"); labelNode != nil {
			label = string(l.src[labelNode.StartByte():labelNode.EndByte()])
		}
	} else if txtNode := namedChildOfType(node, l.lang, "flow_vertex_text"); txtNode != nil {
		id = string(l.src[txtNode.StartByte():txtNode.EndByte()])
	}
	if id == "" {
		return nil
	}

	b := &sirena.Boundary{
		Kind:     sirena.BoundaryKindGroup,
		Name:     id,
		Metadata: map[string]sirena.Value{},
	}
	if label != "" {
		b.Metadata["label"] = sirena.String{Value: label}
	}

	// Walk the inner block.
	inner := namedChildOfType(node, l.lang, "flow_stmt_subgraph_inner")
	if inner != nil {
		for k := 0; k < inner.NamedChildCount(); k++ {
			child := inner.NamedChild(k)
			switch child.Type(l.lang) {
			case "flow_stmt_vertice":
				// Lower the statement; collect any newly registered elements
				// into the boundary's Children rather than sys.Elements.
				beforeOrder := append([]string(nil), l.elemOrder...)
				l.lowerStmt(child, sys)
				// Any element added after beforeOrder is a new child of this boundary.
				for _, name := range l.elemOrder[len(beforeOrder):] {
					b.Children = append(b.Children, l.elemMap[name])
				}
				// Remove new elements from elemOrder so they don't land in sys.Elements.
				l.elemOrder = beforeOrder
			case "flow_stmt_subgraph":
				nested := l.lowerSubgraph(child, sys)
				if nested != nil {
					b.Children = append(b.Children, nested)
				}
			}
		}
	}

	return b
}

// lowerStmt walks one flow_stmt_vertice, which has an alternating sequence of
// flow_node and link-node children. A chained statement like a→b→c produces
// edges a→b and b→c. Vertices first seen in a statement are registered as
// Elements.
func (l *lowerer) lowerStmt(stmt *gt.Node, sys *sirena.SystemDecl) {
	// Collect named children; they alternate: flow_node, link, flow_node, link, …
	count := stmt.NamedChildCount()
	if count == 0 {
		return
	}

	// prevID holds the From-side vertex id as we walk the chain.
	prevID := ""

	for i := 0; i < count; i++ {
		child := stmt.NamedChild(i)
		childType := child.Type(l.lang)

		switch childType {
		case "flow_node":
			id := l.registerNode(child)
			prevID = id

		case "flow_link_simplelink", "flow_link_arrowtext":
			// A link must be followed by a flow_node. Look ahead.
			if i+1 >= count {
				break
			}
			toNode := stmt.NamedChild(i + 1)
			if toNode.Type(l.lang) != "flow_node" {
				break
			}
			toID := l.registerNode(toNode)
			label := l.arrowLabel(child)

			e := &sirena.Edge{
				From:      prevID,
				To:        toID,
				FromRef:   &sirena.Ref{Name: prevID},
				ToRef:     &sirena.Ref{Name: toID},
				Kind:      sirena.EdgeKindFlow,
				Direction: sirena.DirForward,
			}
			if label != "" {
				e.Label = label
			}
			sys.Edges = append(sys.Edges, e)

			// Advance past the consumed toNode; the loop increment will step i
			// to the next child after toNode.
			prevID = toID
			i++ // skip the toNode; already processed
		}
	}
}

// registerNode extracts the vertex id from a flow_node, registers the
// corresponding Element (deduplicating by name), and returns the id.
func (l *lowerer) registerNode(node *gt.Node) string {
	// flow_node → flow_vertex → flow_vertex_id (+ optional shape child)
	vertex := namedChildOfType(node, l.lang, "flow_vertex")
	if vertex == nil {
		return ""
	}
	idNode := namedChildOfType(vertex, l.lang, "flow_vertex_id")
	if idNode == nil {
		return ""
	}
	id := string(l.src[idNode.StartByte():idNode.EndByte()])

	// If already registered, return existing id (first declaration wins).
	if _, exists := l.elemMap[id]; exists {
		return id
	}

	el := &sirena.Element{
		Kind:     sirena.ElementKindNode,
		Name:     id,
		Metadata: map[string]sirena.Value{},
	}

	// Look for a shape child (any named child that is not flow_vertex_id).
	label, shapeStr := l.extractLabelAndShape(vertex)
	if label != "" {
		el.Metadata["label"] = sirena.String{Value: label}
	}
	if shapeStr != "" {
		el.Metadata["shape"] = sirena.Ident{Value: shapeStr}
	}

	l.elemOrder = append(l.elemOrder, id)
	l.elemMap[id] = el
	return id
}

// extractLabelAndShape returns the display label and shape string for a
// flow_vertex node. It looks for the first named child that is not
// flow_vertex_id (the shape node), then finds the label text within it.
func (l *lowerer) extractLabelAndShape(vertex *gt.Node) (label, shape string) {
	for i := 0; i < vertex.NamedChildCount(); i++ {
		c := vertex.NamedChild(i)
		nodeType := c.Type(l.lang)
		if nodeType == "flow_vertex_id" {
			continue
		}
		// This is the shape node.
		shape = shapeFor(nodeType)

		// Extract label text from flow_text_literal or flow_vertex_text children.
		label = l.extractText(c)
		return label, shape
	}
	return "", ""
}

// extractText finds the text content of a shape node. The label lives in a
// flow_text_literal or flow_vertex_text named child.
func (l *lowerer) extractText(shapeNode *gt.Node) string {
	for i := 0; i < shapeNode.NamedChildCount(); i++ {
		c := shapeNode.NamedChild(i)
		t := c.Type(l.lang)
		if t == "flow_text_literal" || t == "flow_vertex_text" {
			return string(l.src[c.StartByte():c.EndByte()])
		}
	}
	return ""
}

// arrowLabel extracts the label text from a flow_link_arrowtext node.
// Returns "" for flow_link_simplelink (no label).
func (l *lowerer) arrowLabel(linkNode *gt.Node) string {
	if linkNode.Type(l.lang) != "flow_link_arrowtext" {
		return ""
	}
	for i := 0; i < linkNode.NamedChildCount(); i++ {
		c := linkNode.NamedChild(i)
		if c.Type(l.lang) == "flow_arrow_text" {
			return string(l.src[c.StartByte():c.EndByte()])
		}
	}
	return ""
}

// namedChildOfType returns the first named child of n whose Type equals
// nodeType, or nil if none is found.
func namedChildOfType(n *gt.Node, lang *gt.Language, nodeType string) *gt.Node {
	for i := 0; i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if c.Type(lang) == nodeType {
			return c
		}
	}
	return nil
}
