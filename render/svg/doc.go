// Package svg renders a sirena layout to deterministic, self-contained
// SVG.
//
// Three properties define the output:
//
//   - Self-contained. Labels are drawn as bundled-font glyph <path>
//     elements, never <text>, so a rendered file needs no fonts at view
//     time and cannot leak user strings into markup (every character
//     becomes geometry). Colors come from an allowlisted set of
//     CSS-variable design tokens declared inline.
//   - Deterministic. Elements emit in a fixed order (boundaries by depth
//     then position, nodes by position, edges by endpoint identity) from
//     a bytes.Buffer with no templates, so the same layout always yields
//     byte-identical SVG. The conformance corpus hashes the bytes to
//     guard this.
//   - Safe. User text is escaped (escape.go) wherever it could reach
//     markup, and only allowlisted tokens (theme.go) may emit as CSS
//     variables.
package svg
