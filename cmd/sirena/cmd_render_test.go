package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeWorkspace(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestRunRender_DefaultsToStdout(t *testing.T) {
	dir := writeWorkspace(t, map[string]string{
		"sys.sir": "service api\ndatabase db\napi -> db: reads",
		"app.view.sir": `view "app" {
  include: [
    service "api",
    database "db"
  ]
}`,
	})
	var out, errBuf bytes.Buffer
	code := RunRender([]string{filepath.Join(dir, "app.view.sir")}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errBuf.String())
	}
	if !strings.HasPrefix(out.String(), "<?xml") {
		t.Errorf("stdout should begin with <?xml; got %.40q", out.String())
	}
}

func TestRunRender_WritesFile(t *testing.T) {
	dir := writeWorkspace(t, map[string]string{"sys.sir": "service api\nservice db\napi -> db"})
	outFile := filepath.Join(dir, "out.svg")
	var out, errBuf bytes.Buffer
	code := RunRender([]string{"-o", outFile, filepath.Join(dir, "sys.sir")}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errBuf.String())
	}
	if out.Len() != 0 {
		t.Errorf("stdout should be empty when -o given; got %q", out.String())
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !bytes.HasPrefix(data, []byte("<?xml")) {
		t.Errorf("output file should be SVG; got %.40q", data)
	}
}

func TestRunRender_ThemeFlag(t *testing.T) {
	dir := writeWorkspace(t, map[string]string{"sys.sir": "service api"})
	sysFile := filepath.Join(dir, "sys.sir")

	var out, errBuf bytes.Buffer
	if code := RunRender([]string{"--theme", "earth-default", sysFile}, &out, &errBuf); code != 0 {
		t.Fatalf("earth-default theme: exit %d, stderr %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "--sirena-bg:") {
		t.Errorf("output missing earth-default tokens")
	}

	out.Reset()
	errBuf.Reset()
	code := RunRender([]string{"--theme", "bogus", sysFile}, &out, &errBuf)
	if code != 2 {
		t.Errorf("unknown theme exit = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "SIR-RENDER-UNKNOWN-THEME") {
		t.Errorf("stderr missing SIR-RENDER-UNKNOWN-THEME: %s", errBuf.String())
	}
}

func TestRunRender_StrictBudget(t *testing.T) {
	dir := writeWorkspace(t, map[string]string{
		"sys.sir": "service api\nservice db",
		"v.view.sir": `view "v" {
  include: [service "api", service "db"]
  budget { nodes: 1 }
}`,
	})
	var out, errBuf bytes.Buffer
	code := RunRender([]string{"--strict-budget", filepath.Join(dir, "v.view.sir")}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("strict budget breach exit = %d, want 1; stderr %s", code, errBuf.String())
	}
	if out.Len() != 0 {
		t.Errorf("no SVG should be emitted on strict breach; got %q", out.String())
	}
	if !strings.Contains(errBuf.String(), "SIR-RENDER-BUDGET-EXCEEDED") {
		t.Errorf("stderr missing SIR-RENDER-BUDGET-EXCEEDED: %s", errBuf.String())
	}
}

func TestRunRender_SystemFile(t *testing.T) {
	dir := writeWorkspace(t, map[string]string{"sys.sir": "service api\ndatabase db\napi -> db: reads"})
	var out, errBuf bytes.Buffer
	code := RunRender([]string{filepath.Join(dir, "sys.sir")}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("system-file render exit %d, stderr: %s", code, errBuf.String())
	}
	if !strings.HasPrefix(out.String(), "<?xml") {
		t.Errorf("stdout should begin with <?xml; got %.40q", out.String())
	}
}
