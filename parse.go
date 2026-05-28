package sirena

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammargen"
)

// sirenaLang is the compiled sirena tree-sitter language. Generated lazily
// via sync.Once so importing the package does not pay the LR-table cost
// until the first Parse call.
var (
	sirenaLangOnce sync.Once
	sirenaLang     *gotreesitter.Language
	sirenaLangErr  error
)

// parserPools mirrors mdpp.parserPools: one *gotreesitter.ParserPool per
// language, keyed by the *Language pointer. v0.1 only has one language,
// so the pool always resolves to the same entry — but the sync.Map shape
// keeps us compatible with future languages (e.g. a sirena++ dialect)
// without restructuring callers.
var parserPools sync.Map

// loadLanguage compiles the sirena grammar exactly once.
func loadLanguage() (*gotreesitter.Language, error) {
	sirenaLangOnce.Do(func() {
		sirenaLang, sirenaLangErr = grammargen.GenerateLanguage(SirenaGrammar())
	})
	return sirenaLang, sirenaLangErr
}

// parserPoolFor returns the *gotreesitter.ParserPool bound to lang,
// creating it on first use.
func parserPoolFor(lang *gotreesitter.Language) *gotreesitter.ParserPool {
	if lang == nil {
		return nil
	}
	if pool, ok := parserPools.Load(lang); ok {
		return pool.(*gotreesitter.ParserPool)
	}
	pool := gotreesitter.NewParserPool(lang)
	actual, _ := parserPools.LoadOrStore(lang, pool)
	return actual.(*gotreesitter.ParserPool)
}

// MustParse panics on error. Use for test fixtures and tooling that wants
// the simpler signature.
func MustParse(src []byte) *Document {
	doc, err := Parse(src)
	if err != nil {
		panic(err)
	}
	return doc
}

// Parse runs the sirena grammar against src and returns a *Document with
// byte-accurate Range fields on every node. Parse is panic-safe: a
// recovered panic yields a Document carrying a SIR-PARSE-000 diagnostic
// (mirroring mdpp's approach in parse.go).
//
// For v0.1, Parse only understands element declarations and metadata
// blocks. Subsequent tasks extend the grammar and the lowering.
func Parse(src []byte) (doc *Document, err error) {
	defer func() {
		if r := recover(); r != nil {
			doc = &Document{
				Range: Range{Start: 0, End: len(src)},
			}
			err = fmt.Errorf("SIR-PARSE-000: parser recovered from panic: %v", r)
		}
	}()

	lang, lerr := loadLanguage()
	if lerr != nil {
		return nil, fmt.Errorf("SIR-PARSE-001: load sirena language: %w", lerr)
	}

	pool := parserPoolFor(lang)
	if pool == nil {
		return nil, fmt.Errorf("SIR-PARSE-002: no parser pool for language")
	}

	tree, perr := pool.Parse(src)
	if perr != nil {
		return nil, fmt.Errorf("SIR-PARSE-003: parse failed: %w", perr)
	}
	defer tree.Release()

	root := tree.RootNode()
	if root == nil {
		return &Document{Range: Range{Start: 0, End: len(src)}}, nil
	}

	return lowerDocument(root, lang, src), nil
}

// lowerDocument walks the CST root (which corresponds to source_file in
// the grammar) and produces the typed IR. Every source_file becomes
// exactly one SystemDecl in v0.1; explicit `system { ... }` blocks land
// in a later task.
func lowerDocument(root *gotreesitter.Node, lang *gotreesitter.Language, src []byte) *Document {
	docRange := Range{Start: int(root.StartByte()), End: int(root.EndByte())}

	sys := &SystemDecl{Range: docRange}

	for i := 0; i < root.NamedChildCount(); i++ {
		child := root.NamedChild(i)
		if child == nil {
			continue
		}
		switch child.Type(lang) {
		case "element_decl":
			if e := lowerElement(child, lang, src); e != nil {
				sys.Elements = append(sys.Elements, e)
			}
		case "boundary_decl":
			if b := lowerBoundary(child, lang, src); b != nil {
				sys.Boundaries = append(sys.Boundaries, b)
			}
		case "edge_decl":
			if e := lowerEdge(child, lang, src); e != nil {
				sys.Edges = append(sys.Edges, e)
			}
		}
	}

	return &Document{
		Systems: []*SystemDecl{sys},
		Range:   docRange,
	}
}

