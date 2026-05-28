package lint

import "m31labs.dev/sirena"

// DanglingOverride surfaces SIR-OVERRIDE-DANGLING from
// ValidateOverridesAgainstWorkspace. The override resolver in the root
// package is the canonical source; the lint pack wraps it so the rule
// participates in Run's deduplicated union.
//
// A nil workspace returns nil — ValidateOverridesAgainstWorkspace is
// already nil-safe, but the explicit check here keeps the rule's
// behavior obvious without forcing the reader to chase down the
// underlying function's contract.
func DanglingOverride(ws *sirena.Workspace) []sirena.Diagnostic {
	if ws == nil {
		return nil
	}
	return sirena.ValidateOverridesAgainstWorkspace(ws)
}
