// Package font is sirena's bundled text-measurement and glyph-outline
// source for the SVG renderer.
//
// The renderer draws labels as vector <path> elements rather than
// <text>, so a rendered SVG carries no dependency on the viewer having
// any particular font installed and reproduces byte-for-byte everywhere.
// To do that it needs the glyph outlines and advance widths up front:
// those are baked into glyphs.go as path data in font (em) units, keyed
// by rune.
//
// # The bundled font
//
// v0.0.2-internal bundles a subset of Inter (https://rsms.me/inter),
// licensed under the SIL Open Font License 1.1 (see LICENSE-Inter). The
// subset covers printable ASCII (U+0020–U+007E), the Latin-1 supplement
// (U+00A0–U+00FF), and a handful of typographic punctuation marks. The
// generator is font-agnostic; the project may re-bake from any OFL font
// (e.g. Source Sans Pro, or the locally-available Liberation Sans, also
// OFL 1.1) by re-running it with a different -ttf.
//
// # Regenerating glyphs.go
//
// glyphs.go is generated, committed, and never hand-edited. To rebuild
// it from a TrueType/OpenType font:
//
//	go run ./render/svg/font/generate -ttf render/svg/font/generate/Inter-Regular.ttf -out render/svg/font/glyphs.go
//
// The generator (render/svg/font/generate) parses the font with
// golang.org/x/image/font/sfnt and emits each glyph's outline as an SVG
// path in font units. sfnt emits outlines in SVG's Y-down orientation
// (ascenders are negative Y), so the renderer scales by fontSize/EmSize
// without a Y flip. x/image is a generator-only dependency — the runtime font
// package imports only the baked data.
package font