// lowerBoundary converts a single boundary_decl CST node into a *Boundary.
// The CST exposes three named fields: "kind" (the keyword), "name" (the
// STRING literal — its quotes are stripped here), and "body" (the
// boundary_block whose children become Node entries).
func lowerBoundary(node *gotreesitter.Node, lang *gotreesitter.Language, src []byte) *Boundary {
	kindNode := node.ChildByFieldName("kind", lang)
	nameNode := node.ChildByFieldName("name", lang)
	bodyNode := node.ChildByFieldName("body", lang)
	if kindNode == nil || nameNode == nil || bodyNode == nil {
		return nil
	}

	b := &Boundary{
		Kind:  boundaryKindForKeyword(nodeText(kindNode, src)),
		Name:  decodeStringLiteral(nodeText(nameNode, src)),
		Range: Range{Start: int(node.StartByte()), End: int(node.EndByte())},
	}

	if metaNode := node.ChildByFieldName("metadata", lang); metaNode != nil {
		b.Metadata = lowerMetadataBlock(metaNode, lang, src)
	}

	for i := 0; i < bodyNode.NamedChildCount(); i++ {
		child := bodyNode.NamedChild(i)
		if child == nil {
			continue
		}
		switch child.Type(lang) {
		case "element_decl":
			if e := lowerElement(child, lang, src); e != nil {
				b.Children = append(b.Children, e)
			}
		case "boundary_decl":
			if nested := lowerBoundary(child, lang, src); nested != nil {
				b.Children = append(b.Children, nested)
			}
		case "edge_decl":
			if e := lowerEdge(child, lang, src); e != nil {
				b.Children = append(b.Children, e)
			}
		}
	}

	return b
}

// lowerEdge converts a single edge_decl CST node into an *Edge. The CST
// exposes five named fields: "from" (source IDENT), "arrow" (the literal
// arrow shape), "to" (destination IDENT), and the optional "kind" and
// "label". Untyped edges (no kind suffix) default to EdgeKindFlow per the
// v0.1 spec.
//
// Direction semantics: `<-` and `<->` do NOT swap From/To at parse time.
// The Direction field captures the arrow shape as written so the printer
// can round-trip byte-for-byte; downstream consumers (resolver, layout)
// interpret the direction.
func lowerEdge(node *gotreesitter.Node, lang *gotreesitter.Language, src []byte) *Edge {
	fromNode := node.ChildByFieldName("from", lang)
	arrowNode := node.ChildByFieldName("arrow", lang)
	toNode := node.ChildByFieldName("to", lang)
	if fromNode == nil || arrowNode == nil || toNode == nil {
		return nil
	}

	e := &Edge{
		From:      nodeText(fromNode, src),
		To:        nodeText(toNode, src),
		Direction: directionForArrow(nodeText(arrowNode, src)),
		Kind:      EdgeKindFlow, // untyped fallback per spec
		Range:     Range{Start: int(node.StartByte()), End: int(node.EndByte())},
	}

	if kindNode := node.ChildByFieldName("kind", lang); kindNode != nil {
		e.Kind = edgeKindForKeyword(nodeText(kindNode, src))
	}
	if labelNode := node.ChildByFieldName("label", lang); labelNode != nil {
		e.Label = decodeStringLiteral(nodeText(labelNode, src))
	}
	if metaNode := node.ChildByFieldName("metadata", lang); metaNode != nil {
		e.Metadata = lowerMetadataBlock(metaNode, lang, src)
	}

	return e
}

