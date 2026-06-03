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
// [--update path] <go-dir>`. It reads the Go module rooted at <go-dir> with
// gotreesitter, models each package as a node and each intra-module import as
// a depends_on edge, and writes the result as sirena source (--format sir,
// the default) or a rendered SVG (--format svg).
//
// With --update PATH it runs the agent-emission round trip instead: it
// assigns stable sids, reconciles the fresh graph against the prior .gen.sir
// at PATH (carrying sids forward and recording prior_source_ref on renames),
// and writes the regenerated .gen.sir back to PATH. --update ignores -o and
// --format (the round-trip output is always sirena source written to PATH).
//
// Exit codes: 0 on success, 1 on I/O / emit / render error, 2 on misuse
// (bad flags, unknown format or theme).
func RunEmit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("emit", flag.ContinueOnError)
	fs.SetOutput(stderr)
	outPath := fs.String("o", "", "output file path (default: stdout)")
	format := fs.String("format", "sir", "output format: sir or svg")
	themeName := fs.String("theme", svg.DefaultThemeName, "theme name (svg format only)")
	update := fs.String("update", "", "reconcile against and rewrite a .gen.sir round-trip file at PATH")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: sirena emit [-o out] [--format sir|svg] [--theme name] [--update path] <go-dir>")
		return 2
	}

	doc, err := emit.GoPackageGraph(fs.Arg(0))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	if *update != "" {
		return runEmitUpdate(doc, *update, stderr)
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

// runEmitUpdate performs the agent-emission round trip: assign sids to the
// freshly emitted doc, reconcile it against the prior .gen.sir at path (when
// one exists) so matched declarations keep their sid and renames record a
// prior_source_ref, then print and write the result back to path.
func runEmitUpdate(doc *sirena.Document, path string, stderr io.Writer) int {
	if err := sirena.EmitSIDs(doc, sirena.EmissionOptions{}); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	prior, rerr := os.ReadFile(path)
	switch {
	case rerr == nil:
		priorDoc, perr := sirena.Parse(prior)
		if perr != nil {
			fmt.Fprintf(stderr, "sirena: emit --update: parse prior %s: %v\n", path, perr)
			return 1
		}
		newEls := flattenEmitElements(doc)
		res := sirena.Reconcile(flattenEmitElements(priorDoc), newEls, sirena.EmissionOptions{})
		for i, ne := range newEls {
			if ne == nil || i >= len(res) {
				continue
			}
			if res[i].SID != "" {
				ne.Metadata["sid"] = sirena.String{Value: res[i].SID}
			}
			if res[i].PriorSourceRef != "" {
				ne.Metadata["prior_source_ref"] = sirena.String{Value: res[i].PriorSourceRef}
			} else {
				delete(ne.Metadata, "prior_source_ref")
			}
		}
	case os.IsNotExist(rerr):
		// First generation: no prior to reconcile against; EmitSIDs above
		// already gave every element a fresh identity.
	default:
		fmt.Fprintf(stderr, "sirena: emit --update: read %s: %v\n", path, rerr)
		return 1
	}

	out, err := sirena.Print(doc)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// flattenEmitElements returns every Element in doc in a stable order
// (system elements, then boundary children depth-first). Emit documents are
// flat single-system graphs today; the recursion keeps it correct if that
// changes.
func flattenEmitElements(doc *sirena.Document) []*sirena.Element {
	var out []*sirena.Element
	if doc == nil {
		return out
	}
	for _, sys := range doc.Systems {
		out = append(out, sys.Elements...)
		for _, b := range sys.Boundaries {
			out = append(out, flattenBoundaryElements(b)...)
		}
	}
	return out
}

func flattenBoundaryElements(b *sirena.Boundary) []*sirena.Element {
	var out []*sirena.Element
	for _, c := range b.Children {
		switch v := c.(type) {
		case *sirena.Element:
			out = append(out, v)
		case *sirena.Boundary:
			out = append(out, flattenBoundaryElements(v)...)
		}
	}
	return out
}
