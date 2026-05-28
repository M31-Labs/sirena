package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParse_TextMode covers the default `sirena parse <file>` shape:
// a one-line summary on stdout, exit code 0, and no stderr noise on the
// happy path. The summary's exact phrasing is part of the CLI contract —
// downstream scripts grep for "Parsed N systems" to confirm the parser
// saw what they expected.
func TestParse_TextMode(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "in.sir")
	if err := os.WriteFile(file, []byte("service api"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	code := RunParse([]string{file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code: %d, stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "Parsed 1 systems") {
		t.Errorf("stdout: %s", out.String())
	}
}

// TestParse_JSONMode confirms `--json` switches the output to a JSON object
// with top-level "document" and "diagnostics" keys. The conformance corpus
// generator (Phase 12) consumes this mode to produce goldens, so the shape
// is load-bearing.
func TestParse_JSONMode(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "in.sir")
	if err := os.WriteFile(file, []byte("service api"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	code := RunParse([]string{"--json", file}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code: %d, stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), `"document"`) || !strings.Contains(out.String(), `"diagnostics"`) {
		t.Errorf("stdout missing keys: %s", out.String())
	}
}

// TestParse_MissingFileExits1 covers the I/O error path: a missing input
// file should surface the OS error on stderr and exit 1, not panic or
// print an empty document.
func TestParse_MissingFileExits1(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := RunParse([]string{filepath.Join(t.TempDir(), "nope.sir")}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code: %d, stderr: %s", code, errBuf.String())
	}
}

// TestParse_UsageOnNoArgs ensures the usage banner exits 2 (the conventional
// "command-line misuse" code) when the file argument is missing.
func TestParse_UsageOnNoArgs(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := RunParse([]string{}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code: %d", code)
	}
	if !strings.Contains(errBuf.String(), "usage:") {
		t.Errorf("stderr: %s", errBuf.String())
	}
}
