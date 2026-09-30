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
