package scene3d

import (
	"encoding/json"
	"reflect"
	"testing"

	"m31labs.dev/gosx/scene"
)

func TestChoreographyAbsoluteRevealTraceAndFocus(t *testing.T) {
	lr := diagram(t)
	node := lr.NodePlacements[0].Node.Name
	edge := lr.EdgeRoutes[0].Edge.From + "->" + lr.EdgeRoutes[0].Edge.To
	data, err := Build(lr, Options{Steps: []Step{{Label: "Overview"}, {Label: "Reveal", Reveal: []string{node}}, {Label: "Trace", Trace: []string{edge}, Focus: []string{node}}, {Label: "Overview"}}})
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		SlideSteps Timeline `json:"slideSteps"`
	}
	if err = json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	frames := payload.SlideSteps.Frames
	if !reflect.DeepEqual(frames[0].Commands, frames[3].Commands) {
		t.Fatal("overview does not restore full absolute state")
	}
	removed, created := false, false
	for _, c := range frames[1].Commands {
		if c.Kind == scene.CommandRemoveObject {
			removed = true
		}
	}
	for _, c := range frames[2].Commands {
		if c.Kind == scene.CommandCreateObject {
			created = true
		}
	}
	if !removed || !created {
		t.Fatal("reveal/trace commands missing")
	}
	for _, steps := range [][]Step{{{Focus: []string{"missing"}}}, {{Trace: []string{node}}}} {
		if _, err = Build(lr, Options{Steps: steps}); err == nil {
			t.Fatal("invalid semantic target accepted")
		}
	}
}

// Consume the serialized create/remove protocol, rather than comparing two
// overview command lists: every intermediate hide/show must restore labels.
func TestChoreographyRestoresNodeAndRelationshipLabels(t *testing.T) {
	lr := diagram(t)
	data, err := Build(lr, Options{Steps: []Step{
		{Label: "Overview"},
		{Label: "Only browser", Reveal: []string{"browser"}},
		{Label: "All visible", Reveal: []string{"browser", "api", "db"}},
		{Label: "Overview again"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Scene      scene.SceneIR `json:"scene"`
		SlideSteps Timeline      `json:"slideSteps"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	original := map[string]scene.LabelIR{}
	for _, label := range payload.Scene.Labels {
		original[label.ID] = label
	}
	if len(original) != 5 {
		t.Fatalf("fixture must contain three node and two relationship labels, got %d", len(original))
	}
	for _, order := range [][]int{{0, 1, 2, 3}, {1, 3, 1, 2}, {2}} {
		state := map[string]scene.LabelIR{}
		for id, label := range original {
			state[id] = label
		}
		for _, index := range order {
			for _, command := range payload.SlideSteps.Frames[index].Commands {
				switch command.Kind {
				case scene.CommandRemoveObject:
					delete(state, command.ObjectID)
				case scene.CommandCreateObject:
					encoded, err := json.Marshal(command.Data)
					if err != nil {
						t.Fatal(err)
					}
					var create struct {
						Kind  string        `json:"kind"`
						Props scene.LabelIR `json:"props"`
					}
					if err := json.Unmarshal(encoded, &create); err != nil {
						t.Fatal(err)
					}
					if create.Kind == "label" {
						if create.Props.ID != command.ObjectID {
							t.Fatal("label command identity mismatch")
						}
						state[command.ObjectID] = create.Props
					}
				}
			}
			want := original
			if index == 1 {
				want = map[string]scene.LabelIR{"label:browser": original["label:browser"]}
			}
			if !reflect.DeepEqual(state, want) {
				t.Fatalf("seek order %v frame %d: label state differs (got %v, want %v)", order, index, state, want)
			}
		}
	}
}
