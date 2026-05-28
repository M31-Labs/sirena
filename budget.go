// Package sirena: budget evaluation.
//
// budget.go runs the budget evaluator: post-view-selection and pre-
// layout, it counts what would render and flags every declared cap that
// the projection overflows. The evaluator returns a *BudgetReport with
// one Breach per overflowing field plus a small set of Suggestion hints
// the renderer (or LSP, or mdpp) can surface to the author.
//
// Design notes:
//
//   - A nil view, a view without a budget block, or a view whose every
//     cap holds collapses to a nil report. Callers treat "no report" as
//     "no breaches" so the no-budget path stays branch-free downstream.
//
//   - Suggestions are best-effort hints, not authoritative. The heuristics
//     are intentionally simple:
//
//   - Nodes breach → CollapseBoundary targeting the largest included
//     boundary; if no boundaries exist, fall back to SplitView.
//
//   - Nodes breach > 3x the cap → also emit RaiseZoomLevel as a
//     "this view is too big" fallback hint pending LOD support.
//
//   - EdgesPerNode breach → DropEdgeKind targeting the highest-degree
//     node, with the rationale naming the lowest-priority kind among
//     that node's edges (priority: depends_on < flow < subscribes <
//     publishes < reads < writes < calls).
//
//   - Per-node edge degree is computed over rv.Edges only, matching what
//     the renderer will actually draw. Endpoints are matched by name
//     against Element.Name; Edge.From / Edge.To carry the verbatim
//     identifier so a namespaced "platform.kafka" matches an included
//     element by bare name "kafka" only when no namespace is set.
//
//   - Depth is the maximum boundary nesting depth observed in rv.Boundaries
//     (using each boundary's Children to recurse). BoundaryFanout is the
//     max immediate-child count across the included boundaries.
//
//   - LabelChars is the longest rendered label: Metadata["label"] String
//     when set, else the declared Name.
package sirena

// BudgetReport is the result of evaluating a view against its declared
// budget. Breaches names what's over-quota; Suggestions offers actionable
// curation recommendations. The renderer's strict-mode returns this and
// refuses to lay out; permissive mode returns it alongside the
// LayoutResult so callers can surface warnings.
type BudgetReport struct {
	Breaches    []Breach
	Suggestions []Suggestion
}

// Breach is one budget overflow. Field names the violated cap; Limit is
// the declared value and Actual is the observed count. Target points at
// the element or boundary that triggered the breach; for whole-view
// breaches (nodes, depth) Target is the zero Ref.
type Breach struct {
	Field  string // "nodes", "edges_per_node", "depth", "boundary_fanout", "label_chars"
	Limit  int
	Actual int
	Target Ref
}

// Suggestion is one actionable curation hint. Kind discriminates the
// move the author is being nudged toward; Target points at the element
// or boundary the move applies to; Estimate quantifies the expected
// reduction; Rationale is a short human-readable explanation.
type Suggestion struct {
	Kind      SuggestionKind
	Target    Ref
	Estimate  SuggestionEstimate
	Rationale string
}

// SuggestionKind enumerates the curation moves the evaluator can
// recommend. The zero value SuggestionUnknown is reserved so misuse
// surfaces obviously.
type SuggestionKind int

const (
	// SuggestionUnknown is the zero value; not a valid suggestion kind.
	SuggestionUnknown SuggestionKind = iota
	// SuggestionCollapseBoundary recommends collapsing a specific boundary
	// into a single summary node — the cheapest move for shrinking a
	// projection without losing the boundary's group identity.
	SuggestionCollapseBoundary
	// SuggestionDropEdgeKind recommends dropping every edge of a given
	// kind from the view. The Rationale field names the kind.
	SuggestionDropEdgeKind
	// SuggestionSplitView recommends splitting the view into two — for
	// severe nodes breaches or views with no boundaries to collapse.
	SuggestionSplitView
	// SuggestionRaiseZoomLevel is a placeholder for LOD-aware views: when
	// the projection exceeds 3x its node cap, the evaluator emits this as
	// a "this view is too big; consider a higher-level summary" hint.
	SuggestionRaiseZoomLevel
)

