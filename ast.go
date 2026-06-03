package sirena

// Range is a half-open byte span [Start, End) in the source.
// Matches mdpp's Range shape so downstream tools compose natively.
type Range struct {
	Start int
	End   int
}

// Document is the root of the IR.
type Document struct {
	Imports []*Import
	Systems []*SystemDecl
	Views   []*ViewDecl
	// Manifest carries the workspace-level defaults loaded from
	// sirena.yaml. It is nil unless the document was produced via a
	// workspace operation that loaded a manifest; Parse, which only
	// sees a single .sir file body, never populates it.
	Manifest *Manifest
	// parseDiagnostics carries any diagnostic surfaced by Parse itself —
	// today, just the SIR-PARSE-RECOVERABLE entry written when the
	// panic-recovery path triggers. Document.Diagnostics aggregates this
	// with the structural Validate findings so callers see a single
	// unified slice. The field is unexported because the parser owns its
	// emission shape; external code that wants to attach findings to a
	// document should produce its own slice rather than mutating ours.
	parseDiagnostics []Diagnostic
	Range            Range
}

// Diagnostics returns the union of every diagnostic attached to this
// document. The current sources are:
//
//   - parse-time recoveries (SIR-PARSE-RECOVERABLE) populated by Parse on
//     the panic-recovery path.
//   - structural findings (SIR-VALIDATE-*) from Validate(d), which is
//     re-run on every call so the result reflects the document's current
//     IR state without the caller having to remember to re-validate.
//
// Workspace-level diagnostics (resolve, edge target binding, override
// dangling, budget breach, orphan grace) are NOT included here — those
// come from Workspace.ResolveDiagnostics, Render's BudgetReport, etc. —
// because they're scoped to a workspace+view pair, not a single file.
//
// Diagnostics is nil-safe: calling on a nil receiver returns a nil slice.
func (d *Document) Diagnostics() []Diagnostic {
	if d == nil {
		return nil
	}
	out := append([]Diagnostic(nil), d.parseDiagnostics...)
	out = append(out, Validate(d)...)
	return out
}

// Import declares that another sirena source file's symbols should be
// available in this file's namespace, prefixed by the imported file's stem
// (e.g. `import "../shared/platform.sir"` makes its symbols reachable as
// `platform.<name>`). Resolution happens in Phase 5; parse-time just
// records the directive.
type Import struct {
	Path  string // verbatim string after "import" (relative or absolute as written)
	Range Range
}

// Ref is a resolved identifier reference. At parse time, Name is the
// identifier as written and Namespace is the optional prefix from a
// qualified_ident (e.g. "platform" in "platform.kafka"). Definition is
// populated by the workspace resolver in Phase 5.
type Ref struct {
	Namespace string // optional; empty for bare IDENT
	Name      string // the identifier as written
	Range     Range
	// Definition is the resolved declaration this ref points at. Nil before
	// workspace resolution runs; populated by ws.ResolveWorkspace (Phase 5)
	// when the workspace contains a matching symbol. May remain nil for
	// refs that couldn't be resolved — those produce SIR-REF-UNRESOLVED
	// diagnostics.
	Definition Node
}

// SystemDecl groups every top-level system-file declaration parsed from
// one source. View declarations are added in Task 8.
type SystemDecl struct {
	Elements   []*Element
	Boundaries []*Boundary
	Edges      []*Edge
	Range      Range
}

// Node is the closed sum type for things that can appear as children
// of a boundary or at top level: *Element, *Boundary, and *Edge. The
// unexported sentinel keeps the surface closed — external packages
// cannot add new Node variants.
type Node interface {
	isSirenaNode()
}

// Element is a typed system component declaration.
type Element struct {
	Kind     ElementKind
	Name     string
	Metadata map[string]Value
	// Override marks the element as a hand-authored override of a generated
	// `.gen.sir` declaration with the same name. Nil when no `@override`
	// annotation was written. See Override for the field-scoped semantics.
	Override *Override
	Range    Range
}

