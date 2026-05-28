// Package sirena: view evaluation.
//
// view.go projects a workspace through one ViewDecl's directives and
// returns a ResolvedView — the filtered, summarized, preset-decorated
// shape that Phase 7's budget evaluator and Plan 2's layout engine
// consume.
//
// Evaluation runs in a fixed order so author intent is predictable:
//
//  1. Selection: walk v.Include and gather elements, boundaries, and
//     edges via ws.Resolve and ws.Files.
//  2. Explicit collapse: any boundary named in v.Collapse is replaced
//     with a BoundarySummary, and its child elements / nested
//     boundaries are pulled out of the view.
//  3. Preset auto-collapse: when a preset is set, boundaries with more
//     children than Preset.AutoCollapseChildCount are summarized too
//     UNLESS the boundary appears in v.Expand. Explicit v.Collapse from
//     step 2 always beats v.Expand.
//  4. Render defaults: when a preset is set and recognized, populate
//     ResolvedView.RenderDefaults with the preset's per-kind defaults
//     keyed by element name.
//
// View evaluation does not mutate workspace IR. Element / Boundary /
// Edge pointers in the returned ResolvedView come straight from the
// source documents; downstream consumers must treat them as read-only.
package sirena

// ResolvedView is the output of view evaluation: a projection of the
// workspace filtered by the view's include / expand / collapse rules
// with preset render defaults applied. It is the canonical input to
// Phase 7's budget evaluator and Plan 2's layout engine.
type ResolvedView struct {
	// Source is the ViewDecl that produced this resolution.
	Source *ViewDecl

	// Elements is the set of elements included in the view. Element
	// pointers come from the workspace's source documents; do not
	// mutate them.
	Elements []*Element

	// Boundaries is the set of boundaries included in the view.
	// Collapsed boundaries appear as Summaries instead.
	Boundaries []*Boundary

	// Edges is the set of edges visible in the view. Includes incoming
	// edges to selected elements when "edges: incoming to" selectors
	// are present; includes outgoing edges from selected elements for
	// "edges: outgoing from".
	Edges []*Edge

	// Summaries represents boundaries whose `collapse:` directive
	// replaced their children with a single summary node. Each carries
	// the original boundary, the count of hidden children, and the
	// original boundary's label.
	Summaries []*BoundarySummary

	// RenderDefaults maps included element names to the preset's visual
	// defaults. Empty when no preset was applied. Stored on the view
	// (not on individual elements) so the same element can render
	// differently in different views.
	RenderDefaults map[string]*RenderDefault
}

// BoundarySummary represents a collapsed boundary in a ResolvedView.
// HiddenChildren is the count of children (elements + nested
// boundaries) replaced by this summary. Label is the boundary's `label`
// metadata when present, falling back to the boundary's declared Name.
type BoundarySummary struct {
	Boundary       *Boundary // original boundary (read-only reference)
	HiddenChildren int       // count of children replaced by this summary
	Label          string    // boundary's `label:` metadata, or its Name if no label
}

// RenderDefault carries the per-element visual defaults a preset
// applies. The fields are advisory: the renderer may override based on
// element-level metadata.
type RenderDefault struct {
	Shape string // e.g. "rectangle", "cylinder" (database default), "queue"
	Color string // hex color or named token e.g. "earth-bg-2"
	Icon  string // optional icon name (renderer-specific resolution)
}

// Preset describes a named bundle of view-time defaults: per-element-
// kind render defaults plus the auto-collapse threshold the evaluator
// applies to boundaries with many children. v0.1 ships only "default";
// later plans add themed presets.
type Preset struct {
	Name string
	// RenderForKind returns the rendering defaults for an element kind.
	RenderForKind map[ElementKind]*RenderDefault
	// AutoCollapseChildCount is the default child-count threshold for
	// auto-collapsing boundaries. 0 means no auto-collapse.
	AutoCollapseChildCount int
}

