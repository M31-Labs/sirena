package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLint_Workspace covers the workspace branch: pointing `sirena lint`
// at a directory triggers OpenWorkspace + lint.Run. The fixture wires an
// edge to a nonexistent target, which produces SIR-REF-UNRESOLVED at
// error severity — the CLI must exit 1 and surface the code on stdout
// so CI greps can pin specific rules.
func TestLint_Workspace(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "x.sir"), []byte("service api\napi -> ghost: calls"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	code := RunLint([]string{root}, &out, &errBuf)
	if code != 1 {
		t.Errorf("expected exit 1 on error diagnostic, got %d; stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "SIR-REF-UNRESOLVED") {
		t.Errorf("expected SIR-REF-UNRESOLVED: %s", out.String())
	}
}

// TestLint_CleanWorkspace confirms exit 0 when the workspace has no
// error-severity diagnostics. A single isolated service is well-formed.
func TestLint_CleanWorkspace(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "x.sir"), []byte("service api"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	code := RunLint([]string{root}, &out, &errBuf)
	if code != 0 {
		t.Errorf("expected exit 0 on clean workspace, got %d; stdout: %s stderr: %s",
			code, out.String(), errBuf.String())
	}
}

// TestLint_SingleFile covers the file branch — when the target is not a
// directory, the CLI falls back to Document.Diagnostics so authors can
// lint an isolated source without scaffolding a workspace.
func TestLint_SingleFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "in.sir")
	if err := os.WriteFile(file, []byte("service api"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	code := RunLint([]string{file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("expected exit 0, got %d; stderr: %s", code, errBuf.String())
	}
}

// TestLint_MissingTargetExits1 — the stat error path.
func TestLint_MissingTargetExits1(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := RunLint([]string{filepath.Join(t.TempDir(), "nope")}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code: %d", code)
	}
}

// TestLint_UsageOnNoArgs guards the misuse path.
func TestLint_UsageOnNoArgs(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := RunLint([]string{}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code: %d", code)
	}
}