func (*Element) isSirenaNode() {}

// Boundary is a typed container that groups elements and nested boundaries.
// The Kind determines default rendering treatment (dashed for trust, solid
// for network, etc.) and is the primary structural axis used by layout.
// Boundaries are declared with a string-literal Name so authors can use
// arbitrary domain language (e.g. "pci", "us-east-1") without colliding
// with the identifier grammar used by elements.
type Boundary struct {
	Kind     BoundaryKind
	Name     string           // declared as a STRING literal, e.g. "pci"
	Children []Node           // *Element, *Boundary, and *Edge
	Metadata map[string]Value // empty for v0.1; later tasks allow metadata blocks on boundaries
	// Override marks the boundary as a hand-authored override of a generated
	// `.gen.sir` declaration with the same name. Nil when no `@override`
	// annotation was written. See Override for the field-scoped semantics.
	Override *Override
	Range    Range
}

func (*Boundary) isSirenaNode() {}

// BoundaryKind enumerates the typed boundary nouns. The zero value is
// reserved as BoundaryKindUnknown so misuse surfaces obviously.
type BoundaryKind int

const (
	// BoundaryKindUnknown is the zero value; not a valid boundary kind.
	BoundaryKindUnknown BoundaryKind = iota
	// BoundaryKindTrust is a security/trust boundary (rendered dashed).
	BoundaryKindTrust
	// BoundaryKindNetwork is a network-topology boundary (e.g. VPC, subnet).
	BoundaryKindNetwork
	// BoundaryKindDeployment is a deployment-target boundary (e.g. region, cluster).
	BoundaryKindDeployment
	// BoundaryKindTeam is an organizational ownership boundary.
	BoundaryKindTeam
	// BoundaryKindGroup is a generic visual grouping boundary used by
	// ingesters (e.g. Mermaid subgraph). It is the boundary analog of
	// ElementKindNode: a catchall kind when no typed boundary keyword applies.
	// Added in Phase D (ADR 0002) as a permitted additive enum value per ADR 0001.
	BoundaryKindGroup
)

// String returns the source keyword for this kind, e.g. "trust".
func (k BoundaryKind) String() string {
	switch k {
	case BoundaryKindTrust:
		return "trust"
	case BoundaryKindNetwork:
		return "network"
	case BoundaryKindDeployment:
		return "deployment"
	case BoundaryKindTeam:
		return "team"
	case BoundaryKindGroup:
		return "group"
	default:
		return "unknown"
	}
}

// boundaryKindForKeyword maps a source keyword to its BoundaryKind.
// Returns BoundaryKindUnknown if the keyword is not a recognized kind.
func boundaryKindForKeyword(kw string) BoundaryKind {
	switch kw {
	case "trust":
		return BoundaryKindTrust
	case "network":
		return BoundaryKindNetwork
	case "deployment":
		return BoundaryKindDeployment
	case "team":
		return BoundaryKindTeam
	case "group":
		return BoundaryKindGroup
	default:
		return BoundaryKindUnknown
	}
}

// Edge is a typed directional relationship between two elements.
// From and To are *unresolved* identifier names at parse time; the workspace
// resolver (Phase 5) populates ref bindings. Reverse/bidirectional shorthand
// (<- and <->) does NOT swap From/To at parse time — Direction carries the
// arrow shape so the printer round-trips byte-for-byte.
//
// FromRef and ToRef carry the same identifier in structured form
// (Namespace + Name) for downstream consumers that need to distinguish a
// qualified `platform.kafka` from a bare `kafka`. The string fields remain
// the verbatim source text so the printer and external Go callers do not
// have to reconstruct the namespace prefix themselves.
type Edge struct {
	From      string // source identifier as written (may be namespaced: "platform.kafka")
	To        string // destination identifier as written
	FromRef   *Ref   // structured form of From (Namespace + Name); never nil after Parse
	ToRef     *Ref   // structured form of To (Namespace + Name); never nil after Parse
	Kind      EdgeKind
	Direction Direction
	Label     string           // optional; empty when no label string follows
	Metadata  map[string]Value // empty for v0.1; later tasks add metadata blocks on edges
	// Override marks the edge as a hand-authored override of a generated
	// `.gen.sir` declaration with the same endpoints/kind. Nil when no
	// `@override` annotation was written. The plan describes overrides on
	// element / boundary declarations explicitly; edges carry the same
	// field so a code-emitting agent that generates edges can mark them
	// curated by the same mechanism. See Override.
	Override *Override
	Range    Range
}

