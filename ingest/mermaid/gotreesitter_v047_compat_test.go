package mermaid

import (
	"bytes"
	"testing"
)

// TestGotreesitterV047MermaidCompatibility keeps common Mermaid authoring
// syntax working with the v0.47 runtime: indented statements and a slash in
// a multi-word arrow label.
func TestGotreesitterV047MermaidCompatibility(t *testing.T) {
	src := []byte("flowchart LR\n" +
		"  Browser([Browser])\n" +
		"  Gateway[API Gateway]\n" +
		"  Browser -->|HTTP POST /login| Gateway\n")
	doc, diags, err := Parse(src, Options{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if doc == nil || len(doc.Systems) != 1 || len(doc.Systems[0].Edges) != 1 {
		t.Fatalf("unexpected document shape: %#v", doc)
	}
	if got := doc.Systems[0].Edges[0].Label; got != "HTTP POST /login" {
		t.Fatalf("edge label = %q, want original Mermaid text", got)
	}
	for _, d := range diags {
		if d.Code == "SIR-MERMAID-NOT-A-GRAPH" || d.Code == "SIR-MERMAID-PARSE" {
			t.Fatalf("unexpected fatal/parser diagnostic: %+v", d)
		}
	}
}

// TestGotreesitterV047PreservesNodeLabelPipesAndSlashes ensures the
// parser-safe arrow-label rewrite does not touch ordinary node-label text.
// A node label may contain both characters even though an edge label uses
// pipes as delimiters.
func TestGotreesitterV047PreservesNodeLabelPipesAndSlashes(t *testing.T) {
	src := []byte("flowchart LR\n" +
		"  A[\"foo|bar/baz\"] --> B\n")
	clean, _, _ := normalize(src)
	if !bytes.Contains(clean, []byte("foo|bar/baz")) {
		t.Fatalf("node-label text was rewritten in normalized source: %q", clean)
	}
	doc, diags, err := Parse(src, Options{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if doc == nil || len(doc.Systems) != 1 || len(doc.Systems[0].Elements) != 2 {
		t.Fatalf("unexpected document shape: %#v", doc)
	}
	if findElement(doc.Systems[0].Elements, "A") == nil {
		t.Fatal("element A not found")
	}
	for _, d := range diags {
		if d.Code == "SIR-MERMAID-NOT-A-GRAPH" || d.Code == "SIR-MERMAID-PARSE" {
			t.Fatalf("unexpected fatal/parser diagnostic: %+v", d)
		}
	}
}

func TestGotreesitterV047UnclosedArrowLabelDoesNotRewriteFollowingNode(t *testing.T) {
	src := []byte("flowchart LR\n" +
		"  A -->|missing B[foo/bar]\n")
	clean, _, _ := normalize(src)
	if !bytes.Contains(clean, []byte("B[foo/bar]")) {
		t.Fatalf("incomplete arrow label rewrote following node text: %q", clean)
	}
}
