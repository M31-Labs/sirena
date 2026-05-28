package lint

import (
	"fmt"

	"m31labs.dev/sirena"
)

// QueueWithoutPublisher returns SIR-KIND-QUEUE-NO-PUBLISHER for every
// queue element with no incoming edge of kind publishes (or writes).
// Queues exist to be written to; one without a publisher is almost
// certainly a stub or a typo. The check uses workspace-wide edge
// counting via collectEdgeCounts so an edge declared in a separate file
// counts the same as one in the queue's own file.
//
// A nil workspace returns nil.
func QueueWithoutPublisher(ws *sirena.Workspace) []sirena.Diagnostic {
	if ws == nil {
		return nil
	}
	counts := collectEdgeCounts(ws)
	var diags []sirena.Diagnostic
	for _, el := range allElements(ws) {
		if el == nil || el.Kind != sirena.ElementKindQueue {
			continue
		}
		c := counts[el.Name]
		if !c.hasPublisher() {
			diags = append(diags, sirena.Diagnostic{
				Code:     "SIR-KIND-QUEUE-NO-PUBLISHER",
				Severity: sirena.SeverityWarning,
				Message:  fmt.Sprintf("queue %q has no publisher (no incoming publishes/writes edge)", el.Name),
				Range:    el.Range,
			})
		}
	}
	return diags
}

// QueueWithoutSubscriber returns SIR-KIND-QUEUE-NO-SUBSCRIBER for every
// queue element with no outgoing edge of kind subscribes (or reads).
// Symmetric companion to QueueWithoutPublisher.
func QueueWithoutSubscriber(ws *sirena.Workspace) []sirena.Diagnostic {
	if ws == nil {
		return nil
	}
	counts := collectEdgeCounts(ws)
	var diags []sirena.Diagnostic
	for _, el := range allElements(ws) {
		if el == nil || el.Kind != sirena.ElementKindQueue {
			continue
		}
		c := counts[el.Name]
		if !c.hasSubscriber() {
			diags = append(diags, sirena.Diagnostic{
				Code:     "SIR-KIND-QUEUE-NO-SUBSCRIBER",
				Severity: sirena.SeverityWarning,
				Message:  fmt.Sprintf("queue %q has no subscriber (no outgoing subscribes/reads edge)", el.Name),
				Range:    el.Range,
			})
		}
	}
	return diags
}

// DeadElement returns SIR-REF-DEAD-ELEMENT for every element with zero
// incoming AND zero outgoing edges across the workspace. A standalone
// element may be intentional (e.g. a placeholder for an external system
// the model hasn't wired up yet), but in practice it's almost always a
// drafting mistake — the lint pack flags it as a warning so the author
// either wires it up or removes it.
func DeadElement(ws *sirena.Workspace) []sirena.Diagnostic {
	if ws == nil {
		return nil
	}
	counts := collectEdgeCounts(ws)
	var diags []sirena.Diagnostic
	for _, el := range allElements(ws) {
		if el == nil {
			continue
		}
		c := counts[el.Name]
		if c.incoming == 0 && c.outgoing == 0 {
			diags = append(diags, sirena.Diagnostic{
				Code:     "SIR-REF-DEAD-ELEMENT",
				Severity: sirena.SeverityWarning,
				Message:  fmt.Sprintf("element %q has no incoming or outgoing edges", el.Name),
				Range:    el.Range,
			})
		}
	}
	return diags
}

// edgeCount aggregates per-element edge counts split by direction and
// kind. The fields are tracked separately so the queue-specific rules
// can check "has any publishes/writes incoming?" without re-walking
// every edge per rule.
type edgeCount struct {
	incoming           int
	outgoing           int
	incomingPublishes  int // publishes OR writes
	outgoingSubscribes int // subscribes OR reads
}

func (c edgeCount) hasPublisher() bool  { return c.incomingPublishes > 0 }
func (c edgeCount) hasSubscriber() bool { return c.outgoingSubscribes > 0 }

