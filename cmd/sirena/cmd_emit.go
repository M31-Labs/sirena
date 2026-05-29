package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"m31labs.dev/sirena"
	"m31labs.dev/sirena/emit"
	_ "m31labs.dev/sirena/layout" // register the layout engine into sirena.Render
	"m31labs.dev/sirena/render/svg"
)

// RunEmit implements `sirena emit [-o out] [--format sir|svg] [--theme name]
// <go-dir>`. It reads the Go module rooted at <go-dir> with gotreesitter,
// models each package as a node and each intra-module import as a depends_on
// edge, and writes the result as sirena source (--format sir, the default)
// or a rendered SVG (--format svg).
//
// Exit codes: 0 on success, 1 on I/O / emit / render error, 2 on misuse
// (bad flags, unknown format or theme).
func RunEmit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("emit", flag.ContinueOnError)
	fs.SetOutput(stderr)
	outPath := fs.String("o", "", "output file path (default: stdout)")
	format := fs.String("format", "sir", "output format: sir or svg")
	themeName := fs.String("theme", svg.DefaultThemeName, "theme name (svg format only)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: sirena emit [-o out] [--format sir|svg] [--theme name] <go-dir>")
		return 2
	}

	doc, err := emit.GoPackageGraph(fs.Arg(0))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	var out []byte
	switch *format {
	case "sir":
		out, err = sirena.Print(doc)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	case "svg":
		theme, terr := svg.ThemeForName(*themeName)
		if terr != nil {
			fmt.Fprintf(stderr, "SIR-EMIT-UNKNOWN-THEME: unknown theme %q\n", *themeName)
			return 2
		}
		lr, _, rerr := sirena.Render(sirena.AllElementsView(doc), sirena.RenderOptions{})
		if rerr != nil {
			fmt.Fprintln(stderr, rerr)
			return 1
		}
		out, err = svg.Render(lr, theme)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	default:
		fmt.Fprintf(stderr, "SIR-EMIT-UNKNOWN-FORMAT: unknown format %q (want sir or svg)\n", *format)
		return 2
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
