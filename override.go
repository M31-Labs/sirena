// Package sirena: round-trip mechanism for code-generated diagrams.
//
// This file implements the agent-emission machinery that lets a code-
// understanding agent regenerate a `.gen.sir` file across renames,
// reorganizations, and hand-authored curation without losing the identity
// of previously-emitted declarations.
//
// The three concerns are split across three entrypoints:
//
//   - EmitSIDs assigns ULID `sid:` metadata to every Element / Boundary /
//     Edge that lacks one. Existing sids are preserved. Used by generators
//     so every fresh emission has a stable identity key.
//
//   - Reconcile matches a fresh emission against a prior one and returns
//     per-element IdentityResolutions describing how each new declaration
//     was matched. Precedence: SID > exact source_ref > fuzzy
//     (symbol_kind + qualified-name suffix) > new.
//
//   - MergeOverride applies a hand-authored @override declaration's policy
//     against the generator-emitted twin. Field-scoped, mode-aware: All,
//     Fields, or Except.
//
// The dangling-override diagnostic — emitted when a hand declaration
// references a name no .gen.sir declares — lives in
// ValidateOverridesAgainstWorkspace below.
package sirena

import (
	"crypto/rand"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

// EmissionOptions configures the SID emitter and the round-trip resolver.
//
// All fields are optional. Tests inject deterministic Clock and Entropy so
// fixtures stay stable; production code passes a zero EmissionOptions and
// gets the wall clock + crypto/rand.
type EmissionOptions struct {
	// Clock is the time source for ULID generation; defaults to time.Now.
	Clock func() time.Time
	// Entropy is the random source for ULID; defaults to crypto/rand.Reader.
	Entropy io.Reader
}

// IdentityResolution describes how a new declaration was matched against a
// prior set. Used by Reconcile to produce diff-friendly outputs.
type IdentityResolution struct {
	// SID is the identity carried forward to the new declaration. It is
	// either the prior element's sid (on any match) or the new element's
	// own sid (on no match).
	SID string
	// PriorSourceRef is the prior declaration's source_ref when the source_ref
	// changed across the regeneration. Empty when source_ref was unchanged or
	// the element is new. Generators write this back into the new
	// `.gen.sir` so reviewers can see the rename history once; the next
	// clean regeneration drops it.
	PriorSourceRef string
	// MatchedVia records the heuristic that produced the match. Used by
	// diagnostic and explain pipelines to surface why two declarations
	// were treated as the same identity.
	MatchedVia IdentityMatch
}

// IdentityMatch discriminates how Reconcile matched a new declaration to a
// prior one. The zero value is IdentityMatchNew so an unset resolution
// surfaces as a fresh element rather than as a silent false match.
type IdentityMatch int

const (
	// IdentityMatchNew indicates no prior element matched; the new
	// declaration is a fresh identity.
	IdentityMatchNew IdentityMatch = iota
	// IdentityMatchBySID indicates the prior and new declarations carry the
	// same `sid:` metadata value — the strongest match.
	IdentityMatchBySID
	// IdentityMatchBySourceRef indicates the prior and new declarations
	// carry the same exact `source_ref:` value — used when the generator
	// hasn't yet filled `sid:` on the new emission.
	IdentityMatchBySourceRef
	// IdentityMatchByFuzzy indicates a fuzzy match on `symbol_kind` and the
	// qualified-name suffix of `source_ref` — the rename-tracking heuristic.
	IdentityMatchByFuzzy
)

// String returns a stable short label for the match, used in diagnostic
// output and explain pipelines.
func (m IdentityMatch) String() string {
	switch m {
	case IdentityMatchNew:
		return "new"
	case IdentityMatchBySID:
		return "sid"
	case IdentityMatchBySourceRef:
		return "source_ref"
	case IdentityMatchByFuzzy:
		return "fuzzy"
	default:
		return "unknown"
	}
}

// EmitSIDs walks doc and assigns a ULID `sid:` metadata value to every
// Element / Boundary / Edge that lacks one. Existing sids are preserved
// unchanged. Generators producing `.gen.sir` files should call EmitSIDs
// after constructing the Document and before Print; the resolver uses sid
// as the round-trip identity key independent of source_ref drift.
//
// opts.Clock defaults to time.Now; opts.Entropy defaults to crypto/rand.Reader.
// Default behavior is non-deterministic by design; tests override both.
//
// EmitSIDs returns an error only when ulid.New fails (entropy exhaustion).
// A nil doc is a no-op.
func EmitSIDs(doc *Document, opts EmissionOptions) error {
	if doc == nil {
		return nil
	}
	clock := opts.Clock
	if clock == nil {
		clock = time.Now
	}
	entropy := opts.Entropy
	if entropy == nil {
		entropy = rand.Reader
	}

	assign := func(md *map[string]Value) error {
		if *md == nil {
			*md = map[string]Value{}
		}
		if _, has := (*md)["sid"]; has {
			return nil
		}
		id, err := ulid.New(ulid.Timestamp(clock()), entropy)
		if err != nil {
			return fmt.Errorf("sirena: EmitSIDs: ulid.New: %w", err)
		}
		(*md)["sid"] = String{Value: id.String()}
		return nil
	}

	for _, sys := range doc.Systems {
		if sys == nil {
			continue
		}
		for _, e := range sys.Elements {
			if e == nil {
				continue
			}
			if err := assign(&e.Metadata); err != nil {
				return err
			}
		}
		for _, b := range sys.Boundaries {
			if err := walkBoundarySIDs(b, assign); err != nil {
				return err
			}
		}
		for _, ed := range sys.Edges {
			if ed == nil {
				continue
			}
			if err := assign(&ed.Metadata); err != nil {
				return err
			}
		}
	}
	return nil
}

// walkBoundarySIDs assigns a sid to b itself and recurses through its
// Element / Boundary / Edge children so deeply-nested declarations also
// participate in round-trip identity tracking.
func walkBoundarySIDs(b *Boundary, assign func(*map[string]Value) error) error {
	if b == nil {
		return nil
	}
	if err := assign(&b.Metadata); err != nil {
		return err
	}
	for _, child := range b.Children {
		switch c := child.(type) {
		case *Element:
			if c == nil {
				continue
			}
			if err := assign(&c.Metadata); err != nil {
				return err
			}
		case *Boundary:
			if err := walkBoundarySIDs(c, assign); err != nil {
				return err
			}
		case *Edge:
			if c == nil {
				continue
			}
			if err := assign(&c.Metadata); err != nil {
				return err
			}
		}
	}
	return nil
}

// Reconcile matches each new element against the prior set and returns
// per-element identity resolutions in the same order as newElements.
//
// Precedence: SID > exact source_ref > fuzzy > new. A single prior element
// is consumed by at most one resolution; the first new element to match a
// given prior wins, so generators emitting in stable order produce stable
// resolutions.
//
// priorElements is the previously-emitted set (e.g. read from an existing
// `.gen.sir`); newElements is the regenerator's fresh output. Both slices
// are inspected by metadata; pointers are not mutated.
func Reconcile(priorElements, newElements []*Element, opts EmissionOptions) []IdentityResolution {
	_ = opts // reserved for future entropy-aware fallback when assigning sids to new elements

	resolutions := make([]IdentityResolution, len(newElements))
	claimed := make([]bool, len(priorElements))

	for i, ne := range newElements {
		if ne == nil {
			resolutions[i] = IdentityResolution{MatchedVia: IdentityMatchNew}
			continue
		}
		newSID := metaString(ne.Metadata, "sid")
		newSourceRef := metaString(ne.Metadata, "source_ref")
		newSymbolKind := metaString(ne.Metadata, "symbol_kind")

		// 1) SID match — highest precedence.
		if newSID != "" {
			if idx := findSIDMatch(priorElements, claimed, newSID); idx >= 0 {
				priorSourceRef := metaString(priorElements[idx].Metadata, "source_ref")
				res := IdentityResolution{
					SID:        newSID,
					MatchedVia: IdentityMatchBySID,
				}
				if priorSourceRef != "" && priorSourceRef != newSourceRef {
					res.PriorSourceRef = priorSourceRef
				}
				claimed[idx] = true
				resolutions[i] = res
				continue
			}
		}

		// 2) Exact source_ref match — carry sid forward, no prior_source_ref.
		if newSourceRef != "" {
			if idx := findSourceRefMatch(priorElements, claimed, newSourceRef); idx >= 0 {
				priorSID := metaString(priorElements[idx].Metadata, "sid")
				resolutions[i] = IdentityResolution{
					SID:        priorSID,
					MatchedVia: IdentityMatchBySourceRef,
				}
				claimed[idx] = true
				continue
			}
		}

		// 3) Fuzzy match — same symbol_kind and qualified-name suffix.
		if newSymbolKind != "" && newSourceRef != "" {
			if idx := findFuzzyMatch(priorElements, claimed, newSymbolKind, qualifiedNameSuffix(newSourceRef)); idx >= 0 {
				priorSID := metaString(priorElements[idx].Metadata, "sid")
				priorSourceRef := metaString(priorElements[idx].Metadata, "source_ref")
				resolutions[i] = IdentityResolution{
					SID:            priorSID,
					PriorSourceRef: priorSourceRef,
					MatchedVia:     IdentityMatchByFuzzy,
				}
				claimed[idx] = true
				continue
			}
		}

		// 4) New element — keep whatever sid was already present.
		resolutions[i] = IdentityResolution{
			SID:        newSID,
			MatchedVia: IdentityMatchNew,
		}
	}
	return resolutions
}

// findSIDMatch returns the index in prior of the first unclaimed element
// whose `sid:` metadata equals target, or -1 if none match.
func findSIDMatch(prior []*Element, claimed []bool, target string) int {
	for i, p := range prior {
		if p == nil || claimed[i] {
			continue
		}
		if metaString(p.Metadata, "sid") == target {
			return i
		}
	}
	return -1
}

// findSourceRefMatch returns the index in prior of the first unclaimed
// element whose `source_ref:` metadata equals target exactly, or -1.
func findSourceRefMatch(prior []*Element, claimed []bool, target string) int {
	for i, p := range prior {
		if p == nil || claimed[i] {
			continue
		}
		if metaString(p.Metadata, "source_ref") == target {
			return i
		}
	}
	return -1
}

// findFuzzyMatch returns the index in prior of the first unclaimed element
// whose `symbol_kind` matches symbolKind AND whose `source_ref`'s
// qualified-name suffix matches suffix, or -1 if none match.
//
// The fuzzy match is the rename-tracking heuristic: a function moved into
// a different package keeps its symbol name, so the suffix after the final
// `.` or `#` survives. Combined with `symbol_kind` (function, type, ...)
// it produces few false positives in practice.
func findFuzzyMatch(prior []*Element, claimed []bool, symbolKind, suffix string) int {
	if suffix == "" {
		return -1
	}
	for i, p := range prior {
		if p == nil || claimed[i] {
			continue
		}
		if metaString(p.Metadata, "symbol_kind") != symbolKind {
			continue
		}
		if qualifiedNameSuffix(metaString(p.Metadata, "source_ref")) == suffix {
			return i
		}
	}
	return -1
}

// qualifiedNameSuffix returns the trailing identifier of a source_ref URI,
// e.g. `code://go/foo/bar#FunName` → `FunName`. When no `#` is present
// falls back to the last `.`-separated segment. Returns the empty string
// for an empty input.
func qualifiedNameSuffix(sourceRef string) string {
	if sourceRef == "" {
		return ""
	}
	if i := strings.LastIndex(sourceRef, "#"); i >= 0 {
		return sourceRef[i+1:]
	}
	if i := strings.LastIndex(sourceRef, "."); i >= 0 {
		return sourceRef[i+1:]
	}
	return sourceRef
}

// metaString returns the unwrapped string value at key, or "" if missing
// or not a String. Safe on a nil map.
func metaString(md map[string]Value, key string) string {
	if md == nil {
		return ""
	}
	v, ok := md[key]
	if !ok {
		return ""
	}
	s, ok := v.(String)
	if !ok {
		return ""
	}
	return s.Value
}

// MergeOverride applies a hand-authored override declaration's policy
// against the generator-emitted twin. The two inputs are read-only; the
// returned *Element has a freshly-allocated Metadata map and inherits
// hand's Kind, Name, and Override.
//
// Modes (see OverrideMode):
//
//   - OverrideAll: hand wins wholesale; generator metadata is dropped.
//   - OverrideFields: result starts from gen's metadata; each field named
//     in hand.Override.Fields is overwritten by hand's value (if present).
//   - OverrideExcept: result starts from hand's metadata; each field named
//     in hand.Override.Fields is overwritten by gen's value (if present).
//
// When hand.Override is nil, MergeOverride returns hand unchanged — there
// is nothing to merge. When gen is nil and hand declares OverrideAll or
// OverrideFields with nothing to pull from, hand's metadata is returned.
func MergeOverride(gen, hand *Element) *Element {
	if hand == nil {
		return nil
	}
	if hand.Override == nil {
		return hand
	}

	merged := &Element{
		Kind:     hand.Kind,
		Name:     hand.Name,
		Override: hand.Override,
		Range:    hand.Range,
		Metadata: map[string]Value{},
	}

	switch hand.Override.Mode {
	case OverrideAll:
		// Hand wins wholesale.
		for k, v := range hand.Metadata {
			merged.Metadata[k] = v
		}
	case OverrideFields:
		// Start from gen; overlay listed fields from hand.
		if gen != nil {
			for k, v := range gen.Metadata {
				merged.Metadata[k] = v
			}
		}
		for _, name := range hand.Override.Fields {
			if v, ok := hand.Metadata[name]; ok {
				merged.Metadata[name] = v
			}
		}
	case OverrideExcept:
		// Start from hand; overlay listed fields from gen.
		for k, v := range hand.Metadata {
			merged.Metadata[k] = v
		}
		if gen != nil {
			for _, name := range hand.Override.Fields {
				if v, ok := gen.Metadata[name]; ok {
					merged.Metadata[name] = v
				} else {
					// Field listed in Except but not on gen: drop the
					// hand-authored value so the result reflects the
					// generator's authority over the field.
					delete(merged.Metadata, name)
				}
			}
		}
	default:
		// Unknown mode — fall back to hand's metadata. The validator
		// should already have rejected this case upstream.
		for k, v := range hand.Metadata {
			merged.Metadata[k] = v
		}
	}
	return merged
}

// ValidateOverridesAgainstWorkspace returns SIR-OVERRIDE-DANGLING for any
// hand-authored declaration whose @override annotation references a name
// no generator-owned .gen.sir file declares.
//
// The check is workspace-scoped: every FileKindSystem file's elements and
// boundaries with Override != nil are matched by Name against every
// FileKindGenSystem file's elements and boundaries. Edges are excluded —
// edges have no Name and the spec describes overrides on element /
// boundary declarations.
//
// A nil workspace returns nil. Files that fail to load are silently
// skipped — the underlying load error is reported through the workspace's
// own diagnostic surface, not duplicated here.
func ValidateOverridesAgainstWorkspace(ws *Workspace) []Diagnostic {
	if ws == nil {
		return nil
	}

	// Build the set of names declared in any .gen.sir file.
	genNames := map[string]bool{}
	for _, f := range ws.Files {
		if f.Kind != FileKindGenSystem {
			continue
		}
		doc, err := ws.LoadDocument(f)
		if err != nil || doc == nil {
			continue
		}
		collectGenNames(doc, genNames)
	}

	// Walk every hand-authored file and flag overrides with no gen match.
	var diags []Diagnostic
	for _, f := range ws.Files {
		if f.Kind != FileKindSystem {
			continue
		}
		doc, err := ws.LoadDocument(f)
		if err != nil || doc == nil {
			continue
		}
		diags = append(diags, walkDanglingOverrides(doc, genNames)...)
	}
	return diags
}

// collectGenNames recurses through a generator-owned document and records
// every Element and Boundary name in into.
func collectGenNames(doc *Document, into map[string]bool) {
	for _, sys := range doc.Systems {
		if sys == nil {
			continue
		}
		for _, e := range sys.Elements {
			if e != nil {
				into[e.Name] = true
			}
		}
		for _, b := range sys.Boundaries {
			collectGenNamesFromBoundary(b, into)
		}
	}
}

// collectGenNamesFromBoundary records b's name and recurses through its
// children so nested declarations also count as declared.
func collectGenNamesFromBoundary(b *Boundary, into map[string]bool) {
	if b == nil {
		return
	}
	into[b.Name] = true
	for _, child := range b.Children {
		switch c := child.(type) {
		case *Element:
			if c != nil {
				into[c.Name] = true
			}
		case *Boundary:
			collectGenNamesFromBoundary(c, into)
		}
	}
}

// walkDanglingOverrides walks a hand-authored document and emits a
// diagnostic for every Element / Boundary with a non-nil Override whose
// Name is absent from genNames.
func walkDanglingOverrides(doc *Document, genNames map[string]bool) []Diagnostic {
	var diags []Diagnostic
	for _, sys := range doc.Systems {
		if sys == nil {
			continue
		}
		for _, e := range sys.Elements {
			if e == nil || e.Override == nil {
				continue
			}
			if !genNames[e.Name] {
				diags = append(diags, danglingDiagnostic(e.Name, e.Override.Range))
			}
		}
		for _, b := range sys.Boundaries {
			diags = append(diags, walkBoundaryOverrides(b, genNames)...)
		}
	}
	return diags
}

// walkBoundaryOverrides emits a dangling diagnostic for b itself when
// applicable, then recurses through children.
func walkBoundaryOverrides(b *Boundary, genNames map[string]bool) []Diagnostic {
	if b == nil {
		return nil
	}
	var diags []Diagnostic
	if b.Override != nil && !genNames[b.Name] {
		diags = append(diags, danglingDiagnostic(b.Name, b.Override.Range))
	}
	for _, child := range b.Children {
		switch c := child.(type) {
		case *Element:
			if c == nil || c.Override == nil {
				continue
			}
			if !genNames[c.Name] {
				diags = append(diags, danglingDiagnostic(c.Name, c.Override.Range))
			}
		case *Boundary:
			diags = append(diags, walkBoundaryOverrides(c, genNames)...)
		}
	}
	return diags
}

// danglingDiagnostic constructs the canonical SIR-OVERRIDE-DANGLING entry
// for a hand-authored @override with no matching gen declaration.
func danglingDiagnostic(name string, rng Range) Diagnostic {
	return Diagnostic{
		Code:     "SIR-OVERRIDE-DANGLING",
		Severity: SeverityWarning,
		Message:  fmt.Sprintf("@override on %q has no matching declaration in any .gen.sir file in the workspace", name),
		Range:    rng,
	}
}
