package layout

import (
	"reflect"
	"strings"
	"testing"

	"m31labs.dev/sirena"
)

func TestStoryboardCardsReserveWidthAndHeightIndependently(t *testing.T) {
	for _, kind := range []string{"class", "er"} {
		t.Run(kind, func(t *testing.T) {
			wide := &sirena.ResolvedView{Elements: []*sirena.Element{
				{Name: "record", Metadata: map[string]sirena.Value{"fields": sirena.String{Value: strings.Repeat("W", 40)}}},
				{Name: "peer"},
			}}
			tall := &sirena.ResolvedView{Elements: []*sirena.Element{
				{Name: "record", Metadata: map[string]sirena.Value{"fields": sirena.String{Value: strings.Repeat("id;", 20)}}},
				{Name: "peer"},
			}}
			views := []*sirena.ResolvedView{wide, tall}
			standalone := make([]*sirena.LayoutResult, len(views))
			for i, view := range views {
				var err error
				standalone[i], _, err = Render(view, sirena.RenderOptions{Diagram: kind})
				if err != nil {
					t.Fatal(err)
				}
			}
			frames, err := Storyboard(views, sirena.RenderOptions{Diagram: kind})
			if err != nil {
				t.Fatal(err)
			}
			for i, frame := range frames {
				for j, node := range frame.NodePlacements {
					original := standalone[i].NodePlacements[j].Bounds
					if node.Bounds.Width() < original.Width() || node.Bounds.Height() < original.Height() {
						t.Fatalf("state %d %s: reserved %v cannot fit content requiring %v", i, node.Node.Name, node.Bounds, original)
					}
					if node.Bounds != frames[0].NodePlacements[j].Bounds {
						t.Fatal("actor slots moved")
					}
				}
			}
			if wide.Elements[0].Metadata["fields"].(sirena.String).Value != strings.Repeat("W", 40) ||
				tall.Elements[0].Metadata["fields"].(sirena.String).Value != strings.Repeat("id;", 20) {
				t.Fatal("authored metadata changed")
			}
			again, err := Storyboard(views, sirena.RenderOptions{Diagram: kind})
			if err != nil || !reflect.DeepEqual(frames, again) {
				t.Fatal("storyboard is not deterministic")
			}
		})
	}
}

func TestStoryboardReservesLongestCaptionForStableRelationship(t *testing.T) {
	caption := strings.Repeat("wide relationship ", 8)
	views := make([]*sirena.ResolvedView, 2)
	for i, label := range []string{"calls", caption} {
		views[i] = &sirena.ResolvedView{Elements: []*sirena.Element{{Name: "a"}, {Name: "b"}},
			Edges: []*sirena.Edge{{From: "a", To: "b", Label: label}}}
	}
	frames, err := Storyboard(views, sirena.RenderOptions{Diagram: "class"})
	if err != nil {
		t.Fatal(err)
	}
	for _, frame := range frames {
		gap := frame.NodePlacements[1].Bounds.Min.X - frame.NodePlacements[0].Bounds.Max.X
		if gap < labelWidth(caption)+32 {
			t.Fatalf("gutter %g cannot fit later caption of width %g", gap, labelWidth(caption))
		}
	}
	if views[0].Edges[0].Label != "calls" || views[1].Edges[0].Label != caption {
		t.Fatal("authored captions changed")
	}
}
