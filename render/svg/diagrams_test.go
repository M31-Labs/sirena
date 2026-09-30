package svg

import (
	"m31labs.dev/sirena"
	"m31labs.dev/sirena/layout"
	"strings"
	"testing"
)

func TestSequenceLabelsLifelinesAndArrowDirections(t *testing.T) {
	doc, err := sirena.Parse([]byte(`client browser { label: "Browser" }
service api { label: "API & <safe>" }
browser -> api: calls "Request"
api <- browser: flow "Return"
browser <-> api: flow "Duplex"
`))
	if err != nil {
		t.Fatal(err)
	}
	lr, _, err := layout.Render(sirena.AllElementsView(doc), sirena.RenderOptions{Diagram: "sequence"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := Render(lr, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Count(s, `class="lifeline"`) != 2 || strings.Count(s, `marker-end=`) != 2 || strings.Count(s, `marker-start=`) != 2 {
		t.Fatal("lifelines or directional messages missing")
	}
	if !strings.Contains(s, `aria-label="API &amp; &lt;safe&gt;"`) || !strings.Contains(s, `<title>API &amp; &lt;safe&gt;</title>`) {
		t.Fatal("human-readable label missing or unsafe")
	}
	if strings.Contains(s, ":root") || strings.Contains(s, "\n.node rect") {
		t.Fatal("inline SVG styles leak into other diagrams")
	}
}

func TestThemeScopesFollowTokenContents(t *testing.T) {
	a := &Theme{Name: "same", Tokens: map[string]string{"--sirena-bg": "#fff"}}
	b := &Theme{Name: "same", Tokens: map[string]string{"--sirena-bg": "#000"}}
	if themeScope(a) == themeScope(b) {
		t.Fatal("different inline themes share a style namespace")
	}
	if themeScope(a) != themeScope(&Theme{Tokens: map[string]string{"--sirena-bg": "#fff"}}) {
		t.Fatal("same tokens have nondeterministic scope")
	}
}

func TestViewportIncludesExternalRelationshipCaption(t *testing.T) {
	lr := &sirena.LayoutResult{Bounds: sirena.Rect{Max: sirena.Point{X: 60, Y: 40}}, EdgeRoutes: []*sirena.EdgeRoute{{Points: []sirena.Point{{X: 0, Y: 0}, {X: 200, Y: 100}}, Label: &sirena.EdgeLabel{Anchor: sirena.Point{X: -100, Y: -40}, Text: "A relationship caption outside the node boxes"}}}}
	bounds := svgBounds(lr)
	if bounds.Min.X >= -100 || bounds.Min.Y >= -40 || bounds.Max.X < 200 || bounds.Max.Y < 100 {
		t.Fatalf("routed geometry or caption is clipped: %+v", bounds)
	}
	if lr.Bounds.Max.X != 60 || lr.Bounds.Min.X != 0 {
		t.Fatal("viewport calculation mutated the source layout")
	}
}

func TestDirectionalMarkersUseRelationshipTokens(t *testing.T) {
	var routes []*sirena.EdgeRoute
	for kind := sirena.EdgeKindCalls; kind <= sirena.EdgeKindFlow; kind++ {
		routes = append(routes, &sirena.EdgeRoute{Edge: &sirena.Edge{Kind: kind, Direction: sirena.DirBidirectional}, Points: []sirena.Point{{}, {X: 100}}})
	}
	theme, _ := ThemeForName(DefaultThemeName)
	data, err := Render(&sirena.LayoutResult{Bounds: sirena.Rect{Max: sirena.Point{X: 100, Y: 100}}, EdgeRoutes: routes}, theme)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for kind := sirena.EdgeKindCalls; kind <= sirena.EdgeKindFlow; kind++ {
		name := kind.String()
		if !strings.Contains(s, `-`+name+`" viewBox=`) || !strings.Contains(s, `fill="var(--sirena-edge-stroke-`+strings.ReplaceAll(name, "_", "-")+`)"`) || strings.Count(s, `url(#sirena-arrow-`+themeScope(theme)+`-`+name+`)`) != 2 {
			t.Fatalf("relationship %s lacks matching start/end marker tokens", name)
		}
	}
	if strings.Contains(s, ".edge.kind-reads path {") {
		t.Fatal("relationship stroke rules leak into caption glyph paths")
	}
}
