package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"m31labs.dev/sirena"
	"m31labs.dev/sirena/ingest/mermaid"
	_ "m31labs.dev/sirena/layout" // register the layout engine into sirena.Render
	"m31labs.dev/sirena/render/svg"
)

// RunRender implements `sirena render <view-or-system> [-o out.svg]
// [--theme name] [--strict-budget] [--from mermaid|sirena] [--infer]`.
//
// Extension routing:
//   - .mmd / .mermaid → Mermaid ingestion path
//   - .sir / .view.sir → existing sirena path (unchanged)
//   - --from mermaid|sirena overrides extension detection (for stdin or odd exts)
//
// Exit codes: 0 on success, 1 on I/O / parse / budget-breach error, 2 on
// misuse (bad flags, unknown theme, unknown extension).
func RunRender(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	fs.SetOutput(stderr)
	outPath := fs.String("o", "", "output file path (default: stdout)")
	themeName := fs.String("theme", svg.DefaultThemeName, "theme name")
	strict := fs.Bool("strict-budget", false, "fail if the view exceeds its budget")
	fromFlag := fs.String("from", "", "force input format: mermaid or sirena")
	infer := fs.Bool("infer", false, "promote Mermaid shapes/labels to typed sirena kinds")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: sirena render [-o out.svg] [--theme name] [--strict-budget] [--from mermaid|sirena] [--infer] <view-or-system>")
		return 2
	}

	theme, err := svg.ThemeForName(*themeName)
	if err != nil {
		fmt.Fprintf(stderr, "SIR-RENDER-UNKNOWN-THEME: unknown theme %q\n", *themeName)
		return 2
	}

	target := fs.Arg(0)

	// Determine which render path to use.
	format, code := resolveFormat(target, *fromFlag, stderr)
	if code != 0 {
		return code
	}

	var (
		svgBytes []byte
	)
	switch format {
	case "mermaid":
		svgBytes, code = renderMermaid(target, *infer, *strict, theme, stderr)
	case "sirena":
		svgBytes, code = renderSirena(target, *strict, theme, stderr)
	}
	if code != 0 {
		return code
	}

	if *outPath != "" {
		if err := os.WriteFile(*outPath, svgBytes, 0o644); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	if _, err := stdout.Write(svgBytes); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// resolveFormat determines the render format ("mermaid" or "sirena") for the
// given target path, optionally overridden by the --from flag.
func resolveFormat(target, from string, stderr io.Writer) (string, int) {
	if from != "" {
		switch from {
		case "mermaid", "sirena":
			return from, 0
		default:
			fmt.Fprintf(stderr, "SIR-RENDER-UNKNOWN-FROM: unknown --from value %q (want mermaid or sirena)\n", from)
			return "", 2
		}
	}
	ext := strings.ToLower(filepath.Ext(target))
	// Handle .view.sir as a special compound extension.
	if strings.HasSuffix(strings.ToLower(target), ".view.sir") {
		return "sirena", 0
	}
	switch ext {
	case ".mmd", ".mermaid":
		return "mermaid", 0
	case ".sir":
		return "sirena", 0
	default:
		fmt.Fprintf(stderr, "SIR-RENDER-UNKNOWN-EXT: cannot determine format for %q; use --from mermaid|sirena\n", filepath.Base(target))
		return "", 2
	}
}

// renderMermaid ingests a Mermaid file and renders it to SVG bytes.
func renderMermaid(target string, infer bool, strict bool, theme *svg.Theme, stderr io.Writer) ([]byte, int) {
	src, err := os.ReadFile(target)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return nil, 1
	}

	doc, diags, err := mermaid.Parse(src, mermaid.Options{Infer: infer})

	// Print all diagnostics to stderr regardless of error.
	for _, d := range diags {
		fmt.Fprintf(stderr, "%s: %s\n", d.Code, d.Message)
	}

	if err != nil {
		// err from mermaid.Parse is a fatal error (e.g. SIR-MERMAID-NOT-A-GRAPH).
		fmt.Fprintln(stderr, err)
		return nil, 1
	}
	if doc == nil {
		fmt.Fprintln(stderr, "SIR-MERMAID-NOT-A-GRAPH: input is not a Mermaid flowchart")
		return nil, 1
	}

	rv := sirena.AllElementsView(doc)
	lr, report, rerr := sirena.Render(rv, sirena.RenderOptions{StrictBudget: strict})
	if errors.Is(rerr, sirena.ErrBudgetExceeded) {
		fmt.Fprintln(stderr, "SIR-RENDER-BUDGET-EXCEEDED: view exceeds its declared budget")
		printBudget(stderr, report)
		return nil, 1
	}
	if rerr != nil {
		fmt.Fprintln(stderr, rerr)
		return nil, 1
	}
	if lr == nil {
		fmt.Fprintln(stderr, "SIR-RENDER-NO-LAYOUT: layout engine produced no result")
		return nil, 1
	}

	out, serr := svg.Render(lr, theme)
	if serr != nil {
		fmt.Fprintln(stderr, serr)
		return nil, 1
	}
	return out, 0
}

// renderSirena loads and renders a .sir or .view.sir file via the existing path.
func renderSirena(target string, strict bool, theme *svg.Theme, stderr io.Writer) ([]byte, int) {
	rv, code := resolveRenderView(target, stderr)
	if code != 0 {
		return nil, code
	}

	lr, report, err := sirena.Render(rv, sirena.RenderOptions{StrictBudget: strict})
	if errors.Is(err, sirena.ErrBudgetExceeded) {
		fmt.Fprintln(stderr, "SIR-RENDER-BUDGET-EXCEEDED: view exceeds its declared budget")
		printBudget(stderr, report)
		return nil, 1
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return nil, 1
	}
	if lr == nil {
		fmt.Fprintln(stderr, "SIR-RENDER-NO-LAYOUT: layout engine produced no result")
		return nil, 1
	}

	out, err := svg.Render(lr, theme)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return nil, 1
	}
	return out, 0
}

// resolveRenderView loads the target and produces the ResolvedView to
// render: a .view.sir file's first view (evaluated against its
// workspace), or an all-elements synthesis of a .sir system file.
func resolveRenderView(target string, stderr io.Writer) (*sirena.ResolvedView, int) {
	if _, err := os.Stat(target); err != nil {
		fmt.Fprintln(stderr, err)
		return nil, 1
	}
	ws, err := sirena.OpenWorkspace(filepath.Dir(target))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return nil, 1
	}
	src, err := os.ReadFile(target)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return nil, 1
	}
	doc, err := sirena.Parse(src)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return nil, 1
	}
	if len(doc.Views) > 0 {
		rv, err := sirena.EvaluateView(ws, doc.Views[0])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return nil, 1
		}
		return rv, 0
	}
	return sirena.AllElementsView(doc), 0
}

// printBudget writes a budget report's breaches to w, one per line.
func printBudget(w io.Writer, report *sirena.BudgetReport) {
	if report == nil {
		return
	}
	for _, b := range report.Breaches {
		fmt.Fprintf(w, "  %s: %d over limit %d\n", b.Field, b.Actual, b.Limit)
	}
}
