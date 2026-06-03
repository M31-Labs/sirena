package lint_test

import (
	"testing"

	"m31labs.dev/sirena"
	"m31labs.dev/sirena/lint"
)

// TestManifestName_FlagsLegacyTOML confirms the ManifestName rule surfaces the
// SIR-MANIFEST-LEGACY-NAME warning that OpenWorkspace deposits when a workspace
// is opened via the deprecated sirena.toml name.
func TestManifestName_FlagsLegacyTOML(t *testing.T) {
	root := tmpWorkspace(t, map[string]string{
		"sirena.toml": "default_preset: default\n",
		"x.sir":       "service api",
	})
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, d := range lint.ManifestName(ws) {
		if d.Code == "SIR-MANIFEST-LEGACY-NAME" {
			found = true
			if d.Severity != sirena.SeverityWarning {
				t.Errorf("severity = %v, want warning", d.Severity)
			}
		}
	}
	if !found {
		t.Errorf("expected SIR-MANIFEST-LEGACY-NAME from ManifestName for sirena.toml")
	}
}

// TestManifestName_QuietForYAML confirms the canonical sirena.yaml name does
// not trip the rule.
func TestManifestName_QuietForYAML(t *testing.T) {
	root := tmpWorkspace(t, map[string]string{
		"sirena.yaml": "default_preset: default\n",
		"x.sir":       "service api",
	})
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	if diags := lint.ManifestName(ws); len(diags) != 0 {
		t.Errorf("sirena.yaml must not flag; got %+v", diags)
	}
}

// TestManifestName_NilWorkspace guards the nil-receiver branch.
func TestManifestName_NilWorkspace(t *testing.T) {
	if diags := lint.ManifestName(nil); diags != nil {
		t.Errorf("nil workspace should yield nil, got %+v", diags)
	}
}
