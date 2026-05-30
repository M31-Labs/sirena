package mermaid

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"m31labs.dev/sirena"
	_ "m31labs.dev/sirena/layout" // register the layout engine into sirena.Render
	"m31labs.dev/sirena/render/svg"
)

// representativeFlowchart is a well-formed Mermaid diagram used for the G1
// eyeball gate: one subgraph, labeled edges, and a mix of shapes.
const representativeFlowchart = `flowchart TD
  Client([User Browser])
  GW[API Gateway]
  Auth[(Auth DB)]
  Cache{{Cache}}

  subgraph backend[Backend Services]
    SVC[Service A]
    Worker[Worker]
  end

  Client -->|request| GW
  GW -->|authenticate| Auth
  GW --> SVC
  SVC -->|enqueue| Worker
  SVC --> Cache
`

// TestRenderSmoke_EndToEnd is the G1 eyeball gate.
// It parses a representative flowchart through the full chain:
// Parse → AllElementsView → Render → svg.Render, asserts the SVG is
// structurally valid, and writes it to testdata/_eyeball.svg for manual
// inspection.
func TestRenderSmoke_EndToEnd(t *testing.T) {
	doc, diags, err := Parse([]byte(representativeFlowchart), Options{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	for _, d := range diags {
		if d.Code == "SIR-MERMAID-NOT-A-GRAPH" || d.Code == "SIR-MERMAID-PARSE" {
			t.Errorf("unexpected fatal diagnostic: %s: %s", d.Code, d.Message)
		}
	}
	if doc == nil {
		t.Fatal("Parse returned nil doc")
	}

	// Verify element/edge/boundary counts are sane.
	if len(doc.Systems) != 1 {
		t.Fatalf("want 1 system, got %d", len(doc.Systems))
	}
	sys := doc.Systems[0]
	if len(sys.Elements) < 2 {
		t.Errorf("want ≥2 top-level elements, got %d", len(sys.Elements))
	}
	if len(sys.Edges) < 1 {
		t.Errorf("want ≥1 edge, got %d", len(sys.Edges))
	}
	if len(sys.Boundaries) < 1 {
		t.Errorf("want ≥1 boundary (subgraph), got %d", len(sys.Boundaries))
	}
	t.Logf("elements=%d edges=%d boundaries=%d", len(sys.Elements), len(sys.Edges), len(sys.Boundaries))

	// Full render chain: AllElementsView → Render → svg.Render.
	rv := sirena.AllElementsView(doc)
	lr, _, rerr := sirena.Render(rv, sirena.RenderOptions{})
	if rerr != nil {
		t.Fatalf("sirena.Render: %v", rerr)
	}
	if lr == nil {
		t.Fatal("sirena.Render returned nil LayoutResult (layout engine not registered?)")
	}

	theme, err := svg.ThemeForName(svg.DefaultThemeName)
	if err != nil {
		t.Fatalf("ThemeForName: %v", err)
	}
	svgBytes, err := svg.Render(lr, theme)
	if err != nil {
		t.Fatalf("svg.Render: %v", err)
	}
	if len(svgBytes) == 0 {
		t.Fatal("svg.Render returned empty output")
	}
	if !bytes.HasPrefix(svgBytes, []byte("<")) {
		t.Errorf("SVG does not start with '<'")
	}
	if !bytes.Contains(svgBytes, []byte("<svg")) {
		t.Error("SVG output missing <svg tag")
	}
	if !bytes.Contains(svgBytes, []byte("</svg>")) {
		t.Error("SVG output missing </svg> closing tag")
	}

	// Write the eyeball SVG for manual inspection.
	eyeball := filepath.Join("testdata", "_eyeball.svg")
	if werr := os.WriteFile(eyeball, svgBytes, 0o644); werr != nil {
		t.Logf("warning: could not write eyeball SVG: %v", werr)
	} else {
		t.Logf("eyeball SVG written to %s (%d bytes)", eyeball, len(svgBytes))
	}
}
