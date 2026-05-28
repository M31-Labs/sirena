package sirena

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
)

// Print serializes a *Document back to canonical sirena source bytes.
// The output is deterministic: same input → same bytes. Print is the
// formatter — there is no separate formatter contract.
//
// Round-trip law: Parse(Print(doc)) yields a structurally-equal *Document.
// Format determinism: Print's output matches what `sirena fmt` produces.
// Both laws are gated by the Phase 12 conformance corpus.
//
// Canonical output rules:
//   - Two-space indent per nesting level; trailing newline on the document.
//   - Metadata keys sorted lexically within each block.
//   - Empty blocks elide the body entirely (`service api`); empty boundary
//     bodies render as `{}` with no inner whitespace.
//   - Top-level declarations are separated by exactly one blank line.
//   - String values are emitted with strconv.Quote so reparsing through
//     decodeStringLiteral recovers the original Go-encoded value.
//   - Floats with no fractional part print as integer literals; otherwise
//     Go's %g default applies.
//   - @override annotations sit between the name and the optional metadata
//     block; field lists are emitted in alphabetical order.
func Print(doc *Document) ([]byte, error) {
	if doc == nil {
		return nil, fmt.Errorf("SIR-PRINT-001: nil document")
	}
	p := &printer{buf: &bytes.Buffer{}}
	if err := p.writeDocument(doc); err != nil {
		return nil, err
	}
	return p.buf.Bytes(), nil
}

// Format parses and reprints in one call. Equivalent to Print(MustParse(src))
// but error-safe — returns the parse error rather than panicking. Format is
// idempotent: Format(Format(x)) == Format(x).
func Format(src []byte) ([]byte, error) {
	doc, err := Parse(src)
	if err != nil {
		return nil, err
	}
	return Print(doc)
}

// printer carries the in-progress output buffer plus the current
// indentation depth. The methods on it produce canonical text;
// callers should not interact with the buffer directly.
type printer struct {
	buf   *bytes.Buffer
	depth int
}

// indent writes the current depth's worth of two-space units.
func (p *printer) indent() {
	for i := 0; i < p.depth; i++ {
		p.buf.WriteString("  ")
	}
}

// writeDocument serializes the document as: imports first (declaration
// order), then each system's contents in the source-order field grouping
// (elements, then boundaries, then edges), then views. Top-level
// declarations are separated by exactly one blank line. The output ends
// with one trailing newline.
//
// Order rationale: the parser produces one SystemDecl per source file and
// groups children by Kind (Elements/Boundaries/Edges) without recording
// interleaved source order. Emitting Elements → Boundaries → Edges is a
// stable canonical choice; the round-trip property only requires that
// reparsing yields an equivalent IR, not that the byte stream matches the
// original source. Phase 12's conformance corpus will exercise this.
func (p *printer) writeDocument(doc *Document) error {
	first := true

	emitSep := func() {
		if !first {
			p.buf.WriteString("\n")
		}
		first = false
	}

	for _, imp := range doc.Imports {
		emitSep()
		p.writeImport(imp)
	}

	for _, sys := range doc.Systems {
		for _, e := range sys.Elements {
			emitSep()
			if err := p.writeElement(e); err != nil {
				return err
			}
		}
		for _, b := range sys.Boundaries {
			emitSep()
			if err := p.writeBoundary(b); err != nil {
				return err
			}
		}
		for _, e := range sys.Edges {
			emitSep()
			if err := p.writeEdge(e); err != nil {
				return err
			}
		}
	}

	for _, v := range doc.Views {
		emitSep()
		if err := p.writeView(v); err != nil {
			return err
		}
	}

	// Empty documents still get a trailing newline; tools that diff
	// against `\n` see a consistent shape.
	if first {
		p.buf.WriteString("\n")
	}

	return nil
}

