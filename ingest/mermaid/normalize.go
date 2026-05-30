package mermaid

import (
	"bytes"
	"sort"

	"m31labs.dev/sirena"
)

// srcEdit records a single contiguous deletion from the pre-deletion (keyword-
// rewritten) buffer. cleanOff is the offset in the final clean buffer where
// the run of deleted bytes starts; deletedBytes is how many bytes were removed
// from the keyword-rewritten buffer at that point.
type srcEdit struct {
	// preOff is the byte offset in the pre-deletion (keyword-rewritten) buffer
	// of the start of the deletion.
	preOff int
	// deletedBytes is how many consecutive bytes were removed.
	deletedBytes int
}

// srcMap maps final-clean buffer byte offsets back to original-source byte
// offsets. It handles two transforms applied in sequence:
//
//  1. rewriteGraphKeyword: adds shiftDelta bytes at shiftAt in the clean buffer
//     (the keyword rewrite is an expansion, not a deletion).
//  2. stripAndNeutralize: removes styling lines (deletions), tracked in edits.
//
// To map a clean offset back to original:
//
//	a. Undo deletions: walk edits (sorted by preOff) to recover the
//	   pre-deletion (keyword-rewritten) offset.
//	b. Undo keyword shift: subtract shiftDelta if offset >= shiftAt.
type srcMap struct {
	// Keyword rewrite: "graph" → "flowchart" (+4 bytes at shiftAt).
	shiftAt    int // end of "flowchart" in the keyword-rewritten buffer
	shiftDelta int // bytes added (4 for graph→flowchart)

	// edits is the ordered list of deletions in the keyword-rewritten buffer.
	// Sorted by preOff ascending.
	edits []srcEdit
}

// orig maps a clean-source byte offset back to the original-source offset.
func (m srcMap) orig(cleanOffset int) int {
	// Step 1: undo deletions to get the keyword-rewritten offset.
	// Each edit removes deletedBytes bytes at preOff in the keyword-rewritten
	// buffer. Bytes before preOff are unchanged; bytes at or after preOff in
	// the final buffer are shifted forward by deletedBytes.
	//
	// We walk edits in order: for each edit, if the clean offset is past the
	// point where that edit was applied (in the pre-deletion buffer), we add
	// the deleted bytes back.
	preOffset := cleanOffset
	for _, e := range m.edits {
		// e.preOff is the position in the pre-deletion buffer where bytes were
		// removed. In the final (post-deletion) buffer, all bytes that were
		// originally before e.preOff are still before that point; bytes after
		// the deletion are shifted back by deletedBytes. So if preOffset (after
		// accounting for previous edits) >= e.preOff, add e.deletedBytes.
		if preOffset >= e.preOff {
			preOffset += e.deletedBytes
		}
	}

	// Step 2: undo keyword shift.
	if m.shiftDelta != 0 && preOffset >= m.shiftAt {
		return preOffset - m.shiftDelta
	}
	return preOffset
}

