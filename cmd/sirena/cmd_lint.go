package main

import (
	"fmt"
	"io"
	"os"

	"m31labs.dev/sirena"
	"m31labs.dev/sirena/lint"
)

// RunLint implements `sirena lint <workspace-or-file>`. When the target
// is a directory, the workspace loader runs the full lint pack (refs,
// kinds, overrides, versions, dead-element). For a single file we fall
// back to Document.Diagnostics — the doc-scoped subset — since the
// workspace-level rules need a corpus to compare against.
//
// Diagnostics are emitted as `<target>[<start>:<end>] <severity> <code>: <message>`
// — byte offsets in lieu of line:col since the v0.1 diagnostic shape
// only pins findings to byte spans. The line:col upgrade is a Phase 13
// concern.
//
// Exit codes: 0 if all diagnostics are non-error severity (or none at
// all), 1 if any error-severity diagnostic surfaces or on I/O error,
// 2 on misuse.
func RunLint(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: sirena lint <workspace-or-file>")
		return 2
	}
	target := args[0]
	info, err := os.Stat(target)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	var diags []sirena.Diagnostic
	if info.IsDir() {
		ws, err := sirena.OpenWorkspace(target)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		diags = lint.Run(ws)
	} else {
		src, err := os.ReadFile(target)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		doc, err := sirena.Parse(src)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		diags = doc.Diagnostics()
	}

	for _, d := range diags {
		var loc string
		if d.Range.Start > 0 || d.Range.End > 0 {
			loc = fmt.Sprintf("[%d:%d]", d.Range.Start, d.Range.End)
		}
		fmt.Fprintf(stdout, "%s%s %s %s: %s\n",
			target, loc, d.Severity.String(), d.Code, d.Message)
	}

	for _, d := range diags {
		if d.Severity == sirena.SeverityError {
			return 1
		}
	}
	return 0
}
