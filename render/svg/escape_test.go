package svg

import (
	"html"
	"strings"
	"testing"
)

func TestEscape_AmpLtGt(t *testing.T) {
	if got := escapeText("a<b>c&d"); got != "a&lt;b&gt;c&amp;d" {
		t.Errorf("escapeText = %q, want a&lt;b&gt;c&amp;d", got)
	}
}

func TestEscape_QuotesInAttribute(t *testing.T) {
	if got := escapeAttr(`"'`); got != "&quot;&#39;" {
		t.Errorf("escapeAttr = %q, want &quot;&#39;", got)
	}
}

func TestEscape_AdversarialLabel(t *testing.T) {
	in := `a"><script>alert(1)</script>`
	for _, esc := range []string{escapeText(in), escapeAttr(in)} {
		if strings.Contains(esc, "<script>") {
			t.Errorf("literal <script> survived escaping: %q", esc)
		}
		if strings.Contains(esc, "<") || strings.Contains(esc, ">") {
			t.Errorf("raw angle bracket survived escaping: %q", esc)
		}
	}
	if got := html.UnescapeString(escapeAttr(in)); got != in {
		t.Errorf("attribute escaping is not round-trippable: got %q, want %q", got, in)
	}
}

func TestEscape_ControlCharacters(t *testing.T) {
	in := "a\x00b\x08c\nd"
	got := escapeText(in)
	if strings.ContainsRune(got, 0x00) || strings.ContainsRune(got, 0x08) {
		t.Errorf("control characters leaked into output: %q", got)
	}
	if !strings.ContainsRune(got, '�') {
		t.Errorf("expected replacement character for control bytes: %q", got)
	}
	if !strings.Contains(got, "&#10;") {
		t.Errorf("newline should be encoded as &#10;: %q", got)
	}
}
