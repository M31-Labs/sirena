package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderDiagramAndAutomaticTour(t *testing.T) {
	file := filepath.Join(t.TempDir(), "request.sir")
	if err := os.WriteFile(file, []byte("client browser { label: \"Browser\" }\nservice api { label: \"API\" }\nbrowser -> api: calls \"request\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, flags := range [][]string{{"--diagram", "sequence"}, {"--diagram", "radial"}, {"--diagram", "sequence", "--scene3d", "--tour", "relationships", "--motion-style", "float"}} {
		var out, errs bytes.Buffer
		args := append(flags, file)
		if code := RunRender(args, &out, &errs); code != 0 {
			t.Fatalf("%v: exit %d: %s", flags, code, errs.String())
		}
		if strings.Contains(strings.Join(flags, " "), "scene3d") && !strings.Contains(out.String(), `"slideSteps"`) {
			t.Fatal("tour missing from CLI output")
		}
	}
	for _, flags := range [][]string{{"--diagram", "bad"}, {"--tour", "nodes"}, {"--scene3d", "--tour", "nodes", "--steps", "steps.json"}, {"--scene3d", "--motion-style", "bad"}, {"--scene3d", "--motion-style", "float", "--motion-speed", "NaN"}, {"--scene3d", "--motion", "--motion-distance", "1"}} {
		var out, errs bytes.Buffer
		if code := RunRender(append(flags, file), &out, &errs); code != 2 {
			t.Fatalf("invalid flags %v: exit %d", flags, code)
		}
	}
}
