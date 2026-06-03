package lint

import "m31labs.dev/sirena"

// ManifestName surfaces SIR-MANIFEST-LEGACY-NAME from ws.ResolveDiagnostics.
//
// OpenWorkspace deposits SIR-MANIFEST-LEGACY-NAME (severity Warning) when a
// workspace is opened via the deprecated sirena.toml manifest name instead of
// the canonical sirena.yaml. The file format is unchanged (YAML); only the
// name moved. Forwarding it through the lint pack nudges authors to rename
// without breaking the workspace.
//
// A nil workspace returns nil.
func ManifestName(ws *sirena.Workspace) []sirena.Diagnostic {
	if ws == nil {
		return nil
	}
	var out []sirena.Diagnostic
	for _, d := range ws.ResolveDiagnostics() {
		if d.Code == "SIR-MANIFEST-LEGACY-NAME" {
			out = append(out, d)
		}
	}
	return out
}
