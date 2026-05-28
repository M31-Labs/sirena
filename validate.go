package sirena

import (
	"fmt"
	"regexp"
)

// wellKnownOverrideFields lists the v0.1 field names that are always valid
// targets of an @override(...) annotation. Anything not in this set must be
// present as a metadata key on the same declaration; otherwise it's flagged
// as SIR-VALIDATE-OVERRIDE-UNKNOWN-FIELD.
//
// The set spans the structural fields of Element / Boundary / Edge plus the
// two well-known metadata keys that the v0.1 spec carves out (`url`,
// `description`) so authors don't need a metadata block just to declare an
// override target.
var wellKnownOverrideFields = map[string]bool{
	"kind":             true,
	"name":             true,
	"label":            true,
	"tags":             true,
	"owner":            true,
	"source_ref":       true,
	"sid":              true,
	"prior_source_ref": true,
	"metadata":         true,
	"url":              true,
	"description":      true,
}

// sourceRefPattern is the canonical v0.1 source_ref / prior_source_ref URI
// shape: code://<lang>/<module>#<symbol>. The lang segment forbids `/` so
// the module segment is unambiguous; the symbol segment after `#` must be
// non-empty. Reused for both source_ref and prior_source_ref checks.
var sourceRefPattern = regexp.MustCompile(`^code://[^/]+/[^#]+#.+$`)

// Validate runs the v0.1 structural checks over doc and returns every
// diagnostic discovered. Validate never mutates doc; a nil doc returns nil.
//
// The v0.1 rules are intentionally narrow:
//
//   - SIR-VALIDATE-DUP-NAME: a name (element or boundary) redeclared in the
//     same scope. Scope = direct parent (SystemDecl or Boundary). Edges have
//     no name and are exempt.
//   - SIR-VALIDATE-OVERRIDE-UNKNOWN-FIELD: an @override(field, ...) list
//     names a field that is neither a well-known v0.1 field (see
//     wellKnownOverrideFields) nor a metadata key declared on the same
//     declaration.
//   - SIR-VALIDATE-SID-COLLISION: the same sid metadata value reused across
//     two declarations within one document. Workspace-wide collision checks
//     run at resolve-time (Phase 5).
//   - SIR-VALIDATE-BAD-SOURCE-REF: a source_ref or prior_source_ref string
//     that does not match the canonical code://<lang>/<module>#<symbol> URI.
//
// Edge-target resolution is deferred to the resolver (Phase 5) — Validate
// does not check whether From/To identifiers actually point to declared
// elements.
func Validate(doc *Document) []Diagnostic {
	if doc == nil {
		return nil
	}
	v := &validator{
		sidFirst: map[string]Range{},
	}
	for _, sys := range doc.Systems {
		v.walkSystem(sys)
	}
	return v.diags
}

// validator carries the cross-cutting state Validate accumulates as it
// walks the document: the running diagnostic list and the sid-first-seen
// map for the per-document sid collision check.
type validator struct {
	diags    []Diagnostic
	sidFirst map[string]Range
}

// walkSystem checks the direct children of a SystemDecl: top-level elements
// and boundaries share one name scope; edges are unnamed. Boundaries recurse
// via walkBoundary to check their own scope independently.
func (v *validator) walkSystem(sys *SystemDecl) {
	if sys == nil {
		return
	}
	seen := map[string]bool{}
	for _, e := range sys.Elements {
		if seen[e.Name] {
			v.dupName(e.Name, e.Range)
		} else {
			seen[e.Name] = true
		}
		v.checkElement(e)
	}
	for _, b := range sys.Boundaries {
		if seen[b.Name] {
			v.dupName(b.Name, b.Range)
		} else {
			seen[b.Name] = true
		}
		v.walkBoundary(b)
	}
	for _, e := range sys.Edges {
		v.checkEdge(e)
	}
}

// walkBoundary checks a boundary's own decl-level rules (override,
// sid, source_ref) then recurses through its children, treating the
// children list as a fresh name scope independent of the parent.
func (v *validator) walkBoundary(b *Boundary) {
	if b == nil {
		return
	}
	v.checkOverride(b.Override, b.Metadata)
	v.checkSid(b.Metadata, b.Range)
	v.checkSourceRefs(b.Metadata, b.Range)

	seen := map[string]bool{}
	for _, child := range b.Children {
		switch c := child.(type) {
		case *Element:
			if seen[c.Name] {
				v.dupName(c.Name, c.Range)
			} else {
				seen[c.Name] = true
			}
			v.checkElement(c)
		case *Boundary:
			if seen[c.Name] {
				v.dupName(c.Name, c.Range)
			} else {
				seen[c.Name] = true
			}
			v.walkBoundary(c)
		case *Edge:
			v.checkEdge(c)
		}
	}
}

