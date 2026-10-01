package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"m31labs.dev/sirena/fence"
	"os"
	"path/filepath"
)

// RunStoryboard writes stable SVG states and a portable frame manifest.
func RunStoryboard(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("storyboard", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("out", "storyboard", "output directory")
	kind := fs.String("diagram", "", "diagram family")
	theme := fs.String("theme", "", "SVG theme")
	duration := fs.Int("duration", 700, "transition duration (ms)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *duration < 0 || *duration > 10000 {
		fmt.Fprintln(stderr, "duration must be 0–10000 ms")
		return 2
	}
	var sources [][]byte
	for _, name := range fs.Args() {
		data, err := os.ReadFile(name)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		sources = append(sources, data)
	}
	frames, err := fence.Storyboard(sources, fence.Options{Diagram: *kind, Theme: *theme, StrictBudget: true})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := os.MkdirAll(*out, 0755); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	names := []string{}
	for i, data := range frames {
		name := fmt.Sprintf("state-%02d.svg", i)
		if err := os.WriteFile(filepath.Join(*out, name), data, 0644); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		names = append(names, name)
	}
	manifest, _ := json.MarshalIndent(struct {
		Frames   []string `json:"frames"`
		Duration int      `json:"durationMS"`
	}{names, *duration}, "", "  ")
	if err := os.WriteFile(filepath.Join(*out, "storyboard.json"), manifest, 0644); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "%d states written to %s\n", len(frames), *out)
	return 0
}
