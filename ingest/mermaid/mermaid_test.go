package mermaid

import "testing"

func TestParse_MinimalFlowchart_NoError(t *testing.T) {
	doc, diags, err := Parse([]byte("flowchart LR\n  a --> b\n"), Options{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(doc.Systems) != 1 {
		t.Fatalf("want 1 system, got %d", len(doc.Systems))
	}
	if got := len(doc.Systems[0].Elements); got != 2 {
		t.Fatalf("want 2 elements, got %d", got)
	}
	if got := len(doc.Systems[0].Edges); got != 1 {
		t.Fatalf("want 1 edge, got %d", got)
	}
	_ = diags
}
