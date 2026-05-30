package mermaid

import (
	"testing"

	"m31labs.dev/sirena"
)

// TestInfer_CylinderAndEdgeKeyword tests the primary inference case:
// cylinder→Database and edge label matching a sirena edge-kind keyword.
func TestInfer_CylinderAndEdgeKeyword(t *testing.T) {
	src := []byte("flowchart LR\n  A[(DB)] -->|reads| B[Svc]\n")

	t.Run("infer_on", func(t *testing.T) {
		doc, _, err := Parse(src, Options{Infer: true})
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}
		sys := doc.Systems[0]

		// Find element A.
		var elemA *sirena.Element
		for _, e := range sys.Elements {
			if e.Name == "A" {
				elemA = e
				break
			}
		}
		if elemA == nil {
			t.Fatal("element A not found")
		}

		// A should be promoted to ElementKindDatabase.
		if elemA.Kind != sirena.ElementKindDatabase {
			t.Errorf("element A: want ElementKindDatabase, got %v", elemA.Kind)
		}

		// Shape metadata must still be present.
		shapeVal, ok := elemA.Metadata["shape"]
		if !ok {
			t.Error("element A: shape metadata missing after inference")
		} else if ident, ok := shapeVal.(sirena.Ident); !ok || ident.Value != "cylinder" {
			t.Errorf("element A: want shape=cylinder, got %v", shapeVal)
		}

		// Edge A→B should be promoted to EdgeKindReads.
		if len(sys.Edges) != 1 {
			t.Fatalf("want 1 edge, got %d", len(sys.Edges))
		}
		edge := sys.Edges[0]
		if edge.Kind != sirena.EdgeKindReads {
			t.Errorf("edge A→B: want EdgeKindReads, got %v", edge.Kind)
		}
		// Label must be preserved.
		if edge.Label != "reads" {
			t.Errorf("edge A→B: want Label %q, got %q", "reads", edge.Label)
		}
	})

	t.Run("infer_off", func(t *testing.T) {
		doc, _, err := Parse(src, Options{Infer: false})
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}
		sys := doc.Systems[0]

		var elemA *sirena.Element
		for _, e := range sys.Elements {
			if e.Name == "A" {
				elemA = e
				break
			}
		}
		if elemA == nil {
			t.Fatal("element A not found")
		}

		// Without inference, A stays ElementKindNode.
		if elemA.Kind != sirena.ElementKindNode {
			t.Errorf("element A: want ElementKindNode, got %v", elemA.Kind)
		}

		// Shape metadata still present.
		shapeVal, ok := elemA.Metadata["shape"]
		if !ok {
			t.Error("element A: shape metadata missing (infer_off)")
		} else if ident, ok := shapeVal.(sirena.Ident); !ok || ident.Value != "cylinder" {
			t.Errorf("element A: want shape=cylinder, got %v", shapeVal)
		}

		// Edge stays EdgeKindFlow.
		if len(sys.Edges) != 1 {
			t.Fatalf("want 1 edge, got %d", len(sys.Edges))
		}
		if sys.Edges[0].Kind != sirena.EdgeKindFlow {
			t.Errorf("edge A→B: want EdgeKindFlow, got %v", sys.Edges[0].Kind)
		}
	})
}

// TestInfer_AmbiguousShapeStaysNode ensures diamond/rect are not promoted.
func TestInfer_AmbiguousShapeStaysNode(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"diamond", "flowchart LR\n  A{Decision}\n"},
		{"rect", "flowchart LR\n  A[Box]\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc, _, err := Parse([]byte(tc.src), Options{Infer: true})
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}
			sys := doc.Systems[0]
			if len(sys.Elements) == 0 {
				t.Fatal("no elements found")
			}
			elem := sys.Elements[0]
			if elem.Kind != sirena.ElementKindNode {
				t.Errorf("shape %s: want ElementKindNode, got %v", tc.name, elem.Kind)
			}
		})
	}
}

// TestInfer_NonKeywordLabelStaysFlow ensures an edge with a non-keyword label
// stays EdgeKindFlow but the label is preserved.
func TestInfer_NonKeywordLabelStaysFlow(t *testing.T) {
	src := []byte("flowchart LR\n  A -->|HTTP| B\n")
	doc, _, err := Parse(src, Options{Infer: true})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	sys := doc.Systems[0]
	if len(sys.Edges) != 1 {
		t.Fatalf("want 1 edge, got %d", len(sys.Edges))
	}
	edge := sys.Edges[0]
	if edge.Kind != sirena.EdgeKindFlow {
		t.Errorf("want EdgeKindFlow, got %v", edge.Kind)
	}
	if edge.Label != "HTTP" {
		t.Errorf("want Label %q, got %q", "HTTP", edge.Label)
	}
}
