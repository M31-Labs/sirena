package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"m31labs.dev/sirena"
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

// svcElementSID parses a .gen.sir file and returns the sid metadata of the
// element named "svc", or "" if absent.
func svcElementSID(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	doc, err := sirena.Parse(data)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	for _, sys := range doc.Systems {
		for _, e := range sys.Elements {
			if e.Name == "svc" {
				s, _ := e.Metadata["sid"].(sirena.String)
				return s.Value
			}
		}
	}
	return ""
}

// TestEmit_UpdateFreshWritesIdentity confirms `emit --update <path>` on a
// non-existent path writes a .gen.sir carrying the full round-trip identity
// metadata (sid + source_ref + symbol_kind) for every package.
func TestEmit_UpdateFreshWritesIdentity(t *testing.T) {
	dir := emitFixtureModule(t)
	genPath := filepath.Join(t.TempDir(), "services.gen.sir")

	var out, errBuf bytes.Buffer
	code := RunEmit([]string{"--update", genPath, dir}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit %d; stderr: %s", code, errBuf.String())
	}
	data, err := os.ReadFile(genPath)
	if err != nil {
		t.Fatalf("gen file not written: %v", err)
	}
	s := string(data)
	for _, want := range []string{"sid:", "source_ref:", `symbol_kind: "package"`} {
		if !strings.Contains(s, want) {
			t.Errorf("gen output missing %q:\n%s", want, s)
		}
	}
}

// TestEmit_UpdateRenameCarriesIdentity is the moat test: regenerating against
// a prior .gen.sir after a module rename must carry each package's sid forward
// (stable identity) and record the old source_ref in prior_source_ref.
func TestEmit_UpdateRenameCarriesIdentity(t *testing.T) {
	dir := t.TempDir()
	genPath := filepath.Join(t.TempDir(), "services.gen.sir")

	writeModule := func(module string) {
		files := map[string]string{
			"go.mod":     "module " + module + "\n\ngo 1.26\n",
			"main.go":    "package main\n\nimport _ \"" + module + "/svc\"\n\nfunc main() {}\n",
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
	}

	// First emission under the original module path.
	writeModule("example.com/demo")
	var out, errBuf bytes.Buffer
	if code := RunEmit([]string{"--update", genPath, dir}, &out, &errBuf); code != 0 {
		t.Fatalf("first emit --update exit %d: %s", code, errBuf.String())
	}
	sid1 := svcElementSID(t, genPath)
	if sid1 == "" {
		t.Fatal("svc has no sid after first emit")
	}

	// Rename the module; the svc package's source_ref changes but its
	// identity should survive via the fuzzy (symbol_kind + name-suffix) match.
	writeModule("example.com/renamed")
	out.Reset()
	errBuf.Reset()
	if code := RunEmit([]string{"--update", genPath, dir}, &out, &errBuf); code != 0 {
		t.Fatalf("second emit --update exit %d: %s", code, errBuf.String())
	}
	sid2 := svcElementSID(t, genPath)

	if sid2 != sid1 {
		t.Errorf("svc sid changed across module rename: %q -> %q (identity not carried)", sid1, sid2)
	}
	data, _ := os.ReadFile(genPath)
	if !strings.Contains(string(data), `prior_source_ref: "code://go/example.com/demo#svc"`) {
		t.Errorf("expected prior_source_ref recording the rename; got:\n%s", data)
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
