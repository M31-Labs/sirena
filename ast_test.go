package sirena

import "testing"

// TestIR_ZeroValue asserts every public IR struct has the expected
// zero-state: nil slices/maps/pointers, empty strings, and zero-value
// enums set to the reserved Unknown constants. The goal is to catch
// accidental non-zero defaults that would silently change downstream
// consumer behavior.
func TestIR_ZeroValue(t *testing.T) {
	var d Document
	if d.Imports != nil {
		t.Error("Document.Imports zero value not nil")
	}
	if d.Systems != nil {
		t.Error("Document.Systems zero value not nil")
	}
	if d.Views != nil {
		t.Error("Document.Views zero value not nil")
	}
	if d.Manifest != nil {
		t.Error("Document.Manifest zero value not nil")
	}
	if d.Range.Start != 0 || d.Range.End != 0 {
		t.Errorf("Document.Range zero = %+v, want {0 0}", d.Range)
	}

	var imp Import
	if imp.Path != "" {
		t.Error("Import.Path zero not empty")
	}
	if imp.Range != (Range{}) {
		t.Error("Import.Range zero not Range{}")
	}

	var ref Ref
	if ref.Namespace != "" {
		t.Error("Ref.Namespace zero not empty")
	}
	if ref.Name != "" {
		t.Error("Ref.Name zero not empty")
	}
	if ref.Range != (Range{}) {
		t.Error("Ref.Range zero not Range{}")
	}
	if ref.Definition != nil {
		t.Error("Ref.Definition zero not nil")
	}

	var sys SystemDecl
	if sys.Elements != nil {
		t.Error("SystemDecl.Elements zero not nil")
	}
	if sys.Boundaries != nil {
		t.Error("SystemDecl.Boundaries zero not nil")
	}
	if sys.Edges != nil {
		t.Error("SystemDecl.Edges zero not nil")
	}
	if sys.Range != (Range{}) {
		t.Error("SystemDecl.Range zero not Range{}")
	}

	var e Element
	if e.Kind != ElementKindUnknown {
		t.Errorf("Element.Kind zero = %v, want ElementKindUnknown", e.Kind)
	}
	if e.Name != "" {
		t.Error("Element.Name zero not empty")
	}
	if e.Metadata != nil {
		t.Error("Element.Metadata zero not nil")
	}
	if e.Override != nil {
		t.Error("Element.Override zero not nil")
	}
	if e.Range != (Range{}) {
		t.Error("Element.Range zero not Range{}")
	}

	var b Boundary
	if b.Kind != BoundaryKindUnknown {
		t.Errorf("Boundary.Kind zero = %v, want BoundaryKindUnknown", b.Kind)
	}
	if b.Name != "" {
		t.Error("Boundary.Name zero not empty")
	}
	if b.Children != nil {
		t.Error("Boundary.Children zero not nil")
	}
	if b.Metadata != nil {
		t.Error("Boundary.Metadata zero not nil")
	}
	if b.Override != nil {
		t.Error("Boundary.Override zero not nil")
	}
	if b.Range != (Range{}) {
		t.Error("Boundary.Range zero not Range{}")
	}

	var ed Edge
	if ed.From != "" || ed.To != "" {
		t.Error("Edge.From / Edge.To zero not empty")
	}
	if ed.FromRef != nil || ed.ToRef != nil {
		t.Error("Edge.FromRef / Edge.ToRef zero not nil")
	}
	if ed.Kind != EdgeKindUnknown {
		t.Errorf("Edge.Kind zero = %v, want EdgeKindUnknown", ed.Kind)
	}
	if ed.Direction != DirUnknown {
		t.Errorf("Edge.Direction zero = %v, want DirUnknown", ed.Direction)
	}
	if ed.Label != "" {
		t.Error("Edge.Label zero not empty")
	}
	if ed.Metadata != nil {
		t.Error("Edge.Metadata zero not nil")
	}
	if ed.Override != nil {
		t.Error("Edge.Override zero not nil")
	}
	if ed.Range != (Range{}) {
		t.Error("Edge.Range zero not Range{}")
	}

	var v ViewDecl
	if v.Name != "" || v.Title != "" || v.Preset != "" || v.Theme != "" {
		t.Error("ViewDecl string fields zero not empty")
	}
	if v.Include != nil || v.Expand != nil || v.Collapse != nil {
		t.Error("ViewDecl slice fields zero not nil")
	}
	if v.Layout != nil || v.Budget != nil {
		t.Error("ViewDecl pointer fields zero not nil")
	}
	if v.Metadata != nil {
		t.Error("ViewDecl.Metadata zero not nil")
	}
	if v.Range != (Range{}) {
		t.Error("ViewDecl.Range zero not Range{}")
	}

	var sel Selector
	if sel.Kind != SelectorUnknown {
		t.Errorf("Selector.Kind zero = %v, want SelectorUnknown", sel.Kind)
	}
	if sel.ElementKind != ElementKindUnknown {
		t.Errorf("Selector.ElementKind zero = %v, want ElementKindUnknown", sel.ElementKind)
	}
	if sel.Target != "" {
		t.Error("Selector.Target zero not empty")
	}
	if sel.Range != (Range{}) {
		t.Error("Selector.Range zero not Range{}")
	}

	var lh LayoutHints
	if lh.Direction != "" {
		t.Error("LayoutHints.Direction zero not empty")
	}
	if lh.Pin != nil {
		t.Error("LayoutHints.Pin zero not nil")
	}
	if lh.Metadata != nil {
		t.Error("LayoutHints.Metadata zero not nil")
	}
	if lh.Range != (Range{}) {
		t.Error("LayoutHints.Range zero not Range{}")
	}

	var bg Budget
	if bg.Nodes != 0 || bg.EdgesPerNode != 0 || bg.Depth != 0 ||
		bg.BoundaryFanout != 0 || bg.LabelChars != 0 {
		t.Errorf("Budget zero numeric fields non-zero: %+v", bg)
	}
	if bg.Range != (Range{}) {
		t.Error("Budget.Range zero not Range{}")
	}

	var o Override
	if o.Mode != OverrideUnknown {
		t.Errorf("Override.Mode zero = %v, want OverrideUnknown", o.Mode)
	}
	if o.Fields != nil {
		t.Error("Override.Fields zero not nil")
	}
	if o.Range != (Range{}) {
		t.Error("Override.Range zero not Range{}")
	}

	var r Range
	if r.Start != 0 || r.End != 0 {
		t.Errorf("Range zero = %+v, want {0 0}", r)
	}

	var diag Diagnostic
	if diag.Code != "" || diag.Severity != SeverityError || diag.Message != "" {
		t.Error("Diagnostic zero non-zero")
	}
	if diag.Range != (Range{}) {
		t.Error("Diagnostic.Range zero not Range{}")
	}

	var ws Workspace
	if ws.Root != "" || ws.Mode != ModeUnknown || ws.Manifest != nil || ws.Files != nil {
		t.Error("Workspace zero non-zero")
	}

	var wf WorkspaceFile
	if wf.Path != "" || wf.RelPath != "" || wf.Kind != FileKindUnknown ||
		wf.Document != nil || wf.LoadErr != nil {
		t.Error("WorkspaceFile zero non-zero")
	}

	var s Symbol
	if s.Node != nil || s.File != nil || s.QualifiedName != "" {
		t.Error("Symbol zero non-zero")
	}

	var fo FenceOptions
	if fo.WorkspaceRoot != "" || fo.ViewRef != "" {
		t.Error("FenceOptions zero non-zero")
	}

	var rv ResolvedView
	if rv.Source != nil {
		t.Error("ResolvedView.Source zero not nil")
	}
	if rv.Elements != nil || rv.Boundaries != nil || rv.Edges != nil ||
		rv.Summaries != nil {
		t.Error("ResolvedView slice fields zero not nil")
	}
	if rv.RenderDefaults != nil {
		t.Error("ResolvedView.RenderDefaults zero not nil")
	}

	var bs BoundarySummary
	if bs.Boundary != nil {
		t.Error("BoundarySummary.Boundary zero not nil")
	}
	if bs.HiddenChildren != 0 {
		t.Errorf("BoundarySummary.HiddenChildren zero = %d", bs.HiddenChildren)
	}
	if bs.Label != "" {
		t.Error("BoundarySummary.Label zero not empty")
	}

	var rd RenderDefault
	if rd.Shape != "" || rd.Color != "" || rd.Icon != "" {
		t.Error("RenderDefault zero non-zero")
	}

	var p Preset
	if p.Name != "" || p.RenderForKind != nil || p.AutoCollapseChildCount != 0 {
		t.Error("Preset zero non-zero")
	}

	var br BudgetReport
	if br.Breaches != nil || br.Suggestions != nil {
		t.Error("BudgetReport zero non-zero")
	}

	var bch Breach
	if bch.Field != "" || bch.Limit != 0 || bch.Actual != 0 {
		t.Error("Breach zero non-zero")
	}
	if bch.Target != (Ref{}) {
		t.Error("Breach.Target zero not Ref{}")
	}

	var sg Suggestion
	if sg.Kind != SuggestionUnknown {
		t.Errorf("Suggestion.Kind zero = %v, want SuggestionUnknown", sg.Kind)
	}
	if sg.Target != (Ref{}) {
		t.Error("Suggestion.Target zero not Ref{}")
	}
	if sg.Estimate != (SuggestionEstimate{}) {
		t.Error("Suggestion.Estimate zero non-zero")
	}
	if sg.Rationale != "" {
		t.Error("Suggestion.Rationale zero not empty")
	}

	var se SuggestionEstimate
	if se.NodesReclaimed != 0 || se.EdgesReclaimed != 0 {
		t.Error("SuggestionEstimate zero non-zero")
	}

	var ro RenderOptions
	if ro.StrictBudget {
		t.Error("RenderOptions.StrictBudget zero not false")
	}

	var lr LayoutResult
	if lr.View != nil {
		t.Error("LayoutResult.View zero not nil")
	}

	var eo EmissionOptions
	if eo.Clock != nil || eo.Entropy != nil {
		t.Error("EmissionOptions zero non-zero")
	}

	var ires IdentityResolution
	if ires.SID != "" || ires.PriorSourceRef != "" || ires.MatchedVia != IdentityMatchNew {
		t.Errorf("IdentityResolution zero = %+v", ires)
	}

	var im IdentityMatch
	if im != IdentityMatchNew {
		t.Errorf("IdentityMatch zero = %v, want IdentityMatchNew", im)
	}

	var ro2 ReconcileOptions
	if ro2.Clock != nil || ro2.Entropy != nil || ro2.OrphanGracePeriod != 0 {
		t.Errorf("ReconcileOptions zero non-zero: %+v", ro2)
	}

	var rr ReconcileResult
	if rr.Resolutions != nil || rr.Orphans != nil || rr.Diagnostics != nil {
		t.Errorf("ReconcileResult zero non-zero: %+v", rr)
	}

	var or OrphanRecord
	if or.Element != nil || or.Status != OrphanRetained || !or.FirstSeen.IsZero() || or.Reason != "" {
		t.Errorf("OrphanRecord zero non-zero: %+v", or)
	}

	var os OrphanStatus
	if os != OrphanRetained {
		t.Errorf("OrphanStatus zero = %v, want OrphanRetained", os)
	}
}

