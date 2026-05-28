package lint_test

import (
	"testing"

	"m31labs.dev/sirena"
	"m31labs.dev/sirena/lint"
)

// TestVersions_OlderMinor declares an older sirena_version in the
// manifest. OpenWorkspace accepts it (with an upgrade-hint diagnostic)
// and stashes the SIR-VERSION-AHEAD warning in synthDiagnostics; the
// Versions rule must surface that warning.
func TestVersions_OlderMinor(t *testing.T) {
	root := tmpWorkspace(t, map[string]string{
		"sirena.toml": "sirena_version: \"0.0\"\n",
		"x.sir":       "service api",
	})
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	diags := lint.Versions(ws)
	var found bool
	for _, d := range diags {
		if d.Code == "SIR-VERSION-AHEAD" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected SIR-VERSION-AHEAD (older), got %+v", diags)
	}
}

// TestVersions_CurrentVersion is the negative twin: an exact-match
// sirena_version should not trip the rule.
func TestVersions_CurrentVersion(t *testing.T) {
	root := tmpWorkspace(t, map[string]string{
		"sirena.toml": "sirena_version: \"" + sirena.CurrentSpecVersion + "\"\n",
		"x.sir":       "service api",
	})
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	diags := lint.Versions(ws)
	for _, d := range diags {
		if d.Code == "SIR-VERSION-AHEAD" {
			t.Errorf("current version should not flag: %+v", d)
		}
	}
}

// TestVersions_NilWorkspace guards the nil-receiver branch.
func TestVersions_NilWorkspace(t *testing.T) {
	if diags := lint.Versions(nil); diags != nil {
		t.Errorf("nil workspace should yield nil, got %+v", diags)
	}
}
