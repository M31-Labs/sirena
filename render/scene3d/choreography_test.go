package scene3d

import (
	"encoding/json"
	"m31labs.dev/gosx/scene"
	"reflect"
	"testing"
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