// TestIR_OrphanStatusString pins every OrphanStatus constant to its short
// diagnostic label. OrphanRetained is the zero value so an unset status
// surfaces as a kept-in-place orphan rather than as a silent deletion.
func TestIR_OrphanStatusString(t *testing.T) {
	cases := map[OrphanStatus]string{
		OrphanRetained: "retained",
		OrphanDeleted:  "deleted",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("OrphanStatus(%d).String() = %q, want %q", s, got, want)
		}
	}
}

// TestIR_IdentityMatchString pins every IdentityMatch constant to its
// short diagnostic label. IdentityMatchNew is the zero value so an unset
// resolution surfaces as a fresh element rather than as a silent match.
func TestIR_IdentityMatchString(t *testing.T) {
	cases := map[IdentityMatch]string{
		IdentityMatchNew:         "new",
		IdentityMatchBySID:       "sid",
		IdentityMatchBySourceRef: "source_ref",
		IdentityMatchByFuzzy:     "fuzzy",
	}
	for m, want := range cases {
		if got := m.String(); got != want {
			t.Errorf("IdentityMatch(%d).String() = %q, want %q", m, got, want)
		}
	}
}

// TestIR_WorkspaceModeString pins every WorkspaceMode constant to its
// short diagnostic label. ModeUnknown is the zero value so a forgotten
// Mode assignment surfaces as "unknown" in diagnostics rather than as a
// real mode.
func TestIR_WorkspaceModeString(t *testing.T) {
	cases := map[WorkspaceMode]string{
		ModeUnknown:            "unknown",
		ModeStandalone:         "standalone",
		ModeSelfContained:      "self-contained",
		ModeWorkspaceResolving: "workspace-resolving",
	}
	for m, want := range cases {
		if got := m.String(); got != want {
			t.Errorf("WorkspaceMode(%d).String() = %q, want %q", m, got, want)
		}
	}
}

