package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"m31labs.dev/sirena"
	"m31labs.dev/sirena/ingest/mermaid"
	_ "m31labs.dev/sirena/layout"
	"m31labs.dev/sirena/render/svg"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// RunBench measures the warmed parse/layout/SVG pipeline, excluding process
// startup, file I/O and compilation. It never measures browser FPS.
func RunBench(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("bench", flag.ContinueOnError)
	fs.SetOutput(stderr)
	runs := fs.Int("runs", 7, "measured iterations, after two warm-ups")
	kind := fs.String("diagram", "", "override diagram family")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *runs < 1 || *runs > 100 || fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: sirena bench [--runs 7] [--diagram family] file.sir|file.mmd")
		return 2
	}
	src, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	ext := filepath.Ext(fs.Arg(0))
	if ext != ".sir" && ext != ".mmd" && ext != ".mermaid" {
		fmt.Fprintln(stderr, "bench accepts .sir, .mmd or .mermaid")
		return 2
	}
	type sample struct {
		Millis float64 `json:"pipelineMillis"`
		Bytes  int     `json:"svgBytes"`
		Alloc  uint64  `json:"allocatedBytes"`
	}
	samples := []sample{}
	for i := -2; i < *runs; i++ {
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		start := time.Now()
		var doc *sirena.Document
		if ext == ".sir" {
			doc, err = sirena.Parse(src)
		} else {
			doc, _, err = mermaid.Parse(src, mermaid.Options{})
		}
		if err == nil {
			for _, d := range doc.Diagnostics() {
				if d.Severity == sirena.SeverityError {
					err = fmt.Errorf("%s", d.Message)
					break
				}
			}
		}
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		lr, _, err := sirena.Render(sirena.AllElementsView(doc), sirena.RenderOptions{Diagram: *kind, StrictBudget: true})
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		data, err := svg.Render(lr, nil)
		elapsed := float64(time.Since(start).Microseconds()) / 1000
		runtime.ReadMemStats(&after)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if i >= 0 {
			samples = append(samples, sample{elapsed, len(data), after.TotalAlloc - before.TotalAlloc})
		}
	}
	if err = json.NewEncoder(stdout).Encode(struct {
		Phase   string   `json:"phase"`
		Samples []sample `json:"samples"`
	}{"warm parse+layout+SVG; excludes CLI startup and I/O", samples}); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
