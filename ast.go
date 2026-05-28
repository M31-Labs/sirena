package sirena

// Range is a half-open byte span [Start, End) in the source.
// Matches mdpp's Range shape so downstream tools compose natively.
type Range struct {
	Start int
	End   int
}

// Document is the root of the IR.
type Document struct {
	Systems []*SystemDecl
	Range   Range
}

// SystemDecl groups every top-level system-file declaration parsed from
// one source. View declarations are added in Task 8.
type SystemDecl struct {
	Elements   []*Element
	Boundaries []*Boundary
	Range      Range
}

// Node is the closed sum type for things that can appear as children
// of a boundary or at top level: *Element and *Boundary today; future
// tasks add *Edge to this list. The unexported sentinel keeps the
// surface closed — external packages cannot add new Node variants.
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
	Children []Node           // *Element and *Boundary; future: *Edge
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