// normalize performs three edits on the raw Mermaid source before it is
// handed to the grammar parser:
//
//  1. Rewrites a leading "graph" keyword to "flowchart" (+4 bytes; recorded
//     in the returned srcMap so CST ranges still map to the original source).
//  2. Strips whole lines whose statement keyword is classDef, style, linkStyle,
//     click, or class (the application statement). Styling lines are DELETED
//     from the clean buffer (not blanked), eliminating GLR error-recovery that
//     would otherwise drop subsequent diagram_flow statements. The deleted byte
//     ranges are recorded in the srcMap so all remaining CST offsets still map
//     correctly to the original source. Each stripped line emits one
//     SIR-MERMAID-STYLE-DROPPED warning with a Range pointing at the
//     original-source line span (inclusive of the newline).
//  3. Replaces statement-position semicolons with spaces (length-preserving).
//     Semicolons inside "quoted strings" or [label]/(label) brackets are left
//     untouched.
func normalize(src []byte) (clean []byte, preDiags []sirena.Diagnostic, smap srcMap) {
	clean, smap = rewriteGraphKeyword(src)
	clean, preDiags, smap = stripAndNeutralize(clean, preDiags, smap)
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
		// Skip %% comment lines and %%{init:…}%% directives (both start with "%%").
		if i+1 < n && src[i] == '%' && src[i+1] == '%' {
			for i < n && src[i] != '\n' {
				i++
			}
			if i < n {
				i++ // consume \n
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
// production and must be stripped before parsing. Order matters: "classDef"
// must appear before "class" so that the whole-word check for "class" does
// not need to special-case the "classDef" prefix — the loop breaks on first
// match.
var stylingKeywords = [][]byte{
	[]byte("classDef"),
	[]byte("style"),
	[]byte("linkStyle"),
	[]byte("click"),
	[]byte("class"),
}

// stripAndNeutralize scans the (possibly already keyword-rewritten) source
// line-by-line to:
//   - delete styling lines (classDef/style/linkStyle/click/class at statement
//     start) from the clean buffer, recording deletions in smap.edits so that
//     remaining CST byte offsets can be mapped back to the original source.
//   - replace statement-position semicolons with spaces (length-preserving).
//
// Deletions include the trailing newline, so the clean buffer has fewer bytes
// than the keyword-rewritten buffer. The smap is updated in place.
func stripAndNeutralize(src []byte, diags []sirena.Diagnostic, smap srcMap) ([]byte, []sirena.Diagnostic, srcMap) {
	// out is built by accumulating kept segments of src.
	out := make([]byte, 0, len(src))

	// prePos tracks our position in src (the keyword-rewritten buffer).
	// cleanPos tracks the corresponding position in out (post-deletion buffer).
	prePos := 0

	for prePos < len(src) {
		// Find end of line in src.
		lineStart := prePos
		lineEnd := prePos
		for lineEnd < len(src) && src[lineEnd] != '\n' {
			lineEnd++
		}
		// lineEnd points at '\n' or len(src).
		// lineEndIncl is one past the newline (or len(src) if no trailing newline).
		lineEndIncl := lineEnd
		if lineEnd < len(src) {
			lineEndIncl = lineEnd + 1 // include the '\n'
		}

		line := src[lineStart:lineEnd]

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
			// Emit the STYLE-DROPPED diagnostic using the ORIGINAL source offsets.
			// smap.orig maps keyword-rewritten offsets back to original offsets.
			// At this point smap only has the keyword-shift; deletions haven't been
			// recorded yet, so smap.orig gives correct keyword-rewritten→original
			// offsets for this line (it's still in the keyword-rewritten buffer).
			origStart := smap.orig(lineStart)
			origEnd := smap.orig(lineEnd)
			diags = append(diags, sirena.Diagnostic{
				Code:     "SIR-MERMAID-STYLE-DROPPED",
				Severity: sirena.SeverityWarning,
				Message:  "Mermaid styling directive dropped (no sirena equivalent): " + string(trimmed[:len(kw)]),
				Range:    sirena.Range{Start: origStart, End: origEnd},
			})
			// Record the deletion in smap. preOff is where these bytes are in
			// the keyword-rewritten buffer. deletedBytes is the full line
			// including the newline (lineEndIncl - lineStart bytes).
			// We record the edit at the clean buffer position where the deletion
			// would appear, i.e., len(out) (the number of kept bytes so far).
			smap.edits = append(smap.edits, srcEdit{
				preOff:       lineStart,
				deletedBytes: lineEndIncl - lineStart,
			})
			// Do NOT append this line to out — it is deleted.
			stripped = true
			break
		}

		if !stripped {
			// Append this line to out (with its newline), then neutralize
			// semicolons in the appended segment.
			segStart := len(out)
			out = append(out, src[lineStart:lineEndIncl]...)
			segEnd := segStart + (lineEnd - lineStart)
			neutralizeSemicolons(out, segStart, segEnd)
		}

		prePos = lineEndIncl
	}

	// Ensure edits are sorted by preOff (they should already be in order since
	// we process lines top-to-bottom, but sort for safety).
	sort.Slice(smap.edits, func(i, j int) bool {
		return smap.edits[i].preOff < smap.edits[j].preOff
	})

	return out, diags, smap
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