// builtinPresets maps preset names to their definitions. v0.1 ships
// "default"; later plans add themed presets (earth-dark, hand-drawn,
// etc.). Adding a new preset here automatically makes it reachable via
// PresetForName.
var builtinPresets = map[string]*Preset{
	"default": {
		Name: "default",
		RenderForKind: map[ElementKind]*RenderDefault{
			ElementKindService:  {Shape: "rectangle", Color: "earth-bg-2"},
			ElementKindDatabase: {Shape: "cylinder", Color: "earth-bg-3"},
			ElementKindQueue:    {Shape: "queue", Color: "earth-bg-4"},
			ElementKindCache:    {Shape: "rectangle", Color: "earth-bg-5"},
			ElementKindJob:      {Shape: "rectangle", Color: "earth-bg-6"},
			ElementKindExternal: {Shape: "rectangle", Color: "earth-bg-1"},
			ElementKindClient:   {Shape: "rectangle", Color: "earth-bg-1"},
			ElementKindGateway:  {Shape: "rectangle", Color: "earth-bg-2"},
			ElementKindNode:     {Shape: "rectangle", Color: "earth-bg-2"},
		},
		AutoCollapseChildCount: 12,
	},
}

// PresetForName returns the named preset, or nil if unknown. Callers
// branch on the nil return rather than getting a synthetic placeholder
// so unknown-preset diagnostics surface explicitly in later phases.
func PresetForName(name string) *Preset {
	if name == "" {
		return nil
	}
	return builtinPresets[name]
}

