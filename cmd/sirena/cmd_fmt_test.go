package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFmt_CheckCleanExits0 confirms that `--check` on an already-formatted
// file exits 0 and prints nothing. The CI workflow gates on this exit code
// to detect drift.
func TestFmt_CheckCleanExits0(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "in.sir")
	if err := os.WriteFile(file, []byte("service api\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	code := RunFmt([]string{"--check", file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code: %d, stderr: %s", code, errBuf.String())
	}
	if out.Len() != 0 {
		t.Errorf("expected empty stdout, got: %s", out.String())
	}
}

// TestFmt_CheckDirtyExits1 covers the inverse: a file whose canonical form
// differs from its on-disk bytes must exit 1 and surface the path on stdout
// so callers can pipe it through xargs to a fixer.
func TestFmt_CheckDirtyExits1(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "in.sir")
	if err := os.WriteFile(file, []byte("service   api  "), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	code := RunFmt([]string{"--check", file}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code: %d, stdout: %s", code, out.String())
	}
	if !strings.Contains(out.String(), file) {
		t.Errorf("expected file path in stdout, got: %s", out.String())
	}
}

// TestFmt_WriteRewrites confirms `-w` rewrites the file in place. The
// canonical shape for "service   api" is "service api\n" — collapsed
// whitespace plus a trailing newline.
func TestFmt_WriteRewrites(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "in.sir")
	if err := os.WriteFile(file, []byte("service   api"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	code := RunFmt([]string{"-w", file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code: %d, stderr: %s", code, errBuf.String())
	}
	formatted, _ := os.ReadFile(file)
	if string(formatted) != "service api\n" {
		t.Errorf("rewrite produced: %q", string(formatted))
	}
}

// TestFmt_DefaultStdout confirms that without `-w` or `--check`, the
// formatted output goes to stdout — the gofmt-style default.
func TestFmt_DefaultStdout(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "in.sir")
	if err := os.WriteFile(file, []byte("service   api"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	code := RunFmt([]string{file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code: %d, stderr: %s", code, errBuf.String())
	}
	if out.String() != "service api\n" {
		t.Errorf("stdout: %q", out.String())
	}
}

// TestFmt_UsageOnNoArgs guards the "missing positional" path.
func TestFmt_UsageOnNoArgs(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := RunFmt([]string{}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code: %d", code)
	}
}
