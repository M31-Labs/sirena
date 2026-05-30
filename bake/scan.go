package bake

import (
	"strings"
)

// Kind classifies a detected diagram block.
type Kind int

const (
	// KindRaw is a raw backtick fence whose info-string language is "mermaid",
	// "sir", or "sirena".
	KindRaw Kind = iota
	// KindBaked is an already-baked block: the <!-- sirena-baked:<lang> -->
	// marker through the closing </details> tag.
	KindBaked
)

func (k Kind) String() string {
	switch k {
	case KindRaw:
		return "raw"
	case KindBaked:
		return "baked"
	default:
		return "unknown"
	}
}

// Block is one detected diagram region in a markdown document.
//
// Start and End are byte offsets into the original source: src[Start:End] is
// the full text that Phase C will replace (including fence delimiters or the
// full baked-block markup).
//
// Body is the verbatim diagram source without fence delimiters. For KindBaked
// blocks it is extracted from the ```text fence inside the <details> section.
//
// Lang is the info-string language token: "mermaid", "sir", or "sirena".
type Block struct {
	Kind  Kind
	Lang  string
	Body  string
	Start int // inclusive byte offset
	End   int // exclusive byte offset
}

// targetLang reports whether s is one of the three diagram languages sirena
// recognises.
func targetLang(s string) bool {
	return s == "mermaid" || s == "sir" || s == "sirena"
}

// backtickRunLen returns the number of leading backtick characters in s, or 0
// if the line does not start with at least three backticks.
func backtickRunLen(s string) int {
	n := 0
	for _, c := range s {
		if c != '`' {
			break
		}
		n++
	}
	if n < 3 {
		return 0
	}
	return n
}

// fenceInfoLang extracts the first whitespace-delimited token from the
// info-string that follows the opening backtick run. Returns "" if the line
// does not start with a qualifying backtick run.
func fenceInfoLang(line string, runLen int) string {
	if len(line) < runLen {
		return ""
	}
	info := strings.TrimRight(line[runLen:], "\n\r")
	info = strings.TrimSpace(info)
	// first token (language tag)
	tok := info
	if i := strings.IndexAny(info, " \t"); i >= 0 {
		tok = info[:i]
	}
	return tok
}

// parseBakedMarker checks whether line matches <!-- sirena-baked:<lang> --> and
// returns the lang if so.
func parseBakedMarker(line string) (lang string, ok bool) {
	s := strings.TrimRight(line, "\n\r")
	const prefix = "<!-- sirena-baked:"
	const suffix = " -->"
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, suffix) {
		return "", false
	}
	lang = s[len(prefix) : len(s)-len(suffix)]
	return lang, lang != ""
}

// splitLines splits src into lines, preserving the trailing newline in each
// line. It also returns the byte offset of the start of each line so callers
// can map line index → byte offset.
func splitLines(src []byte) (lines []string, offsets []int) {
	s := string(src)
	off := 0
	for len(s) > 0 {
		offsets = append(offsets, off)
		idx := strings.IndexByte(s, '\n')
		if idx < 0 {
			// last line has no trailing newline
			lines = append(lines, s)
			off += len(s)
			break
		}
		line := s[:idx+1]
		lines = append(lines, line)
		off += len(line)
		s = s[idx+1:]
	}
	return lines, offsets
}

// Scan parses src as markdown (bytes) and returns all detected diagram blocks
// in document order.
//
// Only backtick fences are recognised; tilde fences are ignored. Any fence
// whose language is not a target language opens an opaque block whose contents
// are skipped until the matching close — preventing false positives from
// code examples that happen to contain sirena/mermaid fences.
func Scan(src []byte) []Block {
	lines, offsets := splitLines(src)
	// lineEnd returns the exclusive byte offset just after line i.
	lineEnd := func(i int) int {
		if i+1 < len(offsets) {
			return offsets[i+1]
		}
		return len(src)
	}

	var blocks []Block
	i := 0
	for i < len(lines) {
		line := lines[i]

		// ── Check for already-baked marker ────────────────────────────────
		if lang, ok := parseBakedMarker(line); ok {
			if b, advance, found := scanBakedBlock(lines, offsets, lineEnd, i, lang); found {
				blocks = append(blocks, b)
				i = advance
				continue
			}
			// not a valid baked block — treat as plain text
			i++
			continue
		}

		// ── Check for backtick fence opener ───────────────────────────────
		runLen := backtickRunLen(line)
		if runLen == 0 {
			i++
			continue
		}
		lang := fenceInfoLang(line, runLen)

		if !targetLang(lang) {
			// Opaque block: skip until matching close.
			i = skipOpaqueBlock(lines, i, runLen)
			continue
		}

		// Raw diagram fence.
		blockStart := offsets[i]
		i++ // move past opening fence
		var bodyBuilder strings.Builder
		for i < len(lines) {
			bl := lines[i]
			// closing fence: same run length (or more), no info string, only backticks + optional whitespace
			if isClosingFence(bl, runLen) {
				i++ // consume closing fence
				break
			}
			bodyBuilder.WriteString(bl)
			i++
		}
		blockEnd := lineEnd(i - 1)
		blocks = append(blocks, Block{
			Kind:  KindRaw,
			Lang:  lang,
			Body:  bodyBuilder.String(),
			Start: blockStart,
			End:   blockEnd,
		})
	}
	return blocks
}

