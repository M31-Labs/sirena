package main

import (
	"fmt"
	"io"
)

// printUsage emits the top-level CLI help banner. Kept in its own file
// so the dispatch in main.go stays a thin switch statement and the help
// copy is easy to find and edit independently.
func printUsage(w io.Writer) {
	fmt.Fprintln(w, `sirena - diagram language toolchain

Usage:
  sirena parse [--json] <file>             Parse and dump the IR
  sirena fmt [-w] [--check] <file>...      Format files
  sirena lint <workspace-or-file>          Run lint rules
  sirena render [-o f] [--theme t] <file>  Render a view or system to SVG
  sirena new system|view <name>            Scaffold a new file

See https://github.com/m31labs/sirena for documentation.`)
}
