package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"m31labs.dev/sirena"
)

// RunParse implements `sirena parse [--json] <file>`. Without --json it
// prints a one-line summary suitable for human consumption; with --json
// it emits a structured object the conformance corpus generator
// (Phase 12) consumes as a golden.
//
// Exit codes: 0 on success, 1 on I/O or parse error, 2 on misuse.
func RunParse(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("parse", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "emit JSON document + diagnostics")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: sirena parse [--json] <file>")
		return 2
	}
	src, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	doc, err := sirena.Parse(src)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	if !*jsonOut {
		fmt.Fprintf(stdout, "Parsed %d systems, %d imports, %d views; %d diagnostics\n",
			len(doc.Systems), len(doc.Imports), len(doc.Views), len(doc.Diagnostics()))
		return 0
	}

	out := map[string]any{
		"document":    doc,
		"diagnostics": doc.Diagnostics(),
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
