package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// chdirTo cd's into dir for the test and restores the previous cwd via
// t.Cleanup. The CLI's `new` subcommand writes relative paths
// (`services/<name>.sir`), so tests need a sandboxed working directory.
func chdirTo(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}

// TestNew_System covers the `new system <name>` happy path. The scaffolded
// file must live at services/<name>.sir and contain a `service <name>`
// declaration so it parses cleanly with no further edits.
func TestNew_System(t *testing.T) {
	chdirTo(t, t.TempDir())
	var out, errBuf bytes.Buffer
	code := RunNew([]string{"system", "payments"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code: %d, stderr: %s", code, errBuf.String())
	}
	path := filepath.Join("services", "payments.sir")
	body, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read scaffolded file: %v", readErr)
	}
	if !strings.Contains(string(body), "service payments") {
		t.Errorf("scaffold content: %s", string(body))
	}
}

// TestNew_View covers `new view <name>` — output lives under views/ with
// the .view.sir double extension that signals "view file" to the workspace
// loader.
func TestNew_View(t *testing.T) {
	chdirTo(t, t.TempDir())
	var out, errBuf bytes.Buffer
	code := RunNew([]string{"view", "data-plane"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code: %d, stderr: %s", code, errBuf.String())
	}
	path := filepath.Join("views", "data-plane.view.sir")
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), `view "data-plane"`) {
		t.Errorf("scaffold content: %s", string(body))
	}
}

// TestNew_RejectsBadKind — anything that isn't system|view should fail
// with the usage exit code so authors get an immediate, parseable error.
func TestNew_RejectsBadKind(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := RunNew([]string{"banana", "x"}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code: %d", code)
	}
}

// TestNew_RefusesOverwrite guards against clobbering existing files —
// the scaffold path must be empty before write, otherwise users could
// lose hand-edited content to a stray `sirena new`.
func TestNew_RefusesOverwrite(t *testing.T) {
	chdirTo(t, t.TempDir())
	if err := os.MkdirAll("services", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("services", "payments.sir"), []byte("# existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	code := RunNew([]string{"system", "payments"}, &out, &errBuf)
	if code != 1 {
		t.Errorf("expected exit 1 on clobber attempt, got %d", code)
	}
	body, _ := os.ReadFile(filepath.Join("services", "payments.sir"))
	if string(body) != "# existing\n" {
		t.Errorf("scaffolder clobbered existing file: %q", string(body))
	}
}
