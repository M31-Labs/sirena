package mermaid

import (
	"bytes"
	"testing"

	"m31labs.dev/sirena"
)

// TestNormalize_GraphKeywordAndSemicolon verifies that the legacy "graph"
// keyword is rewritten to "flowchart" and that statement-position semicolons
// are replaced with spaces — producing a clean source with no SIR-MERMAID-*
// diagnostics.
func TestNormalize_GraphKeywordAndSemicolon(t *testing.T) {
	src := []byte("graph LR;\n a-->b;\n")
	clean, diags, smap := normalize(src)

	// No styling diags expected.
	for _, d := range diags {
		t.Errorf("unexpected diagnostic %q: %s", d.Code, d.Message)
	}

	// "graph" must have been rewritten to "flowchart".
	const wantPrefix = "flowchart"
	if len(clean) < len(wantPrefix) || string(clean[:len(wantPrefix)]) != wantPrefix {
		t.Errorf("want clean to start with %q, got %q", wantPrefix, string(clean[:min(len(clean), 20)]))
	}

	// Semicolons must be replaced with spaces (not present in output).
	for i, b := range clean {
		if b == ';' {
			t.Errorf("unexpected ';' at clean offset %d", i)
		}
	}

	// srcMap: offset before keyword end maps to itself (identity below shift).
	// The keyword ends at offset 9 ("flowchart" = 9 bytes).
	// In the original, offset 0 → 0; offset after keyword end shifts back by 4.
	// "flowchart" is 9 bytes; "graph" is 5 bytes; so shiftAt == 9.
	// An offset of 9 in clean == offset 5 in orig (9 - 4 == 5).
	if got := smap.orig(9); got != 5 {
		t.Errorf("smap.orig(9) = %d, want 5", got)
	}
	// An offset below the keyword end is identity.
	if got := smap.orig(3); got != 3 {
		t.Errorf("smap.orig(3) = %d, want 3", got)
	}
	// Verify the full cleaned source parses without error by calling Parse.
	doc, parseDiags, err := Parse([]byte("graph LR;\n a-->b;\n"), Options{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(doc.Systems) != 1 {
		t.Fatalf("want 1 system, got %d", len(doc.Systems))
	}
	for _, d := range parseDiags {
		if d.Code == "SIR-MERMAID-PARSE" {
			t.Errorf("unexpected SIR-MERMAID-PARSE diagnostic: %s", d.Message)
		}
	}
}

// TestNormalize_ClassDefStripped verifies that a classDef line is deleted
// from the clean buffer with a SIR-MERMAID-STYLE-DROPPED warning, the
// srcMap correctly maps the post-deletion clean buffer offsets back to the
// original source, and no phantom element appears in the parsed document.
func TestNormalize_ClassDefStripped(t *testing.T) {
	// src: "flowchart LR\n A-->B\n classDef x fill:#f9f\n"
	//       offset:         0..12  13..19   20..41
	// "flowchart LR\n" = 13 bytes, " A-->B\n" = 7 bytes, " classDef x fill:#f9f\n" = 22 bytes
	src := []byte("flowchart LR\n A-->B\n classDef x fill:#f9f\n")
	clean, diags, smap := normalize(src)

	// Expect exactly one SIR-MERMAID-STYLE-DROPPED warning.
	styleDrops := 0
	for _, d := range diags {
		if d.Code == "SIR-MERMAID-STYLE-DROPPED" {
			styleDrops++
			if d.Severity != sirena.SeverityWarning {
				t.Errorf("SIR-MERMAID-STYLE-DROPPED should be Warning, got %v", d.Severity)
			}
			// Range should point at the original classDef line.
			// "flowchart LR\n" = 13 bytes, " A-->B\n" = 7 bytes → offset 20.
			origLineStart := 20
			if d.Range.Start != origLineStart {
				t.Errorf("diagnostic Range.Start = %d, want %d", d.Range.Start, origLineStart)
			}
		}
	}
	if styleDrops != 1 {
		t.Errorf("want 1 SIR-MERMAID-STYLE-DROPPED, got %d", styleDrops)
	}

	// The classDef line is deleted: clean must be shorter than src.
	// src = 42 bytes, classDef line = " classDef x fill:#f9f\n" = 22 bytes.
	// clean must be 42 - 22 = 20 bytes.
	wantCleanLen := len(src) - len(" classDef x fill:#f9f\n")
	if len(clean) != wantCleanLen {
		t.Errorf("clean length = %d, want %d (deletion should have shortened it)", len(clean), wantCleanLen)
	}

	// The clean buffer must not contain "classDef".
	if bytes.Contains(clean, []byte("classDef")) {
		t.Error("clean buffer contains 'classDef' — line was not deleted")
	}

	// The srcMap must correctly map clean offsets to original offsets.
	// clean = "flowchart LR\n A-->B\n" (20 bytes).
	// Offset 19 in clean is the '\n' at end of " A-->B\n" (originally also offset 19).
	// No edits affect bytes before the deleted line (offset 0..19), so identity.
	if got := smap.orig(0); got != 0 {
		t.Errorf("smap.orig(0) = %d, want 0", got)
	}
	if got := smap.orig(13); got != 13 {
		t.Errorf("smap.orig(13) = %d, want 13", got)
	}

	// Parse the original source and confirm no phantom "classDef" element.
	doc, _, err := Parse(src, Options{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(doc.Systems) != 1 {
		t.Fatalf("want 1 system, got %d", len(doc.Systems))
	}
	for _, el := range doc.Systems[0].Elements {
		if el.Name == "classDef" || el.Name == "x" {
			t.Errorf("phantom element %q from classDef line should not appear", el.Name)
		}
	}
}

// TestNormalize_ClassDefAndClassNoContentLoss verifies the critical case:
// classDef + class application statements interspersed with edges must NOT
// cause content loss. All three edges must survive, no SIR-MERMAID-PARSE
// diagnostic must be emitted, and no phantom elements from styling keywords
// must appear.
func TestNormalize_ClassDefAndClassNoContentLoss(t *testing.T) {
	src := []byte("flowchart LR\n  A --> B\n  classDef foo fill:#f9f\n  C --> D\n  class A foo\n  E --> F\n")

	doc, diags, err := Parse(src, Options{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if doc == nil {
		t.Fatal("Parse returned nil doc")
	}
	if len(doc.Systems) == 0 {
		t.Fatal("want at least 1 system")
	}

	// No SIR-MERMAID-PARSE diagnostic allowed.
	for _, d := range diags {
		if d.Code == "SIR-MERMAID-PARSE" {
			t.Errorf("SIR-MERMAID-PARSE diagnostic present (content loss): %s", d.Message)
		}
	}

	// Exactly 2 STYLE-DROPPED diagnostics: one for classDef, one for class.
	styleDropped := 0
	for _, d := range diags {
		if d.Code == "SIR-MERMAID-STYLE-DROPPED" {
			styleDropped++
		}
	}
	if styleDropped != 2 {
		t.Errorf("want 2 SIR-MERMAID-STYLE-DROPPED (classDef + class), got %d", styleDropped)
	}

	// All three edges must survive.
	edges := doc.Systems[0].Edges
	if len(edges) != 3 {
		t.Errorf("want 3 edges (A→B, C→D, E→F), got %d", len(edges))
	}

	// No phantom elements from styling keywords.
	for _, el := range doc.Systems[0].Elements {
		switch el.Name {
		case "classDef", "class", "foo":
			t.Errorf("phantom element %q from styling line should not appear", el.Name)
		}
	}
}

// TestNormalize_ClassVsClassDef verifies that "classDef" still matches its own
// rule and that "class" as the first token of a line matches the class-
// application rule (whole-word, not prefix of classDef).
func TestNormalize_ClassVsClassDef(t *testing.T) {
	src := []byte("flowchart LR\n  A --> B\n  classDef myStyle fill:#f9f\n  class A myStyle\n  B --> C\n")

	_, diags, _ := normalize(src)

	var classDefs, classApps int
	for _, d := range diags {
		if d.Code != "SIR-MERMAID-STYLE-DROPPED" {
			continue
		}
		switch d.Message {
		case "Mermaid styling directive dropped (no sirena equivalent): classDef":
			classDefs++
		case "Mermaid styling directive dropped (no sirena equivalent): class":
			classApps++
		}
	}
	if classDefs != 1 {
		t.Errorf("want 1 STYLE-DROPPED for classDef, got %d", classDefs)
	}
	if classApps != 1 {
		t.Errorf("want 1 STYLE-DROPPED for class application, got %d", classApps)
	}

	// Parse — no PARSE diagnostic, both edges survive.
	doc, parseDiags, err := Parse(src, Options{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	for _, d := range parseDiags {
		if d.Code == "SIR-MERMAID-PARSE" {
			t.Errorf("SIR-MERMAID-PARSE present: %s", d.Message)
		}
	}
	if doc == nil || len(doc.Systems) == 0 {
		t.Fatal("nil doc or no systems")
	}
	if got := len(doc.Systems[0].Edges); got != 2 {
		t.Errorf("want 2 edges (A→B, B→C), got %d", got)
	}
}

// TestNormalize_Negatives checks that "graphql" is not rewritten, that a
// semicolon inside a label bracket is left untouched, and that "classDef"
// inside a label is not stripped.
func TestNormalize_Negatives(t *testing.T) {
	// (a) "graphql" id — the first content line is "flowchart LR", so the
	// "graphql" keyword appears inside a vertex id and must not be rewritten.
	srcGraphQL := []byte("flowchart LR\n graphql-->B\n")
	clean, _, _ := normalize(srcGraphQL)
	// The flowchart keyword was already present — the rewriter must not
	// have replaced any occurrence of "graph" inside the identifier.
	// "graphql" is on line 2 (inside the diagram), not a leading keyword.
	if string(clean) != string(srcGraphQL) {
		t.Errorf("graphql id: clean != src; got %q", string(clean))
	}

	// (b) A "graph" id that starts a diagram line but is immediately a keyword
	// does get rewritten — ensure the negative: "graphql" token at statement
	// start is not confused as the diagram-type keyword.
	srcGraphQLLeading := []byte("graphql LR\n A-->B\n")
	cleanGQL, _, _ := normalize(srcGraphQLLeading)
	// "graphql" does not match "graph<space>" so it must be left untouched.
	if string(cleanGQL) != string(srcGraphQLLeading) {
		t.Errorf("graphql leading token: should not be rewritten; got %q", string(cleanGQL))
	}

	// (c) Semicolon inside label brackets — must not be replaced.
	srcLabel := []byte("flowchart LR\n A[\"a;b\"]-->B\n")
	cleanLabel, _, _ := normalize(srcLabel)
	// Find the semicolon in cleanLabel — it must still be ';'.
	found := false
	for _, b := range cleanLabel {
		if b == ';' {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("semicolon inside quoted label was incorrectly neutralized")
	}

	// (d) "classDef" appearing inside a label must not be stripped.
	srcClassDefLabel := []byte("flowchart LR\n A[\"classDef x\"]-->B\n")
	cleanCDL, diags, _ := normalize(srcClassDefLabel)
	for _, d := range diags {
		if d.Code == "SIR-MERMAID-STYLE-DROPPED" {
			t.Errorf("SIR-MERMAID-STYLE-DROPPED emitted for classDef inside a label")
		}
	}
	// The line must be untouched (no blanking occurred).
	const lineContent = " A[\"classDef x\"]-->B"
	if string(cleanCDL[13:13+len(lineContent)]) != lineContent {
		t.Errorf("classDef in label: line should be untouched, got %q", string(cleanCDL[13:13+len(lineContent)]))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
