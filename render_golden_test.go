package sirena_test

// render_golden_test.go locks the SVG renderer's output byte-for-byte
// against checked-in goldens. It is the visual-regression guard the
// conformance corpus lacked: because each golden captures the glyph
// <path> transforms, a regression in label orientation (e.g. reintroducing
// the Y-flip that rendered every label upside-down) diffs immediately.
//
// Regenerate goldens with:  go test ./ -run TestRenderGolden -update-render

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"m31labs.dev/sirena"
	_ "m31labs.dev/sirena/layout" // register the layout engine into sirena.Render
	"m31labs.dev/sirena/render/svg"
)

var updateRender = flag.Bool("update-render", false, "update SVG render goldens")

// renderGoldenCases are small system files chosen to exercise the renderer
// broadly: multiple element kinds (theme fills), a labeled edge (glyph
// rendering + label background), and a boundary with nested elements
// (container rendering + a boundary label).
var renderGoldenCases = []struct {
	name string
	src  string
}{
	{
		name: "elements_and_labeled_edge",
		src:  "service api\ndatabase db\napi -> db: reads \"user records\"\n",
	},
	{
		name: "boundary_with_children",
		src:  "boundary deployment \"us-east\" {\n  service api\n  service worker\n}\n",
	},
	{
		name: "kinds_and_edges",
		src:  "gateway edge\nservice api\ncache redis\nqueue jobs\nedge -> api: calls\napi -> redis: reads\napi -> jobs: publishes\n",
	},
}

func TestRenderGolden(t *testing.T) {
	theme, err := svg.ThemeForName(svg.DefaultThemeName)
	if err != nil {
		t.Fatalf("ThemeForName: %v", err)
	}
	for _, tc := range renderGoldenCases {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := sirena.Parse([]byte(tc.src))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			lr, _, err := sirena.Render(sirena.AllElementsView(doc), sirena.RenderOptions{})
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			got, err := svg.Render(lr, theme)
			if err != nil {
				t.Fatalf("svg.Render: %v", err)
			}

			goldenPath := filepath.Join("testdata", "render", tc.name+".golden.svg")
			if *updateRender {
				if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden (run with -update-render to create): %v", err)
			}
			if string(got) != string(want) {
				t.Errorf("%s: SVG output differs from golden.\nRegenerate with -update-render if this change is intended.", tc.name)
			}
		})
	}
}

// TestRenderGolden_Deterministic guards invariant #8: identical input must
// produce byte-identical SVG across runs within a single version.
func TestRenderGolden_Deterministic(t *testing.T) {
	theme, _ := svg.ThemeForName(svg.DefaultThemeName)
	src := []byte(renderGoldenCases[0].src)
	render := func() string {
		doc := sirena.MustParse(src)
		lr, _, err := sirena.Render(sirena.AllElementsView(doc), sirena.RenderOptions{})
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		out, err := svg.Render(lr, theme)
		if err != nil {
			t.Fatalf("svg.Render: %v", err)
		}
		return string(out)
	}
	if a, b := render(), render(); a != b {
		t.Error("svg.Render is non-deterministic across runs for identical input")
	}
}