// String returns the canonical short label for this suggestion kind.
// Used by diagnostics and downstream UIs (mdpp, LSP) that want a stable
// machine-readable handle for the hint.
func (k SuggestionKind) String() string {
	switch k {
	case SuggestionCollapseBoundary:
		return "collapse-boundary"
	case SuggestionDropEdgeKind:
		return "drop-edge-kind"
	case SuggestionSplitView:
		return "split-view"
	case SuggestionRaiseZoomLevel:
		return "raise-zoom-level"
	default:
		return "unknown"
	}
}

// SuggestionEstimate quantifies the expected reduction a suggestion
// would buy. v0.1 fills in NodesReclaimed (children of the collapse
// target, halves of a split) and EdgesReclaimed (count of edges that
// would be dropped). The fields are advisory; the renderer may ignore
// them.
type SuggestionEstimate struct {
	NodesReclaimed int
	EdgesReclaimed int
}

// EvaluateBudget runs the budget evaluator against a resolved view.
// Returns nil if no budget is declared OR no breaches are observed;
// otherwise returns a populated *BudgetReport. The evaluator runs
// post-view-selection and pre-layout — callers typically invoke it
// through Render.
func EvaluateBudget(rv *ResolvedView) *BudgetReport {
	if rv == nil || rv.Source == nil || rv.Source.Budget == nil {
		return nil
	}
	b := rv.Source.Budget

	report := &BudgetReport{}
	totalNodes := len(rv.Elements) + len(rv.Boundaries) + len(rv.Summaries)

	// Nodes cap. Whole-view breach; Target stays zero.
	if b.Nodes > 0 && totalNodes > b.Nodes {
		report.Breaches = append(report.Breaches, Breach{
			Field:  "nodes",
			Limit:  b.Nodes,
			Actual: totalNodes,
		})
		appendNodesSuggestions(report, rv, b.Nodes, totalNodes)
	}

	// EdgesPerNode cap. We compute degree across rv.Edges and report
	// the worst offender (the cap is per-node so naming every node over
	// would create noise on a hub-and-spoke topology).
	if b.EdgesPerNode > 0 {
		if worst, degree, kinds := worstNodeDegree(rv); degree > b.EdgesPerNode {
			report.Breaches = append(report.Breaches, Breach{
				Field:  "edges_per_node",
				Limit:  b.EdgesPerNode,
				Actual: degree,
				Target: refForElement(worst),
			})
			appendEdgesPerNodeSuggestion(report, worst, kinds)
		}
	}

	// Depth cap. Whole-view breach.
	if b.Depth > 0 {
		if d := maxBoundaryDepth(rv.Boundaries); d > b.Depth {
			report.Breaches = append(report.Breaches, Breach{
				Field:  "depth",
				Limit:  b.Depth,
				Actual: d,
			})
		}
	}

	// BoundaryFanout cap. Per-boundary breach; Target names the worst.
	if b.BoundaryFanout > 0 {
		if worst, fanout := worstBoundaryFanout(rv.Boundaries); fanout > b.BoundaryFanout {
			report.Breaches = append(report.Breaches, Breach{
				Field:  "boundary_fanout",
				Limit:  b.BoundaryFanout,
				Actual: fanout,
				Target: refForBoundary(worst),
			})
		}
	}

	// LabelChars cap. Per-node breach; Target names the longest label.
	if b.LabelChars > 0 {
		if name, length, kind := longestLabel(rv); length > b.LabelChars {
			breach := Breach{
				Field:  "label_chars",
				Limit:  b.LabelChars,
				Actual: length,
				Target: Ref{Name: name},
			}
			_ = kind // kept for future use; the Ref shape doesn't carry kind
			report.Breaches = append(report.Breaches, breach)
		}
	}

	if len(report.Breaches) == 0 {
		return nil
	}
	return report
}

