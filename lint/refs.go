package lint

import "m31labs.dev/sirena"

// UnresolvedRefs surfaces SIR-REF-UNRESOLVED entries from
// ws.ResolveWorkspace. The resolver is the canonical source for
// unresolved-edge findings — this rule is a thin filter so the lint
// pack's deduplication can fold them into the single Run output without
// duplicating the underlying walk.
//
// A nil workspace returns nil.
func UnresolvedRefs(ws *sirena.Workspace) []sirena.Diagnostic {
	if ws == nil {
		return nil
	}
	var out []sirena.Diagnostic
	for _, d := range ws.ResolveWorkspace() {
		if d.Code == "SIR-REF-UNRESOLVED" {
			out = append(out, d)
		}
	}
	return out
}
