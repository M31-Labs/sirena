// Package sirena_test contains the golden-file conformance corpus runner
// for sirena's parser, printer, and downstream pipeline. Each subdirectory
// under testdata/conformance/ is one case.
package sirena_test

import (
	"bytes"
	"encoding/json"
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

// TestConformance_IRGoldenPerCase asserts that for every case directory
// under testdata/conformance/, the JSON encoding of Parse(input.sir) +
// Diagnostics() matches ir.golden.json byte-for-byte. The encoding
// shape mirrors `sirena parse --json` (cmd/sirena/cmd_parse.go) so the
// regen script and the runner stay aligned.
//
// Cases without an ir.golden.json yet are silently skipped, mirroring
// the format-determinism runner's tolerance during corpus build-out.
func TestConformance_IRGoldenPerCase(t *testing.T) {
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
		goldenPath := filepath.Join(caseDir, "ir.golden.json")

		input, err := os.ReadFile(inputPath)
		if err != nil {
			t.Errorf("%s: read input.sir: %v", e.Name(), err)
			continue
		}
		golden, err := os.ReadFile(goldenPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			t.Errorf("%s: read ir.golden.json: %v", e.Name(), err)
			continue
		}
		doc, err := sirena.Parse(input)
		if err != nil {
			t.Errorf("%s: Parse: %v", e.Name(), err)
			continue
		}
		out := map[string]any{
			"document":    doc,
			"diagnostics": doc.Diagnostics(),
		}
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			t.Errorf("%s: encode: %v", e.Name(), err)
			continue
		}
		if !bytes.Equal(buf.Bytes(), golden) {
			t.Errorf("%s: IR JSON differs from ir.golden.json\n--- got ---\n%s\n--- want ---\n%s",
				e.Name(), buf.String(), string(golden))
		}
		checked++
	}

	if checked == 0 {
		t.Skip("no conformance cases with ir.golden.json yet — corpus is filled in Phase 12")
	}
}