// writeImport emits a single import directive on its own line.
func (p *printer) writeImport(imp *Import) {
	p.indent()
	p.buf.WriteString("import ")
	p.writeStringLiteral(imp.Path)
	p.buf.WriteString("\n")
}

// writeElement emits an element declaration, optionally followed by an
// `@override` annotation and a metadata block. Elements with no metadata
// render as a single line (`service api`); elements with metadata get the
// `{ key: value }` form on subsequent lines with sorted keys.
func (p *printer) writeElement(e *Element) error {
	p.indent()
	p.buf.WriteString(e.Kind.String())
	p.buf.WriteString(" ")
	p.buf.WriteString(e.Name)
	if e.Override != nil {
		p.writeOverride(e.Override)
	}
	if len(e.Metadata) > 0 {
		p.buf.WriteString(" {\n")
		p.depth++
		if err := p.writeSortedMetadata(e.Metadata); err != nil {
			return err
		}
		p.depth--
		p.indent()
		p.buf.WriteString("}")
	}
	p.buf.WriteString("\n")
	return nil
}

// writeBoundary emits a boundary declaration. The optional metadata block
// (boundary-level pairs) precedes the body block. An empty body renders as
// `{}` with no inner whitespace; a body with children renders multiline
// with children indented one level deeper.
func (p *printer) writeBoundary(b *Boundary) error {
	p.indent()
	p.buf.WriteString("boundary ")
	p.buf.WriteString(b.Kind.String())
	p.buf.WriteString(" ")
	p.writeStringLiteral(b.Name)
	if b.Override != nil {
		p.writeOverride(b.Override)
	}
	if len(b.Metadata) > 0 {
		p.buf.WriteString(" {\n")
		p.depth++
		if err := p.writeSortedMetadata(b.Metadata); err != nil {
			return err
		}
		p.depth--
		p.indent()
		p.buf.WriteString("}")
	}
	if len(b.Children) == 0 {
		p.buf.WriteString(" {}\n")
		return nil
	}
	p.buf.WriteString(" {\n")
	p.depth++
	for _, child := range b.Children {
		if err := p.writeNode(child); err != nil {
			return err
		}
	}
	p.depth--
	p.indent()
	p.buf.WriteString("}\n")
	return nil
}

// writeNode dispatches a boundary child to its writer.
func (p *printer) writeNode(n Node) error {
	switch x := n.(type) {
	case *Element:
		return p.writeElement(x)
	case *Boundary:
		return p.writeBoundary(x)
	case *Edge:
		return p.writeEdge(x)
	default:
		return fmt.Errorf("SIR-PRINT-002: unknown Node type %T", n)
	}
}

// writeEdge emits an edge declaration. The verbatim From/To strings are
// used so qualified identifiers (e.g. "platform.kafka") round-trip without
// reconstruction. EdgeKindFlow is the untyped fallback and prints with no
// trailing `: kind`; any other kind emits `: kind` optionally followed by
// a quoted label.
func (p *printer) writeEdge(e *Edge) error {
	p.indent()
	p.buf.WriteString(e.From)
	p.buf.WriteString(" ")
	p.buf.WriteString(e.Direction.String())
	p.buf.WriteString(" ")
	p.buf.WriteString(e.To)
	if e.Override != nil {
		p.writeOverride(e.Override)
	}
	// EdgeKindFlow is the untyped fallback — omit the `: kind` suffix to
	// match the source author's untyped declaration. Typed kinds emit the
	// suffix and optionally a label.
	if e.Kind != EdgeKindUnknown && e.Kind != EdgeKindFlow {
		p.buf.WriteString(": ")
		p.buf.WriteString(e.Kind.String())
		if e.Label != "" {
			p.buf.WriteString(" ")
			p.writeStringLiteral(e.Label)
		}
	}
	if len(e.Metadata) > 0 {
		p.buf.WriteString(" {\n")
		p.depth++
		if err := p.writeSortedMetadata(e.Metadata); err != nil {
			return err
		}
		p.depth--
		p.indent()
		p.buf.WriteString("}")
	}
	p.buf.WriteString("\n")
	return nil
}

