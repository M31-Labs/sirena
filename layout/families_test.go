package layout

import (
	"m31labs.dev/sirena"
	"math"
	"reflect"
	"testing"
)

func TestDiagramFamiliesKeepNodesRelationshipsAndGeometry(t *testing.T) {
	rv := diagramView(t)
	rv.Elements[0].Metadata["lane"] = sirena.String{Value: "Customer"}
	rv.Elements[1].Metadata["fields"] = sirena.String{Value: "id: UUID; Wide WWW field: string"}
	for _, kind := range []string{"state", "class", "er", "swimlane", "timeline"} {
		t.Run(kind, func(t *testing.T) {
			lr, _, err := Render(rv, sirena.RenderOptions{Diagram: kind})
			if err != nil {
				t.Fatal(err)
			}
			if len(lr.NodePlacements) != len(rv.Elements) || len(lr.EdgeRoutes) != len(rv.Edges) {
				t.Fatal("lost graph semantics")
			}
			for i, a := range lr.NodePlacements {
				if a.Bounds.Width() <= 0 || a.Bounds.Height() <= 0 {
					t.Fatal("empty node")
				}
				for _, b := range lr.NodePlacements[i+1:] {
					if a.Bounds.Intersects(b.Bounds) {
						t.Fatalf("overlap: %s, %s", a.Node.Name, b.Node.Name)
					}
				}
			}
			again, _, _ := Render(rv, sirena.RenderOptions{Diagram: kind})
			if !reflect.DeepEqual(lr, again) {
				t.Fatal("non-deterministic layout")
			}
			if sirena.DiagramName(rv) != "architecture" {
				t.Fatal("mutated authored view")
			}
		})
	}
}
func TestTimelinePreservesDurationRatiosAndRejectsInvalidTimes(t *testing.T) {
	rv := diagramView(t)
	rv.Elements[0].Metadata["duration"] = sirena.Number{Value: 2}
	rv.Elements[1].Metadata["duration"] = sirena.Number{Value: 6}
	lr, _, err := Render(rv, sirena.RenderOptions{Diagram: "timeline"})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(lr.NodePlacements[1].Bounds.Width()/lr.NodePlacements[0].Bounds.Width()-3) > 1e-9 {
		t.Fatal("duration ratios changed")
	}
	for _, value := range []sirena.Value{sirena.Number{Value: -1}, sirena.Number{Value: math.NaN()}, sirena.String{Value: "tomorrow"}} {
		rv.Elements[0].Metadata["start"] = value
		if _, _, err := Render(rv, sirena.RenderOptions{Diagram: "timeline"}); err == nil {
			t.Fatal("invalid timestamp accepted")
		}
	}
}
