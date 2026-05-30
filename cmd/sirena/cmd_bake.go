package main

import (
	"flag"
	"fmt"
	"io"

	"m31labs.dev/sirena/bake"
	"m31labs.dev/sirena/render/svg"
)

// RunBake implements `sirena bake [--theme name] [--infer] <markdown-file>...`.
//
// Each markdown file is scanned for diagram fences (mermaid, sirena/sir); each
// fence is rendered to SVG and the markdown is rewritten in place. A per-file
// summary is printed to stdout. Block errors and I/O errors are printed to
// stderr.
//
// Exit codes:
//
//	0  all files baked with no block errors
//	1  at least one file returned an error or had a block error
//	2  misuse — no args, bad flag, or unknown theme
func RunBake(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("bake", flag.ContinueOnError)
	fs.SetOutput(stderr)
	themeName := fs.String("theme", svg.DefaultThemeName, "theme name")
	infer := fs.Bool("infer", false, "promote Mermaid shapes/labels to typed sirena kinds")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "usage: sirena bake [--theme name] [--infer] <markdown-file>...")
		return 2
	}

	// Validate the theme once up front — all files share the same theme.
	if _, err := svg.ThemeForName(*themeName); err != nil {
		fmt.Fprintf(stderr, "SIR-BAKE-UNKNOWN-THEME: unknown theme %q\n", *themeName)
		return 2
	}

	opts := bake.Options{
		Infer: *infer,
		Theme: *themeName,
	}

	exitCode := 0
	for _, mdPath := range fs.Args() {
		res, err := bake.Bake(mdPath, opts)

		// Print per-block errors to stderr.
		for _, be := range res.BlockErrors {
			fmt.Fprintf(stderr, "%s: diagram %d (%s): %v\n", mdPath, be.Index, be.Lang, be.Err)
		}
		if err != nil && len(res.BlockErrors) == 0 {
			// Hard error (I/O, bad theme inside bake, etc.) without block errors.
			fmt.Fprintf(stderr, "%s: %v\n", mdPath, err)
		}

		// Print per-file summary to stdout.
		fmt.Fprintf(stdout, "baked %s: %d diagrams, %d svgs\n", mdPath, res.BlocksBaked, res.SVGsWritten)

		if err != nil || len(res.BlockErrors) > 0 {
			exitCode = 1
		}
	}
	return exitCode
}
