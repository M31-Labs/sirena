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
package sirena
