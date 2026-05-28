package lint_test

import (
	"testing"

	"m31labs.dev/sirena"
	"m31labs.dev/sirena/lint"
)

// TestRun_NoIssues drives the happy path: a workspace where every element
// has at least one neighbour, no queues, no overrides, no version skew.
// Run should return no diagnostics — every rule has to stay silent on a
// well-formed workspace, or the linter loses signal.
func TestRun_NoIssues(t *testing.T) {
	root := tmpWorkspace(t, map[string]string{
		"x.sir": `service api
database db
api -> db: writes
db -> api: reads`,
	})
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	diags := lint.Run(ws)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics, got %+v", diags)
	}
}

// TestRun_NilWorkspace covers the defensive nil-receiver branch. A CLI
// wrapper that forgot to construct the workspace shouldn't panic the lint
// runner; nil in, nil out.
func TestRun_NilWorkspace(t *testing.T) {
	if diags := lint.Run(nil); diags != nil {
		t.Errorf("nil workspace should yield nil, got %+v", diags)
	}
}

// TestRun_Deduplicates verifies that the (Code, Range.Start, Range.End)
// dedup key collapses duplicate emissions. The unresolved edge to "ghost"
// is touched by ResolveWorkspace (which UnresolvedRefs re-exports), and
// also potentially by other rules that walk edges; the dedup pass must
// surface exactly one SIR-REF-UNRESOLVED for the same offending edge.
func TestRun_Deduplicates(t *testing.T) {
	root := tmpWorkspace(t, map[string]string{
		"x.sir": `service api
api -> ghost: calls`,
	})
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	diags := lint.Run(ws)
	var unresolvedCount int
	for _, d := range diags {
		if d.Code == "SIR-REF-UNRESOLVED" {
			unresolvedCount++
		}
	}
	if unresolvedCount != 1 {
		t.Errorf("dedup failed: %d SIR-REF-UNRESOLVED entries, want 1", unresolvedCount)
	}
}
