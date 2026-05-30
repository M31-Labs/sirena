package mermaid

import (
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

// TestDiag_RangeMapping_CombinedTransforms verifies that diagnostic ranges
// correctly map back to the original source when BOTH transforms are applied:
// a "graph→flowchart" keyword rewrite (shift) and a classDef line deletion.
//
// Input layout (byte offsets in the ORIGINAL source):
//
//	 0- 8: "graph TD\n"            (9 bytes)
//	 9-18: "  A --> B\n"           (10 bytes)
//	19-31: "  classDef x\n"        (13 bytes) — deleted by normalize
//	32-46: "  direction TB\n"      (15 bytes) — UNSUPPORTED stmt
//	47-56: "  C --> D\n"           (10 bytes)
//
// After normalize:
//  1. rewriteGraphKeyword: "graph"→"flowchart" adds 4 bytes (shiftAt=9, delta=4).
//  2. stripAndNeutralize: deletes the classDef line (13 bytes in the
//     keyword-rewritten buffer).
//
// The UNSUPPORTED diagnostic for "direction TB" must point to original offsets
// [34, 46) — not the clean-buffer offsets — confirming that srcMap.orig
// correctly composes both edits.
func TestDiag_RangeMapping_CombinedTransforms(t *testing.T) {
	// "graph TD" triggers the keyword rewrite (+4 bytes shift).
	// "classDef x" triggers a STYLE-DROPPED deletion.
	// "direction TB" triggers an UNSUPPORTED diagnostic.
	src := []byte("graph TD\n  A --> B\n  classDef x\n  direction TB\n  C --> D\n")

	_, diags, _ := Parse(src, Options{})

	// STYLE-DROPPED must point at the classDef line in the original source.
	// "graph TD\n" = 9 bytes, "  A --> B\n" = 10 bytes → classDef starts at 19.
	// The diagnostic Range.End is the end of content (excl \n): 19+12 = 31.
	styleDropped := findDiag(diags, "SIR-MERMAID-STYLE-DROPPED")
	if styleDropped == nil {
		t.Fatal("expected SIR-MERMAID-STYLE-DROPPED diagnostic")
	}
	const wantStyleStart = 19
	const wantStyleEnd = 31
	if styleDropped.Range.Start != wantStyleStart {
		t.Errorf("STYLE-DROPPED Range.Start = %d, want %d (original classDef offset)",
			styleDropped.Range.Start, wantStyleStart)
	}
	if styleDropped.Range.End != wantStyleEnd {
		t.Errorf("STYLE-DROPPED Range.End = %d, want %d", styleDropped.Range.End, wantStyleEnd)
	}

	// UNSUPPORTED must point at "direction TB" in the ORIGINAL source.
	// After classDef line: 19+13=32 → "  direction TB\n" starts at 32.
	// "direction TB" itself starts at 34 (32+2 leading spaces).
	// Range covers the full flow_stmt_direction node span: [34, 46).
	unsupported := findDiag(diags, "SIR-MERMAID-UNSUPPORTED")
	if unsupported == nil {
		t.Fatal("expected SIR-MERMAID-UNSUPPORTED diagnostic")
	}
	const wantUnsupStart = 34 // "direction TB" start in original
	const wantUnsupEnd = 46   // "direction TB" end in original (14 chars from offset 32+2)
	if unsupported.Range.Start != wantUnsupStart {
		t.Errorf("UNSUPPORTED Range.Start = %d, want %d (original 'direction TB' offset); "+
			"srcMap.orig is not composing keyword-shift and deletion-edits correctly",
			unsupported.Range.Start, wantUnsupStart)
	}
	if unsupported.Range.End != wantUnsupEnd {
		t.Errorf("UNSUPPORTED Range.End = %d, want %d", unsupported.Range.End, wantUnsupEnd)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

// findDiagByCode returns the first Diagnostic whose Code equals code, or nil.
func findDiag(diags []sirena.Diagnostic, code string) *sirena.Diagnostic {
	for i := range diags {
		if diags[i].Code == code {
			return &diags[i]
		}
	}
	return nil
}
