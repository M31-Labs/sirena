package bake

import (
	"strings"
	"testing"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func checkBlock(t *testing.T, tag string, got Block, wantKind Kind, wantLang, wantBody string) {
	t.Helper()
	if got.Kind != wantKind {
		t.Errorf("%s: Kind = %v, want %v", tag, got.Kind, wantKind)
	}
	if got.Lang != wantLang {
		t.Errorf("%s: Lang = %q, want %q", tag, got.Lang, wantLang)
	}
	if got.Body != wantBody {
		t.Errorf("%s: Body = %q, want %q", tag, got.Body, wantBody)
	}
	if got.Start < 0 || got.End <= got.Start {
		t.Errorf("%s: invalid span [%d, %d)", tag, got.Start, got.End)
	}
}

// spanMatchesRegion verifies that src[b.Start:b.End] contains the expected
// first and last non-empty lines to catch off-by-one errors in span accounting.
func spanMatchesRegion(t *testing.T, tag string, src []byte, b Block, wantFirst, wantLast string) {
	t.Helper()
	region := string(src[b.Start:b.End])
	lines := strings.Split(strings.TrimRight(region, "\n"), "\n")
	if len(lines) == 0 {
		t.Errorf("%s: empty span region", tag)
		return
	}
	if lines[0] != wantFirst {
		t.Errorf("%s: span first line = %q, want %q", tag, lines[0], wantFirst)
	}
	if lines[len(lines)-1] != wantLast {
		t.Errorf("%s: span last line = %q, want %q", tag, lines[len(lines)-1], wantLast)
	}
}

// ── TestScan_RawMermaid ───────────────────────────────────────────────────────

func TestScan_RawMermaid(t *testing.T) {
	src := []byte("```mermaid\ngraph LR\n  A --> B\n```\n")
	blocks := Scan(src)
	if len(blocks) != 1 {
		t.Fatalf("want 1 block, got %d", len(blocks))
	}
	checkBlock(t, "mermaid", blocks[0], KindRaw, "mermaid", "graph LR\n  A --> B\n")
	spanMatchesRegion(t, "mermaid", src, blocks[0], "```mermaid", "```")
}

// ── TestScan_RawSirena ────────────────────────────────────────────────────────

func TestScan_RawSirena(t *testing.T) {
	src := []byte("```sirena\nA -> B\n```\n")
	blocks := Scan(src)
	if len(blocks) != 1 {
		t.Fatalf("want 1 block (sirena), got %d", len(blocks))
	}
	checkBlock(t, "sirena", blocks[0], KindRaw, "sirena", "A -> B\n")
}

func TestScan_RawSir(t *testing.T) {
	src := []byte("```sir\nA -> B\n```\n")
	blocks := Scan(src)
	if len(blocks) != 1 {
		t.Fatalf("want 1 block (sir), got %d", len(blocks))
	}
	checkBlock(t, "sir", blocks[0], KindRaw, "sir", "A -> B\n")
}

// ── TestScan_AlreadyBaked ─────────────────────────────────────────────────────

// bakedDoc is a minimal already-baked block as Phase C will emit it.
const bakedDoc = `<!-- sirena-baked:mermaid -->
![diagram 1](foo.1.svg)
<details><summary>diagram source</summary>

` + "```" + `text
graph LR
  A --> B
` + "```" + `

</details>
`

func TestScan_AlreadyBaked(t *testing.T) {
	src := []byte(bakedDoc)
	blocks := Scan(src)
	if len(blocks) != 1 {
		t.Fatalf("want 1 baked block, got %d", len(blocks))
	}
	checkBlock(t, "baked", blocks[0], KindBaked, "mermaid", "graph LR\n  A --> B\n")
	spanMatchesRegion(t, "baked", src, blocks[0], "<!-- sirena-baked:mermaid -->", "</details>")
}

// ── TestScan_MultipleFences ───────────────────────────────────────────────────

func TestScan_MultipleFences(t *testing.T) {
	src := []byte("# Heading\n\n```mermaid\ngraph LR\n  A --> B\n```\n\nSome text.\n\n```sirena\nX -> Y\n```\n")
	blocks := Scan(src)
	if len(blocks) != 2 {
		t.Fatalf("want 2 blocks, got %d", len(blocks))
	}
	checkBlock(t, "block[0]", blocks[0], KindRaw, "mermaid", "graph LR\n  A --> B\n")
	checkBlock(t, "block[1]", blocks[1], KindRaw, "sirena", "X -> Y\n")
	// spans must be in document order and non-overlapping
	if blocks[1].Start <= blocks[0].End {
		t.Errorf("spans overlap: [%d,%d) and [%d,%d)", blocks[0].Start, blocks[0].End, blocks[1].Start, blocks[1].End)
	}
}

// ── TestScan_OpaqueNesting ────────────────────────────────────────────────────

// A ```go block whose body literally contains ```mermaid must yield ZERO blocks.
func TestScan_OpaqueNesting(t *testing.T) {
	src := []byte("```go\nfmt.Println(`hello`)\n// ```mermaid\n// graph LR\n//   A-->B\n// ```\n```\n")
	blocks := Scan(src)
	if len(blocks) != 0 {
		t.Errorf("want 0 blocks (opaque nesting), got %d: %+v", len(blocks), blocks)
	}
}

// A more direct nesting test: literal mermaid fence inside a go fence.
func TestScan_OpaqueNesting_Direct(t *testing.T) {
	src := []byte("```go\n```mermaid\nA-->B\n```\n```\n")
	blocks := Scan(src)
	if len(blocks) != 0 {
		t.Errorf("want 0 blocks (direct nesting), got %d: %+v", len(blocks), blocks)
	}
}

// ── TestScan_TildeFence ───────────────────────────────────────────────────────

func TestScan_TildeFence(t *testing.T) {
	src := []byte("~~~mermaid\ngraph LR\n  A --> B\n~~~\n")
	blocks := Scan(src)
	if len(blocks) != 0 {
		t.Errorf("want 0 blocks (tilde fence not supported), got %d", len(blocks))
	}
}

// ── TestScan_NoFences ─────────────────────────────────────────────────────────

func TestScan_NoFences(t *testing.T) {
	src := []byte("# Hello\n\nJust plain text.\n\nNo code blocks here.\n")
	blocks := Scan(src)
	if len(blocks) != 0 {
		t.Errorf("want 0 blocks (no fences), got %d", len(blocks))
	}
}

// ── TestScan_BodyWithArrows ───────────────────────────────────────────────────

// Mermaid bodies contain --> and -- freely; they must be captured verbatim.
func TestScan_BodyWithArrows(t *testing.T) {
	body := "graph LR\n  A --> B\n  B --label--> C\n  C -- text --> D\n"
	src := []byte("```mermaid\n" + body + "```\n")
	blocks := Scan(src)
	if len(blocks) != 1 {
		t.Fatalf("want 1 block, got %d", len(blocks))
	}
	if blocks[0].Body != body {
		t.Errorf("Body = %q, want %q", blocks[0].Body, body)
	}
}

// ── TestScan_FourBacktickFence ────────────────────────────────────────────────

// A fence opened with ```` must be closed by ```` not ```.
func TestScan_FourBacktickFence(t *testing.T) {
	src := []byte("````mermaid\ngraph LR\n  A --> B\n````\n")
	blocks := Scan(src)
	if len(blocks) != 1 {
		t.Fatalf("want 1 block (4-backtick), got %d", len(blocks))
	}
	checkBlock(t, "4bt", blocks[0], KindRaw, "mermaid", "graph LR\n  A --> B\n")
}

// ── TestScan_MixedRawAndBaked ─────────────────────────────────────────────────

func TestScan_MixedRawAndBaked(t *testing.T) {
	src := []byte("# Doc\n\n" + bakedDoc + "\nSome text.\n\n```sirena\nX -> Y\n```\n")
	blocks := Scan(src)
	if len(blocks) != 2 {
		t.Fatalf("want 2 blocks (1 baked + 1 raw), got %d", len(blocks))
	}
	if blocks[0].Kind != KindBaked {
		t.Errorf("blocks[0].Kind = %v, want KindBaked", blocks[0].Kind)
	}
	if blocks[1].Kind != KindRaw {
		t.Errorf("blocks[1].Kind = %v, want KindRaw", blocks[1].Kind)
	}
}

// ── TestScan_NonDiagramFenceSkipped ──────────────────────────────────────────

func TestScan_NonDiagramFenceSkipped(t *testing.T) {
	src := []byte("```go\nfunc main() {}\n```\n\n```mermaid\nA --> B\n```\n")
	blocks := Scan(src)
	if len(blocks) != 1 {
		t.Fatalf("want 1 block (go block skipped), got %d", len(blocks))
	}
	checkBlock(t, "only mermaid", blocks[0], KindRaw, "mermaid", "A --> B\n")
}