// checkElement runs the per-decl rules that apply to a single Element:
// override field whitelist, sid collision, source_ref shape.
func (v *validator) checkElement(e *Element) {
	if e == nil {
		return
	}
	v.checkOverride(e.Override, e.Metadata)
	v.checkSid(e.Metadata, e.Range)
	v.checkSourceRefs(e.Metadata, e.Range)
}

// checkEdge runs the per-decl rules that apply to an Edge. Edges have no
// name, so the dup-name rule is skipped; everything else mirrors the
// element checks.
func (v *validator) checkEdge(e *Edge) {
	if e == nil {
		return
	}
	v.checkOverride(e.Override, e.Metadata)
	v.checkSid(e.Metadata, e.Range)
	v.checkSourceRefs(e.Metadata, e.Range)
}

// checkOverride emits SIR-VALIDATE-OVERRIDE-UNKNOWN-FIELD for every field
// in the override's Fields list that is neither well-known nor present as
// a metadata key on the same declaration. Both OverrideFields and
// OverrideExcept lists are validated identically — the named fields must
// be recognizable in either mode.
func (v *validator) checkOverride(o *Override, md map[string]Value) {
	if o == nil {
		return
	}
	for _, f := range o.Fields {
		if wellKnownOverrideFields[f] {
			continue
		}
		if _, ok := md[f]; ok {
			continue
		}
		v.diags = append(v.diags, Diagnostic{
			Code:     "SIR-VALIDATE-OVERRIDE-UNKNOWN-FIELD",
			Severity: SeverityError,
			Message:  fmt.Sprintf("@override field %q is not a known v0.1 field and is not a metadata key on this declaration", f),
			Range:    o.Range,
		})
	}
}

// checkSid records the (sid → range) of the first occurrence and emits
// SIR-VALIDATE-SID-COLLISION on every subsequent occurrence. Non-String
// sid values are silently ignored — the grammar already constrains the
// shape, and a non-String would be a parser bug, not a sid collision.
func (v *validator) checkSid(md map[string]Value, declRange Range) {
	if md == nil {
		return
	}
	val, ok := md["sid"]
	if !ok {
		return
	}
	s, ok := val.(String)
	if !ok {
		return
	}
	if first, dup := v.sidFirst[s.Value]; dup {
		v.diags = append(v.diags, Diagnostic{
			Code:     "SIR-VALIDATE-SID-COLLISION",
			Severity: SeverityError,
			Message:  fmt.Sprintf("sid %q reused (first declared at byte %d)", s.Value, first.Start),
			Range:    declRange,
		})
		return
	}
	v.sidFirst[s.Value] = declRange
}

// checkSourceRefs emits SIR-VALIDATE-BAD-SOURCE-REF for any source_ref or
// prior_source_ref metadata value that does not match the canonical URI
// shape. Non-String values are skipped for the same reason as sid: the
// grammar already constrains the shape.
func (v *validator) checkSourceRefs(md map[string]Value, declRange Range) {
	if md == nil {
		return
	}
	for _, key := range [...]string{"source_ref", "prior_source_ref"} {
		val, ok := md[key]
		if !ok {
			continue
		}
		s, ok := val.(String)
		if !ok {
			continue
		}
		if sourceRefPattern.MatchString(s.Value) {
			continue
		}
		v.diags = append(v.diags, Diagnostic{
			Code:     "SIR-VALIDATE-BAD-SOURCE-REF",
			Severity: SeverityError,
			Message:  fmt.Sprintf("%s %q does not match canonical form code://<lang>/<module>#<symbol>", key, s.Value),
			Range:    declRange,
		})
	}
}

// dupName appends a SIR-VALIDATE-DUP-NAME diagnostic pinned at the second
// declaration's range. The first declaration's location is intentionally
// not embedded in the message for v0.1 — a richer Phase 9 diagnostic will
// link the two sites; the simple form here keeps the rule self-contained.
func (v *validator) dupName(name string, declRange Range) {
	v.diags = append(v.diags, Diagnostic{
		Code:     "SIR-VALIDATE-DUP-NAME",
		Severity: SeverityError,
		Message:  fmt.Sprintf("name %q redeclared in scope", name),
		Range:    declRange,
	})
}
