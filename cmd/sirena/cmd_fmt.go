package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"m31labs.dev/sirena"
)

// RunFmt implements `sirena fmt [-w] [--check] <file>...`. Mirrors the
// gofmt UX: default behavior prints the formatted output to stdout, -w
// rewrites in place, --check exits 1 if any input is not already in
// canonical form. The three modes are mutually exclusive in practice —
// passing both -w and --check is allowed by the flag parser but --check
// takes precedence (no rewrite, exit-1 on drift).
//
// Exit codes: 0 on success, 1 if --check finds drift or on I/O error,
// 2 on misuse.
func RunFmt(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("fmt", flag.ContinueOnError)
	fs.SetOutput(stderr)
	write := fs.Bool("w", false, "rewrite file in place")
	check := fs.Bool("check", false, "exit 1 if any file needs formatting")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "usage: sirena fmt [-w] [--check] <file>...")
		return 2
	}

	changed := false
	for _, path := range fs.Args() {
		src, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		formatted, err := sirena.Format(src)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		dirty := string(formatted) != string(src)
		if dirty {
			changed = true
		}
		switch {
		case *check:
			if dirty {
				fmt.Fprintln(stdout, path)
			}
		case *write:
			if dirty {
				if err := os.WriteFile(path, formatted, 0o644); err != nil {
					fmt.Fprintln(stderr, err)
					return 1
				}
			}
		default:
			if _, err := stdout.Write(formatted); err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
		}
	}
	if *check && changed {
		return 1
	}
	return 0
}
