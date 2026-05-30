package mermaid

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"m31labs.dev/sirena"
	_ "m31labs.dev/sirena/layout" // register the layout engine into sirena.Render
	"m31labs.dev/sirena/render/svg"
)

// corpusExpectations captures per-file edge/element expectations for corpus
// files that have known structure. The map key is the filename base.
// minEdges: the minimum number of edges the parsed doc must contain.
// minElems: the minimum number of elements the parsed doc must contain.
type corpusExpectation struct {
	minEdges int
	minElems int
}

// corpusExpected lists files with structural expectations beyond "≥1 element".
// Files not listed here only need ≥1 element and no fatal diagnostics.
var corpusExpected = map[string]corpusExpectation{
	// Two edges: "A -->|query| B" and "A --> C", after classDef/style stripping.
	"02_classdef_style_dropped.mmd": {minEdges: 2, minElems: 3},
}

// TestCorpus_ParseAndRender loads every *.mmd file under testdata/corpus/,
// asserts it parses without fatal diagnostics (STYLE-DROPPED is OK and
// SIR-MERMAID-PARSE must NOT appear — even alongside STYLE-DROPPED), produces
// ≥1 element, and renders end-to-end to valid SVG. Files in corpusExpected
// must also satisfy their minimum edge counts.
func TestCorpus_ParseAndRender(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "corpus", "*.mmd"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no corpus files found under testdata/corpus/")
	}

	theme, err := svg.ThemeForName(svg.DefaultThemeName)
	if err != nil {
		t.Fatalf("ThemeForName: %v", err)
	}

	for _, f := range files {
		f := f
		name := filepath.Base(f)
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatalf("read %s: %v", f, err)
			}

			doc, diags, err := Parse(src, Options{})
			if err != nil {
				t.Fatalf("Parse fatal error: %v", err)
			}

			// Fatal diagnostics are not allowed.
			// SIR-MERMAID-PARSE must NEVER appear — the normalize pass now
			// deletes styling lines so that the grammar never enters
			// error-recovery regardless of STYLE-DROPPED.
			for _, d := range diags {
				switch d.Code {
				case "SIR-MERMAID-NOT-A-GRAPH":
					t.Errorf("fatal diagnostic %s: %s", d.Code, d.Message)
				case "SIR-MERMAID-PARSE":
					t.Errorf("SIR-MERMAID-PARSE diagnostic present (content loss bug): %s", d.Message)
				}
			}

			if doc == nil {
				t.Fatal("Parse returned nil doc (no diagram_flow found)")
			}

			// Must produce at least one element.
			elemCount := 0
			for _, sys := range doc.Systems {
				elemCount += countElements(sys)
			}
			if elemCount == 0 {
				t.Error("doc has 0 elements")
			}

			// Per-file structural assertions.
			if exp, ok := corpusExpected[name]; ok {
				if elemCount < exp.minElems {
					t.Errorf("want ≥%d elements, got %d", exp.minElems, elemCount)
				}
				edgeCount := 0
				for _, sys := range doc.Systems {
					edgeCount += len(sys.Edges)
				}
				if edgeCount < exp.minEdges {
					t.Errorf("want ≥%d edges, got %d (content-loss regression)", exp.minEdges, edgeCount)
				}
			}

			// Full render chain.
			rv := sirena.AllElementsView(doc)
			lr, _, rerr := sirena.Render(rv, sirena.RenderOptions{})
			if rerr != nil {
				t.Fatalf("sirena.Render: %v", rerr)
			}
			if lr == nil {
				t.Fatal("sirena.Render returned nil LayoutResult")
			}
			svgBytes, err := svg.Render(lr, theme)
			if err != nil {
				t.Fatalf("svg.Render: %v", err)
			}
			if !bytes.Contains(svgBytes, []byte("<svg")) {
				t.Error("SVG output missing <svg tag")
			}
			if !bytes.Contains(svgBytes, []byte("</svg>")) {
				t.Error("SVG output missing </svg>")
			}
		})
	}
}

// TestCorpus_Determinism checks that two consecutive Parse calls on each
// corpus file produce byte-identical sirena.Print output.
func TestCorpus_Determinism(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "corpus", "*.mmd"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no corpus files found under testdata/corpus/")
	}

	for _, f := range files {
		f := f
		name := filepath.Base(f)
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatalf("read %s: %v", f, err)
			}

			doc1, _, err1 := Parse(src, Options{})
			if err1 != nil || doc1 == nil {
				t.Skipf("Parse failed (%v), skipping determinism check", err1)
			}
			doc2, _, err2 := Parse(src, Options{})
			if err2 != nil || doc2 == nil {
				t.Skipf("Parse failed on second call (%v)", err2)
			}

			printed1, perr1 := sirena.Print(doc1)
			printed2, perr2 := sirena.Print(doc2)
			if perr1 != nil {
				t.Fatalf("Print(doc1): %v", perr1)
			}
			if perr2 != nil {
				t.Fatalf("Print(doc2): %v", perr2)
			}
			if !bytes.Equal(printed1, printed2) {
				t.Errorf("non-deterministic output:\nfirst:\n%s\nsecond:\n%s",
					strings.TrimSpace(string(printed1)),
					strings.TrimSpace(string(printed2)))
			}
		})
	}
}

// countElements recursively counts all elements (including boundary children).
func countElements(sys *sirena.SystemDecl) int {
	n := len(sys.Elements)
	for _, b := range sys.Boundaries {
		n += countBoundaryElements(b)
	}
	return n
}

func countBoundaryElements(b *sirena.Boundary) int {
	n := 0
	for _, child := range b.Children {
		switch v := child.(type) {
		case *sirena.Element:
			n++
		case *sirena.Boundary:
			n += countBoundaryElements(v)
		}
	}
	return n
}
