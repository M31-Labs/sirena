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
  sirena render --scene3d [--shader f.sel] [--targets ids] [--motion-style spin|float] [--tour nodes|relationships] [-o scene.json] <file>
                                         Export a native GoSX scene with optional material and keyframes
  sirena parse [--json] <file>             Parse and dump the IR
  sirena fmt [-w] [--check] <file>...      Format files
  sirena lint <workspace-or-file>          Run lint rules
  sirena render [--diagram architecture|sequence|radial] [-o f] [--theme t] <file>
                                         Render a view or system to SVG
  sirena bench [--runs 7] file.sir|file.mmd
                                         Measure warm parse/layout/SVG bytes and allocations
  sirena storyboard --diagram bar --out frames before.sir after.sir
                                         Export stable SVG states and a frame manifest
  sirena render --help                    Show all render controls and examples
  sirena bake [--theme t] [--infer] <md>...  Bake diagram fences in markdown to SVG
  sirena emit [--format sir|svg] <go-dir>  Emit a Go module's package graph
  sirena new system|view <name>            Scaffold a new file

See https://github.com/m31labs/sirena for documentation.`)
}
