package lint

import "m31labs.dev/sirena"

// Versions surfaces SIR-VERSION-AHEAD and SIR-ORPHAN entries from
// ws.ResolveDiagnostics.
//
// SIR-VERSION-AHEAD is deposited by OpenWorkspace when the workspace
// manifest declares an older or newer sirena_version than the build's
// CurrentSpecVersion; ResolveDiagnostics surfaces the stashed entry on
// every call.
//
// SIR-ORPHAN is emitted by ReconcileWithOrphans when a generator drops
// a prior declaration and the orphan-handling pass elects to retain it
// for a grace window. v0.1's LoadDocument pipeline does not call
// ReconcileWithOrphans on its own — that's the generator's
// responsibility — so SIR-ORPHAN only surfaces here for workspaces
// whose synthDiagnostics were seeded externally. We forward the code
// anyway so a future generator that does seed orphan entries into the
// workspace gets surfaced through Run without changing the lint pack.
//
// A nil workspace returns nil.
func Versions(ws *sirena.Workspace) []sirena.Diagnostic {
	if ws == nil {
		return nil
	}
	var out []sirena.Diagnostic
	for _, d := range ws.ResolveDiagnostics() {
		if d.Code == "SIR-VERSION-AHEAD" || d.Code == "SIR-ORPHAN" {
			out = append(out, d)
		}
	}
	return out
}