// writeView emits a view declaration: title, preset, include selectors,
// expand/collapse lists, theme, layout sub-block, budget sub-block, and
// any forward-compatible metadata pairs. Recognized directives are
// emitted in a fixed canonical order (independent of source order) so
// reprinting is deterministic.
//
// Order: title, preset, include, expand, collapse, theme, layout,
// budget, then any other metadata pairs in sorted key order.
func (p *printer) writeView(v *ViewDecl) error {
	p.indent()
	p.buf.WriteString("view ")
	p.writeStringLiteral(v.Name)

	// Determine whether the view body has any content. An empty body
	// elides everything to `{}` to match the empty-boundary convention.
	empty := v.Title == "" && v.Preset == "" && v.Theme == "" &&
		len(v.Include) == 0 && len(v.Expand) == 0 && len(v.Collapse) == 0 &&
		v.Layout == nil && v.Budget == nil && len(v.Metadata) == 0
	if empty {
		p.buf.WriteString(" {}\n")
		return nil
	}

	p.buf.WriteString(" {\n")
	p.depth++

	if v.Title != "" {
		p.indent()
		p.buf.WriteString("title: ")
		p.writeStringLiteral(v.Title)
		p.buf.WriteString("\n")
	}
	if v.Preset != "" {
		p.indent()
		p.buf.WriteString("preset: ")
		p.buf.WriteString(v.Preset)
		p.buf.WriteString("\n")
	}
	if len(v.Include) > 0 {
		p.writeSelectorList(v.Include)
	}
	if len(v.Expand) > 0 {
		p.indent()
		p.buf.WriteString("expand: ")
		p.writeNameList(v.Expand)
		p.buf.WriteString("\n")
	}
	if len(v.Collapse) > 0 {
		p.indent()
		p.buf.WriteString("collapse: ")
		p.writeNameList(v.Collapse)
		p.buf.WriteString("\n")
	}
	if v.Theme != "" {
		p.indent()
		p.buf.WriteString("theme: ")
		p.buf.WriteString(v.Theme)
		p.buf.WriteString("\n")
	}
	if v.Layout != nil {
		if err := p.writeLayout(v.Layout); err != nil {
			return err
		}
	}
	if v.Budget != nil {
		p.writeBudget(v.Budget)
	}
	if len(v.Metadata) > 0 {
		if err := p.writeSortedMetadata(v.Metadata); err != nil {
			return err
		}
	}

	p.depth--
	p.indent()
	p.buf.WriteString("}\n")
	return nil
}

// writeSelectorList emits the include list with one selector per line
// inside `[ ]`. The selectors appear in declaration order — they are a
// curated rendering instruction, not a metadata bag, so reordering would
// surprise authors.
func (p *printer) writeSelectorList(sels []Selector) {
	p.indent()
	p.buf.WriteString("include: [\n")
	p.depth++
	for _, s := range sels {
		p.indent()
		p.writeSelector(s)
		p.buf.WriteString("\n")
	}
	p.depth--
	p.indent()
	p.buf.WriteString("]\n")
}

// writeSelector emits a single selector line. element_selector uses the
// ElementKind keyword recovered from the discriminator; edges_selector
// reconstructs the `incoming to` / `outgoing from` preposition pair.
func (p *printer) writeSelector(s Selector) {
	switch s.Kind {
	case SelectorBoundary:
		p.buf.WriteString("boundary ")
		p.writeStringLiteral(s.Target)
	case SelectorElement:
		p.buf.WriteString(s.ElementKind.String())
		p.buf.WriteString(" ")
		p.writeStringLiteral(s.Target)
	case SelectorEdgesIncoming:
		p.buf.WriteString("edges: incoming to ")
		p.writeStringLiteral(s.Target)
	case SelectorEdgesOutgoing:
		p.buf.WriteString("edges: outgoing from ")
		p.writeStringLiteral(s.Target)
	default:
		// SelectorUnknown is a parse-time safety net; emit a comment-like
		// marker so the malformed shape surfaces in `sirena fmt` output
		// rather than vanishing silently. We don't have comments yet, so
		// the placeholder is an unrecognized identifier that the parser
		// will reject — a deliberately loud failure mode.
		p.buf.WriteString("unknown_selector")
	}
}