// TestIR_FileKindString pins every FileKind constant to its short label.
// FileKindUnknown stringifies to "unknown" so misclassified files surface
// in diagnostics rather than as a real kind.
func TestIR_FileKindString(t *testing.T) {
	cases := map[FileKind]string{
		FileKindUnknown:   "unknown",
		FileKindSystem:    "system",
		FileKindGenSystem: "gen-system",
		FileKindView:      "view",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("FileKind(%d).String() = %q, want %q", k, got, want)
		}
	}
}

// TestIR_SeverityString pins every Severity constant to its short label.
// SeverityError is the zero value so an empty Diagnostic prints as an error
// — the most-urgent rendering for the most-likely-unintentional state.
func TestIR_SeverityString(t *testing.T) {
	cases := map[Severity]string{
		SeverityError:   "error",
		SeverityWarning: "warning",
		SeverityInfo:    "info",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("Severity(%d).String() = %q, want %q", s, got, want)
		}
	}
}

// TestIR_RangeInvariants documents the End >= Start half-open-span
// contract. Range is a plain struct with no enforcement; the invariant is
// enforced by the parser when it populates the field. This test pins the
// contract so future refactors that change the shape break the test.
func TestIR_RangeInvariants(t *testing.T) {
	cases := []Range{
		{0, 0},
		{5, 5},
		{0, 100},
		{10, 20},
	}
	for _, r := range cases {
		if r.End < r.Start {
			t.Errorf("End<Start invariant violated: %+v", r)
		}
	}
}

