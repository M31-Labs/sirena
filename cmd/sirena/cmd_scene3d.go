package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"m31labs.dev/sirena"
	"m31labs.dev/sirena/ingest/mermaid"
	"m31labs.dev/sirena/render/scene3d"
)

func renderScene3D(target, format string, infer, strict bool, shaderPath, material, targets string, motion bool, stepsPath string, stderr io.Writer) ([]byte, int) {
	var rv *sirena.ResolvedView
	if format == "sirena" {
		var code int
		rv, code = resolveRenderView(target, stderr)
		if code != 0 {
			return nil, code
		}
	} else {
		src, err := os.ReadFile(target)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return nil, 1
		}
		doc, diags, err := mermaid.Parse(src, mermaid.Options{Infer: infer})
		for _, diag := range diags {
			fmt.Fprintf(stderr, "%s: %s\n", diag.Code, diag.Message)
		}
		if err != nil || doc == nil {
			fmt.Fprintf(stderr, "cannot ingest Mermaid flowchart: %v\n", err)
			return nil, 1
		}
		rv = sirena.AllElementsView(doc)
	}
	lr, report, err := sirena.Render(rv, sirena.RenderOptions{StrictBudget: strict})
	if report != nil {
		printBudget(stderr, report)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return nil, 1
	}
	opts := scene3d.Options{Material: material, Motion: motion}
	for _, id := range strings.Split(targets, ",") {
		if id = strings.TrimSpace(id); id != "" {
			opts.Targets = append(opts.Targets, id)
		}
	}
	if shaderPath != "" {
		opts.Shader, err = os.ReadFile(shaderPath)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return nil, 1
		}
	}
	if stepsPath != "" {
		src, err := os.ReadFile(stepsPath)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return nil, 1
		}
		if len(src) > 4<<20 {
			fmt.Fprintln(stderr, "Scene3D keyframes exceed 4 MiB")
			return nil, 1
		}
		decoder := json.NewDecoder(bytes.NewReader(src))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&opts.Steps); err != nil {
			fmt.Fprintln(stderr, err)
			return nil, 1
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			fmt.Fprintln(stderr, "expected one keyframe array")
			return nil, 1
		}
	}
	out, err := scene3d.Build(lr, opts)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return nil, 1
	}
	return out, 0
}
