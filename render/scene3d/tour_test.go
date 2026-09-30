package scene3d

import (
	"encoding/json"
	"m31labs.dev/gosx/scene"
	"m31labs.dev/sirena"
	"m31labs.dev/sirena/layout"
	"testing"
)

func TestSequenceToursUseNativeMotionAndReset(t *testing.T) {
	lr, _, err := layout.Render(diagram(t).View, sirena.RenderOptions{Diagram: "sequence"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := Build(lr, Options{Tour: "relationships", MotionStyle: "float"})
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
	frames := props.SlideSteps.Frames
	if len(frames) != len(lr.EdgeRoutes)+2 {
		t.Fatal("missing automatic tour steps")
	}
	if string(mustJSON(frames[0].Commands)) != string(mustJSON(frames[len(frames)-1].Commands)) {
		t.Fatal("tour end does not reset overview")
	}
	lines, floating := 0, false
	for _, obj := range props.Scene.Objects {
		if obj.Kind == "lines" {
			lines++
		}
		if obj.DriftSpeed != 0 {
			floating = true
		}
	}
	if lines < len(lr.Lifelines)+len(lr.EdgeRoutes) || !floating {
		t.Fatal("lifelines or native float motion missing")
	}
}

func TestTourOptionsAndBounds(t *testing.T) {
	for _, opts := range []Options{{Tour: "wrong"}, {MotionStyle: "wrong"}, {Tour: "nodes", Steps: []Step{{Label: "Manual"}}}} {
		if _, err := Build(diagram(t), opts); err == nil {
			t.Fatal("invalid tour configuration accepted")
		}
	}
	lr := diagram(t)
	for i := 0; i < 127; i++ {
		lr.NodePlacements = append(lr.NodePlacements, &sirena.NodePlacement{Node: &sirena.Element{Name: string(rune(1000 + i))}, Bounds: lr.NodePlacements[0].Bounds})
	}
	if _, err := Build(lr, Options{Tour: "nodes"}); err == nil {
		t.Fatal("oversized tour accepted")
	}
}

func TestNativeMotionControls(t *testing.T) {
	speed, distance := 1.4, .3
	data, err := Build(diagram(t), Options{MotionStyle: "float", MotionSpeed: &speed, MotionDistance: &distance})
	if err != nil {
		t.Fatal(err)
	}
	var props struct {
		Scene scene.SceneIR `json:"scene"`
	}
	if err := json.Unmarshal(data, &props); err != nil {
		t.Fatal(err)
	}
	if props.Scene.Objects[0].DriftSpeed != speed || props.Scene.Objects[0].ShiftY != distance {
		t.Fatal("motion controls lost")
	}
	for _, obj := range props.Scene.Objects {
		if obj.Kind == "lines" {
			continue
		}
		found := false
		for _, label := range props.Scene.Labels {
			if label.ID != "label:"+obj.ID {
				continue
			}
			found = true
			if label.DriftSpeed != obj.DriftSpeed || label.DriftPhase != obj.DriftPhase || label.ShiftY != obj.ShiftY {
				t.Fatalf("label %s does not follow native float", label.ID)
			}
			if label.Y <= obj.Y || label.Z <= obj.Z || label.Priority <= 0 || label.WhiteSpace != "pre" {
				t.Fatalf("label %s lacks readable anchor or collision priority", label.ID)
			}
		}
		if !found {
			t.Fatalf("missing label for %s", obj.ID)
		}
	}
	speed = 0
	props.Scene = scene.SceneIR{}
	data, err = Build(diagram(t), Options{MotionStyle: "float", MotionSpeed: &speed})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &props); err != nil {
		t.Fatal(err)
	}
	if props.Scene.Objects[0].DriftSpeed != 0 || props.Scene.Objects[0].ShiftY != 0 {
		t.Fatal("zero speed does not disable motion")
	}
}
