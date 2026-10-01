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

func TestStoryboardPublicationDoesNotWriteThroughLinks(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "output")
	if err := os.Mkdir(out, 0755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "input.sir")
	if err := os.WriteFile(source, []byte(`service api { value: 12 }`), 0644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(dir, "outside.svg")
	if err := os.WriteFile(outside, []byte("unrelated content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(out, "state-00.svg")); err != nil {
		t.Skip("symlink creation unavailable:", err)
	}
	if err := os.Link(outside, filepath.Join(out, "state-01.svg")); err != nil {
		t.Fatal(err)
	}
	priorManifest := filepath.Join(dir, "outside.json")
	prior := []byte(`{"frames":[]}`)
	if err := os.WriteFile(priorManifest, prior, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(priorManifest, filepath.Join(out, "storyboard.json")); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := RunStoryboard([]string{"--diagram", "bar", "--out", out, source, source}, &stdout, &stderr); code != 0 {
		t.Fatal(code, stderr.String())
	}
	for _, file := range []string{"state-00.svg", "state-01.svg", "storyboard.json"} {
		info, err := os.Lstat(filepath.Join(out, file))
		if err != nil || !info.Mode().IsRegular() {
			t.Fatal("generated file is not regular", file, err)
		}
	}
	if data, err := os.ReadFile(outside); err != nil || string(data) != "unrelated content" {
		t.Fatal("linked target overwritten", err)
	}
	if data, err := os.ReadFile(priorManifest); err != nil || !bytes.Equal(data, prior) {
		t.Fatal("manifest target overwritten", err)
	}
	if files, err := filepath.Glob(filepath.Join(out, ".sirena-storyboard-*")); err != nil || len(files) != 0 {
		t.Fatal("temporary files leaked", files, err)
	}
}
