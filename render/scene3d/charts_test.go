package scene3d_test

import (
	"bytes"
	"testing"

	"m31labs.dev/sirena"
	"m31labs.dev/sirena/layout"
	"m31labs.dev/sirena/render/scene3d"
)

func TestChartAndTreeTours(t *testing.T) {
	for _, kind := range []string{"mindmap", "bar", "gantt"} {
		source := `database a { label: "Readable data" value: -2 start: "2026-10-01" end: "2026-10-03" }
service b { value: 6 start: "2026-10-03" end: "2026-10-09" }`
		if kind == "mindmap" {
			source += "\na -> b: flow"
		}
		doc, err := sirena.Parse([]byte(source))
		if err != nil {
			t.Fatal(err)
		}
		lr, _, err := layout.Render(sirena.AllElementsView(doc), sirena.RenderOptions{Diagram: kind})
		if err != nil {
			t.Fatal(err)
		}
		data, err := scene3d.Build(lr, scene3d.Options{Tour: "nodes", MotionStyle: "float"})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(data, []byte("slideSteps")) || !bytes.Contains(data, []byte("Readable data")) {
			t.Fatal("lost labels or steps")
		}
		if kind == "bar" && !bytes.Contains(data, []byte("-2")) {
			t.Fatal("bar value omitted")
		}
	}
}
func TestPieRejectsUnsupported3D(t *testing.T) {
	if _, err := scene3d.Build(&sirena.LayoutResult{Diagram: "pie"}, scene3d.Options{}); err == nil {
		t.Fatal("pie silently exported legend as geometry")
	}
}
func TestNativePlotsKeepDataAndAxes(t *testing.T) {
	for _, kind := range []string{"line", "scatter", "radar"} {
		doc, err := sirena.Parse([]byte(`service a { x: 0 y: 1 value: 1 axis: "Speed" }
service b { x: 1 y: 2 value: 2 axis: "Clarity" }
service c { x: 2 y: 4 value: 4 axis: "Breadth" }`))
		if err != nil {
			t.Fatal(err)
		}
		lr, _, err := layout.Render(sirena.AllElementsView(doc), sirena.RenderOptions{Diagram: kind})
		if err != nil {
			t.Fatal(err)
		}
		data, err := scene3d.Build(lr, scene3d.Options{Tour: "nodes"})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(data, []byte("chart:axes")) || !bytes.Contains(data, []byte("slideSteps")) {
			t.Fatal("axes or tour missing")
		}
		if kind == "scatter" && bytes.Contains(data, []byte("chart:series:0")) {
			t.Fatal("scatter connected points")
		}
	}
}
