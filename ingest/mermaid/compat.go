package mermaid

import "m31labs.dev/sirena"

// Compat is the compatibility outcome of ingesting one Mermaid source into
// sirena. It is the classification axis behind the published compatibility
// matrix (docs/mermaid-compatibility.md). It deliberately does NOT make any
// pixel-parity claim — it reports only whether the diagram's structure
// survives ingestion, and what was lost.
type Compat int

const (
	// CompatFull: the diagram ingests cleanly — every node and edge is
	// represented, no diagnostics. (Visual styling is never claimed to
	// match Mermaid; see the matrix preamble.)
	CompatFull Compat = iota
	// CompatStyleDropped: the geometry ingests, but one or more styling
	// directives (classDef, style, linkStyle, …) had no sirena equivalent
	// and were dropped. The diagram still renders; only appearance differs.
	CompatStyleDropped
	// CompatPartial: some content was silently dropped — a parse error
	// skipped statements (SIR-MERMAID-PARSE) or an unsupported statement
	// type was ignored (SIR-MERMAID-UNSUPPORTED). The diagram renders, but
	// it is missing structure present in the source.
	CompatPartial
	// CompatUnsupported: nothing renders. The input is not a flowchart
	// (SIR-MERMAID-NOT-A-GRAPH) — e.g. sequenceDiagram, pie, gantt.
	CompatUnsupported
)

// String returns the stable lowercase label used in the compatibility matrix.
func (c Compat) String() string {
	switch c {
	case CompatFull:
		return "full"
	case CompatStyleDropped:
		return "styling-dropped"
	case CompatPartial:
		return "partial"
	case CompatUnsupported:
		return "unsupported"
	default:
		return "unknown"
	}
}

// Classify ingests a Mermaid source and reports its compatibility outcome
// along with the diagnostics Parse produced. It is the single source of
// truth for the published compatibility matrix and is safe for tools to call
// directly.
//
// Precedence is worst-case-wins so the label never overstates support:
// nothing-renders (Unsupported) beats content-loss (Partial), which beats
// styling-loss (StyleDropped), which beats clean (Full).
func Classify(src []byte) (Compat, []sirena.Diagnostic) {
	doc, diags, err := Parse(src, Options{})

	// Nothing rendered: not a flowchart at all.
	if err != nil || doc == nil {
		return CompatUnsupported, diags
	}

	var styleDropped bool
	for _, d := range diags {
		switch d.Code {
		case "SIR-MERMAID-PARSE", "SIR-MERMAID-UNSUPPORTED":
			// Content was silently dropped — the strongest non-fatal signal.
			return CompatPartial, diags
		case "SIR-MERMAID-STYLE-DROPPED":
			styleDropped = true
		}
	}
	if styleDropped {
		return CompatStyleDropped, diags
	}
	return CompatFull, diags
}
