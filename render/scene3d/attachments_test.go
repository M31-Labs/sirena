package scene3d

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"testing"

	"m31labs.dev/gosx/scene"
)

func createdObject(t *testing.T, frame Frame, id string) scene.ObjectIR {
	t.Helper()
	for _, command := range frame.Commands {
		if command.Kind != scene.CommandCreateObject || command.ObjectID != id {
			continue
		}
		var payload struct {
			Props scene.ObjectIR `json:"props"`
		}
		if err := json.Unmarshal(mustJSON(command.Data), &payload); err != nil {
			t.Fatal(err)
		}
		return payload.Props
	}
	t.Fatalf("no creation of %s", id)
	return scene.ObjectIR{}
}

func TestMovedActorRetainsPortsArrowheadsAndCaption(t *testing.T) {
	lr := diagram(t)
	x, y, z, factor := 2.0, 1.4, .7, 1.5
	opts := Options{Steps: []Step{{Label: "Overview"}, {Label: "Moved", DurationMS: 900, Easing: "linear", Patches: []Patch{{Target: "api", X: &x, Y: &y, Z: &z, Scale: &factor}}, Camera: &scene.IRCamera{Z: 8, FOV: 45, Near: .1, Far: 100}}, {Label: "Reset"}}}
	data, err := Build(lr, opts)
	if err != nil {
		t.Fatal(err)
	}
	var props struct {
		Scene scene.SceneIR `json:"scene"`
		Steps Timeline      `json:"slideSteps"`
	}
	if err := json.Unmarshal(data, &props); err != nil {
		t.Fatal(err)
	}
	objects := map[string]scene.ObjectIR{}
	for _, obj := range props.Scene.Objects {
		objects[obj.ID] = obj
	}
	index := -1
	for i, r := range lr.EdgeRoutes {
		if r.Edge.From == "browser" && r.Edge.To == "api" {
			index = i
		}
	}
	if index < 0 {
		t.Fatal("missing browser request route")
	}
	id := fmt.Sprintf("edge:%d", index)
	edge := createdObject(t, props.Steps.Frames[1], id)
	base, actor := objects[id], objects["api"]
	last := len(lr.EdgeRoutes[index].Points) - 1
	port := base.Points[last]
	want := scene.Vec3(x+(port.X-actor.X)*factor, y+(port.Y-actor.Y)*factor, z+(port.Z-actor.Z)*factor)
	got := edge.Points[last]
	if math.Abs(got.X-want.X) > 1e-9 || math.Abs(got.Y-want.Y) > 1e-9 || math.Abs(got.Z-want.Z) > 1e-9 {
		t.Fatalf("edge detached: got %+v want %+v", got, want)
	}
	if len(edge.LineSegments) != len(base.LineSegments) || len(edge.Points) != len(base.Points) {
		t.Fatal("arrow topology changed during a pose")
	}
	for i := last + 1; i < len(edge.Points); i += 2 {
		if edge.Points[i] != got {
			t.Fatal("arrowhead tip detached from target")
		}
	}
	if !bytes.Equal(mustJSON(props.Steps.Frames[0].Commands), mustJSON(props.Steps.Frames[2].Commands)) {
		t.Fatal("direct reset differs from initial state")
	}
	if props.Steps.Frames[1].DurationMS != 900 || props.Steps.Frames[1].Easing != "linear" {
		t.Fatal("timing metadata lost")
	}
	labelMoved := false
	cameraReset := false
	for _, c := range props.Steps.Frames[1].Commands {
		if c.Kind == scene.CommandCreateObject && c.ObjectID == "label:"+id {
			var p struct {
				Props scene.LabelIR `json:"props"`
			}
			json.Unmarshal(mustJSON(c.Data), &p)
			for _, old := range props.Scene.Labels {
				if old.ID == p.Props.ID && p.Props.X != old.X {
					labelMoved = true
				}
			}
		}
	}
	for _, c := range props.Steps.Frames[2].Commands {
		if c.Kind == scene.CommandSetCamera {
			var camera scene.IRCamera
			json.Unmarshal(mustJSON(c.Data), &camera)
			cameraReset = camera.Z == 11 && camera.FOV == 50
		}
	}
	if !labelMoved || !cameraReset {
		t.Fatal("caption or camera did not follow absolute pose")
	}
}

func TestHiddenActorHidesAttachedRelationships(t *testing.T) {
	hidden := false
	data, err := Build(diagram(t), Options{Steps: []Step{{Label: "Base"}, {Label: "Hide", Patches: []Patch{{Target: "api", Visible: &hidden}}}, {Label: "Restore"}}})
	if err != nil {
		t.Fatal(err)
	}
	var props struct {
		Steps Timeline `json:"slideSteps"`
	}
	json.Unmarshal(data, &props)
	for _, id := range []string{"edge:0", "edge:1"} {
		removed := false
		for _, c := range props.Steps.Frames[1].Commands {
			if c.ObjectID == id && c.Kind == scene.CommandRemoveObject {
				removed = true
			}
			if c.ObjectID == id && c.Kind == scene.CommandCreateObject {
				t.Fatal("hidden actor retains relationship")
			}
		}
		if !removed {
			t.Fatal("relationship not removed")
		}
		createdObject(t, props.Steps.Frames[2], id)
	}
}

func TestStoryTimingAndCameraValidation(t *testing.T) {
	for _, step := range []Step{{DurationMS: -1}, {DurationMS: 600001}, {Easing: "surprise"}, {Camera: &scene.IRCamera{Kind: "invalid"}}, {Camera: &scene.IRCamera{FOV: 180}}, {Camera: &scene.IRCamera{Near: 10, Far: 1}}} {
		if _, err := Build(diagram(t), Options{Steps: []Step{step}}); err == nil {
			t.Fatalf("accepted invalid step: %+v", step)
		}
	}
}
