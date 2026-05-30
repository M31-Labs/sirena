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

// ---- Mermaid extension routing tests (.mmd / .mermaid) ----

func TestRunRender_MmdFile_ProducesWellFormedSVG(t *testing.T) {
	dir := writeWorkspace(t, map[string]string{
		"diagram.mmd": "flowchart LR\n  A[(DB)] -->|reads| B",
	})
	var out, errBuf bytes.Buffer
	code := RunRender([]string{filepath.Join(dir, "diagram.mmd")}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errBuf.String())
	}
	svg := out.String()
	if !strings.Contains(svg, "<svg") || !strings.Contains(svg, "</svg>") {
		t.Errorf("output is not well-formed SVG; got %.80q", svg)
	}
}

func TestRunRender_MermaidExtension_ProducesWellFormedSVG(t *testing.T) {
	dir := writeWorkspace(t, map[string]string{
		"diagram.mermaid": "flowchart LR\n  A --> B",
	})
	var out, errBuf bytes.Buffer
	code := RunRender([]string{filepath.Join(dir, "diagram.mermaid")}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "<svg") {
		t.Errorf("output missing <svg; got %.80q", out.String())
	}
}

func TestRunRender_InferFlag_FlipsCylinderToDatabase(t *testing.T) {
	// A[(DB)] uses cylinder shape — without --infer it stays ElementKindNode,
	// with --infer it promotes to ElementKindDatabase. The SVG output differs
	// because the database shape renders differently.
	dir := writeWorkspace(t, map[string]string{
		"diagram.mmd": "flowchart LR\n  A[(DB)] --> B",
	})

	var outNoInfer, errNoInfer bytes.Buffer
	codeNoInfer := RunRender([]string{filepath.Join(dir, "diagram.mmd")}, &outNoInfer, &errNoInfer)
	if codeNoInfer != 0 {
		t.Fatalf("no-infer exit %d, stderr: %s", codeNoInfer, errNoInfer.String())
	}

	var outInfer, errInfer bytes.Buffer
	codeInfer := RunRender([]string{"--infer", filepath.Join(dir, "diagram.mmd")}, &outInfer, &errInfer)
	if codeInfer != 0 {
		t.Fatalf("--infer exit %d, stderr: %s", codeInfer, errInfer.String())
	}

	// Both must be valid SVG.
	if !strings.Contains(outNoInfer.String(), "<svg") {
		t.Error("no-infer output missing <svg")
	}
	if !strings.Contains(outInfer.String(), "<svg") {
		t.Error("--infer output missing <svg")
	}
	// The SVG bytes must differ because cylinder→database changes the render.
	if outNoInfer.String() == outInfer.String() {
		t.Error("expected different SVG with --infer (cylinder→database shape change)")
	}
}

func TestRunRender_NonGraphMmd_NonZeroExitDiagnosticOnStderr(t *testing.T) {
	dir := writeWorkspace(t, map[string]string{
		"seq.mmd": "sequenceDiagram\n  A->>B: hi",
	})
	var out, errBuf bytes.Buffer
	code := RunRender([]string{filepath.Join(dir, "seq.mmd")}, &out, &errBuf)
	if code == 0 {
		t.Fatalf("expected non-zero exit for non-graph mmd, got 0; stdout: %s", out.String())
	}
	if out.Len() != 0 {
		t.Errorf("no SVG should be written on error; got %q", out.String())
	}
	if !strings.Contains(errBuf.String(), "SIR-MERMAID-NOT-A-GRAPH") {
		t.Errorf("stderr missing SIR-MERMAID-NOT-A-GRAPH; got: %s", errBuf.String())
	}
}

func TestRunRender_FromSirena_OnTxtFile(t *testing.T) {
	// --from sirena lets sirena-syntax content in a .txt file go through the
	// existing .sir render path.
	dir := writeWorkspace(t, map[string]string{
		"sys.txt": "service api\ndatabase db\napi -> db: reads",
	})
	var out, errBuf bytes.Buffer
	code := RunRender([]string{"--from", "sirena", filepath.Join(dir, "sys.txt")}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "<svg") {
		t.Errorf("output missing <svg; got %.80q", out.String())
	}
}

func TestRunRender_UnknownExtension_NoFromFlag_Errors(t *testing.T) {
	dir := writeWorkspace(t, map[string]string{
		"diagram.xyz": "flowchart LR\n  A --> B",
	})
	var out, errBuf bytes.Buffer
	code := RunRender([]string{filepath.Join(dir, "diagram.xyz")}, &out, &errBuf)
	if code == 0 {
		t.Fatalf("expected non-zero exit for unknown extension without --from")
	}
	if !strings.Contains(errBuf.String(), "SIR-RENDER-UNKNOWN-EXT") {
		t.Errorf("stderr missing SIR-RENDER-UNKNOWN-EXT; got: %s", errBuf.String())
	}
}