// appendNodesSuggestions emits curation hints for a nodes-cap breach.
// The primary recommendation is collapsing the largest included
// boundary; if no boundaries exist, fall back to SplitView. Severe
// breaches (>3x cap) additionally emit RaiseZoomLevel as a "this view
// is too big" fallback.
func appendNodesSuggestions(report *BudgetReport, rv *ResolvedView, limit, actual int) {
	if largest := largestBoundary(rv.Boundaries); largest != nil {
		report.Suggestions = append(report.Suggestions, Suggestion{
			Kind:   SuggestionCollapseBoundary,
			Target: refForBoundary(largest),
			Estimate: SuggestionEstimate{
				NodesReclaimed: len(largest.Children),
			},
			Rationale: "collapsing the largest included boundary is the cheapest way to bring the projection back under its node cap",
		})
	} else {
		// No boundaries to collapse — recommend splitting the view.
		report.Suggestions = append(report.Suggestions, Suggestion{
			Kind: SuggestionSplitView,
			Estimate: SuggestionEstimate{
				NodesReclaimed: actual / 2,
			},
			Rationale: "no boundaries are included; splitting the view in half is the simplest reduction",
		})
	}
	if actual > 3*limit {
		report.Suggestions = append(report.Suggestions, Suggestion{
			Kind:      SuggestionRaiseZoomLevel,
			Rationale: "view exceeds 3x node cap; consider a higher-level summary view",
		})
	}
}

// appendEdgesPerNodeSuggestion emits a DropEdgeKind hint for an edges-
// per-node breach. The recommended kind is the lowest-priority edge
// kind among the worst node's edges (priority order documented at the
// edgeKindPriority helper).
func appendEdgesPerNodeSuggestion(report *BudgetReport, worst *Element, kinds map[EdgeKind]int) {
	if len(kinds) == 0 {
		return
	}
	lowest := EdgeKindUnknown
	lowestPriority := -1
	for k := range kinds {
		p := edgeKindPriority(k)
		if lowest == EdgeKindUnknown || p < lowestPriority {
			lowest = k
			lowestPriority = p
		}
	}
	report.Suggestions = append(report.Suggestions, Suggestion{
		Kind:   SuggestionDropEdgeKind,
		Target: refForElement(worst),
		Estimate: SuggestionEstimate{
			EdgesReclaimed: kinds[lowest],
		},
		Rationale: "drop edges of kind \"" + lowest.String() + "\" — lowest signal among this node's edges",
	})
}

// edgeKindPriority returns a stable priority for an edge kind: lower is
// less important and therefore the better drop candidate. The ordering
// reflects v0.1's intuition that depends_on edges carry the least visual
// signal at the system level while calls edges carry the most. The
// numbers are arbitrary but ordered.
func edgeKindPriority(k EdgeKind) int {
	switch k {
	case EdgeKindDependsOn:
		return 0
	case EdgeKindFlow:
		return 1
	case EdgeKindSubscribes:
		return 2
	case EdgeKindPublishes:
		return 3
	case EdgeKindReads:
		return 4
	case EdgeKindWrites:
		return 5
	case EdgeKindCalls:
		return 6
	default:
		return 7
	}
}

// worstNodeDegree returns the included element with the highest edge
// degree across rv.Edges, its degree count, and the breakdown by edge
// kind. Endpoints are matched by name against Element.Name; namespaced
// edge endpoints match only when the namespace is empty (the resolver
// would have already pointed FromRef/ToRef.Definition at the right
// element, but EvaluateView does not call ResolveWorkspace, so we keep
// the matching simple and explicit).
func worstNodeDegree(rv *ResolvedView) (*Element, int, map[EdgeKind]int) {
	if len(rv.Edges) == 0 || len(rv.Elements) == 0 {
		return nil, 0, nil
	}
	degree := map[string]int{}
	byKind := map[string]map[EdgeKind]int{}
	for _, e := range rv.Edges {
		from := edgeEndpointName(e.From)
		to := edgeEndpointName(e.To)
		if from != "" {
			degree[from]++
			if byKind[from] == nil {
				byKind[from] = map[EdgeKind]int{}
			}
			byKind[from][e.Kind]++
		}
		if to != "" && to != from {
			degree[to]++
			if byKind[to] == nil {
				byKind[to] = map[EdgeKind]int{}
			}
			byKind[to][e.Kind]++
		}
	}
	var worst *Element
	worstCount := 0
	for _, el := range rv.Elements {
		if c := degree[el.Name]; c > worstCount {
			worst = el
			worstCount = c
		}
	}
	if worst == nil {
		return nil, 0, nil
	}
	return worst, worstCount, byKind[worst.Name]
}

