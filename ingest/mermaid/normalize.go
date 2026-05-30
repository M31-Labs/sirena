package mermaid

import (
	"bytes"

	"m31labs.dev/sirena"
)

// srcMap records the single byte-offset shift introduced by rewriting the
// leading "graph" keyword to "flowchart" (+4 bytes). All other normalize
// edits are length-preserving, so the smap is identity for clean offsets
// below the shift point.
type srcMap struct {
	// shiftAt is the clean-source byte offset of the end of the keyword
	// rewrite. Offsets < shiftAt are identity; offsets >= shiftAt subtract
	// shiftDelta to recover the original offset.
	shiftAt    int
	shiftDelta int // how many bytes were added (4 for graph→flowchart)
}

// orig maps a clean-source byte offset back to the original-source offset.
func (m srcMap) orig(cleanOffset int) int {
	if m.shiftDelta != 0 && cleanOffset >= m.shiftAt {
		return cleanOffset - m.shiftDelta
	}
	return cleanOffset
}

// normalize performs three length-preserving (or offset-recorded) edits on
// the raw Mermaid source before it is handed to the grammar parser:
//
//  1. Rewrites a leading "graph" keyword to "flowchart" (+4 bytes; recorded
//     in the returned srcMap so CST ranges still map to the original source).
//  2. Replaces statement-position semicolons with spaces (length-preserving).
//     Semicolons inside "quoted strings" or [label]/(label) brackets are left
//     untouched.
//  3. Blanks out whole lines whose statement keyword is classDef, style,
//     linkStyle, or click (length-preserving: spaces replace the line content
//     but the newline is kept). Each stripped line emits one
//     SIR-MERMAID-STYLE-DROPPED warning with a Range pointing at the
//     original-source line span.
func normalize(src []byte) (clean []byte, preDiags []sirena.Diagnostic, smap srcMap) {
	clean, smap = rewriteGraphKeyword(src)
	clean, preDiags = stripAndNeutralize(clean, preDiags, smap)
	return clean, preDiags, smap
}

// rewriteGraphKeyword rewrites the leading "graph" diagram-type keyword to
// "flowchart" if present. It only matches a bare "graph" at the start of the
// first non-blank, non-comment line, followed by whitespace or a direction
// token. It never rewrites "graph" appearing inside an id or label.
func rewriteGraphKeyword(src []byte) ([]byte, srcMap) {
	// Walk past leading blank lines and %% comments to find the first
	// diagram-type line.
	i := 0
	n := len(src)
	for i < n {
		// Skip leading whitespace on this line.
		lineStart := i
		for i < n && (src[i] == ' ' || src[i] == '\t') {
			i++
		}
		// Skip %% comment lines.
		if i+1 < n && src[i] == '%' && src[i+1] == '%' {
			for i < n && src[i] != '\n' {
				i++
			}
			if i < n {
				i++ // consume \n
			}
			continue
		}
		// Skip %{...}% init directives (%%{init:...}%%).
		if i+2 < n && src[i] == '%' && src[i+1] == '%' && src[i+2] == '{' {
			for i < n && src[i] != '\n' {
				i++
			}
			if i < n {
				i++
			}
			continue
		}
		// This is the first content line; check if it starts with "graph".
		_ = lineStart
		if bytes.HasPrefix(src[i:], []byte("graph")) {
			rest := src[i+5:] // after "graph"
			// Must be followed by whitespace, end-of-line, or direction.
			if len(rest) == 0 || rest[0] == ' ' || rest[0] == '\t' || rest[0] == '\r' || rest[0] == '\n' {
				// Rewrite: replace "graph" with "flowchart".
				const oldKw = "graph"
				const newKw = "flowchart"
				out := make([]byte, 0, len(src)+4)
				out = append(out, src[:i]...)
				out = append(out, newKw...)
				out = append(out, src[i+len(oldKw):]...)
				shiftEnd := i + len(newKw) // end of "flowchart" in clean src
				return out, srcMap{shiftAt: shiftEnd, shiftDelta: 4}
			}
		}
		break
	}
	return src, srcMap{}
}

// stylingKeywords lists the Mermaid statement keywords that have no grammar
// production and must be stripped before parsing.
var stylingKeywords = [][]byte{
	[]byte("classDef"),
	[]byte("style"),
	[]byte("linkStyle"),
	[]byte("click"),
}

// stripAndNeutralize scans the (possibly already-rewritten) source
// line-by-line to:
//   - blank styling lines (classDef/style/linkStyle/click at statement start)
//   - replace statement-position semicolons with spaces
func stripAndNeutralize(src []byte, diags []sirena.Diagnostic, smap srcMap) ([]byte, []sirena.Diagnostic) {
	out := make([]byte, len(src))
	copy(out, src)

	lineStart := 0
	for lineStart < len(out) {
		// Find end of line.
		lineEnd := lineStart
		for lineEnd < len(out) && out[lineEnd] != '\n' {
			lineEnd++
		}
		// lineEnd points at '\n' or len(out).
		line := out[lineStart:lineEnd]

		// Skip leading whitespace to find the statement keyword position.
		kwStart := 0
		for kwStart < len(line) && (line[kwStart] == ' ' || line[kwStart] == '\t') {
			kwStart++
		}
		trimmed := line[kwStart:]

		stripped := false
		for _, kw := range stylingKeywords {
			if !bytes.HasPrefix(trimmed, kw) {
				continue
			}
			after := len(kw)
			// Keyword must be followed by whitespace, end-of-line, or end of
			// content (to avoid matching "styleSheet" etc.).
			if after < len(trimmed) && trimmed[after] != ' ' && trimmed[after] != '\t' {
				continue
			}
			// Blank out the line content (preserve the '\n').
			origStart := smap.orig(lineStart)
			origEnd := smap.orig(lineEnd)
			diags = append(diags, sirena.Diagnostic{
				Code:     "SIR-MERMAID-STYLE-DROPPED",
				Severity: sirena.SeverityWarning,
				Message:  "Mermaid styling directive dropped (no sirena equivalent): " + string(trimmed[:len(kw)]),
				Range:    sirena.Range{Start: origStart, End: origEnd},
			})
			for j := lineStart; j < lineEnd; j++ {
				out[j] = ' '
			}
			stripped = true
			break
		}

		// If not stripped, neutralize statement-position semicolons.
		if !stripped {
			neutralizeSemicolons(out, lineStart, lineEnd)
		}

		lineStart = lineEnd + 1 // +1 to skip '\n'; safe even if lineEnd==len(out)
	}
	return out, diags
}

// neutralizeSemicolons replaces statement-position ';' characters in the
// given line with spaces. A ';' inside a double-quoted string or inside
// bracket/paren labels ([...] or (...)) is left untouched.
func neutralizeSemicolons(out []byte, lineStart, lineEnd int) {
	inQuote := false
	depth := 0 // bracket/paren nesting depth
	for i := lineStart; i < lineEnd; i++ {
		ch := out[i]
		switch {
		case ch == '"' && depth == 0:
			inQuote = !inQuote
		case inQuote:
			// inside a quoted string — leave everything alone
		case ch == '[' || ch == '(':
			depth++
		case (ch == ']' || ch == ')') && depth > 0:
			depth--
		case ch == ';' && depth == 0:
			out[i] = ' '
		}
	}
}
