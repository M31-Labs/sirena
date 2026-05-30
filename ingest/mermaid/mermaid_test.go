package mermaid

import (
	"testing"
)

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

// TestParse_EmptyInput verifies that Parse([]byte("")) does not panic and
// returns nil doc, non-nil error, and a SIR-MERMAID-NOT-A-GRAPH diagnostic.
func TestParse_EmptyInput(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Parse(empty) panicked: %v", r)
		}
	}()
	doc, diags, err := Parse([]byte(""), Options{})
	if doc != nil {
		t.Errorf("want nil doc for empty input, got %+v", doc)
	}
	if err == nil {
		t.Error("want non-nil error for empty input")
	}
	if findDiag(diags, "SIR-MERMAID-NOT-A-GRAPH") == nil {
		t.Errorf("want SIR-MERMAID-NOT-A-GRAPH diagnostic, got %v", diags)
	}
}

// TestParse_WhitespaceOnlyInput verifies that whitespace-only input does not
// panic and returns the same NOT-A-GRAPH outcome as empty input.
func TestParse_WhitespaceOnlyInput(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Parse(whitespace) panicked: %v", r)
		}
	}()
	doc, diags, err := Parse([]byte("   \n  \n"), Options{})
	if doc != nil {
		t.Errorf("want nil doc for whitespace-only input, got %+v", doc)
	}
	if err == nil {
		t.Error("want non-nil error for whitespace-only input")
	}
	if findDiag(diags, "SIR-MERMAID-NOT-A-GRAPH") == nil {
		t.Errorf("want SIR-MERMAID-NOT-A-GRAPH diagnostic, got %v", diags)
	}
}

// TestParse_AdversarialNoPanic checks that inputs that are malformed or
// incomplete do not panic. Each case may produce diags or a partial doc; the
// contract is simply: no panic.
func TestParse_AdversarialNoPanic(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{
			// Subgraph keyword with no matching end — parse error but no panic.
			name: "subgraph_no_end",
			src:  "flowchart TD\n  subgraph A\n    B --> C\n",
		},
		{
			// Only a %%{init}%% directive, no diagram body.
			name: "directive_only",
			src:  "%%{init: {}}%%\n",
		},
		{
			// Deeply nested subgraphs (5 levels) — stress test boundary recursion.
			name: "deeply_nested_subgraphs",
			src: "flowchart TD\n" +
				"  subgraph L1\n" +
				"    subgraph L2\n" +
				"      subgraph L3\n" +
				"        subgraph L4\n" +
				"          subgraph L5\n" +
				"            leaf[Leaf]\n" +
				"          end\n" +
				"        end\n" +
				"      end\n" +
				"    end\n" +
				"  end\n",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Parse(%q) panicked: %v", tc.name, r)
				}
			}()
			// No assertions on the result — just confirm no panic.
			_, _, _ = Parse([]byte(tc.src), Options{})
		})
	}
}