func (*Edge) isSirenaNode() {}

// EdgeKind enumerates the typed edge verbs. The zero value is reserved as
// EdgeKindUnknown so misuse surfaces obviously. EdgeKindFlow is the
// untyped fallback used when an edge is declared without a kind suffix.
type EdgeKind int

const (
	// EdgeKindUnknown is the zero value; not a valid edge kind.
	EdgeKindUnknown EdgeKind = iota
	EdgeKindCalls
	EdgeKindReads
	EdgeKindWrites
	EdgeKindPublishes
	EdgeKindSubscribes
	EdgeKindDependsOn
	// EdgeKindFlow is the untyped fallback for edges declared without a
	// kind suffix, e.g. `api -> db`.
	EdgeKindFlow
)

// String returns the source keyword for this kind, e.g. "reads".
func (k EdgeKind) String() string {
	switch k {
	case EdgeKindCalls:
		return "calls"
	case EdgeKindReads:
		return "reads"
	case EdgeKindWrites:
		return "writes"
	case EdgeKindPublishes:
		return "publishes"
	case EdgeKindSubscribes:
		return "subscribes"
	case EdgeKindDependsOn:
		return "depends_on"
	case EdgeKindFlow:
		return "flow"
	default:
		return "unknown"
	}
}

// edgeKindForKeyword maps a source keyword to its EdgeKind. Returns
// EdgeKindUnknown if the keyword is not a recognized edge kind.
func edgeKindForKeyword(kw string) EdgeKind {
	switch kw {
	case "calls":
		return EdgeKindCalls
	case "reads":
		return EdgeKindReads
	case "writes":
		return EdgeKindWrites
	case "publishes":
		return EdgeKindPublishes
	case "subscribes":
		return EdgeKindSubscribes
	case "depends_on":
		return EdgeKindDependsOn
	case "flow":
		return EdgeKindFlow
	default:
		return EdgeKindUnknown
	}
}

// Direction captures the arrow shape of an edge. `<-` (DirReverse) and
// `<->` (DirBidirectional) do NOT swap From/To at parse time — the field
// records the arrow as written so the printer round-trips byte-for-byte.
type Direction int

const (
	// DirUnknown is the zero value; not a valid direction.
	DirUnknown       Direction = iota
	DirForward                 // "->"
	DirReverse                 // "<-"
	DirBidirectional           // "<->"
)

// String returns the source arrow text for this direction.
func (d Direction) String() string {
	switch d {
	case DirForward:
		return "->"
	case DirReverse:
		return "<-"
	case DirBidirectional:
		return "<->"
	default:
		return "?"
	}
}

// directionForArrow maps the source arrow text to its Direction.
func directionForArrow(arrow string) Direction {
	switch arrow {
	case "->":
		return DirForward
	case "<-":
		return DirReverse
	case "<->":
		return DirBidirectional
	default:
		return DirUnknown
	}
}

// ElementKind enumerates the typed element nouns. The zero value is
// reserved as ElementKindUnknown so misuse surfaces obviously.
type ElementKind int

const (
	// ElementKindUnknown is the zero value; not a valid element kind.
	ElementKindUnknown ElementKind = iota
	ElementKindService
	ElementKindDatabase
	ElementKindQueue
	ElementKindCache
	ElementKindJob
	ElementKindExternal
	ElementKindClient
	ElementKindGateway
	ElementKindNode
)

