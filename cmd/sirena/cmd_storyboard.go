package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"m31labs.dev/sirena/fence"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	// Only remove frames recorded by our previous manifest. Preserve unrelated
	// files and never let a manifest path escape this output directory.
	var previous struct {
		Frames []string `json:"frames"`
	}
	if data, readErr := os.ReadFile(filepath.Join(*out, "storyboard.json")); readErr == nil {
		if err := json.Unmarshal(data, &previous); err != nil {
			fmt.Fprintln(stderr, "invalid previous storyboard manifest:", err)
			return 1
		}
	} else if !os.IsNotExist(readErr) {
		fmt.Fprintln(stderr, readErr)
		return 1
	}
	names := []string{}
	for i, data := range frames {
		name := fmt.Sprintf("state-%02d.svg", i)
		if err := writeStoryboardFile(*out, name, data); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		names = append(names, name)
	}
	for _, name := range previous.Frames {
		i, parseErr := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(name, "state-"), ".svg"))
		if parseErr != nil || i < len(frames) || i >= 32 || name != fmt.Sprintf("state-%02d.svg", i) {
			continue
		}
		if err := os.Remove(filepath.Join(*out, name)); err != nil && !os.IsNotExist(err) {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	manifest, _ := json.MarshalIndent(struct {
		Frames   []string `json:"frames"`
		Duration int      `json:"durationMS"`
	}{names, *duration}, "", "  ")
	if err := writeStoryboardFile(*out, "storyboard.json", manifest); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "%d states written to %s\n", len(frames), *out)
	return 0
}

// Publish a complete file by replacing the destination directory entry. An
// existing symlink or hard link never redirects writes into its target file.
func writeStoryboardFile(dir, name string, data []byte) error {
	f, err := os.CreateTemp(dir, ".sirena-storyboard-*")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if err := f.Chmod(0644); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(temp, filepath.Join(dir, name))
}
