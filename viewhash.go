// Package sirena: view hashing.
//
// viewhash.go produces a 32-byte SHA-256 over the inputs that should
// affect rendering for a ResolvedView and deliberately omits the inputs
// that shouldn't. The hash is consumed by:
//
//   - The layout cache, so identical (Document, View) pairs skip relayout.
//   - The renderer's deterministic RNG seeding (spec invariant 8).
//   - Plan 2's incremental layout, which keys cached node positions on
//     the view hash + element id pair.
//
// The contract is asymmetric on purpose:
//
//	Included (changes the hash):
//	  - selected element identifiers (sid or name) — sorted
//	  - selected boundary identifiers (sid or name) — sorted
//	  - edge endpoints + kind + direction — sorted canonical form
//	  - explicit expand / collapse lists — sorted
//	  - preset name
//	  - theme name
//	  - layout direction and pin map — sorted by key
//	  - budget fields (each as int)
//
//	Excluded (does NOT change the hash):
//	  - element / boundary labels, owners, tags (render-time)
//	  - source ranges (byte offsets)
//	  - view title (cosmetic)
//	  - metadata maps on the view itself (forward-compat catch-all)
//
// The encoding is canonical: every section is field-tagged and length-
// prefixed so neighbouring values cannot collide ("ab" + "c" vs "a" +
// "bc"). This is the same trick the manifest hash uses; we keep the
// helpers private here rather than sharing a util package because the
// section names are tightly coupled to the ResolvedView shape.
package sirena

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"sort"
)

// ViewHash returns a 32-byte SHA-256 hash that is deterministic for the
// inputs that should affect rendering and stable against the inputs
// that shouldn't. See the package-level doc above for the exact
// included/excluded sets. A nil ResolvedView hashes to SHA-256 of the
// empty byte slice (a well-known nonzero value) so callers can rely on
// the result being deterministic even on degenerate input.
func ViewHash(rv *ResolvedView) [32]byte {
	if rv == nil {
		return sha256.Sum256(nil)
	}

	var buf bytes.Buffer

	writeSection(&buf, "elements", sortedElementIDs(rv))
	writeSection(&buf, "boundaries", sortedBoundaryIDs(rv))
	writeSection(&buf, "edges", canonicalEdges(rv))
	writeSection(&buf, "expand", sortedStrings(viewSourceExpand(rv)))
	writeSection(&buf, "collapse", sortedStrings(viewSourceCollapse(rv)))
	writeStringField(&buf, "preset", viewSourcePreset(rv))
	writeStringField(&buf, "theme", viewSourceTheme(rv))
	writeLayoutSection(&buf, viewSourceLayout(rv))
	writeBudgetSection(&buf, viewSourceBudget(rv))

	return sha256.Sum256(buf.Bytes())
}

// sortedElementIDs returns the included elements' identifiers in
// ascending order. Identifier preference is sid metadata first (since
// sids are intended to be stable across renames), falling back to the
// declared name. The sid/name prefix keeps the two namespaces distinct
// so a collision (an element named "x" vs an element with sid "x") does
// not silently merge.
func sortedElementIDs(rv *ResolvedView) []string {
	ids := make([]string, 0, len(rv.Elements))
	for _, e := range rv.Elements {
		ids = append(ids, elementID(e))
	}
	sort.Strings(ids)
	return ids
}

// sortedBoundaryIDs mirrors sortedElementIDs for boundaries. Collapsed
// boundaries appear in ResolvedView.Summaries rather than Boundaries
// and are folded in here so the projection's structure stays in the
// hash — two views with the same elements but different collapse sets
// must hash differently even when the visible Boundaries slice happens
// to match.
func sortedBoundaryIDs(rv *ResolvedView) []string {
	ids := make([]string, 0, len(rv.Boundaries)+len(rv.Summaries))
	for _, b := range rv.Boundaries {
		ids = append(ids, boundaryID(b))
	}
	for _, s := range rv.Summaries {
		if s == nil || s.Boundary == nil {
			continue
		}
		ids = append(ids, "summary:"+boundaryID(s.Boundary))
	}
	sort.Strings(ids)
	return ids
}

// elementID picks the most stable identifier available: sid metadata
// when present, else the declared name. The "sid:" / "name:" prefix
// keeps the two namespaces from colliding.
func elementID(e *Element) string {
	if e == nil {
		return "name:"
	}
	if sid := readSID(e.Metadata); sid != "" {
		return "sid:" + sid
	}
	return "name:" + e.Name
}

// boundaryID mirrors elementID for boundaries.
func boundaryID(b *Boundary) string {
	if b == nil {
		return "name:"
	}
	if sid := readSID(b.Metadata); sid != "" {
		return "sid:" + sid
	}
	return "name:" + b.Name
}