// String returns the source keyword for this kind, e.g. "service".
func (k ElementKind) String() string {
	switch k {
	case ElementKindService:
		return "service"
	case ElementKindDatabase:
		return "database"
	case ElementKindQueue:
		return "queue"
	case ElementKindCache:
		return "cache"
	case ElementKindJob:
		return "job"
	case ElementKindExternal:
		return "external"
	case ElementKindClient:
		return "client"
	case ElementKindGateway:
		return "gateway"
	case ElementKindNode:
		return "node"
	default:
		return "unknown"
	}
}

// elementKindForKeyword maps a source keyword to its ElementKind. Returns
// ElementKindUnknown for inputs that are not a recognized element keyword.
func elementKindForKeyword(kw string) ElementKind {
	switch kw {
	case "service":
		return ElementKindService
	case "database":
		return ElementKindDatabase
	case "queue":
		return ElementKindQueue
	case "cache":
		return ElementKindCache
	case "job":
		return ElementKindJob
	case "external":
		return ElementKindExternal
	case "client":
		return ElementKindClient
	case "gateway":
		return ElementKindGateway
	case "node":
		return ElementKindNode
	default:
		return ElementKindUnknown
	}
}

// Value is the discriminated-union type for metadata values. Concrete
// types implementing it: String, Number, Ident, List. The unexported
// sentinel method keeps the surface closed — external packages cannot
// add new Value variants.
type Value interface {
	isSirenaValue()
}

// String is a double-quoted string literal value.
type String struct {
	Value string
	Range Range
}

func (String) isSirenaValue() {}

// Number is a numeric literal value. v0.1 stores all numbers as float64;
// integer-vs-float distinctions are not exposed today because downstream
// consumers do not need them.
type Number struct {
	Value float64
	Range Range
}

func (Number) isSirenaValue() {}

// Ident is a bare identifier value (e.g. an enum-like metadata atom).
type Ident struct {
	Value string
	Range Range
}

func (Ident) isSirenaValue() {}

// List is a comma-separated bracketed list of values.
type List struct {
	Values []Value
	Range  Range
}

func (List) isSirenaValue() {}

// ViewDecl is a view declaration: what to render and how. Views are
// file-level (peers of Imports and Systems), not nested inside any system.
// A view file (`*.view.sir`) typically contains one view, but the grammar
// accepts multiple views per file so the workspace evaluator can pick the
// named one. All directives except Name are optional; an empty body
// (`view "small" {}`) is legal — every field stays zero/nil.
type ViewDecl struct {
	Name     string           // declared as STRING, e.g. "payments-data-plane"
	Title    string           // optional; from `title:` directive
	Preset   string           // optional; from `preset:` (IDENT)
	Theme    string           // optional; from `theme:` (IDENT)
	Include  []Selector       // optional; from `include:` selector list
	Expand   []string         // optional; from `expand:` (list of element/boundary names)
	Collapse []string         // optional; from `collapse:` (list of names)
	Layout   *LayoutHints     // optional; from `layout { ... }` sub-block
	Budget   *Budget          // optional; from `budget { ... }` sub-block
	Metadata map[string]Value // unrecognized top-level view_pair keys land here
	Range    Range
}

// Selector is one item in a view's include list. It expresses which
// elements / boundaries / edges to project from the system into the view.
type Selector struct {
	Kind        SelectorKind // discriminator (boundary, element, edges in/out)
	ElementKind ElementKind  // populated only when Kind == SelectorElement
	// Target is the resolved STRING literal — surrounding double quotes
	// are already stripped by decodeStringLiteral, so a source `"pci"`
	// lands here as `pci`. The printer re-quotes when emitting.
	Target string
	Range  Range
}

// SelectorKind enumerates the selector forms allowed in a view's
// `include:` list. The zero value SelectorUnknown is reserved so misuse
// surfaces obviously.
type SelectorKind int

