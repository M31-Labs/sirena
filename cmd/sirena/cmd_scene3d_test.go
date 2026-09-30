package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRenderScene3DFromSirenaAndMermaid(t *testing.T) {
	dir := t.TempDir()
	for _, test := range []struct{ name, source string }{{"system.sir", "service api { label: \"API\" }\ndatabase db {}\napi -> db: reads \"query\"\n"}, {"flow.mmd", "flowchart LR\n A[API] --> B[(Storage)]\n"}} {
		path := filepath.Join(dir, test.name)
		if err := os.WriteFile(path, []byte(test.source), 0644); err != nil {
			t.Fatal(err)
		}
		var out, diagnostics bytes.Buffer
		if code := RunRender([]string{"--scene3d", "--motion", path}, &out, &diagnostics); code != 0 {
			t.Fatalf("%s: %d %s", test.name, code, diagnostics.String())
		}
		var props map[string]json.RawMessage
		if err := json.Unmarshal(out.Bytes(), &props); err != nil {
			t.Fatal(err)
		}
		if len(props["scene"]) == 0 {
			t.Fatal("missing scene")
		}
	}
}
func TestScene3DRejectsInvalidOptionCombinations(t *testing.T) {
	for _, args := range [][]string{{"--shader", "x.sel", "x.sir"}, {"--scene3d", "--interactive", "x.sir"}, {"--scene3d", "--workflow", "x.json", "x.sir"}, {"--scene3d", "--material", "Ink", "x.sir"}, {"--scene3d", "--targets", "api", "x.sir"}} {
		var out, diagnostics bytes.Buffer
		if code := RunRender(args, &out, &diagnostics); code != 2 {
			t.Fatalf("%v: code=%d %s", args, code, diagnostics.String())
		}
	}
}
