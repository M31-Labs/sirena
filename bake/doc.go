// Package bake implements sirena's markdown fence scanner and rewriter.
//
// The top-level entry point, Bake, finds diagram fences in a markdown file,
// renders each to SVG, and rewrites the fence to a committed-image link with
// the source preserved for idempotent re-bake.
//
// Scan is the detection layer: it parses markdown bytes line-by-line and
// returns an ordered list of [Block] values — one per detected diagram region.
// It is deliberately free of rendering and mdpp dependencies so it can be
// tested and reused in isolation.
//
// Two block kinds are detected:
//   - [KindRaw]: a backtick fence whose info-string language token is
//     "mermaid", "sir", or "sirena".
//   - [KindBaked]: an already-baked block starting with a bodyless
//     <!-- sirena-baked:<lang> --> marker, followed by an image link and a
//     <details> block whose ```text fence holds the original source.
//
// Nesting rule: any backtick fence whose language is not a target language
// opens an opaque block — lines inside are skipped until the matching close.
// Tilde (~~~) fences are not recognised in v1.
package bake
