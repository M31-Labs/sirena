package bake_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"m31labs.dev/sirena/bake"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// tempMD writes content to a temp .md file in a fresh temp dir and returns the
// path. The caller is responsible for cleanup via t.TempDir semantics.
func tempMD(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// readFile reads a file and returns its contents.
func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// assertSVG checks that data looks like a valid SVG document.
func assertSVG(t *testing.T, label string, data []byte) {
	t.Helper()
	s := string(data)
	if !strings.Contains(s, "<svg") || !strings.Contains(s, "</svg>") {
		t.Errorf("%s: SVG does not contain <svg…</svg>; got %d bytes starting with %q",
			label, len(data), truncate(s, 80))
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ── fixtures ──────────────────────────────────────────────────────────────────

// validMermaidBody is a simple flowchart that parses correctly.
const validMermaidBody = "flowchart LR\n  A[Node A] --> B[Node B]\n"

// validSirenaBody is a minimal self-contained sirena document.
const validSirenaBody = "service frontend\nservice backend\nfrontend -> backend: calls\n"

// invalidMermaidBody uses a keyword pattern sirena's mermaid parser treats as
// NOT-A-GRAPH (sequenceDiagram is unsupported).
const invalidMermaidBody = "sequenceDiagram\n  A ->> B: hello\n"

// mixedMD is a markdown file with one mermaid fence and one sirena fence.
var mixedMD = "# Architecture\n\n" +
	"```mermaid\n" + validMermaidBody + "```\n\n" +
	"Some text.\n\n" +
	"```sirena\n" + validSirenaBody + "```\n"

// ── Test 1: end-to-end bake (mermaid + sirena) ────────────────────────────────

func TestBake_MixedFences(t *testing.T) {
	mdPath := tempMD(t, "doc.md", mixedMD)
	dir := filepath.Dir(mdPath)

	res, err := bake.Bake(mdPath, bake.Options{})
	if err != nil {
		t.Fatalf("Bake returned error: %v", err)
	}
	if res.BlocksBaked != 2 {
		t.Errorf("BlocksBaked = %d, want 2", res.BlocksBaked)
	}
	if res.SVGsWritten != 2 {
		t.Errorf("SVGsWritten = %d, want 2", res.SVGsWritten)
	}
	if len(res.BlockErrors) != 0 {
		t.Errorf("unexpected BlockErrors: %v", res.BlockErrors)
	}

	// SVG files must exist and be valid.
	svg1 := filepath.Join(dir, "doc.1.svg")
	svg2 := filepath.Join(dir, "doc.2.svg")
	assertSVG(t, "doc.1.svg", readFile(t, svg1))
	assertSVG(t, "doc.2.svg", readFile(t, svg2))

	// Rewritten markdown must contain the baked-block markers.
	md := string(readFile(t, mdPath))
	if !strings.Contains(md, "<!-- sirena-baked:mermaid -->") {
		t.Errorf("rewritten md missing sirena-baked:mermaid marker")
	}
	if !strings.Contains(md, "<!-- sirena-baked:sirena -->") {
		t.Errorf("rewritten md missing sirena-baked:sirena marker")
	}
	if !strings.Contains(md, "![diagram 1](doc.1.svg)") {
		t.Errorf("rewritten md missing image link for block 1")
	}
	if !strings.Contains(md, "![diagram 2](doc.2.svg)") {
		t.Errorf("rewritten md missing image link for block 2")
	}

	// Source bodies must appear verbatim in the ```text fence (without \r).
	wantBody1 := strings.ReplaceAll(validMermaidBody, "\r", "")
	wantBody2 := strings.ReplaceAll(validSirenaBody, "\r", "")
	if !strings.Contains(md, wantBody1) {
		t.Errorf("rewritten md missing mermaid body; want substring %q", wantBody1)
	}
	if !strings.Contains(md, wantBody2) {
		t.Errorf("rewritten md missing sirena body; want substring %q", wantBody2)
	}

	// The baked block format must use <details>.
	if !strings.Contains(md, "<details><summary>diagram source</summary>") {
		t.Errorf("rewritten md missing <details> section")
	}

	// The ```text fence must appear.
	if !strings.Contains(md, "```text") {
		t.Errorf("rewritten md missing ```text fence")
	}
}

// ── Test 2: idempotency ───────────────────────────────────────────────────────

func TestBake_Idempotent(t *testing.T) {
	mdPath := tempMD(t, "doc.md", mixedMD)
	dir := filepath.Dir(mdPath)

	// First bake.
	if _, err := bake.Bake(mdPath, bake.Options{}); err != nil {
		t.Fatalf("first Bake error: %v", err)
	}
	md1 := readFile(t, mdPath)
	svg1_1 := readFile(t, filepath.Join(dir, "doc.1.svg"))
	svg1_2 := readFile(t, filepath.Join(dir, "doc.2.svg"))

	// Second bake — must be byte-identical.
	if _, err := bake.Bake(mdPath, bake.Options{}); err != nil {
		t.Fatalf("second Bake error: %v", err)
	}
	md2 := readFile(t, mdPath)
	svg2_1 := readFile(t, filepath.Join(dir, "doc.1.svg"))
	svg2_2 := readFile(t, filepath.Join(dir, "doc.2.svg"))

	if !bytes.Equal(md1, md2) {
		t.Errorf("idempotency: markdown changed on second bake")
	}
	if !bytes.Equal(svg1_1, svg2_1) {
		t.Errorf("idempotency: doc.1.svg changed on second bake")
	}
	if !bytes.Equal(svg1_2, svg2_2) {
		t.Errorf("idempotency: doc.2.svg changed on second bake")
	}
}

// TestBake_EditedBlockRebakes verifies that editing one baked block's source
// body and re-baking updates only that diagram.
func TestBake_EditedBlockRebakes(t *testing.T) {
	mdPath := tempMD(t, "doc.md", mixedMD)
	dir := filepath.Dir(mdPath)

	// First bake — establish baseline.
	if _, err := bake.Bake(mdPath, bake.Options{}); err != nil {
		t.Fatalf("first Bake error: %v", err)
	}
	svg2Before := readFile(t, filepath.Join(dir, "doc.2.svg"))

	// Edit the sirena body in the baked markdown: add a new edge.
	md := string(readFile(t, mdPath))
	md = strings.Replace(md,
		validSirenaBody,
		validSirenaBody+"backend -> frontend: responds\n",
		1)
	if err := os.WriteFile(mdPath, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	// Re-bake.
	if _, err := bake.Bake(mdPath, bake.Options{}); err != nil {
		t.Fatalf("re-bake error: %v", err)
	}

	svg2After := readFile(t, filepath.Join(dir, "doc.2.svg"))
	// The sirena SVG must differ because the source changed.
	if bytes.Equal(svg2Before, svg2After) {
		t.Errorf("doc.2.svg should have changed after source edit, but it is byte-identical")
	}
}

// ── Test 3: no-fence file ─────────────────────────────────────────────────────

func TestBake_NoFences(t *testing.T) {
	const content = "# Title\n\nJust plain text.\n"
	mdPath := tempMD(t, "plain.md", content)

	res, err := bake.Bake(mdPath, bake.Options{})
	if err != nil {
		t.Fatalf("Bake returned error: %v", err)
	}
	if res.BlocksBaked != 0 {
		t.Errorf("BlocksBaked = %d, want 0", res.BlocksBaked)
	}
	if res.SVGsWritten != 0 {
		t.Errorf("SVGsWritten = %d, want 0", res.SVGsWritten)
	}

	// File must be byte-identical.
	got := readFile(t, mdPath)
	if string(got) != content {
		t.Errorf("no-fence: file modified unexpectedly\ngot:\n%s\nwant:\n%s", got, content)
	}

	// No SVG files should have been created.
	dir := filepath.Dir(mdPath)
	svgs, _ := filepath.Glob(filepath.Join(dir, "*.svg"))
	if len(svgs) != 0 {
		t.Errorf("no-fence: unexpected SVG files: %v", svgs)
	}
}

// ── Test 4: failure handling ──────────────────────────────────────────────────

func TestBake_FailureNonDestructive(t *testing.T) {
	// A file with an invalid mermaid fence and a valid sirena fence.
	md := "# Test\n\n" +
		"```mermaid\n" + invalidMermaidBody + "```\n\n" +
		"```sirena\n" + validSirenaBody + "```\n"
	mdPath := tempMD(t, "mixed.md", md)
	dir := filepath.Dir(mdPath)

	res, err := bake.Bake(mdPath, bake.Options{})

	// Bake must return a non-nil error (at least one block failed).
	if err == nil {
		t.Errorf("Bake: expected non-nil error for block with invalid mermaid, got nil")
	}

	// Exactly 1 error recorded.
	if len(res.BlockErrors) != 1 {
		t.Errorf("BlockErrors len = %d, want 1; errors: %v", len(res.BlockErrors), res.BlockErrors)
	}

	// The valid sirena block must still have been baked.
	if res.BlocksBaked != 1 {
		t.Errorf("BlocksBaked = %d, want 1 (the valid sirena block)", res.BlocksBaked)
	}
	if res.SVGsWritten != 1 {
		t.Errorf("SVGsWritten = %d, want 1", res.SVGsWritten)
	}

	// doc.2.svg must exist (sirena is block 2) and be valid.
	svg2 := filepath.Join(dir, "mixed.2.svg")
	assertSVG(t, "mixed.2.svg", readFile(t, svg2))

	// doc.1.svg must NOT exist (failed block, never wrote SVG).
	svg1 := filepath.Join(dir, "mixed.1.svg")
	if _, statErr := os.Stat(svg1); statErr == nil {
		t.Errorf("mixed.1.svg must not exist for a failed render block")
	}

	// The invalid mermaid block must be left UNCHANGED in the output file.
	out := string(readFile(t, mdPath))
	if !strings.Contains(out, invalidMermaidBody) {
		t.Errorf("failed block source must be preserved in output file; missing:\n%s", invalidMermaidBody)
	}
	// The invalid block must NOT have been wrapped in sirena-baked markup.
	// (It keeps its original raw fence.)
	if strings.Contains(out, "<!-- sirena-baked:mermaid -->") {
		t.Errorf("failed block must not be wrapped in sirena-baked markup")
	}
}

// ── Test 5b: sirena via fence.Render ─────────────────────────────────────────

// TestBake_SirenaViaFence confirms the sirena render path uses fence.Render
// and produces a valid SVG for a sir block. It also verifies that an empty
// sirena body produces a block failure (not a hard error) and leaves the
// source unchanged.
func TestBake_SirenaViaFence(t *testing.T) {
	t.Run("valid sir block bakes to SVG", func(t *testing.T) {
		md := "```sir\n" + validSirenaBody + "```\n"
		mdPath := tempMD(t, "sirfence.md", md)
		dir := filepath.Dir(mdPath)

		res, err := bake.Bake(mdPath, bake.Options{})
		if err != nil {
			t.Fatalf("Bake returned error: %v", err)
		}
		if res.BlocksBaked != 1 {
			t.Errorf("BlocksBaked = %d, want 1", res.BlocksBaked)
		}
		svg1 := filepath.Join(dir, "sirfence.1.svg")
		assertSVG(t, "sirfence.1.svg", readFile(t, svg1))
	})

	t.Run("empty sirena body records block failure and preserves source", func(t *testing.T) {
		// An empty fence body produces no SVG from fence.Render.
		const origMD = "```sirena\n```\n"
		mdPath := tempMD(t, "empty_sirena.md", origMD)

		res, err := bake.Bake(mdPath, bake.Options{})
		// Must return non-nil error (block failure).
		if err == nil {
			t.Errorf("expected non-nil error for empty sirena body, got nil")
		}
		if len(res.BlockErrors) == 0 {
			t.Errorf("expected BlockErrors to be non-empty")
		}
		// Source must be preserved unchanged.
		got := string(readFile(t, mdPath))
		if got != origMD {
			t.Errorf("source modified unexpectedly\ngot:\n%s\nwant:\n%s", got, origMD)
		}
	})
}

// ── Test 6: file permissions preserved ───────────────────────────────────────

func TestBake_PreservesFileMode(t *testing.T) {
	md := "```mermaid\n" + validMermaidBody + "```\n"
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "perms.md")
	if err := os.WriteFile(mdPath, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := bake.Bake(mdPath, bake.Options{}); err != nil {
		t.Fatalf("Bake returned error: %v", err)
	}

	fi, err := os.Stat(mdPath)
	if err != nil {
		t.Fatalf("stat after bake: %v", err)
	}
	if got := fi.Mode().Perm(); got != 0o644 {
		t.Errorf("file mode = %04o, want 0644", got)
	}
}

// ── Test 7: byte-identical skip (no mtime churn) ──────────────────────────────

func TestBake_ByteIdenticalSkip(t *testing.T) {
	t.Run("no-fence file is not rewritten", func(t *testing.T) {
		const content = "# Plain\n\nNo fences here.\n"
		mdPath := tempMD(t, "plain2.md", content)

		fi0, err := os.Stat(mdPath)
		if err != nil {
			t.Fatal(err)
		}
		mtime0 := fi0.ModTime()

		if _, err := bake.Bake(mdPath, bake.Options{}); err != nil {
			t.Fatalf("Bake returned error: %v", err)
		}

		fi1, err := os.Stat(mdPath)
		if err != nil {
			t.Fatal(err)
		}
		// Content must be identical.
		if string(readFile(t, mdPath)) != content {
			t.Errorf("content changed for no-fence file")
		}
		// Mtime must be unchanged (file was not touched).
		if !fi1.ModTime().Equal(mtime0) {
			t.Errorf("mtime changed for no-fence file: %v → %v", mtime0, fi1.ModTime())
		}
	})

	t.Run("all-failed file is not rewritten", func(t *testing.T) {
		const origMD = "```mermaid\n" + invalidMermaidBody + "```\n"
		mdPath := tempMD(t, "allfail.md", origMD)

		fi0, err := os.Stat(mdPath)
		if err != nil {
			t.Fatal(err)
		}
		mtime0 := fi0.ModTime()

		res, err := bake.Bake(mdPath, bake.Options{})
		if err == nil {
			t.Errorf("expected non-nil error for invalid block")
		}
		if len(res.BlockErrors) == 0 {
			t.Errorf("expected BlockErrors")
		}

		// Content must be identical.
		if string(readFile(t, mdPath)) != origMD {
			t.Errorf("content changed when all blocks failed")
		}
		// Mtime must be unchanged.
		fi1, err := os.Stat(mdPath)
		if err != nil {
			t.Fatal(err)
		}
		if !fi1.ModTime().Equal(mtime0) {
			t.Errorf("mtime changed when all blocks failed: %v → %v", mtime0, fi1.ModTime())
		}
	})
}

// ── Test 5: CRLF body ─────────────────────────────────────────────────────────

func TestBake_CRLFBody(t *testing.T) {
	// Simulate a CRLF-encoded mermaid fence (Windows-style).
	crlfBody := strings.ReplaceAll(validMermaidBody, "\n", "\r\n")
	md := "```mermaid\r\n" + crlfBody + "```\r\n"
	mdPath := tempMD(t, "crlf.md", md)
	dir := filepath.Dir(mdPath)

	res, err := bake.Bake(mdPath, bake.Options{})
	if err != nil {
		t.Fatalf("CRLF Bake error: %v", err)
	}
	if res.BlocksBaked != 1 {
		t.Errorf("BlocksBaked = %d, want 1", res.BlocksBaked)
	}

	// SVG must be valid.
	svg1 := filepath.Join(dir, "crlf.1.svg")
	assertSVG(t, "crlf.1.svg", readFile(t, svg1))

	// The body in the ```text fence must not contain \r.
	out := string(readFile(t, mdPath))
	if strings.Contains(out, "\r") {
		t.Errorf("rewritten markdown contains \\r after CRLF strip")
	}
}
