// Package sirena implements a modernized diagram language and renderer.
//
// Sirena targets arch/systems diagrams as its v0.1 wedge, with a general
// foundation for additional diagram types (sequence, state, ER) in future
// versions. The parser is generated from a tree-sitter grammar compiled
// via gotreesitter's grammargen; the IR is typed; the printer is the only
// formatter. Workspace + view evaluation enable scaling to org-sized
// topologies through curated views.
//
// The design spec lives at hypha://m31labs/sirena (M31 Labs internal).
//
// # Public IR shape rationale (Phase 13 freeze sign-off)
//
// Every public struct field in ast.go satisfies at least one of four
// planned consumer groups. The IR was reviewed field-by-field against
// these consumers before tagging v0.0.1-internal.
//
//   - LSP — needs Range on every node for diagnostics, jump-to-def,
//     find-refs, and hover. Range therefore appears on Document,
//     SystemDecl, Element, Boundary, Edge, Import, Ref, ViewDecl,
//     Selector, LayoutHints, Budget, and Override.
//   - Layout (Plan 2) — needs Metadata for hints (label, owner, tags),
//     Kind enums for default shapes/glyphs, and the structural hierarchy
//     (SystemDecl → Boundary → Children) for region nesting. ViewDecl's
//     Layout, Include, Expand, Collapse, and Budget drive curated-view
//     layout.
//   - Renderers (SVG + GoSX island) — need everything: positioned IR
//     plus Metadata for hover/click affordances and ViewDecl.Theme +
//     Preset for visual style.
//   - Agent emission (.gen.sir round-trip) — needs SID (carried via
//     Metadata), source_ref / prior_source_ref / symbol_kind (Metadata
//     fields), and Override mode (Element.Override / Boundary.Override
//     / Edge.Override) for field-scoped human edits to survive
//     regeneration.
//
// # Freeze gate criteria (Phase 13)
//
//  1. Grammar passes the conformance corpus (87 cases under
//     testdata/conformance/).
//  2. Parse(Print(doc)) structurally round-trips for every corpus case
//     (TestConformance_RoundTripPerCase) and for the hand-written
//     samples (TestRoundTrip_Law).
//  3. Format(input.sir) matches fmt.golden.sir byte-for-byte for every
//     corpus case (TestConformance_FormatDeterminism), and the
//     printer is idempotent.
//  4. Public IR struct fields reviewed against all four consumer
//     groups above.
//
// As of v0.0.1-internal these criteria are met. The IR is FROZEN:
// subsequent Phase 2-6 plans (layout, renderers, LSP, mdpp integration)
// may extend the IR only through additive, backward-compatible changes
// (new optional fields, new enum values). Field removals and signature
// changes require a major-version bump.
package sirena
