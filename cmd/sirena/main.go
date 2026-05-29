// Command sirena is the diagram language toolchain entrypoint. It
// dispatches the first positional argument as a subcommand — parse,
// fmt, lint, render, emit, new — into the Run* functions defined in this
// package.
// Each subcommand is a pure (args, stdout, stderr) -> int call so the
// tests can exercise them with bytes.Buffer fixtures without spawning
// processes.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage(os.Stderr)
		os.Exit(2)
	}
	sub := os.Args[1]
	args := os.Args[2:]
	exit := 0
	switch sub {
	case "parse":
		exit = RunParse(args, os.Stdout, os.Stderr)
	case "fmt":
		exit = RunFmt(args, os.Stdout, os.Stderr)
	case "lint":
		exit = RunLint(args, os.Stdout, os.Stderr)
	case "render":
		exit = RunRender(args, os.Stdout, os.Stderr)
	case "emit":
		exit = RunEmit(args, os.Stdout, os.Stderr)
	case "new":
		exit = RunNew(args, os.Stdout, os.Stderr)
	case "-h", "--help", "help":
		printUsage(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", sub)
		printUsage(os.Stderr)
		exit = 2
	}
	os.Exit(exit)
}