// writeNameList emits a comma-separated bracketed list of bare names.
// Used for expand/collapse, whose entries are element/boundary names.
// Quoted strings are emitted as quoted; identifiers as bare — the parse
// path preserved one or the other based on source shape, but the IR
// stores both as plain Go strings. We emit them quoted whenever they
// contain a character outside the identifier alphabet so reparsing
// reaches the same IR shape.
func (p *printer) writeNameList(names []string) {
	p.buf.WriteString("[")
	for i, n := range names {
		if i > 0 {
			p.buf.WriteString(", ")
		}
		p.writeBareOrQuoted(n)
	}
	p.buf.WriteString("]")
}

// writeBareOrQuoted writes name as a bare identifier when it matches the
// identifier syntax sirena's lexer accepts; otherwise quotes it. Used
// wherever the IR stores a name that may have come from either a bare
// or quoted source token — pin keys, pin values, and name lists.
func (p *printer) writeBareOrQuoted(name string) {
	if isBareIdent(name) {
		p.buf.WriteString(name)
	} else {
		p.writeStringLiteral(name)
	}
}

// writeLayout emits the layout sub-block with direction first, then the
// pin map (sorted by name), then any forward-compatible metadata pairs
// (sorted alphabetically).
func (p *printer) writeLayout(l *LayoutHints) error {
	p.indent()
	p.buf.WriteString("layout {\n")
	p.depth++
	if l.Direction != "" {
		p.indent()
		p.buf.WriteString("direction: ")
		p.buf.WriteString(l.Direction)
		p.buf.WriteString("\n")
	}
	if len(l.Pin) > 0 {
		p.indent()
		p.buf.WriteString("pin: { ")
		keys := make([]string, 0, len(l.Pin))
		for k := range l.Pin {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			if i > 0 {
				p.buf.WriteString(", ")
			}
			// Pin keys are typed as STRING in the grammar (pin_pair), so we
			// always quote them. Values are IDENT in the grammar, so we
			// route them through writeBareOrQuoted for symmetry with the
			// other bare-or-quoted name surfaces.
			p.writeStringLiteral(k)
			p.buf.WriteString(": ")
			p.writeBareOrQuoted(l.Pin[k])
		}
		p.buf.WriteString(" }\n")
	}
	if len(l.Metadata) > 0 {
		if err := p.writeSortedMetadata(l.Metadata); err != nil {
			return err
		}
	}
	p.depth--
	p.indent()
	p.buf.WriteString("}\n")
	return nil
}

// writeBudget emits the budget sub-block. Fields are emitted in the fixed
// canonical order: nodes, edges_per_node, depth, boundary_fanout,
// label_chars. A zero value is the unset signal — those caps are omitted
// from the output so reparsing produces a budget with the same zero
// fields.
func (p *printer) writeBudget(b *Budget) {
	p.indent()
	p.buf.WriteString("budget {\n")
	p.depth++
	type pair struct {
		key string
		val int
	}
	fields := []pair{
		{"nodes", b.Nodes},
		{"edges_per_node", b.EdgesPerNode},
		{"depth", b.Depth},
		{"boundary_fanout", b.BoundaryFanout},
		{"label_chars", b.LabelChars},
	}
	for _, f := range fields {
		if f.val == 0 {
			continue
		}
		p.indent()
		p.buf.WriteString(f.key)
		p.buf.WriteString(": ")
		p.buf.WriteString(strconv.Itoa(f.val))
		p.buf.WriteString("\n")
	}
	p.depth--
	p.indent()
	p.buf.WriteString("}\n")
}

