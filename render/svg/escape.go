package svg

import "strings"

// escapeText escapes a string for use as SVG element text content: the
// XML metacharacters &, <, > are entity-encoded, control characters are
// dropped or replaced, and whitespace control chars are numeric-encoded.
func escapeText(s string) string { return escape(s, false) }

// escapeAttr escapes a string for use inside a double-quoted attribute
// value. In addition to escapeText's rules it encodes both quote
// characters so the value cannot break out of its attribute.
func escapeAttr(s string) string { return escape(s, true) }

// escape is the shared implementation. It never depends on encoding/xml,
// which does not suit SVG attribute strings; the dispatch table below is
// the single source of truth for what crosses into output.
func escape(s string, attr bool) string {
	var b strings.Builder
	b.Grow(len(s) + len(s)/8)
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			if attr {
				b.WriteString("&quot;")
			} else {
				b.WriteByte('"')
			}
		case '\'':
			if attr {
				b.WriteString("&#39;")
			} else {
				b.WriteByte('\'')
			}
		case '\n':
			b.WriteString("&#10;")
		case '\r':
			b.WriteString("&#13;")
		case '\t':
			b.WriteString("&#9;")
		default:
			if r < 0x20 {
				// Disallowed XML control character: substitute U+FFFD.
				b.WriteRune('�')
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}