// TestIR_RangeBoundsAgainstSource sanity-checks End <= len(src) for the
// ranges the parser actually emits. A non-trivial fixture exercises the
// document, element, boundary, and edge ranges in one shot.
func TestIR_RangeBoundsAgainstSource(t *testing.T) {
	src := []byte(`service api
boundary trust "edge" {
  database cache
  api -> cache
}
`)
	doc := MustParse(src)
	if doc.Range.End > len(src) {
		t.Errorf("Document.Range.End=%d > len(src)=%d", doc.Range.End, len(src))
	}
	if doc.Range.Start > doc.Range.End {
		t.Errorf("Document.Range.Start=%d > End=%d", doc.Range.Start, doc.Range.End)
	}
	if len(doc.Systems) != 1 {
		t.Fatalf("expected 1 system, got %d", len(doc.Systems))
	}
	sys := doc.Systems[0]
	for _, e := range sys.Elements {
		if e.Range.End < e.Range.Start || e.Range.End > len(src) {
			t.Errorf("Element %q range invalid: %+v (src len=%d)", e.Name, e.Range, len(src))
		}
	}
	for _, b := range sys.Boundaries {
		if b.Range.End < b.Range.Start || b.Range.End > len(src) {
			t.Errorf("Boundary %q range invalid: %+v (src len=%d)", b.Name, b.Range, len(src))
		}
		for _, child := range b.Children {
			switch c := child.(type) {
			case *Element:
				if c.Range.End < c.Range.Start || c.Range.End > len(src) {
					t.Errorf("nested Element %q range invalid: %+v", c.Name, c.Range)
				}
			case *Edge:
				if c.Range.End < c.Range.Start || c.Range.End > len(src) {
					t.Errorf("nested Edge range invalid: %+v", c.Range)
				}
			case *Boundary:
				if c.Range.End < c.Range.Start || c.Range.End > len(src) {
					t.Errorf("nested Boundary %q range invalid: %+v", c.Name, c.Range)
				}
			}
		}
	}
}

