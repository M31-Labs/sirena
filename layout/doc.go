// Package layout is sirena's deterministic layout engine.
//
// It turns a *sirena.ResolvedView (the filtered, summarized, preset-
// decorated projection produced by view evaluation) into a populated
// *sirena.LayoutResult: absolute positions for every node, summary, and
// boundary, plus orthogonal routes for every edge.
//
// The engine is deterministic for a given (ResolvedView, sirena version)
// pair. The layered preset is fully deterministic; the force-directed
// preset seeds its RNG from sirena.ViewHash(rv) so it, too, reproduces
// byte-for-byte across runs.
//
// # Layering and the import direction
//
// This package imports the root sirena package for the frozen IR types
// (*Element, *Edge, *Boundary, *BoundarySummary, *ResolvedView) and for
// the geometry value types (sirena.Point, sirena.Rect, placements, edge
// routes) that LayoutResult embeds. The dependency points one way:
// layout → root, never the reverse. The root sirena.Render entrypoint
// delegates to this package through a function variable that this
// package's init() registers via sirena.RegisterLayoutComputer; that
// indirection is what keeps the root package free of any import of
// layout (which would cycle).
//
// # Pipeline
//
// The layered preset runs, per boundary scope:
//
//	rank   — longest-path layer assignment (rank.go)
//	order  — median-heuristic crossing reduction (order.go)
//	coord  — Brandes-Köpf x-coordinate assignment (coord.go)
//
// then routes edges orthogonally (edges.go), arranges top-level
// boundaries with the skeleton solver (skeleton.go), and translates
// every per-scope placement into absolute coordinates. The force preset
// (force.go) replaces the cell pipeline with a seeded Fruchterman-
// Reingold relaxation.
package layout
