package scene3d

import (
	"bytes"
	"encoding/json"
	"fmt"
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

func TestCompleteSceneItemLimit(t *testing.T) {
	bounds := sirena.Rect{Max: sirena.Point{X: 100, Y: 100}}
	for _, kind := range []string{"nodes", "summaries", "boundaries", "nested boundaries", "edges", "labeled edges"} {
		t.Run(kind, func(t *testing.T) {
			itemsPerPlacement := 2
			if kind == "edges" {
				itemsPerPlacement = 1
			}
			limit := maxSceneItems / itemsPerPlacement
			makeLayout := func(count int) *sirena.LayoutResult {
				lr := &sirena.LayoutResult{Bounds: bounds}
				var parent *sirena.BoundaryPlacement
				for i := 0; i < count; i++ {
					name := fmt.Sprintf("item-%d", i)
					switch kind {
					case "nodes":
						lr.NodePlacements = append(lr.NodePlacements, &sirena.NodePlacement{Node: &sirena.Element{Name: name}, Bounds: bounds})
					case "summaries":
						lr.SummaryPlacements = append(lr.SummaryPlacements, &sirena.SummaryPlacement{Summary: &sirena.BoundarySummary{Boundary: &sirena.Boundary{Name: name}, Label: name}, Bounds: bounds})
					case "boundaries", "nested boundaries":
						bp := &sirena.BoundaryPlacement{Boundary: &sirena.Boundary{Name: name}, Bounds: bounds}
						if kind == "nested boundaries" && parent != nil {
							parent.Children = []*sirena.BoundaryPlacement{bp}
						} else {
							lr.BoundaryPlacements = append(lr.BoundaryPlacements, bp)
						}
						parent = bp
					case "edges", "labeled edges":
						route := &sirena.EdgeRoute{Edge: &sirena.Edge{}, Points: []sirena.Point{{}, {X: 1}}}
						if kind == "labeled edges" {
							route.Label = &sirena.EdgeLabel{Text: name}
						}
						lr.EdgeRoutes = append(lr.EdgeRoutes, route)
					}
				}
				return lr
			}
			data, err := Build(makeLayout(limit), Options{})
			if err != nil {
				t.Fatalf("exact limit rejected: %v", err)
			}
			var props struct {
				Scene scene.SceneIR `json:"scene"`
			}
			if err := json.Unmarshal(data, &props); err != nil {
				t.Fatal(err)
			}
			if got := len(props.Scene.Objects) + len(props.Scene.Labels); got != maxSceneItems {
				t.Fatalf("emitted %d items, want %d", got, maxSceneItems)
			}
			if _, err := Build(makeLayout(limit+1), Options{}); err == nil || !strings.Contains(err.Error(), "at most 2000 scene objects and labels") {
				t.Fatalf("overflow accepted: %v", err)
			}
		})
	}
}

func TestSceneItemLimitAggregatesPlacementKinds(t *testing.T) {
	bounds := sirena.Rect{Max: sirena.Point{X: 100, Y: 100}}
	lr := &sirena.LayoutResult{Bounds: bounds}
	for i := 0; i < 998; i++ {
		lr.NodePlacements = append(lr.NodePlacements, &sirena.NodePlacement{Node: &sirena.Element{Name: fmt.Sprintf("node-%d", i)}, Bounds: bounds})
	}
	lr.SummaryPlacements = []*sirena.SummaryPlacement{{Summary: &sirena.BoundarySummary{Boundary: &sirena.Boundary{Name: "collapsed"}, Label: "Collapsed"}, Bounds: bounds}}
	lr.BoundaryPlacements = []*sirena.BoundaryPlacement{{Boundary: &sirena.Boundary{Name: "visible"}, Bounds: bounds}}
	// Invalid/nil placements do not emit objects and must not consume the cap.
	lr.NodePlacements = append(lr.NodePlacements, nil, &sirena.NodePlacement{})
	lr.SummaryPlacements = append(lr.SummaryPlacements, nil, &sirena.SummaryPlacement{})
	lr.BoundaryPlacements = append(lr.BoundaryPlacements, nil)
	if _, err := Build(lr, Options{}); err != nil {
		t.Fatalf("exact mixed limit rejected: %v", err)
	}
	lr.EdgeRoutes = []*sirena.EdgeRoute{{Edge: &sirena.Edge{}, Points: []sirena.Point{{}, {X: 1}}}}
	if _, err := Build(lr, Options{}); err == nil || !strings.Contains(err.Error(), "at most 2000") {
		t.Fatalf("mixed overflow accepted: %v", err)
	}
}

func TestBoundaryPlacementCycleRejected(t *testing.T) {
	// Implicit boundaries emit no frame; counting alone cannot detect this cycle.
	implicit := &sirena.BoundaryPlacement{}
	implicit.Children = []*sirena.BoundaryPlacement{implicit}
	lr := &sirena.LayoutResult{Bounds: sirena.Rect{Max: sirena.Point{X: 1, Y: 1}}, BoundaryPlacements: []*sirena.BoundaryPlacement{implicit}}
	if _, err := Build(lr, Options{}); err == nil || !strings.Contains(err.Error(), "cyclic boundary") {
		t.Fatalf("cycle accepted: %v", err)
	}
}
