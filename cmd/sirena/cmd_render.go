package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"m31labs.dev/sirena"
	_ "m31labs.dev/sirena/layout" // register the layout engine into sirena.Render
	"m31labs.dev/sirena/render/svg"
)

// RunRender implements `sirena render <view-or-system> [-o out.svg]
// [--theme name] [--strict-budget]`. It opens the target file's workspace,
// evaluates the view (a .view.sir file) or synthesizes an all-elements
// view (a .sir system file), runs the budget gate plus layout via
// sirena.Render, and emits SVG to a file or stdout.
//
// Exit codes: 0 on success, 1 on I/O / parse / budget-breach error, 2 on
// misuse (bad flags, unknown theme).
func RunRender(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	fs.SetOutput(stderr)
	outPath := fs.String("o", "", "output file path (default: stdout)")
	themeName := fs.String("theme", svg.DefaultThemeName, "theme name")
	strict := fs.Bool("strict-budget", false, "fail if the view exceeds its budget")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: sirena render [-o out.svg] [--theme name] [--strict-budget] <view-or-system>")
		return 2
	}

	theme, err := svg.ThemeForName(*themeName)
	if err != nil {
		fmt.Fprintf(stderr, "SIR-RENDER-UNKNOWN-THEME: unknown theme %q\n", *themeName)
		return 2
	}

	rv, code := resolveRenderView(fs.Arg(0), stderr)
	if code != 0 {
		return code
	}

	lr, report, err := sirena.Render(rv, sirena.RenderOptions{StrictBudget: *strict})
	if errors.Is(err, sirena.ErrBudgetExceeded) {
		fmt.Fprintln(stderr, "SIR-RENDER-BUDGET-EXCEEDED: view exceeds its declared budget")
		printBudget(stderr, report)
		return 1
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if lr == nil {
		fmt.Fprintln(stderr, "SIR-RENDER-NO-LAYOUT: layout engine produced no result")
		return 1
	}

	out, err := svg.Render(lr, theme)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	if *outPath != "" {
		if err := os.WriteFile(*outPath, out, 0o644); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	if _, err := stdout.Write(out); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
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
