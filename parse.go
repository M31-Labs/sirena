package sirena

import (
	_ "embed"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/odvcencio/gotreesitter"
)

// grammarBlob is the pre-compiled sirena language, serialized (gzipped gob)
// at generate-time by ./internal/gengrammar. Loading it skips the LR-table
// construction that grammargen.GenerateLanguage performs. Regenerate after
// changing SirenaGrammar with `go generate ./...`.
//
//go:embed grammar.blob
var grammarBlob []byte

// sirenaLang is the sirena tree-sitter language, loaded lazily via sync.Once
// from grammarBlob so importing the package does not pay the load cost until
// the first Parse call.
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

// loadLanguage loads the pre-compiled sirena language from the embedded
// grammar.blob exactly once.
func loadLanguage() (*gotreesitter.Language, error) {
	sirenaLangOnce.Do(func() {
		sirenaLang, sirenaLangErr = gotreesitter.LoadLanguage(grammarBlob)
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
// recovered panic yields a Document carrying a SIR-PARSE-RECOVERABLE
// diagnostic on its parseDiagnostics slice and an accompanying error.
// Document.Diagnostics surfaces the entry alongside any structural
// Validate findings so callers see one unified slice.
//
// Parse understands the full v0.1 grammar: imports, element / boundary /
// edge declarations with optional metadata blocks and `@override`
// annotations, qualified identifier references on edge endpoints, and
// view declarations with layout hints and budgets.
func Parse(src []byte) (doc *Document, err error) {
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprintf("parser recovered from panic: %v", r)
			doc = &Document{
				Range: Range{Start: 0, End: len(src)},
				parseDiagnostics: []Diagnostic{{
					Code:     "SIR-PARSE-RECOVERABLE",
					Severity: SeverityError,
					Message:  msg,
					Range:    Range{Start: 0, End: len(src)},
				}},
			}
			err = fmt.Errorf("SIR-PARSE-RECOVERABLE: %s", msg)
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

	doc := &Document{Range: docRange}
	sys := &SystemDecl{Range: docRange}

	for i := 0; i < root.NamedChildCount(); i++ {
		child := root.NamedChild(i)
		if child == nil {
			continue
		}
		switch child.Type(lang) {
		case "import_decl":
			if imp := lowerImport(child, lang, src); imp != nil {
				doc.Imports = append(doc.Imports, imp)
			}
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
		case "view_decl":
			if v := lowerView(child, lang, src); v != nil {
				doc.Views = append(doc.Views, v)
			}
		}
	}

	doc.Systems = []*SystemDecl{sys}
	return doc
}

// lowerImport converts an import_decl CST node into an *Import. The CST
// exposes a single named field "path" referencing the STRING literal; the
// surrounding quotes are stripped via decodeStringLiteral so the Path
// matches the verbatim string written in source (e.g.
// "../shared/platform.sir").
func lowerImport(node *gotreesitter.Node, lang *gotreesitter.Language, src []byte) *Import {
	pathNode := node.ChildByFieldName("path", lang)
	if pathNode == nil {
		return nil
	}
	return &Import{
		Path:  decodeStringLiteral(nodeText(pathNode, src)),
		Range: Range{Start: int(node.StartByte()), End: int(node.EndByte())},
	}
}

// lowerRef converts a CST node that is either a bare "identifier" or a
// "qualified_ident" into a *Ref. Qualified idents expose two named
// children: "namespace" (the file-stem prefix) and "name" (the symbol
// within that prefix). Bare identifiers yield a Ref with empty Namespace.
// Returns nil for any other node type — callers should guard against
// CST shape changes.
func lowerRef(node *gotreesitter.Node, lang *gotreesitter.Language, src []byte) *Ref {
	if node == nil {
		return nil
	}
	r := Range{Start: int(node.StartByte()), End: int(node.EndByte())}
	switch node.Type(lang) {
	case "identifier":
		return &Ref{Name: nodeText(node, src), Range: r}
	case "qualified_ident":
		nsNode := node.ChildByFieldName("namespace", lang)
		nmNode := node.ChildByFieldName("name", lang)
		if nsNode == nil || nmNode == nil {
			return nil
		}
		return &Ref{
			Namespace: nodeText(nsNode, src),
			Name:      nodeText(nmNode, src),
			Range:     r,
		}
	}
	return nil
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

	if ovr := node.ChildByFieldName("override", lang); ovr != nil {
		b.Override = lowerOverride(ovr, lang, src)
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

	// nodeText covers both shapes: a bare "identifier" yields its lexeme
	// and a "qualified_ident" yields the verbatim "namespace.name" slice,
	// which is exactly what the From/To string fields are documented to
	// hold. FromRef/ToRef carry the structured form for resolver/lint use.
	e := &Edge{
		From:      nodeText(fromNode, src),
		To:        nodeText(toNode, src),
		FromRef:   lowerRef(fromNode, lang, src),
		ToRef:     lowerRef(toNode, lang, src),
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
	if ovr := node.ChildByFieldName("override", lang); ovr != nil {
		e.Override = lowerOverride(ovr, lang, src)
	}
	if metaNode := node.ChildByFieldName("metadata", lang); metaNode != nil {
		e.Metadata = lowerMetadataBlock(metaNode, lang, src)
	}

	return e
}

// lowerView converts a view_decl CST node into a *ViewDecl. The view
// declaration carries a quoted Name and a `body` block whose children
// are a mix of view_pair directives, layout_decl, and budget_decl
// sub-blocks. Recognized directive keys land in their typed fields;
// anything else is preserved in ViewDecl.Metadata so that future
// additions round-trip cleanly through downstream tools that predate
// them.
func lowerView(node *gotreesitter.Node, lang *gotreesitter.Language, src []byte) *ViewDecl {
	nameNode := node.ChildByFieldName("name", lang)
	bodyNode := node.ChildByFieldName("body", lang)
	if nameNode == nil || bodyNode == nil {
		return nil
	}

	v := &ViewDecl{
		Name:  decodeStringLiteral(nodeText(nameNode, src)),
		Range: Range{Start: int(node.StartByte()), End: int(node.EndByte())},
	}

	for i := 0; i < bodyNode.NamedChildCount(); i++ {
		child := bodyNode.NamedChild(i)
		if child == nil {
			continue
		}
		switch child.Type(lang) {
		case "view_pair":
			lowerViewPair(child, lang, src, v)
		case "layout_decl":
			if l := lowerLayoutDecl(child, lang, src); l != nil {
				v.Layout = l
			}
		case "budget_decl":
			if b := lowerBudgetDecl(child, lang, src); b != nil {
				v.Budget = b
			}
		}
	}

	return v
}

// lowerViewPair dispatches a single view_pair into the appropriate field
// of v. Pairs with keys outside the recognized set are preserved in
// v.Metadata so that callers can round-trip forward-compatible directives
// without losing data.
func lowerViewPair(node *gotreesitter.Node, lang *gotreesitter.Language, src []byte, v *ViewDecl) {
	keyNode := node.ChildByFieldName("key", lang)
	valNode := node.ChildByFieldName("value", lang)
	if keyNode == nil || valNode == nil {
		return
	}
	key := nodeText(keyNode, src)

	switch key {
	case "title":
		if s, ok := lowerStringScalar(valNode, src); ok {
			v.Title = s
		}
	case "preset":
		if s, ok := lowerIdentScalar(valNode, src); ok {
			v.Preset = s
		}
	case "theme":
		if s, ok := lowerIdentScalar(valNode, src); ok {
			v.Theme = s
		}
	case "include":
		if valNode.Type(lang) == "selector_list" {
			v.Include = lowerSelectorList(valNode, lang, src)
		}
	case "expand":
		if valNode.Type(lang) == "list" {
			v.Expand = lowerNameList(valNode, lang, src)
		}
	case "collapse":
		if valNode.Type(lang) == "list" {
			v.Collapse = lowerNameList(valNode, lang, src)
		}
	default:
		// Forward-compat: stash unknown view-pair keys in Metadata so
		// downstream tools can still see the directive. lowerValue accepts
		// the regular value shapes (string / number / identifier / list);
		// selector_list values are intentionally ignored here — unknown
		// pairs that carry selector lists indicate a typo'd directive name
		// and are dropped to surface the typo via the missing field rather
		// than smuggling selectors into Metadata.
		if val, ok := lowerValue(valNode, lang, src); ok {
			if v.Metadata == nil {
				v.Metadata = make(map[string]Value)
			}
			v.Metadata[key] = val
		}
	}
}

// lowerStringScalar returns the decoded string content of a "string" CST
// node. Returns ("", false) when node is any other type so the caller
// can surface the type mismatch (today by silently skipping; future
// lints will diagnose).
func lowerStringScalar(node *gotreesitter.Node, src []byte) (string, bool) {
	if node == nil {
		return "", false
	}
	// We check the node text shape rather than calling Type because this
	// helper is used after a known field lookup where the caller already
	// confirmed the field exists. The leading quote is the unambiguous
	// signal that this is a string literal.
	t := nodeText(node, src)
	if len(t) >= 2 && t[0] == '"' && t[len(t)-1] == '"' {
		return decodeStringLiteral(t), true
	}
	return "", false
}

// lowerIdentScalar returns the lexeme of an identifier-shaped CST node.
// Returns ("", false) when node is nil or its text is empty.
func lowerIdentScalar(node *gotreesitter.Node, src []byte) (string, bool) {
	if node == nil {
		return "", false
	}
	t := nodeText(node, src)
	if t == "" {
		return "", false
	}
	return t, true
}

// lowerSelectorList walks a selector_list CST node and returns the
// expanded slice of Selector values. Unknown selector subtypes (none
// today, but the grammar may grow) are skipped silently.
func lowerSelectorList(node *gotreesitter.Node, lang *gotreesitter.Language, src []byte) []Selector {
	var out []Selector
	for i := 0; i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		if sel, ok := lowerSelector(child, lang, src); ok {
			out = append(out, sel)
		}
	}
	return out
}

// lowerSelector converts one selector CST node into a Selector. The
// node type discriminates among boundary_selector, element_selector,
// and edges_selector; each populates the matching Selector fields. The
// returned bool is false for unrecognized node types so the caller can
// skip them.
func lowerSelector(node *gotreesitter.Node, lang *gotreesitter.Language, src []byte) (Selector, bool) {
	r := Range{Start: int(node.StartByte()), End: int(node.EndByte())}
	switch node.Type(lang) {
	case "boundary_selector":
		target := node.ChildByFieldName("target", lang)
		if target == nil {
			return Selector{}, false
		}
		return Selector{
			Kind:   SelectorBoundary,
			Target: decodeStringLiteral(nodeText(target, src)),
			Range:  r,
		}, true
	case "element_selector":
		kind := node.ChildByFieldName("kind", lang)
		target := node.ChildByFieldName("target", lang)
		if kind == nil || target == nil {
			return Selector{}, false
		}
		return Selector{
			Kind:        SelectorElement,
			ElementKind: elementKindForKeyword(nodeText(kind, src)),
			Target:      decodeStringLiteral(nodeText(target, src)),
			Range:       r,
		}, true
	case "edges_selector":
		target := node.ChildByFieldName("target", lang)
		if target == nil {
			return Selector{}, false
		}
		// The `direction` field's text is the verbatim source slice
		// covering either "incoming to" or "outgoing from" (whitespace
		// between the keywords preserved). We discriminate on the first
		// token; the preposition is fixed per direction by the grammar.
		dirText := ""
		if dirNode := node.ChildByFieldName("direction", lang); dirNode != nil {
			dirText = nodeText(dirNode, src)
		}
		kind := SelectorUnknown
		if strings.HasPrefix(dirText, "incoming") {
			kind = SelectorEdgesIncoming
		} else if strings.HasPrefix(dirText, "outgoing") {
			kind = SelectorEdgesOutgoing
		}
		return Selector{
			Kind:   kind,
			Target: decodeStringLiteral(nodeText(target, src)),
			Range:  r,
		}, true
	}
	return Selector{}, false
}

// lowerNameList extracts the textual names from a value list, preserving
// element order. STRING items are decoded; IDENT items pass through
// verbatim. NUMBER and nested list items are skipped (they make no sense
// in expand/collapse contexts and a Phase 10 lint will surface them).
func lowerNameList(node *gotreesitter.Node, lang *gotreesitter.Language, src []byte) []string {
	var out []string
	for i := 0; i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		switch child.Type(lang) {
		case "string":
			out = append(out, decodeStringLiteral(nodeText(child, src)))
		case "identifier":
			out = append(out, nodeText(child, src))
		}
	}
	return out
}

// lowerLayoutDecl converts a layout_decl CST node into a *LayoutHints.
// Recognized keys ("direction", "pin") land in typed fields; everything
// else lands in LayoutHints.Metadata to round-trip forward-compatible
// hints through downstream tooling.
func lowerLayoutDecl(node *gotreesitter.Node, lang *gotreesitter.Language, src []byte) *LayoutHints {
	body := node.ChildByFieldName("body", lang)
	if body == nil {
		return nil
	}

	l := &LayoutHints{
		Range: Range{Start: int(node.StartByte()), End: int(node.EndByte())},
	}

	for i := 0; i < body.NamedChildCount(); i++ {
		child := body.NamedChild(i)
		if child == nil || child.Type(lang) != "layout_pair" {
			continue
		}
		keyNode := child.ChildByFieldName("key", lang)
		valNode := child.ChildByFieldName("value", lang)
		if keyNode == nil || valNode == nil {
			continue
		}
		key := nodeText(keyNode, src)
		switch key {
		case "direction":
			// Direction is conventionally a kebab atom ("top-down",
			// "left-right") parsed as dashed_ident, but a plain
			// identifier or a quoted string is accepted too so authors
			// who quote everything still round-trip.
			switch valNode.Type(lang) {
			case "identifier", "dashed_ident":
				l.Direction = nodeText(valNode, src)
			case "string":
				l.Direction = decodeStringLiteral(nodeText(valNode, src))
			}
		case "pin":
			if valNode.Type(lang) == "pin_map" {
				l.Pin = lowerPinMap(valNode, lang, src)
			}
		default:
			if val, ok := lowerValue(valNode, lang, src); ok {
				if l.Metadata == nil {
					l.Metadata = make(map[string]Value)
				}
				l.Metadata[key] = val
			}
		}
	}

	return l
}

// lowerPinMap walks a pin_map CST node and returns a map of name → side.
// Both halves of each pin_pair are required by the grammar; nil/missing
// fields cause the pair to be skipped silently.
func lowerPinMap(node *gotreesitter.Node, lang *gotreesitter.Language, src []byte) map[string]string {
	out := make(map[string]string)
	for i := 0; i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child == nil || child.Type(lang) != "pin_pair" {
			continue
		}
		nameNode := child.ChildByFieldName("name", lang)
		sideNode := child.ChildByFieldName("side", lang)
		if nameNode == nil || sideNode == nil {
			continue
		}
		out[decodeStringLiteral(nodeText(nameNode, src))] = nodeText(sideNode, src)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// lowerBudgetDecl converts a budget_decl CST node into a *Budget. Each
// recognized key maps to a typed int field via strconv.ParseFloat → int
// (the grammar permits the same number shape as metadata values, so
// floats are accepted but truncated; Phase 10 will lint fractional
// budget values). Unrecognized keys are dropped here — budgets have a
// fixed schema and forward-compat for new caps will require a code
// change anyway, so silently ignoring is preferable to smuggling typo'd
// keys into a downstream consumer that won't enforce them.
func lowerBudgetDecl(node *gotreesitter.Node, lang *gotreesitter.Language, src []byte) *Budget {
	body := node.ChildByFieldName("body", lang)
	if body == nil {
		return nil
	}

	b := &Budget{
		Range: Range{Start: int(node.StartByte()), End: int(node.EndByte())},
	}

	for i := 0; i < body.NamedChildCount(); i++ {
		child := body.NamedChild(i)
		if child == nil || child.Type(lang) != "budget_pair" {
			continue
		}
		keyNode := child.ChildByFieldName("key", lang)
		valNode := child.ChildByFieldName("value", lang)
		if keyNode == nil || valNode == nil {
			continue
		}
		// The grammar restricts budget_pair values to `number`, which
		// only ever lexes a strconv.ParseFloat-compatible literal, so
		// this err path is dead by construction. We keep the guard for
		// defense-in-depth in case the grammar is loosened later.
		n, err := strconv.ParseFloat(nodeText(valNode, src), 64)
		if err != nil {
			continue
		}
		switch nodeText(keyNode, src) {
		case "nodes":
			b.Nodes = int(n)
		case "edges_per_node":
			b.EdgesPerNode = int(n)
		case "depth":
			b.Depth = int(n)
		case "boundary_fanout":
			b.BoundaryFanout = int(n)
		case "label_chars":
			b.LabelChars = int(n)
		}
	}

	return b
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

	if ovr := node.ChildByFieldName("override", lang); ovr != nil {
		e.Override = lowerOverride(ovr, lang, src)
	}

	if body := node.ChildByFieldName("body", lang); body != nil {
		e.Metadata = lowerMetadataBlock(body, lang, src)
	}

	return e
}

// lowerOverride converts an override_annotation CST node into an *Override.
// Three CST shapes map to three distinct modes:
//
//   - bare `@override`             → OverrideAll,    Fields empty
//   - `@override(label, tags)`     → OverrideFields, Fields = ["label","tags"]
//   - `@override(except: src_ref)` → OverrideExcept, Fields = ["src_ref"]
//
// The discriminator is the presence of the `except` keyword in the args.
// An args group with no `except` (and a possibly empty field list) is
// treated as OverrideFields; the Phase 10 lint will flag literally-empty
// `@override()` because it is indistinguishable from OverrideAll in the IR.
func lowerOverride(node *gotreesitter.Node, lang *gotreesitter.Language, src []byte) *Override {
	o := &Override{
		Range: Range{Start: int(node.StartByte()), End: int(node.EndByte())},
	}

	args := node.ChildByFieldName("args", lang)
	if args == nil {
		o.Mode = OverrideAll
		return o
	}

	if args.ChildByFieldName("except", lang) != nil {
		o.Mode = OverrideExcept
	} else {
		o.Mode = OverrideFields
	}

	if fields := args.ChildByFieldName("fields", lang); fields != nil {
		for i := 0; i < fields.NamedChildCount(); i++ {
			ident := fields.NamedChild(i)
			if ident == nil {
				continue
			}
			// The override_field_list rule contains only identifier nodes
			// (CommaSep1 of `identifier`). Anything else is a CST shape we
			// did not generate — skip to stay forward-compatible.
			if ident.Type(lang) != "identifier" {
				continue
			}
			o.Fields = append(o.Fields, nodeText(ident, src))
		}
	}

	return o
}

// lowerMetadataBlock turns a metadata_block CST node into a
// map[string]Value. Returns nil both when the block is absent (the
// caller-side ChildByFieldName guard never invokes us in that case) and
// when the block has no pairs. We collapse "no metadata declared" and
// "empty metadata block" to the same nil result because empty maps are
// not semantically distinguished by downstream consumers, and the
// canonical nil shape keeps round-trip diffs minimal. Shared by
// lowerElement, lowerBoundary, lowerEdge.
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

// decodeStringLiteral strips the surrounding quotes and resolves the Go
// backslash escape set produced by strconv.Quote — including \xHH, \uHHHH,
// \UHHHHHHHH, and \NNN octal forms — so values round-trip symmetrically
// with the printer's writeStringLiteral.
//
// strconv.Unquote handles every escape the printer can emit. If unquoting
// fails (e.g. an unsupported escape from a hand-written source), we fall
// back to a permissive scan that mirrors the historical behavior: known
// single-char escapes are decoded, unknown escapes pass the trailing byte
// through verbatim.
func decodeStringLiteral(lit string) string {
	if len(lit) < 2 || lit[0] != '"' || lit[len(lit)-1] != '"' {
		return lit
	}
	if s, err := strconv.Unquote(lit); err == nil {
		return s
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