// TestIR_ValueSumTypeClosed asserts every concrete Value type satisfies
// the closed Value interface. Adding a new concrete variant requires a
// new isSirenaValue() method; this test exists so a forgotten method
// surfaces at the test boundary rather than at a downstream consumer.
func TestIR_ValueSumTypeClosed(t *testing.T) {
	vs := []Value{
		String{Value: "x"},
		Number{Value: 1.5},
		Ident{Value: "id"},
		List{Values: []Value{}},
	}
	if len(vs) != 4 {
		t.Fatalf("Value sum type closure broken: got %d variants", len(vs))
	}
}

// TestIR_NodeSumTypeClosed asserts every concrete Node type satisfies the
// closed Node interface. *Element, *Boundary, *Edge are the only allowed
// variants; new ones require a new isSirenaNode() method.
func TestIR_NodeSumTypeClosed(t *testing.T) {
	ns := []Node{&Element{}, &Boundary{}, &Edge{}}
	if len(ns) != 3 {
		t.Fatalf("Node sum type closure broken: got %d variants", len(ns))
	}
}

// TestIR_ElementKindString pins every ElementKind constant to its source
// keyword. The unknown sentinel must stringify to "unknown" so diagnostics
// surface the misuse rather than crashing.
func TestIR_ElementKindString(t *testing.T) {
	cases := map[ElementKind]string{
		ElementKindUnknown:  "unknown",
		ElementKindService:  "service",
		ElementKindDatabase: "database",
		ElementKindQueue:    "queue",
		ElementKindCache:    "cache",
		ElementKindJob:      "job",
		ElementKindExternal: "external",
		ElementKindClient:   "client",
		ElementKindGateway:  "gateway",
		ElementKindNode:     "node",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("ElementKind(%d).String() = %q, want %q", k, got, want)
		}
	}
}

// TestIR_BoundaryKindString pins every BoundaryKind constant to its
// source keyword.
func TestIR_BoundaryKindString(t *testing.T) {
	cases := map[BoundaryKind]string{
		BoundaryKindUnknown:    "unknown",
		BoundaryKindTrust:      "trust",
		BoundaryKindNetwork:    "network",
		BoundaryKindDeployment: "deployment",
		BoundaryKindTeam:       "team",
		BoundaryKindGroup:      "group",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("BoundaryKind(%d).String() = %q, want %q", k, got, want)
		}
	}
}

// TestIR_EdgeKindString pins every EdgeKind constant to its source
// keyword. Flow is the untyped fallback; its string form is the verb
// "flow" (NOT the empty string) so diagnostics print readably.
func TestIR_EdgeKindString(t *testing.T) {
	cases := map[EdgeKind]string{
		EdgeKindUnknown:    "unknown",
		EdgeKindCalls:      "calls",
		EdgeKindReads:      "reads",
		EdgeKindWrites:     "writes",
		EdgeKindPublishes:  "publishes",
		EdgeKindSubscribes: "subscribes",
		EdgeKindDependsOn:  "depends_on",
		EdgeKindFlow:       "flow",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("EdgeKind(%d).String() = %q, want %q", k, got, want)
		}
	}
}

// TestIR_DirectionString pins every Direction constant to its source
// arrow form. The unknown sentinel renders as "?" so diagnostics flag the
// misuse without colliding with a real arrow.
func TestIR_DirectionString(t *testing.T) {
	cases := map[Direction]string{
		DirUnknown:       "?",
		DirForward:       "->",
		DirReverse:       "<-",
		DirBidirectional: "<->",
	}
	for d, want := range cases {
		if got := d.String(); got != want {
			t.Errorf("Direction(%d).String() = %q, want %q", d, got, want)
		}
	}
}

// TestIR_SelectorKindString pins every SelectorKind constant to its
// canonical source form. The edges-incoming/outgoing kinds use the
// colon-qualified form ("edges:incoming", "edges:outgoing") so the
// rendering is unambiguous.
func TestIR_SelectorKindString(t *testing.T) {
	cases := map[SelectorKind]string{
		SelectorUnknown:       "unknown",
		SelectorBoundary:      "boundary",
		SelectorElement:       "element",
		SelectorEdgesIncoming: "edges:incoming",
		SelectorEdgesOutgoing: "edges:outgoing",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("SelectorKind(%d).String() = %q, want %q", s, got, want)
		}
	}
}

