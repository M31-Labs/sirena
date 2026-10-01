package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStoryboardRerunRemovesOnlyPreviousGeneratedFrames(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "input.sir")
	if err := os.WriteFile(source, []byte(`service api { label: "API" value: 12 }`), 0644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "output")
	var stdout, stderr bytes.Buffer
	args := []string{"--diagram", "bar", "--out", out, source, source, source}
	if code := RunStoryboard(args, &stdout, &stderr); code != 0 {
		t.Fatal(code, stderr.String())
	}
	keep := filepath.Join(out, "state-custom.svg")
	if err := os.WriteFile(keep, []byte("user asset"), 0644); err != nil {
		t.Fatal(err)
	}
	if code := RunStoryboard(args[:len(args)-1], &stdout, &stderr); code != 0 {
		t.Fatal(code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(out, "state-02.svg")); !os.IsNotExist(err) {
		t.Fatal("stale generated frame survived", err)
	}
	if data, err := os.ReadFile(keep); err != nil || string(data) != "user asset" {
		t.Fatal("unrelated asset changed", err)
	}
	data, err := os.ReadFile(filepath.Join(out, "storyboard.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Frames []string `json:"frames"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Frames) != 2 {
		t.Fatal(manifest.Frames)
	}
	if code := RunStoryboard(args[:len(args)-2], &stdout, &stderr); code == 0 {
		t.Fatal("one state accepted")
	}
	if after, err := os.ReadFile(filepath.Join(out, "storyboard.json")); err != nil || !bytes.Equal(after, data) {
		t.Fatal("invalid rerun changed manifest", err)
	}
}