// EvaluateView projects the workspace through the view's directives.
// Returns the resolved view or an error if the view references a
// workspace symbol that doesn't exist. View-time diagnostics surface
// through the returned view's diagnostics field (TBD; for v0.1
// evaluation errors are returned as errors, not diagnostics).
//
// Order of operations:
//
//  1. Selection from v.Include.
//  2. Explicit collapse from v.Collapse.
//  3. Preset auto-collapse honoring v.Expand.
//  4. Render-defaults population from v.Preset.
//
// Element / Boundary / Edge pointers in the returned view come straight
// from the workspace's source documents; do not mutate them.
func EvaluateView(ws *Workspace, v *ViewDecl) (*ResolvedView, error) {
	rv := &ResolvedView{Source: v}
	if ws == nil || v == nil {
		return rv, nil
	}
	ws.ensureIndexed()

	// Track included elements and boundaries by pointer to deduplicate
	// across selectors that overlap (e.g. a boundary plus an element
	// that's also a child of that boundary).
	elementSeen := map[*Element]bool{}
	boundarySeen := map[*Boundary]bool{}
	edgeSeen := map[*Edge]bool{}

	addElement := func(e *Element) {
		if e == nil || elementSeen[e] {
			return
		}
		elementSeen[e] = true
		rv.Elements = append(rv.Elements, e)
	}
	addBoundary := func(b *Boundary) {
		if b == nil || boundarySeen[b] {
			return
		}
		boundarySeen[b] = true
		rv.Boundaries = append(rv.Boundaries, b)
		// Bring the boundary's children into the view too. A boundary
		// selector is a "this whole subtree" intent.
		for _, c := range b.Children {
			switch n := c.(type) {
			case *Element:
				addElement(n)
			case *Boundary:
				// recurse via a tiny inline closure-free walk
				// (addBoundary itself is the recursion path)
				if !boundarySeen[n] {
					boundarySeen[n] = true
					rv.Boundaries = append(rv.Boundaries, n)
					// recurse manually to avoid closure self-reference
					collectSubtree(n, addElement, &boundarySeen, &rv.Boundaries)
				}
			}
		}
	}
	addEdge := func(e *Edge) {
		if e == nil || edgeSeen[e] {
			return
		}
		edgeSeen[e] = true
		rv.Edges = append(rv.Edges, e)
	}

	// Pass 1: process every selector in v.Include.
	for _, sel := range v.Include {
		switch sel.Kind {
		case SelectorBoundary:
			if b := findBoundary(ws, sel.Target); b != nil {
				addBoundary(b)
			}
		case SelectorElement:
			if sym := ws.Resolve(sel.Target); sym != nil {
				if e, ok := sym.Node.(*Element); ok {
					addElement(e)
				}
			}
		case SelectorEdgesIncoming:
			for _, e := range allWorkspaceEdges(ws) {
				if edgeTargets(ws, e, sel.Target, false) {
					addEdge(e)
				}
			}
		case SelectorEdgesOutgoing:
			for _, e := range allWorkspaceEdges(ws) {
				if edgeTargets(ws, e, sel.Target, true) {
					addEdge(e)
				}
			}
		}
	}

	// Pass 2: explicit collapse. Each name in v.Collapse becomes a
	// summary; the boundary and its child elements / nested boundaries
	// are pulled out of the view.
	collapseSet := stringSet(v.Collapse)
	if len(collapseSet) > 0 {
		var keepBoundaries []*Boundary
		for _, b := range rv.Boundaries {
			if !collapseSet[b.Name] {
				keepBoundaries = append(keepBoundaries, b)
				continue
			}
			rv.Summaries = append(rv.Summaries, summarize(b))
			// Drop the boundary's child elements and nested boundaries
			// from the view. We rebuild the visible-boundary slice in
			// the same pass and filter elements after.
			pruneSubtree(b, elementSeen, boundarySeen)
		}
		rv.Boundaries = keepBoundaries
		rv.Elements = filterElements(rv.Elements, elementSeen)
	}

	// Pass 3: preset auto-collapse. Boundaries with > threshold
	// children get summarized unless explicitly expanded. Explicit
	// collapse from pass 2 already removed those boundaries from
	// rv.Boundaries so this loop only sees survivors.
	preset := PresetForName(v.Preset)
	if preset != nil && preset.AutoCollapseChildCount > 0 {
		expandSet := stringSet(v.Expand)
		var keepBoundaries []*Boundary
		for _, b := range rv.Boundaries {
			if expandSet[b.Name] {
				keepBoundaries = append(keepBoundaries, b)
				continue
			}
			if len(b.Children) > preset.AutoCollapseChildCount {
				rv.Summaries = append(rv.Summaries, summarize(b))
				pruneSubtree(b, elementSeen, boundarySeen)
				continue
			}
			keepBoundaries = append(keepBoundaries, b)
		}
		rv.Boundaries = keepBoundaries
		rv.Elements = filterElements(rv.Elements, elementSeen)
	}

	// Pass 4: render defaults. Known preset → populate per-element
	// defaults keyed by element Name. Unknown preset is silent in
	// v0.1; the diagnostic story is deferred to Phase 6 polish.
	if preset != nil && len(preset.RenderForKind) > 0 && len(rv.Elements) > 0 {
		rv.RenderDefaults = map[string]*RenderDefault{}
		for _, e := range rv.Elements {
			if rd := preset.RenderForKind[e.Kind]; rd != nil {
				rv.RenderDefaults[e.Name] = rd
			}
		}
	}

	return rv, nil
}

// collectSubtree recurses through a boundary subtree, registering every
// nested element via add and every nested boundary in boundaries /
// boundarySeen. Used by EvaluateView's addBoundary to avoid the
// closure-self-reference dance of a recursive lambda.
func collectSubtree(b *Boundary, add func(*Element), boundarySeen *map[*Boundary]bool, boundaries *[]*Boundary) {
	for _, c := range b.Children {
		switch n := c.(type) {
		case *Element:
			add(n)
		case *Boundary:
			if (*boundarySeen)[n] {
				continue
			}
			(*boundarySeen)[n] = true
			*boundaries = append(*boundaries, n)
			collectSubtree(n, add, boundarySeen, boundaries)
		}
	}
}

// findBoundary walks every workspace file looking for a top-level or
// nested boundary with the given bare name. The workspace symbol table
// indexes only elements / boundaries by bare name; the resolver returns
// either kind, so a bare-name lookup hits boundaries too. We keep this
// helper anyway because it also walks fence workspaces that index lazily
// through a host.
func findBoundary(ws *Workspace, name string) *Boundary {
	if sym := ws.Resolve(name); sym != nil {
		if b, ok := sym.Node.(*Boundary); ok {
			return b
		}
	}
	return nil
}

