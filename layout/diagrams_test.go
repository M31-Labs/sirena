package layout

import (
	"m31labs.dev/sirena"
	"reflect"
	"strings"
	"testing"
)

func diagramView(t *testing.T) *sirena.ResolvedView {
	t.Helper()
	doc, err := sirena.Parse([]byte(`client browser { label: "Browser" }
service api { label: "Wide readable API label" }
database db { label: "Storage" }
browser -> api: calls "request"
api -> db: reads "lookup"
api -> api: calls "retry"
api -> browser: flow "response"
`))
	if err != nil {
		t.Fatal(err)
	}
	return sirena.AllElementsView(doc)
}

func TestSequencePreservesMessageOrderAndSelfCalls(t *testing.T) {
	rv := diagramView(t)
	lr, _, err := Render(rv, sirena.RenderOptions{Diagram: "sequence"})
	if err != nil {
		t.Fatal(err)
	}
	if len(lr.Lifelines) != 3 || len(lr.EdgeRoutes) != 4 {
		t.Fatalf("incomplete sequence: %+v", lr)
	}
	if sirena.DiagramName(rv) != "architecture" {
		t.Fatal("render mutated source view")
	}
	for i, route := range lr.EdgeRoutes {
		if route.Edge != rv.Edges[i] {
			t.Fatal("message order changed")
		}
		if i > 0 && route.Points[0].Y <= lr.EdgeRoutes[i-1].Points[0].Y {
			t.Fatal("message rows overlap")
		}
		if !route.IsOrthogonal() {
			t.Fatal("non-orthogonal message")
		}
	}
	if len(lr.EdgeRoutes[2].Points) != 4 {
		t.Fatal("self-call lost its return leg")
	}
	for _, node := range lr.NodePlacements {
		if node.Bounds.Width() < labelWidth(node.Node.DisplayLabel())+24 {
			t.Fatal("actor label exceeds box")
		}
	}
	a, _, _ := Render(rv, sirena.RenderOptions{Diagram: "sequence"})
	if !reflect.DeepEqual(a, lr) {
		t.Fatal("sequence not deterministic")
	}
}

func TestRadialKeepsRootCenteredAndLabelsSeparate(t *testing.T) {
	rv := diagramView(t)
	for i := 0; i < 20; i++ {
		rv.Elements = append(rv.Elements, &sirena.Element{Name: string(rune('a' + i)), Metadata: map[string]sirena.Value{"label": sirena.String{Value: "WWW Wide readable label"}}})
	}
	lr, _, err := Render(rv, sirena.RenderOptions{Diagram: "radial"})
	if err != nil {
		t.Fatal(err)
	}
	if lr.NodePlacements[0].Bounds.Center() != (sirena.Point{}) {
		t.Fatal("radial root not centered")
	}
	for i, a := range lr.NodePlacements {
		for _, b := range lr.NodePlacements[i+1:] {
			if a.Bounds.Intersects(b.Bounds) {
				t.Fatalf("radial labels overlap: %s %s", a.Node.Name, b.Node.Name)
			}
		}
	}
	if len(lr.EdgeRoutes) != len(rv.Edges) {
		t.Fatal("radial relationships lost")
	}
}

func TestDiagramRejectsLossyViewsAndUnknownKinds(t *testing.T) {
	rv := diagramView(t)
	rv.Boundaries = []*sirena.Boundary{{Name: "scope"}}
	if _, _, err := Render(rv, sirena.RenderOptions{Diagram: "sequence"}); err == nil || !strings.Contains(err.Error(), "flat view") {
		t.Fatal("nested participants silently discarded")
	}
	if _, _, err := Render(rv, sirena.RenderOptions{Diagram: "typo"}); err == nil {
		t.Fatal("unknown diagram accepted")
	}
}
