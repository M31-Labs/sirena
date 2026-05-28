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
	Elements []*Element
	Range    Range
}

// Element is a typed system component declaration.
type Element struct {
	Kind     ElementKind
	Name     string
	Metadata map[string]Value
	Range    Range
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
