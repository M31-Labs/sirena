package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validMermaidMD is a minimal markdown file with one renderable mermaid fence.
const validMermaidMD = "# Doc\n\n```mermaid\nflowchart LR\n  A --> B\n```\n"

// invalidFenceMD contains a sequenceDiagram fence — sirena treats this as
// NOT-A-GRAPH, so it will produce a block error.
const invalidFenceMD = "# Bad\n\n```mermaid\nsequenceDiagram\n  A ->> B: hi\n```\n"

// ── TestRunBake_SingleFile ────────────────────────────────────────────────────

func TestRunBake_SingleFile(t *testing.T) {
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(mdPath, []byte(validMermaidMD), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errBuf bytes.Buffer
	code := RunBake([]string{mdPath}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit %d; stderr: %s", code, errBuf.String())
	}

	// Summary line on stdout must reference the path and diagram counts.
	summary := out.String()
	if !strings.Contains(summary, "doc.md") {
		t.Errorf("stdout missing file name; got %q", summary)
	}
	if !strings.Contains(summary, "1") {
		t.Errorf("stdout missing diagram count; got %q", summary)
	}

	// SVG must have been written next to the markdown.
	svgPath := filepath.Join(dir, "doc.1.svg")
	data, err := os.ReadFile(svgPath)
	if err != nil {
		t.Fatalf("doc.1.svg not written: %v", err)
	}
	if !strings.Contains(string(data), "<svg") {
		t.Errorf("doc.1.svg is not valid SVG; got %.80q", string(data))
	}

	// Markdown must have been rewritten to a baked block.
	md, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(md), "<!-- sirena-baked:mermaid -->") {
		t.Errorf("markdown not rewritten to baked block; got:\n%s", md)
	}
}

// ── TestRunBake_MultiFile ─────────────────────────────────────────────────────

func TestRunBake_MultiFile(t *testing.T) {
	dir := t.TempDir()
	md1 := filepath.Join(dir, "a.md")
	md2 := filepath.Join(dir, "b.md")
	if err := os.WriteFile(md1, []byte(validMermaidMD), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(md2, []byte(validMermaidMD), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errBuf bytes.Buffer
	code := RunBake([]string{md1, md2}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit %d; stderr: %s", code, errBuf.String())
	}

	// Both SVGs must exist.
	for _, name := range []string{"a.1.svg", "b.1.svg"} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s not written: %v", name, err)
		}
	}

	// Summary lines for both files must appear on stdout.
	summary := out.String()
	if !strings.Contains(summary, "a.md") {
		t.Errorf("stdout missing a.md; got %q", summary)
	}
	if !strings.Contains(summary, "b.md") {
		t.Errorf("stdout missing b.md; got %q", summary)
	}
}

// ── TestRunBake_PartialFailure ────────────────────────────────────────────────

// TestRunBake_PartialFailure verifies that a file with a bad fence returns
// non-zero, prints the error on stderr, but still bakes the valid sibling file.
func TestRunBake_PartialFailure(t *testing.T) {
	dir := t.TempDir()
	badMD := filepath.Join(dir, "bad.md")
	goodMD := filepath.Join(dir, "good.md")
	if err := os.WriteFile(badMD, []byte(invalidFenceMD), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(goodMD, []byte(validMermaidMD), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errBuf bytes.Buffer
	code := RunBake([]string{badMD, goodMD}, &out, &errBuf)
	if code == 0 {
		t.Fatalf("expected non-zero exit for partial failure, got 0")
	}

	// Error must appear on stderr.
	if errBuf.Len() == 0 {
		t.Error("expected error on stderr, got nothing")
	}

	// The valid sibling (good.md) must still have been baked.
	svgPath := filepath.Join(dir, "good.1.svg")
	if _, err := os.Stat(svgPath); err != nil {
		t.Errorf("good.1.svg should have been written despite bad.md failure: %v", err)
	}
}

// ── TestRunBake_Misuse ────────────────────────────────────────────────────────

func TestRunBake_NoArgs(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := RunBake([]string{}, &out, &errBuf)
	if code != 2 {
		t.Errorf("no-args exit = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "usage") && !strings.Contains(errBuf.String(), "Usage") {
		t.Errorf("stderr missing usage hint; got %q", errBuf.String())
	}
}

func TestRunBake_UnknownTheme(t *testing.T) {
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(mdPath, []byte(validMermaidMD), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errBuf bytes.Buffer
	code := RunBake([]string{"--theme", "not-a-theme", mdPath}, &out, &errBuf)
	if code != 2 {
		t.Errorf("unknown theme exit = %d, want 2; stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "not-a-theme") {
		t.Errorf("stderr missing theme name; got %q", errBuf.String())
	}
}
