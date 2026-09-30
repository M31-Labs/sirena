package scene3d

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"m31labs.dev/gosx/scene"
	"m31labs.dev/gosx/scene/schema"
	"m31labs.dev/sirena"
	_ "m31labs.dev/sirena/layout"
)

func diagram(t *testing.T) *sirena.LayoutResult {
	t.Helper()
	doc, err := sirena.Parse([]byte(`client browser { label: "Browser" }
service api { label: "API" }
database db { label: "Storage" }
browser -> api: calls "request"
api <-> db: reads "lookup"
`))
	if err != nil {
		t.Fatal(err)
	}
	lr, _, err := sirena.Render(sirena.AllElementsView(doc), sirena.RenderOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return lr
}
func TestDiagramIsDeterministicAndRetainsGeometry(t *testing.T) {
	lr := diagram(t)
	before, _ := json.Marshal(lr.NodePlacements)
	a, err := Build(lr, Options{Motion: true})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Build(lr, Options{Motion: true})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("scene is non-deterministic")
	}
	after, _ := json.Marshal(lr.NodePlacements)
	if !bytes.Equal(before, after) {
		t.Fatal("renderer mutated layout")
	}
	var props map[string]json.RawMessage
	if err := json.Unmarshal(a, &props); err != nil {
		t.Fatal(err)
	}
	report := schema.ValidateJSON(props["scene"], schema.Options{})
	if !report.Valid {
		t.Fatalf("invalid SceneIR: %+v", report.Diagnostics)
	}
	var ir scene.SceneIR
	if err := json.Unmarshal(props["scene"], &ir); err != nil {
		t.Fatal(err)
	}
	if len(ir.Objects) != 5 || len(ir.Labels) != 5 {
		t.Fatalf("objects=%d labels=%d", len(ir.Objects), len(ir.Labels))
	}
	for _, obj := range ir.Objects {
		if obj.Kind == "lines" && len(obj.LineSegments) < 3 {
			t.Fatal("routed relation lost arrowhead")
		}
	}
}
func TestShaderAndAbsoluteKeyframes(t *testing.T) {
	x := 2.0
	scale := 1.5
	opts := Options{Shader: []byte(`material Ink { surface(geo) -> color { return rgb(geo.uv.x, 0.4, 0.8) } }`), Material: "Ink", Targets: []string{"api"}, Steps: []Step{{Label: "Layout"}, {Label: "Move", Patches: []Patch{{Target: "api", X: &x, Scale: &scale}}}, {Label: "Reset"}}}
	data, err := Build(diagram(t), opts)
	if err != nil {
		t.Fatal(err)
	}
	var props struct {
		Scene      scene.SceneIR `json:"scene"`
		SlideSteps Timeline      `json:"slideSteps"`
	}
	if err := json.Unmarshal(data, &props); err != nil {
		t.Fatal(err)
	}
	for _, obj := range props.Scene.Objects {
		if (obj.ShaderBackend == "selena") != (obj.ID == "api") {
			t.Fatalf("unexpected shader on %s", obj.ID)
		}
	}
	frames := props.SlideSteps.Frames
	if len(frames) != 3 || !bytes.Equal(mustJSON(frames[0].Commands), mustJSON(frames[2].Commands)) {
		t.Fatal("backwards seek does not restore layout")
	}
	if !strings.Contains(string(mustJSON(frames[1].Commands)), `"x":2`) {
		t.Fatal("missing keyframe pose")
	}
	opts.Steps[1].Patches[0].Target = "missing"
	if _, err := Build(diagram(t), opts); err == nil {
		t.Fatal("unknown keyframe target accepted")
	}
	opts.Steps = nil
	opts.Targets = []string{"missing"}
	if _, err := Build(diagram(t), opts); err == nil {
		t.Fatal("unknown shader target accepted")
	}
}

func TestShaderOptionsRequireSource(t *testing.T) {
	for _, opts := range []Options{{Material: "Ink"}, {Targets: []string{"api"}}} {
		if _, err := Build(diagram(t), opts); err == nil || !strings.Contains(err.Error(), "require shader source") {
			t.Fatalf("shaderless selection was silently ignored: %v", err)
		}
	}
}
func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
