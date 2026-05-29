package font

// Glyph is one bundled glyph: an SVG path "d" string in font (em) units
// with the font's native Y-up coordinate system, plus the horizontal
// advance in the same units.
type Glyph struct {
	Path    string
	Advance float64
}

// fallbackRune is substituted for any rune absent from the subset.
const fallbackRune = '?'

// Lookup returns the glyph for r, falling back to '?' when r is not in
// the bundled subset. If even '?' is missing (an incomplete subset), the
// zero Glyph is returned.
func Lookup(r rune) Glyph {
	if g, ok := Glyphs[r]; ok {
		return g
	}
	return Glyphs[fallbackRune]
}

// Measure returns the total advance width of text in font (em) units,
// summing each rune's glyph advance (with '?' fallback). Divide by
// EmSize and multiply by a font size to get rendered width.
func Measure(text string) float64 {
	var w float64
	for _, r := range text {
		w += Lookup(r).Advance
	}
	return w
}
