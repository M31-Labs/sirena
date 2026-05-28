# Changelog

All notable changes to sirena will be documented in this file.

## Unreleased

## v0.0.1-internal — 2026-05-28

First internal release. IR frozen per ADR 0001.

### Added

- Tree-sitter grammar (gotreesitter-compiled) covering element/boundary/edge declarations, qualified identifier refs, view declarations with layout hints + budgets, `@override` annotations.
- Typed IR with byte-accurate `Range` provenance on every node.
- Canonical printer with round-trip + format-determinism laws.
- Workspace + symbol-table resolver, edge-target binding, fence-mode synthesis (self-contained / workspace-resolving).
- View evaluation pipeline (selection → collapse → expand → preset render defaults) and `ViewHash` for deterministic layout caching.
- Budget evaluator with four suggestion kinds and strict/permissive `Render` entrypoint.
- Round-trip mechanism for code-generated diagrams: ULID `EmitSIDs`, `Reconcile` with SID > source_ref > fuzzy > new precedence, `MergeOverride` for field-scoped overrides, `ReconcileWithOrphans` with configurable grace period.
- Diagnostic plumbing: `Document.Diagnostics()`, lint sub-package with 6 rules covering kind violations, refs, overrides, orphans, version skew.
- CLI: `sirena parse [--json]`, `sirena fmt [-w] [--check]`, `sirena lint`, `sirena new system|view`.
- 87-case conformance corpus with IR + format goldens.
