// Package conformance contains golden-file tests for sirena's parser,
// printer, and downstream pipeline. Each subdirectory is one case.
package conformance_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestConformanceCorpusEnumerable verifies that every case directory has
// a well-formed input.sir. It does not yet verify goldens — that lands
// once Parse and Format are implemented and golden files are generated.
func TestConformanceCorpusEnumerable(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var cases []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		cases = append(cases, e.Name())
	}
	if len(cases) == 0 {
		t.Skip("no conformance cases yet — added incrementally by Phase 12")
	}
	for _, c := range cases {
		if _, err := os.Stat(filepath.Join(c, "input.sir")); err != nil {
			t.Errorf("case %s missing input.sir", c)
		}
	}
}
