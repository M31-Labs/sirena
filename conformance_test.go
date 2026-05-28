// Package sirena_test contains the golden-file conformance corpus runner
// for sirena's parser, printer, and downstream pipeline. Each subdirectory
// under testdata/conformance/ is one case.
package sirena_test

import (
	"os"
	"path/filepath"
	"testing"

	"m31labs.dev/sirena"
)

const corpusDir = "testdata/conformance"

// TestConformanceCorpusEnumerable verifies that every case directory has
// a well-formed input.sir. It does not yet verify goldens — that lands
// once Parse and Format are implemented and golden files are generated.
func TestConformanceCorpusEnumerable(t *testing.T) {
	entries, err := os.ReadDir(corpusDir)
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
		if _, err := os.Stat(filepath.Join(corpusDir, c, "input.sir")); err != nil {
			t.Errorf("case %s missing input.sir", c)
		}
	}
}

// TestConformance_FormatDeterminism asserts that Format(input.sir) matches
// fmt.golden.sir byte-for-byte for every case in the corpus. The full check
// activates in Phase 12 when goldens are authored; today the test SKIPs
// because no case has a fmt.golden.sir yet.
func TestConformance_FormatDeterminism(t *testing.T) {
	entries, err := os.ReadDir(corpusDir)
	if err != nil {
		t.Fatal(err)
	}

	checked := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		caseDir := filepath.Join(corpusDir, e.Name())
		inputPath := filepath.Join(caseDir, "input.sir")
		goldenPath := filepath.Join(caseDir, "fmt.golden.sir")

		input, err := os.ReadFile(inputPath)
		if err != nil {
			t.Errorf("%s: read input.sir: %v", e.Name(), err)
			continue
		}
		golden, err := os.ReadFile(goldenPath)
		if err != nil {
			if os.IsNotExist(err) {
				// Corpus case without a golden yet — skip until Phase 12 lands.
				continue
			}
			t.Errorf("%s: read fmt.golden.sir: %v", e.Name(), err)
			continue
		}
		out, err := sirena.Format(input)
		if err != nil {
			t.Errorf("%s: Format: %v", e.Name(), err)
			continue
		}
		if string(out) != string(golden) {
			t.Errorf("%s: Format output differs from fmt.golden.sir", e.Name())
		}
		checked++
	}

	if checked == 0 {
		t.Skip("no conformance cases with fmt.golden.sir yet — corpus is filled in Phase 12")
	}
}