const (
	// SelectorUnknown is the zero value; not a valid selector kind.
	SelectorUnknown SelectorKind = iota
	// SelectorBoundary selects a boundary by name: `boundary "pci"`.
	SelectorBoundary
	// SelectorElement selects an element by typed kind keyword + name,
	// e.g. `service "auth-svc"`. The element kind is carried in
	// Selector.ElementKind.
	SelectorElement
	// SelectorEdgesIncoming selects all edges flowing into a named target:
	// `edges: incoming to "api"`.
	SelectorEdgesIncoming
	// SelectorEdgesOutgoing selects all edges flowing out of a named target:
	// `edges: outgoing from "api"`.
	SelectorEdgesOutgoing
)

// String returns the canonical source-level rendering of this selector
// kind. Used for diagnostics and printer round-trips; the printer for
// SelectorElement consults Selector.ElementKind to recover the element
// keyword (e.g. "service").
func (k SelectorKind) String() string {
	switch k {
	case SelectorBoundary:
		return "boundary"
	case SelectorElement:
		return "element"
	case SelectorEdgesIncoming:
		return "edges:incoming"
	case SelectorEdgesOutgoing:
		return "edges:outgoing"
	default:
		return "unknown"
	}
}

// LayoutHints captures the view's optional `layout { ... }` sub-block.
// Pin maps an element / boundary name to a side keyword ("top", "bottom",
// "left", "right"); Direction is the optional flow direction hint such as
// "top-down" or "left-right". Unrecognized layout pair keys land in
// Metadata so future hints round-trip without grammar changes.
type LayoutHints struct {
	Direction string            // optional; "top-down", "left-right", etc.
	Pin       map[string]string // name → side
	Metadata  map[string]Value  // unrecognized keys
	Range     Range
}

// Override marks an element, boundary, or edge as a hand-authored override
// of a generated `.gen.sir` declaration with the same name. Field-scoped
// overrides let humans curate specific fields while the generator continues
// to own the rest. The workspace resolver (Phase 5) merges generated and
// hand-authored declarations using this annotation.
type Override struct {
	Mode   OverrideMode
	Fields []string // names of overridden fields; semantics depend on Mode
	Range  Range
}

// OverrideMode discriminates how Override.Fields is interpreted.
type OverrideMode int

const (
	// OverrideUnknown is the zero value; not a valid override mode.
	OverrideUnknown OverrideMode = iota
	// OverrideAll corresponds to a bare `@override` — the hand declaration
	// wins wholesale and Fields is empty.
	OverrideAll
	// OverrideFields corresponds to `@override(label, tags)` — only the
	// listed fields win; unlisted fields stay owned by the generator.
	OverrideFields
	// OverrideExcept corresponds to `@override(except: source_ref)` —
	// every field EXCEPT those listed is overridden; the listed fields
	// stay owned by the generator.
	OverrideExcept
)

// String returns a stable short label for the mode, used for diagnostics
// and the Phase 5 resolver's merge log.
func (m OverrideMode) String() string {
	switch m {
	case OverrideAll:
		return "all"
	case OverrideFields:
		return "fields"
	case OverrideExcept:
		return "except"
	default:
		return "unknown"
	}
}

// Budget caps view complexity. The view evaluator enforces these caps in
// Phase 7 and the renderer refuses to draw a view that overflows them in
// strict mode. The zero value means "no budget declared" — every cap
// stays unenforced (a *Budget of nil on ViewDecl is the canonical "no
// budget" signal; a non-nil *Budget with a zero field signals "this cap
// was explicitly set to zero", which the evaluator treats as forbidding
// anything in that category).
type Budget struct {
	Nodes          int // hard cap on total rendered nodes (elements + boundaries).
	EdgesPerNode   int // cap on edges incident to any single node; overflow drops the lowest-priority edges.
	Depth          int // cap on boundary nesting depth; deeper boundaries are collapsed into their parent.
	BoundaryFanout int // cap on direct children any single boundary may project; overflow groups extras under an ellipsis.
	LabelChars     int // cap on the rendered length of each edge label; overflow is truncated with an ellipsis.
	Range          Range
}