// allWorkspaceEdges returns every edge declared in every system file in
// the workspace, plus those reachable through a workspace-resolving
// fence's host. View edges (.view.sir files) declare no edges so they
// are skipped.
func allWorkspaceEdges(ws *Workspace) []*Edge {
	var out []*Edge
	if ws == nil {
		return out
	}
	for _, f := range ws.Files {
		if f.Kind == FileKindView {
			continue
		}
		doc := f.Document
		if doc == nil {
			// Force a load — the index has already triggered loads for
			// every system file, but a freshly-touched workspace might
			// have a file whose document was evicted. LoadDocument
			// short-circuits on the cached doc otherwise.
			doc, _ = ws.LoadDocument(f)
		}
		out = append(out, allEdges(doc)...)
	}
	if ws.host != nil {
		out = append(out, allWorkspaceEdges(ws.host)...)
	}
	return out
}

// edgeTargets reports whether e's source (outgoing=true) or target
// (outgoing=false) resolves to the named element. The comparison is
// performed against the resolved Symbol.Node so a bare `db` in the
// edge matches an `edges: incoming to "db"` selector even when the
// edge was written with a namespaced reference.
func edgeTargets(ws *Workspace, e *Edge, name string, outgoing bool) bool {
	if e == nil || name == "" {
		return false
	}
	endpoint := e.To
	if outgoing {
		endpoint = e.From
	}
	if endpoint == name {
		return true
	}
	// Last resort: resolve both sides and compare nodes. Useful when an
	// edge endpoint is qualified ("platform.kafka") and the selector
	// names the bare form ("kafka").
	sym := ws.Resolve(name)
	endpointSym := ws.Resolve(endpoint)
	if sym == nil || endpointSym == nil {
		return false
	}
	return sym.Node == endpointSym.Node
}

// summarize builds a BoundarySummary from b: counts immediate children
// (elements + nested boundaries) and uses the boundary's `label`
// metadata for Label when present, falling back to its declared Name.
func summarize(b *Boundary) *BoundarySummary {
	if b == nil {
		return nil
	}
	label := b.Name
	if md, ok := b.Metadata["label"]; ok {
		if s, ok := md.(String); ok && s.Value != "" {
			label = s.Value
		}
	}
	return &BoundarySummary{
		Boundary:       b,
		HiddenChildren: len(b.Children),
		Label:          label,
	}
}

// pruneSubtree walks b's children and clears their entries from
// elementSeen / boundarySeen so a subsequent rv.Elements rebuild drops
// them. The visible-slice rebuild is the caller's job; pruneSubtree only
// updates the dedup sets.
func pruneSubtree(b *Boundary, elementSeen map[*Element]bool, boundarySeen map[*Boundary]bool) {
	if b == nil {
		return
	}
	for _, c := range b.Children {
		switch n := c.(type) {
		case *Element:
			delete(elementSeen, n)
		case *Boundary:
			delete(boundarySeen, n)
			pruneSubtree(n, elementSeen, boundarySeen)
		}
	}
}

// filterElements rebuilds an element slice from a dedup set, preserving
// declaration order. Used after collapse passes that mutate elementSeen
// but leave the slice itself behind.
func filterElements(in []*Element, keep map[*Element]bool) []*Element {
	if len(in) == 0 {
		return in
	}
	out := in[:0]
	for _, e := range in {
		if keep[e] {
			out = append(out, e)
		}
	}
	return out
}

// stringSet builds a set-like map from a slice of names. nil / empty
// input returns nil so callers can use `len(set) > 0` to skip the
// associated pass cheaply.
func stringSet(names []string) map[string]bool {
	if len(names) == 0 {
		return nil
	}
	out := make(map[string]bool, len(names))
	for _, n := range names {
		out[n] = true
	}
	return out
}
