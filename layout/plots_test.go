package layout

import (
	"m31labs.dev/sirena"
	"math"
	"reflect"
	"testing"
)

func TestPlotsAndSharedScales(t *testing.T) {
	a := chartView(t, `service a { x: 0 y: 1 series: "Build" }
service b { x: 2 y: 3 series: "Build" }`)
	b := chartView(t, `service a { x: 0 y: 1 series: "Build" }
service b { x: 4 y: 9 series: "Build" }`)
	for _, kind := range []string{"line", "scatter"} {
		lr, _, err := Render(a, sirena.RenderOptions{Diagram: kind})
		if err != nil {
			t.Fatal(err)
		}
		if lr.Plot.XMin != 0 || lr.Plot.XMax != 2 || len(lr.Plot.Series) != 1 {
			t.Fatal("wrong domains/series")
		}
		again, _, _ := Render(a, sirena.RenderOptions{Diagram: kind})
		if !reflect.DeepEqual(lr, again) {
			t.Fatal("nondeterministic")
		}
		frames, err := Storyboard([]*sirena.ResolvedView{a, b}, sirena.RenderOptions{Diagram: kind})
		if err != nil {
			t.Fatal(err)
		}
		if frames[0].Plot.YMax != 9 || frames[0].NodePlacements[0].Bounds != frames[1].NodePlacements[0].Bounds || frames[0].Bounds != frames[1].Bounds {
			t.Fatal("chart jumped or domain differs")
		}
	}
	for _, v := range []sirena.Value{sirena.Number{Value: math.NaN()}, sirena.Number{Value: math.Inf(1)}, sirena.String{Value: "bad"}} {
		a.Elements[0].Metadata["x"] = v
		if _, _, err := Render(a, sirena.RenderOptions{Diagram: "line"}); err == nil {
			t.Fatal("invalid coordinate accepted")
		}
	}
}
func TestRadarAndSankey(t *testing.T) {
	radar := chartView(t, `service a { axis: "Speed" value: 1 }
service b { axis: "Breadth" value: 2 }
service c { axis: "Clarity" value: 4 }`)
	lr, _, err := Render(radar, sirena.RenderOptions{Diagram: "radar"})
	if err != nil {
		t.Fatal(err)
	}
	if lr.Radar.Maximum != 4 || len(lr.Radar.Series[0].Points) != 3 {
		t.Fatal("radar values lost")
	}
	radar.Elements[1].Metadata["axis"] = sirena.String{Value: "Speed"}
	if _, _, err := Render(radar, sirena.RenderOptions{Diagram: "radar"}); err == nil {
		t.Fatal("duplicate axes accepted")
	}
	flow := chartView(t, `service source
service a
service b
source -> a: flow "30"
source -> b: flow "90"`)
	lr, _, err = Render(flow, sirena.RenderOptions{Diagram: "sankey"})
	if err != nil {
		t.Fatal(err)
	}
	if len(lr.Flows) != 2 || math.Abs(lr.Flows[1].Width/lr.Flows[0].Width-3) > 1e-9 {
		t.Fatal("flow weights not proportional")
	}
	for _, f := range lr.Flows {
		if f.From.Y-f.Width/2 < lr.NodePlacements[0].Bounds.Min.Y-1e-9 || f.From.Y+f.Width/2 > lr.NodePlacements[0].Bounds.Max.Y+1e-9 {
			t.Fatal("flow leaves source box")
		}
	}
	flow.Edges = append(flow.Edges, &sirena.Edge{From: "a", To: "source", Direction: sirena.DirForward, Label: "1"})
	if _, _, err := Render(flow, sirena.RenderOptions{Diagram: "sankey"}); err == nil {
		t.Fatal("cycle accepted")
	}
}
func TestStableGraphAndBarSlots(t *testing.T) {
	a := chartView(t, `service a { value: 1 }
service b { value: 2 }
a -> b: flow "Call"`)
	b := chartView(t, `service b { value: 8 }
service c { value: 4 }
service a { value: 1 }
b -> c: flow "Result"`)
	for _, kind := range []string{"architecture", "state", "class", "er", "mindmap", "bar"} {
		aa, bb := *a, *b
		if kind == "bar" {
			aa.Edges = nil
			bb.Edges = nil
		}
		frames, err := Storyboard([]*sirena.ResolvedView{&aa, &bb}, sirena.RenderOptions{Diagram: kind})
		if err != nil {
			t.Fatal(kind, err)
		}
		slots := map[string]sirena.Rect{}
		for _, np := range frames[0].NodePlacements {
			slots[np.Node.Name] = np.Bounds
		}
		for _, np := range frames[1].NodePlacements {
			if old, ok := slots[np.Node.Name]; ok {
				if kind == "bar" {
					if old.Min.Y != np.Bounds.Min.Y {
						t.Fatal("bar row jumped")
					}
				} else if old != np.Bounds {
					t.Fatal(kind, "actor jumped")
				}
			}
		}
		if frames[0].Bounds != frames[1].Bounds {
			t.Fatal("canvas jumped")
		}
	}
	if a.Elements[0].Metadata["value"].(sirena.Number).Value != 1 || len(a.Elements) != 2 {
		t.Fatal("caller mutated")
	}
}
