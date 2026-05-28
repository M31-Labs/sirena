package sirena_test

import (
	"testing"

	"m31labs.dev/sirena"
)

// TestParseManifest_FullCoverage parses a manifest exercising every
// recognized field (DefaultPreset, Theme, OutputDir, Layout.Direction) and
// verifies each one round-trips into the public Manifest struct.
func TestParseManifest_FullCoverage(t *testing.T) {
	src := []byte(`default_preset: default
theme: earth-default
output_dir: dist/diagrams
layout:
  direction: top-down
`)
	m, err := sirena.ParseManifest(src)
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if m == nil {
		t.Fatal("ParseManifest returned nil manifest")
	}
	if m.DefaultPreset != "default" {
		t.Errorf("DefaultPreset = %q, want %q", m.DefaultPreset, "default")
	}
	if m.Theme != "earth-default" {
		t.Errorf("Theme = %q, want %q", m.Theme, "earth-default")
	}
	if m.OutputDir != "dist/diagrams" {
		t.Errorf("OutputDir = %q, want %q", m.OutputDir, "dist/diagrams")
	}
	if m.Layout == nil {
		t.Fatal("Layout nil, want non-nil")
	}
	if m.Layout.Direction != "top-down" {
		t.Errorf("Layout.Direction = %q, want %q", m.Layout.Direction, "top-down")
	}
}

// TestParseManifest_Empty asserts that a nil / empty input yields a
// zero-value *Manifest (not a nil pointer, not an error). Workspace
// loading code should be able to call ParseManifest on a missing file
// after reading it as empty bytes without special-casing.
func TestParseManifest_Empty(t *testing.T) {
	m, err := sirena.ParseManifest(nil)
	if err != nil {
		t.Fatalf("ParseManifest(nil): %v", err)
	}
	if m == nil {
		t.Fatal("zero manifest should be non-nil")
	}
	if m.DefaultPreset != "" || m.Theme != "" || m.OutputDir != "" {
		t.Errorf("zero manifest has non-zero scalar field(s): %+v", m)
	}
	if m.Layout != nil {
		t.Errorf("zero manifest has non-nil Layout: %+v", m.Layout)
	}
}

// TestParseManifest_PartialFields verifies that omitted fields stay at
// their zero values (empty strings, nil Layout) so workspace defaults
// can layer cleanly on top of partial manifests.
func TestParseManifest_PartialFields(t *testing.T) {
	src := []byte(`theme: earth-default
`)
	m, err := sirena.ParseManifest(src)
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if m.Theme != "earth-default" {
		t.Errorf("Theme = %q, want %q", m.Theme, "earth-default")
	}
	if m.DefaultPreset != "" {
		t.Errorf("DefaultPreset should be empty, got %q", m.DefaultPreset)
	}
	if m.OutputDir != "" {
		t.Errorf("OutputDir should be empty, got %q", m.OutputDir)
	}
	if m.Layout != nil {
		t.Errorf("Layout should be nil, got %+v", m.Layout)
	}
}

// TestParseManifest_MalformedYAML asserts that syntactically broken input
// surfaces as a non-nil error rather than silently producing a zero
// manifest. The exact error text is yaml.v3's; we only check non-nil.
func TestParseManifest_MalformedYAML(t *testing.T) {
	src := []byte(`theme: "unterminated`)
	_, err := sirena.ParseManifest(src)
	if err == nil {
		t.Fatal("expected error for malformed YAML, got nil")
	}
}
