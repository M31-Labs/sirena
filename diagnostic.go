package sirena

// Severity ranks a Diagnostic's urgency. The zero value is SeverityError so
// an empty Diagnostic surfaces as an error — the most-urgent rendering for
// the most-likely-unintentional state.
type Severity int

const (
	// SeverityError blocks downstream work; e.g. duplicate name, malformed
	// override, sid collision. A document with any Error diagnostic is
	// considered invalid by the workspace pipeline.
	SeverityError Severity = iota
	// SeverityWarning surfaces a likely mistake but does not block downstream
	// work. The renderer still emits output; the CLI still exits zero unless
	// strict mode is on.
	SeverityWarning
	// SeverityInfo is an observation, not a problem. Carries hints from the
	// linter (Phase 10) and the budget evaluator (Phase 7).
	SeverityInfo
)

// String returns the short label for this severity, suitable for diagnostic
// rendering and CLI output.
func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityInfo:
		return "info"
	default:
		return "unknown"
	}
}

// Diagnostic is a single validation, lint, or parse finding.
//
// Code is a stable short identifier (e.g. "SIR-VALIDATE-DUP-NAME") that
// tooling consumes for filtering and i18n. Message is a human-readable
// explanation suitable for direct rendering. Range pins the finding to a
// source byte span so editors can underline the offending text.
//
// The full diagnostic plumbing — registries, formatters, exit-code mapping —
// lands in Phase 9 (Task 29). Phase 3 introduces the data shape so the
// validation pass has something to emit.
type Diagnostic struct {
	Code     string
	Severity Severity
	Message  string
	Range    Range
}