// collectEdgeCounts walks every edge in every system file in the
// workspace and tallies incoming/outgoing counts per element name. Names
// that don't appear as any edge endpoint simply don't show up in the
// result map; callers fall through to the zero-value edgeCount.
//
// We index by bare name rather than by *Element pointer so an edge whose
// endpoint name resolves to an element declared in a different file
// still counts toward that element. The resolver populates Edge.FromRef
// and Edge.ToRef with the declaration pointer at ResolveWorkspace time,
// but the lint pack doesn't require resolution to have happened —
// keeping the count keyed by name lets these rules run on a workspace
// before or after Resolve.
func collectEdgeCounts(ws *sirena.Workspace) map[string]edgeCount {
	out := map[string]edgeCount{}
	if ws == nil {
		return out
	}
	for _, f := range ws.Files {
		if f.Kind == sirena.FileKindView {
			continue
		}
		doc, err := ws.LoadDocument(f)
		if err != nil || doc == nil {
			continue
		}
		for _, e := range allDocEdges(doc) {
			if e == nil {
				continue
			}
			fromName := bareName(e.From)
			toName := bareName(e.To)
			fromCount := out[fromName]
			toCount := out[toName]
			fromCount.outgoing++
			toCount.incoming++
			if e.Kind == sirena.EdgeKindPublishes || e.Kind == sirena.EdgeKindWrites {
				toCount.incomingPublishes++
			}
			if e.Kind == sirena.EdgeKindSubscribes || e.Kind == sirena.EdgeKindReads {
				fromCount.outgoingSubscribes++
			}
			out[fromName] = fromCount
			out[toName] = toCount
		}
	}
	return out
}

// bareName strips an optional namespace prefix ("platform.kafka" →
// "kafka") so edge endpoints reference the same element-count bucket as
// the element's own declaration, which always lands keyed by bare name.
func bareName(ref string) string {
	for i := len(ref) - 1; i >= 0; i-- {
		if ref[i] == '.' {
			return ref[i+1:]
		}
	}
	return ref
}

// allElements returns every Element declared in every system file in the
// workspace (top-level and nested inside boundaries). Views are skipped:
// they're projections of declarations that already appear in system
// files.
func allElements(ws *sirena.Workspace) []*sirena.Element {
	var out []*sirena.Element
	if ws == nil {
		return out
	}
	for _, f := range ws.Files {
		if f.Kind == sirena.FileKindView {
			continue
		}
		doc, err := ws.LoadDocument(f)
		if err != nil || doc == nil {
			continue
		}
		for _, sys := range doc.Systems {
			if sys == nil {
				continue
			}
			out = append(out, sys.Elements...)
			for _, b := range sys.Boundaries {
				out = append(out, collectBoundaryElements(b)...)
			}
		}
	}
	return out
}

// collectBoundaryElements recurses into b's children and returns every
// Element declared inside, at any depth. Mirrors the boundary walk shape
// used by resolve.go so nested elements participate in lint just like
// top-level ones.
func collectBoundaryElements(b *sirena.Boundary) []*sirena.Element {
	var out []*sirena.Element
	if b == nil {
		return out
	}
	for _, c := range b.Children {
		switch n := c.(type) {
		case *sirena.Element:
			out = append(out, n)
		case *sirena.Boundary:
			out = append(out, collectBoundaryElements(n)...)
		}
	}
	return out
}

// allDocEdges returns every Edge in doc, including those nested inside
// boundaries. Stand-in for the unexported allEdges helper in resolve.go;
// kept here so the lint package doesn't take a dependency on internal
// symbols of the root package.
func allDocEdges(doc *sirena.Document) []*sirena.Edge {
	var out []*sirena.Edge
	if doc == nil {
		return out
	}
	for _, sys := range doc.Systems {
		if sys == nil {
			continue
		}
		out = append(out, sys.Edges...)
		for _, b := range sys.Boundaries {
			out = append(out, collectBoundaryEdges(b)...)
		}
	}
	return out
}

// collectBoundaryEdges recurses through b's children and returns every
// edge declared inside, at any depth.
func collectBoundaryEdges(b *sirena.Boundary) []*sirena.Edge {
	var out []*sirena.Edge
	if b == nil {
		return out
	}
	for _, c := range b.Children {
		switch n := c.(type) {
		case *sirena.Edge:
			out = append(out, n)
		case *sirena.Boundary:
			out = append(out, collectBoundaryEdges(n)...)
		}
	}
	return out
}