// writeOverride emits an `@override` annotation with the canonical
// surrounding whitespace (leading space, no trailing space). Bare
// OverrideAll has no parens; OverrideFields lists fields alphabetically;
// OverrideExcept renders `(except: f1, f2)` with the same field sort.
func (p *printer) writeOverride(o *Override) {
	p.buf.WriteString(" @override")
	switch o.Mode {
	case OverrideAll:
		// no args
	case OverrideFields:
		p.buf.WriteString("(")
		fields := append([]string(nil), o.Fields...)
		sort.Strings(fields)
		for i, f := range fields {
			if i > 0 {
				p.buf.WriteString(", ")
			}
			p.buf.WriteString(f)
		}
		p.buf.WriteString(")")
	case OverrideExcept:
		p.buf.WriteString("(except: ")
		fields := append([]string(nil), o.Fields...)
		sort.Strings(fields)
		for i, f := range fields {
			if i > 0 {
				p.buf.WriteString(", ")
			}
			p.buf.WriteString(f)
		}
		p.buf.WriteString(")")
	}
}

// writeSortedMetadata writes the key/value pairs from a metadata map in
// lexically-sorted key order, each on its own indented line. The caller
// is responsible for the surrounding braces and indent management.
func (p *printer) writeSortedMetadata(m map[string]Value) error {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		p.indent()
		p.buf.WriteString(k)
		p.buf.WriteString(": ")
		if err := p.writeValue(m[k]); err != nil {
			return err
		}
		p.buf.WriteString("\n")
	}
	return nil
}

// writeValue dispatches on the concrete Value variant. The closed sum
// type guarantees the switch is exhaustive at the IR level; an unknown
// runtime concrete returns an error rather than panicking so a future
// IR evolution surfaces at the printer boundary.
func (p *printer) writeValue(v Value) error {
	switch x := v.(type) {
	case String:
		p.writeStringLiteral(x.Value)
	case Number:
		p.buf.WriteString(formatNumber(x.Value))
	case Ident:
		p.buf.WriteString(x.Value)
	case List:
		p.buf.WriteString("[")
		for i, item := range x.Values {
			if i > 0 {
				p.buf.WriteString(", ")
			}
			if err := p.writeValue(item); err != nil {
				return err
			}
		}
		p.buf.WriteString("]")
	default:
		return fmt.Errorf("SIR-PRINT-003: unknown Value type %T", v)
	}
	return nil
}

// writeStringLiteral emits a Go-style double-quoted string. strconv.Quote
// chooses the escapes our parser's decodeStringLiteral understands —
// `\"`, `\\`, `\n`, `\r`, `\t`, plus `\x` / `\u` for control codepoints.
// Phase 8 may need to switch to a custom encoder if non-ASCII characters
// in labels round-trip awkwardly; today the conformance corpus does not
// exercise that surface.
func (p *printer) writeStringLiteral(s string) {
	p.buf.WriteString(strconv.Quote(s))
}

// formatNumber renders a float64 the way `sirena fmt` should: integer
// shape when the value has no fractional part, Go's default %g otherwise.
// `%g` avoids stray trailing zeros (`3.14`, not `3.140000`).
func formatNumber(f float64) string {
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// isBareIdent reports whether s would lex as an `identifier` token under
// the v0.1 grammar — i.e. matches [A-Za-z_][A-Za-z0-9_]*. Used by the
// expand/collapse name list to decide between bare and quoted emission.
func isBareIdent(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		isLetter := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
		isDigit := c >= '0' && c <= '9'
		if i == 0 {
			if !isLetter {
				return false
			}
			continue
		}
		if !isLetter && !isDigit {
			return false
		}
	}
	return true
}
