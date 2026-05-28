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
	Range   Range
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
// populated by the workspace resolver in Phase 5; the placeholder comment
// below is intentional so future readers know the field is coming.
type Ref struct {
	Namespace string // optional; empty for bare IDENT
	Name      string // the identifier as written
	Range     Range
	// Definition Node // populated by the resolver (Phase 5).
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
	Range    Range
}

func (*Boundary) isSirenaNode() {}

// BoundaryKind enumerates the typed boundary nouns. The zero value is
// reserved as BoundaryKindUnknown so misuse surfaces obviously.
type BoundaryKind int

const (
	// BoundaryKindUnknown is the zero value; not a valid boundary kind.
	BoundaryKindUnknown BoundaryKind = iota
	BoundaryKindTrust
	BoundaryKindNetwork
	BoundaryKindDeployment
	BoundaryKindTeam
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
	Range     Range
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
