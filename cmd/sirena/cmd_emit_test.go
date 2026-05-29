package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// emitFixtureModule writes a minimal two-package Go module and returns its
// root: a `main` package importing ./svc, so emit produces a gateway, a
// service, and one depends_on edge.
func emitFixtureModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":     "module example.com/demo\n\ngo 1.26\n",
		"main.go":    "package main\n\nimport _ \"example.com/demo/svc\"\n\nfunc main() {}\n",
		"svc/svc.go": "package svc\n\nfunc Serve() {}\n",
	}
	for rel, body := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestEmit_SirFormat covers the default `sirena emit <dir>` shape: valid
// sirena source on stdout naming both packages and the depends_on edge.
func TestEmit_SirFormat(t *testing.T) {
	dir := emitFixtureModule(t)
	var out, errBuf bytes.Buffer
	code := RunEmit([]string{dir}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code: %d, stderr: %s", code, errBuf.String())
	}
	s := out.String()
	for _, want := range []string{"gateway demo", "service svc", "depends_on"} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q:\n%s", want, s)
		}
	}
}

// TestEmit_SvgFormat confirms `--format svg` emits an SVG document.
func TestEmit_SvgFormat(t *testing.T) {
	dir := emitFixtureModule(t)
	var out, errBuf bytes.Buffer
	code := RunEmit([]string{"--format", "svg", dir}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code: %d, stderr: %s", code, errBuf.String())
	}
	if !strings.HasPrefix(out.String(), "<?xml") || !strings.Contains(out.String(), "<svg") {
		t.Errorf("expected SVG output, got:\n%s", out.String()[:min(120, out.Len())])
	}
}

// TestEmit_UnknownFormatExits2 covers the misuse path for a bad --format.
func TestEmit_UnknownFormatExits2(t *testing.T) {
	dir := emitFixtureModule(t)
	var out, errBuf bytes.Buffer
	code := RunEmit([]string{"--format", "png", dir}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code: %d, want 2; stderr: %s", code, errBuf.String())
	}
}

// TestEmit_NoArgsUsageExits2 ensures a missing directory prints usage and
// exits 2.
func TestEmit_NoArgsUsageExits2(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := RunEmit([]string{}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code: %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "usage:") {
		t.Errorf("stderr: %s", errBuf.String())
	}
}

// TestEmit_NoModuleExits1 covers the error path when the target has no go.mod.
func TestEmit_NoModuleExits1(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := RunEmit([]string{t.TempDir()}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code: %d, want 1; stderr: %s", code, errBuf.String())
	}
}
