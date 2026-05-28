package lint_test

import (
	"testing"

	"m31labs.dev/sirena"
	"m31labs.dev/sirena/lint"
)

// TestUnresolvedRefs is a thin smoke test: the rule is a re-export of the
// resolver's SIR-REF-UNRESOLVED entries, so we just verify the wiring
// works end-to-end on an obvious bad edge.
func TestUnresolvedRefs(t *testing.T) {
	root := tmpWorkspace(t, map[string]string{
		"x.sir": `service api
api -> ghost: calls`,
	})
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	diags := lint.UnresolvedRefs(ws)
	var found bool
	for _, d := range diags {
		if d.Code == "SIR-REF-UNRESOLVED" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected SIR-REF-UNRESOLVED, got %+v", diags)
	}
}

// TestUnresolvedRefs_FilterOnly is the negative-space twin: the resolver
// also emits SIR-IMPORT-* and similar codes, and the rule must NOT
// surface them. This guards against a future tweak to the resolver
// leaking unrelated codes through the lint surface.
func TestUnresolvedRefs_FilterOnly(t *testing.T) {
	root := tmpWorkspace(t, map[string]string{
		"x.sir": `service api
api -> ghost: calls`,
	})
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	diags := lint.UnresolvedRefs(ws)
	for _, d := range diags {
		if d.Code != "SIR-REF-UNRESOLVED" {
			t.Errorf("UnresolvedRefs leaked non-target code: %+v", d)
		}
	}
}

// TestUnresolvedRefs_NilWorkspace guards against nil-receiver panics from
// the CLI happy-path forgetting to construct the workspace.
func TestUnresolvedRefs_NilWorkspace(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("nil workspace should not panic, got %v", r)
		}
	}()
	if diags := lint.UnresolvedRefs(nil); diags != nil {
		t.Errorf("nil workspace should yield nil, got %+v", diags)
	}
}

// Underscore-import to satisfy the unused-package check while we keep the
// sirena import available for diagnostic type references in future tests.
var _ = sirena.SeverityError
