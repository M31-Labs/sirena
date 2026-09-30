package layout

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"m31labs.dev/sirena"
)

func chartView(t *testing.T, source string) *sirena.ResolvedView {
	t.Helper()
	doc, err := sirena.Parse([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	return sirena.AllElementsView(doc)
}
func TestMindmapForestAndDiagnostics(t *testing.T) {
	rv := chartView(t, `service root { label: "A wide and readable root" }
service left
service right
service leaf
root -> left: flow
root -> right: flow
left -> leaf: flow`)
	lr, _, err := Render(rv, sirena.RenderOptions{Diagram: "mindmap"})
	if err != nil {
		t.Fatal(err)
	}
	for i, a := range lr.NodePlacements {
		for _, b := range lr.NodePlacements[i+1:] {
			if a.Bounds.Intersects(b.Bounds) {
				t.Fatal("tree boxes overlap")
			}
		}
	}
	if lr.NodePlacements[3].Bounds.Min.X <= lr.NodePlacements[1].Bounds.Min.X {
		t.Fatal("child depth lost")
	}
	again, _, _ := Render(rv, sirena.RenderOptions{Diagram: "mindmap"})
	if !reflect.DeepEqual(lr, again) {
		t.Fatal("not deterministic")
	}
	for _, edges := range [][]*sirena.Edge{{{From: "root", To: "left"}, {From: "right", To: "left"}}, {{From: "root", To: "left"}, {From: "left", To: "root"}}} {
		copy := *rv
		copy.Edges = edges
		if _, _, err := Render(&copy, sirena.RenderOptions{Diagram: "mindmap"}); err == nil {
			t.Fatal("invalid forest accepted")
		}
	}
}
func TestChartDataGeometryAndValidation(t *testing.T) {
	rv := chartView(t, `service a { label: "Very wide label" value: -2 }
service b { value: 6 }
service zero { value: 0 }`)
	lr, _, err := Render(rv, sirena.RenderOptions{Diagram: "bar"})
	if err != nil {
		t.Fatal(err)
	}
	a, b := lr.NodePlacements[0].Bounds, lr.NodePlacements[1].Bounds
	if a.Max.X != lr.ChartBaseline || b.Min.X != lr.ChartBaseline || math.Abs(b.Width()/a.Width()-3) > 1e-9 {
		t.Fatal("signed ratio or baseline changed")
	}
	if _, _, err := Render(rv, sirena.RenderOptions{Diagram: "pie"}); err == nil {
		t.Fatal("negative pie accepted")
	}
	rv.Elements[0].Metadata["value"] = sirena.Number{Value: 2}
	pie, _, err := Render(rv, sirena.RenderOptions{Diagram: "pie"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pie.PieSlices) != 3 || pie.PieSlices[0].Fraction != .25 || pie.PieSlices[1].Fraction != .75 {
		t.Fatal("wrong sectors")
	}
	for _, v := range []sirena.Value{sirena.String{Value: "many"}, sirena.Number{Value: math.Inf(1)}, sirena.Number{Value: math.NaN()}} {
		rv.Elements[0].Metadata["value"] = v
		if _, _, err := Render(rv, sirena.RenderOptions{Diagram: "bar"}); err == nil {
			t.Fatal("bad value accepted")
		}
	}
}
func TestGanttUsesCalendarAndPreservesSource(t *testing.T) {
	rv := chartView(t, `service a { start: "2026-02-27" end: "2026-03-02" }
service b { start: "2026-03-02" end: "2026-03-08" }
a -> b: depends_on`)
	lr, _, err := Render(rv, sirena.RenderOptions{Diagram: "gantt"})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(lr.NodePlacements[1].Bounds.Width()/lr.NodePlacements[0].Bounds.Width()-2) > 1e-9 {
		t.Fatal("calendar duration changed")
	}
	if len(lr.EdgeRoutes) != 1 || lr.ChartStart != "2026-02-27" || lr.ChartEnd != "2026-03-08" {
		t.Fatal("lost labels or dependencies")
	}
	if _, ok := rv.Elements[0].Metadata["start"].(sirena.String); !ok {
		t.Fatal("mutated caller")
	}
	rv.Elements[0].Metadata["end"] = sirena.String{Value: "2026-02-27"}
	if _, _, err := Render(rv, sirena.RenderOptions{Diagram: "gantt"}); err == nil || !strings.Contains(err.Error(), "after start") {
		t.Fatal("invalid end accepted")
	}
}
