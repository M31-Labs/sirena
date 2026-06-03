package main

import (
	"flag"
	"fmt"
	"io"

	"m31labs.dev/sirena/bake"
	"m31labs.dev/sirena/render/svg"
)

// RunBake implements
// `sirena bake [--theme name] [--infer] [--check|--dry-run] <markdown-file>...`.
//
// Each markdown file is scanned for diagram fences (mermaid, sirena/sir); each
// fence is rendered to SVG and the markdown is rewritten in place. A per-file
// summary is printed to stdout. Block errors and I/O errors are printed to
// stderr.
//
// --check and --dry-run both render without writing any file and report which
// paths would change. They differ only in exit code: --check exits non-zero
// when anything is stale (for CI), while --dry-run is report-only. When both
// are given, --check wins.
//
// Exit codes:
//
//	0  all files baked (or, under --check, everything up to date) with no errors
//	1  at least one file errored, had a block error, or (under --check) is stale
//	2  misuse — no args, bad flag, or unknown theme
func RunBake(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("bake", flag.ContinueOnError)
	fs.SetOutput(stderr)
	themeName := fs.String("theme", svg.DefaultThemeName, "theme name")
	infer := fs.Bool("infer", false, "promote Mermaid shapes/labels to typed sirena kinds")
	check := fs.Bool("check", false, "report files that would change and exit non-zero if any are stale; writes nothing")
	dryRun := fs.Bool("dry-run", false, "report files that would change without writing; always exits zero")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "usage: sirena bake [--theme name] [--infer] [--check|--dry-run] <markdown-file>...")
		return 2
	}

	// Validate the theme once up front — all files share the same theme.
	if _, err := svg.ThemeForName(*themeName); err != nil {
		fmt.Fprintf(stderr, "SIR-BAKE-UNKNOWN-THEME: unknown theme %q\n", *themeName)
		return 2
	}

	noWrite := *check || *dryRun
	opts := bake.Options{
		Infer:  *infer,
		Theme:  *themeName,
		DryRun: noWrite,
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

		if noWrite {
			// Report staleness instead of writing.
			if len(res.StalePaths) == 0 {
				fmt.Fprintf(stdout, "up to date %s\n", mdPath)
			} else {
				fmt.Fprintf(stdout, "stale %s: %d file(s) would change\n", mdPath, len(res.StalePaths))
				for _, p := range res.StalePaths {
					fmt.Fprintf(stdout, "  %s\n", p)
				}
				if *check {
					exitCode = 1
				}
			}
		} else {
			// Print per-file summary to stdout.
			fmt.Fprintf(stdout, "baked %s: %d diagrams, %d svgs\n", mdPath, res.BlocksBaked, res.SVGsWritten)
		}

		if err != nil || len(res.BlockErrors) > 0 {
			exitCode = 1
		}
	}
	return exitCode
}
