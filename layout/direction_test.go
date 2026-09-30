package layout

import (
	"m31labs.dev/sirena"
	"testing"
)

func TestDirectionAppliesWithinCells(t *testing.T) {
	for _, direction := range []string{"left-right", "right-left", "top-down", "bottom-up"} {
		t.Run(direction, func(t *testing.T) {
			a, b, c := elem("A"), elem("LongerName"), elem("C")
			rv := &sirena.ResolvedView{Source: &sirena.ViewDecl{Name: "v", Layout: &sirena.LayoutHints{Direction: direction}}, Elements: []*sirena.Element{a, b, c}, Edges: []*sirena.Edge{fwd(a.Name, b.Name), fwd(b.Name, c.Name)}}
			lr, err := Compute(rv, LayoutOptions{})
			if err != nil {
				t.Fatal(err)
			}
			boxes := map[string]sirena.Rect{}
			for _, node := range lr.NodePlacements {
				boxes[node.Node.Name] = node.Bounds
			}
			first, last := boxes[a.Name], boxes[c.Name]
			switch direction {
			case "left-right":
				if first.Max.X >= last.Min.X {
					t.Fatal("horizontal ranks overlap or point backwards")
				}
			case "right-left":
				if first.Min.X <= last.Max.X {
					t.Fatal("reversed horizontal ranks overlap or point forwards")
				}
			case "top-down":
				if first.Max.Y >= last.Min.Y {
					t.Fatal("vertical ranks overlap or point backwards")
				}
			case "bottom-up":
				if first.Min.Y <= last.Max.Y {
					t.Fatal("reversed vertical ranks overlap or point forwards")
				}
			}
			for _, route := range lr.EdgeRoutes {
				if !route.IsOrthogonal() {
					t.Fatal("direction broke routing")
				}
			}
			if boxes[b.Name].Width() != nodeWidth(b.Name) {
				t.Fatal("rotation changed label dimensions")
			}
		})
	}
}
