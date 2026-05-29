package font

import (
	"strings"
	"testing"
)

func TestGlyph_LookupASCII(t *testing.T) {
	g, ok := Glyphs['A']
	if !ok {
		t.Fatal("Glyphs['A'] missing from the bundled subset")
	}
	if g.Advance <= 0 {
		t.Errorf("'A' advance = %v, want > 0", g.Advance)
	}
	if !strings.HasPrefix(g.Path, "M") {
		t.Errorf("'A' path should start with a moveto; got %.12q", g.Path)
	}
}

func TestGlyph_FallbackMissing(t *testing.T) {
	if _, ok := Glyphs['☃']; ok {
		t.Error("snowman unexpectedly present in the subset")
	}
	if Lookup('☃') != Glyphs['?'] {
		t.Error("Lookup of a missing rune should fall back to the '?' glyph")
	}
}

func TestAdvance_NonZero(t *testing.T) {
	g := Glyphs['a']
	frac := g.Advance / EmSize
	if frac < 0.45 || frac > 0.75 {
		t.Errorf("'a' advance is %v em (%.2f of EmSize); want ~0.5–0.7", g.Advance, frac)
	}
}