// isClosingFence reports whether line closes a fence opened with openRunLen
// backticks. The spec: a line consisting of at least openRunLen backticks and
// optional trailing whitespace, with no info-string.
func isClosingFence(line string, openRunLen int) bool {
	s := strings.TrimRight(line, "\n\r")
	n := 0
	for _, c := range s {
		if c != '`' {
			break
		}
		n++
	}
	if n < openRunLen {
		return false
	}
	// remainder after the backtick run must be empty or whitespace only
	rest := strings.TrimSpace(s[n:])
	return rest == ""
}

// skipOpaqueBlock advances past an opaque fence (one whose language is not a
// target). i is the index of the opening fence line. Returns the index of the
// first line AFTER the closing fence (or len(lines) if unclosed).
func skipOpaqueBlock(lines []string, i, openRunLen int) int {
	i++ // skip opener
	for i < len(lines) {
		if isClosingFence(lines[i], openRunLen) {
			return i + 1
		}
		i++
	}
	return i
}

// scanBakedBlock attempts to parse an already-baked block starting at line
// index markerIdx. lang is the language extracted from the marker.
//
// Expected structure (lines, each with trailing \n):
//
//	<!-- sirena-baked:<lang> -->
//	![...](...)
//	<details>...<summary>...</summary>
//	               (blank line)
//	```text
//	<body lines>
//	```
//	               (blank line)
//	</details>
//
// Returns the Block, the line index to resume from after </details>, and
// whether the parse succeeded.
func scanBakedBlock(lines []string, offsets []int, lineEnd func(int) int, markerIdx int, lang string) (Block, int, bool) {
	n := len(lines)
	i := markerIdx + 1 // skip marker line

	// Consume the image line (![...](...)). It must be next.
	if i >= n {
		return Block{}, 0, false
	}
	imageLine := strings.TrimRight(lines[i], "\n\r")
	if !strings.HasPrefix(imageLine, "![") {
		return Block{}, 0, false
	}
	i++ // past image line

	// Consume the <details> opening line.
	if i >= n {
		return Block{}, 0, false
	}
	if !strings.HasPrefix(strings.TrimRight(lines[i], "\n\r"), "<details>") {
		return Block{}, 0, false
	}
	i++ // past <details>

	// Scan forward to find the ```text fence inside the <details> block,
	// then the closing ```, then </details>.
	// We allow arbitrary lines between <details> and the text fence
	// (blank lines, <summary>...</summary>, etc.).
	var bodyBuilder strings.Builder
	inTextFence := false
	foundTextFence := false
	foundDetails := false
	endIdx := -1

	for i < n {
		raw := lines[i]
		trimmed := strings.TrimRight(raw, "\n\r")

		if !inTextFence {
			// look for ```text opener
			rl := backtickRunLen(trimmed)
			if rl >= 3 && strings.TrimSpace(trimmed[rl:]) == "text" {
				inTextFence = true
				foundTextFence = true
				i++
				continue
			}
			// look for </details>
			if trimmed == "</details>" {
				endIdx = i
				i++
				foundDetails = true
				break
			}
			i++
			continue
		}

		// Inside ```text fence: look for closing ```
		rl := backtickRunLen(trimmed)
		if rl >= 3 && strings.TrimSpace(trimmed[rl:]) == "" {
			// closing fence of the text block
			inTextFence = false
			i++
			continue
		}
		bodyBuilder.WriteString(raw)
		i++
	}

	if !foundTextFence || !foundDetails {
		return Block{}, 0, false
	}

	blockStart := offsets[markerIdx]
	blockEnd := lineEnd(endIdx)

	return Block{
		Kind:  KindBaked,
		Lang:  lang,
		Body:  bodyBuilder.String(),
		Start: blockStart,
		End:   blockEnd,
	}, i, true
}
