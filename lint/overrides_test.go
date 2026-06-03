package lint_test

import (
	"testing"

	"m31labs.dev/sirena"
	"m31labs.dev/sirena/lint"
)

// TestDanglingOverride mirrors the sirena-package coverage for
// ValidateOverridesAgainstWorkspace. The rule is a thin re-export, so we
// only need to verify the wiring; the underlying rule has its own
// negative-case coverage in the root package.
func TestDanglingOverride(t *testing.T) {
	root := tmpWorkspace(t, map[string]string{
		"sirena.yaml": "default_preset: default\n",
		"x.sir":       `service ghost @override(label)`,
		// No .gen.sir declares ghost — the override has nothing to override.
	})
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	diags := lint.DanglingOverride(ws)
	var found bool
	for _, d := range diags {
		if d.Code == "SIR-OVERRIDE-DANGLING" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected SIR-OVERRIDE-DANGLING, got %+v", diags)
	}
}

// TestDanglingOverride_NilWorkspace guards the nil-receiver branch.
func TestDanglingOverride_NilWorkspace(t *testing.T) {
	if diags := lint.DanglingOverride(nil); diags != nil {
		t.Errorf("nil workspace should yield nil, got %+v", diags)
	}
}

var _ = sirena.SeverityError
