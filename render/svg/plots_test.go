package svg

import (
	"bytes"
	"fmt"
	"m31labs.dev/sirena"
	"m31labs.dev/sirena/layout"
	"strings"
	"testing"
)

func TestPlotSVGIdentitiesAndBudget(t *testing.T) {
	for _, kind := range []string{"line", "scatter", "radar", "sankey"} {
		src := `service a { x: 0 y: 1 value: 1 axis: "Speed" }
service b { x: 1 y: 2 value: 2 axis: "Clarity" }
service c { x: 2 y: 4 value: 4 axis: "Breadth" }`
		if kind == "sankey" {
			src += "\na -> b: flow \"2\"\na -> c: flow \"4\""
		}
		doc, err := sirena.Parse([]byte(src))
		if err != nil {
			t.Fatal(err)
		}
		lr, _, err := layout.Render(sirena.AllElementsView(doc), sirena.RenderOptions{Diagram: kind})
		if err != nil {
			t.Fatal(err)
		}
		data, err := Render(lr, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(data, []byte(`data-morph-id="a"`)) || len(data) > 100000 || bytes.Contains(data, []byte("NaN")) {
			t.Fatal(kind, "identity or output budget")
		}
	}
}

// This deterministic 500-card ceiling runs in the existing Go CI job. It
// catches accidental re-expansion of shared font outlines without wall-clock noise.
func TestLargeCardSVGByteBudget(t *testing.T) {
	var src strings.Builder
	for i := 0; i < 500; i++ {
		fmt.Fprintf(&src, "service n%d { label: \"Common service\" fields: \"id: UUID; name: string\" methods: \"save(); validate()\" }\n", i)
	}
	doc, err := sirena.Parse([]byte(src.String()))
	if err != nil {
		t.Fatal(err)
	}
	lr, _, err := layout.Render(sirena.AllElementsView(doc), sirena.RenderOptions{Diagram: "class"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := Render(lr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > 700000 {
		t.Fatalf("500-card SVG budget exceeded: %d > 700000", len(data))
	}
	if count := bytes.Count(data, []byte(`<path id="sirena-glyph-`)); count > 100 {
		t.Fatalf("glyph reuse lost: %d definitions", count)
	}
}