// TestIR_OverrideModeString pins every OverrideMode constant to its
// short diagnostic label.
func TestIR_OverrideModeString(t *testing.T) {
	cases := map[OverrideMode]string{
		OverrideUnknown: "unknown",
		OverrideAll:     "all",
		OverrideFields:  "fields",
		OverrideExcept:  "except",
	}
	for m, want := range cases {
		if got := m.String(); got != want {
			t.Errorf("OverrideMode(%d).String() = %q, want %q", m, got, want)
		}
	}
}

// TestIR_SuggestionKindString pins every SuggestionKind constant to its
// canonical short label. SuggestionUnknown is the zero value so a
// forgotten Kind assignment surfaces as "unknown" rather than a real
// suggestion.
func TestIR_SuggestionKindString(t *testing.T) {
	cases := map[SuggestionKind]string{
		SuggestionUnknown:          "unknown",
		SuggestionCollapseBoundary: "collapse-boundary",
		SuggestionDropEdgeKind:     "drop-edge-kind",
		SuggestionSplitView:        "split-view",
		SuggestionRaiseZoomLevel:   "raise-zoom-level",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("SuggestionKind(%d).String() = %q, want %q", k, got, want)
		}
	}
}

// TestIR_ElementKindForKeyword_RoundTrip asserts every non-Unknown
// ElementKind survives a String() → elementKindForKeyword() round-trip.
// The constant range is [ElementKindService, ElementKindNode] inclusive.
func TestIR_ElementKindForKeyword_RoundTrip(t *testing.T) {
	for k := ElementKindService; k <= ElementKindNode; k++ {
		if got := elementKindForKeyword(k.String()); got != k {
			t.Errorf("round trip ElementKind(%v): got %v", k, got)
		}
	}
	if got := elementKindForKeyword("not-a-kind"); got != ElementKindUnknown {
		t.Errorf("unknown keyword should map to ElementKindUnknown, got %v", got)
	}
}

// TestIR_BoundaryKindForKeyword_RoundTrip asserts every non-Unknown
// BoundaryKind survives a String() → boundaryKindForKeyword() round-trip.
func TestIR_BoundaryKindForKeyword_RoundTrip(t *testing.T) {
	for k := BoundaryKindTrust; k <= BoundaryKindGroup; k++ {
		if got := boundaryKindForKeyword(k.String()); got != k {
			t.Errorf("round trip BoundaryKind(%v): got %v", k, got)
		}
	}
	if got := boundaryKindForKeyword("not-a-kind"); got != BoundaryKindUnknown {
		t.Errorf("unknown keyword should map to BoundaryKindUnknown, got %v", got)
	}
}

// TestIR_EdgeKindForKeyword_RoundTrip asserts every non-Unknown EdgeKind
// survives a String() → edgeKindForKeyword() round-trip.
func TestIR_EdgeKindForKeyword_RoundTrip(t *testing.T) {
	for k := EdgeKindCalls; k <= EdgeKindFlow; k++ {
		if got := edgeKindForKeyword(k.String()); got != k {
			t.Errorf("round trip EdgeKind(%v): got %v", k, got)
		}
	}
	if got := edgeKindForKeyword("not-a-kind"); got != EdgeKindUnknown {
		t.Errorf("unknown keyword should map to EdgeKindUnknown, got %v", got)
	}
}

// TestIR_DirectionForArrow_RoundTrip asserts every non-Unknown Direction
// survives a String() → directionForArrow() round-trip.
func TestIR_DirectionForArrow_RoundTrip(t *testing.T) {
	for d := DirForward; d <= DirBidirectional; d++ {
		if got := directionForArrow(d.String()); got != d {
			t.Errorf("round trip Direction(%v): got %v", d, got)
		}
	}
	if got := directionForArrow("=>"); got != DirUnknown {
		t.Errorf("unknown arrow should map to DirUnknown, got %v", got)
	}
}
