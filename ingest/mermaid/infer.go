package mermaid

import "m31labs.dev/sirena"

// edgeKindKeywords maps lowercase edge-label strings to their sirena EdgeKind.
// Only unambiguous, exact matches are included. Labels that don't appear here
// remain EdgeKindFlow.
var edgeKindKeywords = map[string]sirena.EdgeKind{
	"calls":      sirena.EdgeKindCalls,
	"reads":      sirena.EdgeKindReads,
	"writes":     sirena.EdgeKindWrites,
	"publishes":  sirena.EdgeKindPublishes,
	"subscribes": sirena.EdgeKindSubscribes,
	"depends_on": sirena.EdgeKindDependsOn,
}

// applyInference promotes unambiguous Mermaid constructs to typed sirena kinds.
// It mutates doc in place. Shape metadata is preserved in all cases.
//
// Promotions applied:
//   - Element with Metadata["shape"] == "cylinder" → ElementKindDatabase.
//     All other shapes remain ElementKindNode (ambiguous).
//   - Edge whose Label exactly matches an edgeKindKeywords entry → that EdgeKind.
//     Non-matching labels remain EdgeKindFlow; the Label string is kept.
func applyInference(doc *sirena.Document) {
	for _, sys := range doc.Systems {
		// Promote elements.
		for _, el := range sys.Elements {
			promoteElement(el)
		}
		// Promote elements nested in boundaries (recursive).
		for _, b := range sys.Boundaries {
			inferBoundary(b)
		}
		// Promote edges.
		for _, e := range sys.Edges {
			promoteEdge(e)
		}
	}
}

// inferBoundary walks a boundary tree, promoting elements and edges within it.
func inferBoundary(b *sirena.Boundary) {
	for _, child := range b.Children {
		switch n := child.(type) {
		case *sirena.Element:
			promoteElement(n)
		case *sirena.Boundary:
			inferBoundary(n)
		case *sirena.Edge:
			promoteEdge(n)
		}
	}
}

// promoteElement promotes an element to a typed kind when its shape is
// unambiguously associated with one sirena ElementKind. Shape metadata is
// preserved regardless of whether a promotion occurs.
func promoteElement(el *sirena.Element) {
	shapeVal, ok := el.Metadata["shape"]
	if !ok {
		return
	}
	ident, ok := shapeVal.(sirena.Ident)
	if !ok {
		return
	}
	switch ident.Value {
	case "cylinder":
		el.Kind = sirena.ElementKindDatabase
		// All other shapes (rect, round, diamond, etc.) are ambiguous; no promotion.
	}
}

// promoteEdge promotes an edge to a typed EdgeKind when its Label exactly
// matches a known sirena edge-kind keyword. The Label string is preserved.
func promoteEdge(e *sirena.Edge) {
	if e.Label == "" {
		return
	}
	if kind, ok := edgeKindKeywords[e.Label]; ok {
		e.Kind = kind
	}
}