// lowerElement converts a single element_decl CST node into an *Element.
func lowerElement(node *gotreesitter.Node, lang *gotreesitter.Language, src []byte) *Element {
	kindNode := node.ChildByFieldName("kind", lang)
	nameNode := node.ChildByFieldName("name", lang)
	if kindNode == nil || nameNode == nil {
		return nil
	}

	kind := elementKindForKeyword(nodeText(kindNode, src))
	name := nodeText(nameNode, src)

	e := &Element{
		Kind:  kind,
		Name:  name,
		Range: Range{Start: int(node.StartByte()), End: int(node.EndByte())},
	}

	if body := node.ChildByFieldName("body", lang); body != nil {
		e.Metadata = lowerMetadataBlock(body, lang, src)
	}

	return e
}

// lowerMetadataBlock walks a metadata_block CST node and returns its pairs
// as a typed map. Returns nil when the block contains no pairs (an empty
// `{}` block stays nil to keep the IR canonical and to preserve the "no
// metadata declared" vs "metadata declared but empty" distinction — we
// collapse both to nil because empty maps are not semantically useful for
// downstream consumers). Shared by lowerElement, lowerBoundary, lowerEdge.
func lowerMetadataBlock(node *gotreesitter.Node, lang *gotreesitter.Language, src []byte) map[string]Value {
	var out map[string]Value
	for i := 0; i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child == nil || child.Type(lang) != "pair" {
			continue
		}
		keyNode := child.ChildByFieldName("key", lang)
		valNode := child.ChildByFieldName("value", lang)
		if keyNode == nil || valNode == nil {
			continue
		}
		v, ok := lowerValue(valNode, lang, src)
		if !ok {
			continue
		}
		if out == nil {
			out = make(map[string]Value)
		}
		out[nodeText(keyNode, src)] = v
	}
	return out
}

// lowerValue converts a CST value node into its typed Value
// representation. Returns (nil, false) when the node is not a recognized
// value variant.
func lowerValue(node *gotreesitter.Node, lang *gotreesitter.Language, src []byte) (Value, bool) {
	r := Range{Start: int(node.StartByte()), End: int(node.EndByte())}
	switch node.Type(lang) {
	case "string":
		return String{Value: decodeStringLiteral(nodeText(node, src)), Range: r}, true
	case "number":
		n, err := strconv.ParseFloat(nodeText(node, src), 64)
		if err != nil {
			return nil, false
		}
		return Number{Value: n, Range: r}, true
	case "identifier":
		return Ident{Value: nodeText(node, src), Range: r}, true
	case "list":
		out := List{Range: r}
		for i := 0; i < node.NamedChildCount(); i++ {
			child := node.NamedChild(i)
			if child == nil {
				continue
			}
			if v, ok := lowerValue(child, lang, src); ok {
				out.Values = append(out.Values, v)
			}
		}
		return out, true
	}
	return nil, false
}

// nodeText returns the slice of src covered by node.
func nodeText(node *gotreesitter.Node, src []byte) string {
	start := int(node.StartByte())
	end := int(node.EndByte())
	if start < 0 || end > len(src) || start > end {
		return ""
	}
	return string(src[start:end])
}

// decodeStringLiteral strips the surrounding quotes and resolves the
// single-character backslash escapes recognized by the grammar. Anything
// not recognized passes through verbatim.
func decodeStringLiteral(lit string) string {
	if len(lit) < 2 || lit[0] != '"' || lit[len(lit)-1] != '"' {
		return lit
	}
	inner := lit[1 : len(lit)-1]
	if !strings.Contains(inner, `\`) {
		return inner
	}
	var b strings.Builder
	b.Grow(len(inner))
	for i := 0; i < len(inner); i++ {
		if inner[i] != '\\' || i+1 >= len(inner) {
			b.WriteByte(inner[i])
			continue
		}
		switch inner[i+1] {
		case '"':
			b.WriteByte('"')
		case '\\':
			b.WriteByte('\\')
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'v':
			b.WriteByte('\v')
		case '0':
			b.WriteByte(0)
		default:
			b.WriteByte(inner[i+1])
		}
		i++
	}
	return b.String()
}