// readSID pulls the "sid" string out of a metadata map. Returns the
// empty string when the key is absent, the value is not a String, or
// the String is itself empty. Sid metadata is the planned rename-stable
// identifier scheme; until parsers populate it, the fallback to name
// handles all current views.
func readSID(md map[string]Value) string {
	if md == nil {
		return ""
	}
	v, ok := md["sid"]
	if !ok {
		return ""
	}
	if s, ok := v.(String); ok {
		return s.Value
	}
	return ""
}

// canonicalEdges flattens each edge to "from|to|kind|direction" and
// sorts the result. Sorting on the full canonical form (rather than on
// just from/to) keeps two edges with the same endpoints but different
// kinds — e.g. api reads db and api writes db — from being treated as
// one entry.
func canonicalEdges(rv *ResolvedView) []string {
	out := make([]string, 0, len(rv.Edges))
	for _, e := range rv.Edges {
		if e == nil {
			continue
		}
		out = append(out, fmt.Sprintf("%s|%s|%s|%s", e.From, e.To, e.Kind.String(), e.Direction.String()))
	}
	sort.Strings(out)
	return out
}

// sortedStrings returns a sorted copy of in. The copy avoids mutating
// the caller's slice (rv.Source.Expand / rv.Source.Collapse come
// straight from the parsed AST and must stay in source order for the
// printer).
func sortedStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	sort.Strings(out)
	return out
}

// writeSection emits a tagged, length-prefixed list of strings. Each
// item is prefixed with its byte length so adjacent strings cannot
// concatenate ambiguously. The section header carries the item count
// so an empty section is distinct from a missing section.
func writeSection(buf *bytes.Buffer, tag string, items []string) {
	fmt.Fprintf(buf, "[%s:%d]", tag, len(items))
	for _, s := range items {
		fmt.Fprintf(buf, "<%d>%s", len(s), s)
	}
}

// writeStringField emits a single tagged string value with its length
// prefix. Used for the scalar inputs (preset, theme) that don't form a
// list.
func writeStringField(buf *bytes.Buffer, tag, v string) {
	fmt.Fprintf(buf, "[%s:%d]%s", tag, len(v), v)
}

// writeLayoutSection emits layout hints in a canonical form: the
// direction string and the pin map serialised as sorted "key=value"
// strings. A nil LayoutHints serialises to a literal "[layout:nil]"
// marker so absent layout is distinct from layout-with-empty-fields.
func writeLayoutSection(buf *bytes.Buffer, l *LayoutHints) {
	if l == nil {
		fmt.Fprintf(buf, "[layout:nil]")
		return
	}
	writeStringField(buf, "layout.direction", l.Direction)
	pins := make([]string, 0, len(l.Pin))
	for k, v := range l.Pin {
		pins = append(pins, k+"="+v)
	}
	sort.Strings(pins)
	writeSection(buf, "layout.pin", pins)
}

// writeBudgetSection emits the five budget caps as a single pipe-
// separated tuple. A nil Budget serialises to "[budget:nil]" so the
// absent-vs-zero distinction (a *Budget with all zero fields is "every
// cap is zero", not "no budget") is preserved.
func writeBudgetSection(buf *bytes.Buffer, b *Budget) {
	if b == nil {
		fmt.Fprintf(buf, "[budget:nil]")
		return
	}
	fmt.Fprintf(buf, "[budget:%d|%d|%d|%d|%d]",
		b.Nodes, b.EdgesPerNode, b.Depth, b.BoundaryFanout, b.LabelChars)
}

// viewSourceExpand, viewSourceCollapse, viewSourcePreset, viewSourceTheme,
// viewSourceLayout, viewSourceBudget guard against a nil rv.Source.
// EvaluateView always populates Source today, but ViewHash is part of
// the public API and a downstream caller building a ResolvedView by
// hand should still get a sensible hash.
func viewSourceExpand(rv *ResolvedView) []string {
	if rv == nil || rv.Source == nil {
		return nil
	}
	return rv.Source.Expand
}

func viewSourceCollapse(rv *ResolvedView) []string {
	if rv == nil || rv.Source == nil {
		return nil
	}
	return rv.Source.Collapse
}

func viewSourcePreset(rv *ResolvedView) string {
	if rv == nil || rv.Source == nil {
		return ""
	}
	return rv.Source.Preset
}

func viewSourceTheme(rv *ResolvedView) string {
	if rv == nil || rv.Source == nil {
		return ""
	}
	return rv.Source.Theme
}

func viewSourceLayout(rv *ResolvedView) *LayoutHints {
	if rv == nil || rv.Source == nil {
		return nil
	}
	return rv.Source.Layout
}

func viewSourceBudget(rv *ResolvedView) *Budget {
	if rv == nil || rv.Source == nil {
		return nil
	}
	return rv.Source.Budget
}
