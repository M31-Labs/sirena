package mermaid

import (
	"strings"
	"testing"

	"m31labs.dev/sirena"
)

// TestDiag_NotAGraph verifies that a non-flowchart Mermaid diagram type
// returns a nil document, a non-nil error, and a SIR-MERMAID-NOT-A-GRAPH
// diagnostic.
func TestDiag_NotAGraph(t *testing.T) {
	src := []byte("sequenceDiagram\n  A->>B: hi\n")
	doc, diags, err := Parse(src, Options{})

	if doc != nil {
		t.Errorf("expected nil doc for non-flowchart input, got %+v", doc)
	}
	if err == nil {
		t.Error("expected non-nil error for non-flowchart input")
	}

	found := findDiag(diags, "SIR-MERMAID-NOT-A-GRAPH")
	if found == nil {
		t.Errorf("expected SIR-MERMAID-NOT-A-GRAPH diagnostic, got %v", diags)
	}
}

// TestDiag_NotAGraph_DirectivePrefix verifies that a flowchart with a leading
// %%{init:…}%% directive is NOT misclassified as NOT-A-GRAPH.
func TestDiag_NotAGraph_DirectivePrefix(t *testing.T) {
	src := []byte("%%{init: {\"theme\": \"dark\"}}%%\nflowchart LR\n  A --> B\n")
	doc, diags, err := Parse(src, Options{})

	if err != nil {
		t.Errorf("unexpected error for flowchart with directive: %v", err)
	}
	if doc == nil {
		t.Error("expected non-nil doc for flowchart with directive prefix")
	}
	if findDiag(diags, "SIR-MERMAID-NOT-A-GRAPH") != nil {
		t.Error("flowchart with %%{init}%% prefix should NOT produce NOT-A-GRAPH")
	}
}

// TestDiag_ParseError verifies that a flowchart with a malformed statement
// (causing root-level parse errors) still lowers valid statements AND emits
// SIR-MERMAID-PARSE.
func TestDiag_ParseError(t *testing.T) {
	// "C -->" with no target causes tree-sitter to push the arrow as an ERROR
	// node at root while still recovering "A --> B" into diagram_flow.
	src := []byte("flowchart LR\n  A --> B\n  C -->\n  D --> E\n")
	doc, diags, err := Parse(src, Options{})

	// Parse should not return fatal: we still produced a (partial) doc.
	if err != nil {
		t.Errorf("unexpected fatal error for partial flowchart: %v", err)
	}
	if doc == nil {
		t.Error("expected non-nil doc for partial flowchart")
	}

	// Valid statement A --> B must still be lowered.
	if doc != nil {
		sys := doc.Systems[0]
		if findEdge(sys.Edges, "A", "B") == nil {
			t.Error("expected edge A→B from valid statement to be present")
		}
	}

	// SIR-MERMAID-PARSE must be present.
	if findDiag(diags, "SIR-MERMAID-PARSE") == nil {
		t.Errorf("expected SIR-MERMAID-PARSE diagnostic, got %v", diags)
	}
}

// TestDiag_Unsupported verifies that a flow_stmt_direction node (recognized
// by the grammar but unhandled by the lowerer) emits SIR-MERMAID-UNSUPPORTED
// while the rest of the flowchart still lowers cleanly.
func TestDiag_Unsupported(t *testing.T) {
	// "direction TB" inside a flowchart body produces a flow_stmt_direction
	// named child of diagram_flow — not handled by lowerStmt/lowerSubgraph.
	src := []byte("flowchart LR\n  A --> B\n  direction TB\n  C --> D\n")
	doc, diags, err := Parse(src, Options{})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if doc == nil {
		t.Fatal("expected non-nil doc")
	}

	sys := doc.Systems[0]
	if findEdge(sys.Edges, "A", "B") == nil {
		t.Error("edge A→B should be present")
	}
	if findEdge(sys.Edges, "C", "D") == nil {
		t.Error("edge C→D should be present")
	}

	if findDiag(diags, "SIR-MERMAID-UNSUPPORTED") == nil {
		t.Errorf("expected SIR-MERMAID-UNSUPPORTED diagnostic, got %v", diags)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

// findDiagByCode returns the first Diagnostic whose Code equals code, or nil.
func findDiag(diags []sirena.Diagnostic, code string) *sirena.Diagnostic {
	for i := range diags {
		if strings.EqualFold(diags[i].Code, code) || diags[i].Code == code {
			return &diags[i]
		}
	}
	return nil
}