// edgeEndpointName strips an optional namespace prefix from an edge
// endpoint identifier and returns the bare name. "platform.kafka" → ""
// (namespaced endpoints don't match bare element names by name alone);
// "kafka" → "kafka".
func edgeEndpointName(endpoint string) string {
	for i := 0; i < len(endpoint); i++ {
		if endpoint[i] == '.' {
			return ""
		}
	}
	return endpoint
}

// maxBoundaryDepth returns the maximum nesting depth of the included
// boundaries. A flat boundary contributes depth 1; a boundary with a
// nested boundary contributes depth 2; and so on.
func maxBoundaryDepth(bs []*Boundary) int {
	max := 0
	for _, b := range bs {
		if d := boundaryDepth(b); d > max {
			max = d
		}
	}
	return max
}

// boundaryDepth recurses into b.Children and returns 1 + the deepest
// nested boundary depth. A boundary with no nested boundaries returns 1.
func boundaryDepth(b *Boundary) int {
	if b == nil {
		return 0
	}
	deepest := 0
	for _, c := range b.Children {
		if nested, ok := c.(*Boundary); ok {
			if d := boundaryDepth(nested); d > deepest {
				deepest = d
			}
		}
	}
	return 1 + deepest
}

// worstBoundaryFanout returns the included boundary with the most
// immediate children and its child count. Nested boundaries' children
// are not counted toward the parent — fanout is a per-boundary metric.
func worstBoundaryFanout(bs []*Boundary) (*Boundary, int) {
	var worst *Boundary
	worstCount := 0
	for _, b := range bs {
		if b == nil {
			continue
		}
		if c := len(b.Children); c > worstCount {
			worst = b
			worstCount = c
		}
	}
	return worst, worstCount
}

// largestBoundary returns the included boundary with the most direct
// children — the best candidate for a CollapseBoundary suggestion since
// collapsing it reclaims the most node slots. Nil when no boundaries
// are included.
func largestBoundary(bs []*Boundary) *Boundary {
	var largest *Boundary
	largestCount := -1
	for _, b := range bs {
		if b == nil {
			continue
		}
		if c := len(b.Children); c > largestCount {
			largest = b
			largestCount = c
		}
	}
	return largest
}

// longestLabel scans every included element and boundary, returning the
// longest rendered label, its length, and the source kind ("element" or
// "boundary"). Element/boundary label comes from Metadata["label"]
// String when set, else the declared Name.
func longestLabel(rv *ResolvedView) (string, int, string) {
	var winner string
	winnerLen := 0
	var kind string
	for _, e := range rv.Elements {
		lbl := elementLabel(e)
		if n := len(lbl); n > winnerLen {
			winner, winnerLen, kind = e.Name, n, "element"
		}
	}
	for _, b := range rv.Boundaries {
		lbl := boundaryLabel(b)
		if n := len(lbl); n > winnerLen {
			winner, winnerLen, kind = b.Name, n, "boundary"
		}
	}
	return winner, winnerLen, kind
}

// elementLabel returns the rendered label for e: Metadata["label"]
// String value when present and non-empty, else e.Name.
func elementLabel(e *Element) string {
	if e == nil {
		return ""
	}
	if v, ok := e.Metadata["label"]; ok {
		if s, ok := v.(String); ok && s.Value != "" {
			return s.Value
		}
	}
	return e.Name
}

// boundaryLabel returns the rendered label for b: Metadata["label"]
// String value when present and non-empty, else b.Name.
func boundaryLabel(b *Boundary) string {
	if b == nil {
		return ""
	}
	if v, ok := b.Metadata["label"]; ok {
		if s, ok := v.(String); ok && s.Value != "" {
			return s.Value
		}
	}
	return b.Name
}

// refForElement builds a Ref handle for an element. The Definition
// pointer lets downstream consumers reach the actual *Element without
// re-resolving.
func refForElement(e *Element) Ref {
	if e == nil {
		return Ref{}
	}
	return Ref{Name: e.Name, Range: e.Range, Definition: e}
}

// refForBoundary builds a Ref handle for a boundary.
func refForBoundary(b *Boundary) Ref {
	if b == nil {
		return Ref{}
	}
	return Ref{Name: b.Name, Range: b.Range, Definition: b}
}
