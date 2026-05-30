// Package mermaid ingests a Mermaid flowchart/graph diagram and lowers it
// into a typed *sirena.Document, which the existing layout engine and SVG
// renderer turn into a fully rendered diagram.
//
// It is the "absorb Mermaid" side of the absorb-and-transcend story: teams
// that already maintain Mermaid diagrams can import them into sirena as a
// starting point, gaining typed boundaries, inference-promoted element kinds,
// and the full sirena render + lint pipeline — without rewriting the source by
// hand.
//
// Parse is the primary entry point. It accepts a raw Mermaid source byte slice
// and returns a *sirena.Document alongside any diagnostics. The Options struct
// enables opt-in inference (--infer) that promotes unambiguous Mermaid shapes
// (cylinder→Database) and edge labels to typed sirena kinds. Styling directives
// (classDef, style, linkStyle, click) are silently dropped with a
// SIR-MERMAID-STYLE-DROPPED warning; they have no structural equivalent in
// the sirena IR.
//
// The legacy "graph" keyword (e.g. "graph TD;") is transparently rewritten to
// "flowchart TD" before parsing, so real-world Mermaid fences that predate the
// "flowchart" keyword work without modification.
//
// Non-flowchart diagram types (sequenceDiagram, classDiagram, etc.) return a
// SIR-MERMAID-NOT-A-GRAPH error diagnostic and a nil document.
package mermaid
