package sirena_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"m31labs.dev/sirena"
	"m31labs.dev/sirena/ingest/mermaid"
)

// TestRoundTrip_Law exercises the spec's round-trip law: for a representative
// set of inputs, Parse(Print(Parse(input))) yields a Document structurally
// equal to Parse(input). Equality compares all fields EXCEPT Range, since
// reprinting shifts byte offsets but should preserve structure.
func TestRoundTrip_Law(t *testing.T) {
	samples := []struct {
		name string
		src  string
	}{
		{
			"element-with-metadata",
			`service api {
  label: "API Service"
  owner: "team:platform"
  tags: [stateless, edge]
}`,
		},
		{
			"boundary-with-children-and-edge",
			`boundary trust "pci" {
  service api
  database db
  api -> db: writes
}`,
		},
		{
			"edges-all-directions-and-kinds",
			`api -> db: reads
worker <- api
peer <-> mirror
api -> queue: publishes
queue -> consumer: subscribes`,
		},
		{
			"imports-and-qualified-edges",
			`import "../shared/platform.sir"

service api
api -> platform.kafka: publishes`,
		},
		{
			"view-with-full-coverage",
			`view "data-plane" {
  title: "Data Plane"
  preset: sociotechnical
  include: [
    boundary "pci"
    service "auth-svc"
  ]
  expand: ["pci"]
  collapse: ["legacy-batch"]
  theme: earth-default
  layout {
    direction: top-down
    pin: { "api": top }
  }
  budget {
    nodes: 30
    edges_per_node: 6
    depth: 4
  }
}`,
		},
		{
			"override-forms",
			`service api @override
database db @override(label, tags) {
  label: "Replaced"
  tags: [hand-curated]
}
gateway gw @override(except: source_ref) {
  label: "Curated"
}`,
		},
	}

	for _, s := range samples {
		t.Run(s.name, func(t *testing.T) {
			doc1 := sirena.MustParse([]byte(s.src))
			printed, err := sirena.Print(doc1)
			if err != nil {
				t.Fatalf("Print: %v", err)
			}
			doc2, err := sirena.Parse(printed)
			if err != nil {
				t.Fatalf("re-Parse failed:\noutput was:\n%s\nerror: %v", string(printed), err)
			}
			if diff := structurallyEqual(doc1, doc2); diff != "" {
				t.Errorf("round-trip not structurally equal:\n%s\nprinted output:\n%s", diff, string(printed))
			}
		})
	}
}

// TestRoundTrip_GroupBoundary verifies two properties for the Phase H
// `group` boundary keyword:
//
//	a. Parse of a literal "boundary group" source produces BoundaryKindGroup
//	   with no diagnostics.
//	b. A Mermaid subgraph (which lowers to BoundaryKindGroup) survives a
//	   full Print→Parse round-trip with the group kind preserved.
func TestRoundTrip_GroupBoundary(t *testing.T) {
	t.Run("parse-group-keyword", func(t *testing.T) {
		src := []byte("boundary group \"net\" {}\n")
		doc, err := sirena.Parse(src)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if diags := doc.Diagnostics(); len(diags) != 0 {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		if len(doc.Systems) == 0 || len(doc.Systems[0].Boundaries) == 0 {
			t.Fatal("expected one boundary in parsed document")
		}
		b := doc.Systems[0].Boundaries[0]
		if b.Kind != sirena.BoundaryKindGroup {
			t.Errorf("boundary Kind = %v, want BoundaryKindGroup", b.Kind)
		}
		if b.Name != "net" {
			t.Errorf("boundary Name = %q, want %q", b.Name, "net")
		}
	})

	t.Run("mermaid-subgraph-roundtrip", func(t *testing.T) {
		// A Mermaid subgraph lowers to a BoundaryKindGroup boundary.
		mermaidSrc := []byte("flowchart TD\n  subgraph payments\n    svc[Payment Service]\n  end\n")
		doc1, diags, err := mermaid.Parse(mermaidSrc, mermaid.Options{})
		if err != nil {
			t.Fatalf("mermaid.Parse: %v", err)
		}
		if len(diags) != 0 {
			t.Logf("mermaid diagnostics (informational): %v", diags)
		}
		if doc1 == nil {
			t.Fatal("mermaid.Parse returned nil document")
		}

		// Confirm the lowered IR has a group boundary.
		if len(doc1.Systems) == 0 || len(doc1.Systems[0].Boundaries) == 0 {
			t.Fatal("expected at least one boundary from mermaid subgraph")
		}
		if doc1.Systems[0].Boundaries[0].Kind != sirena.BoundaryKindGroup {
			t.Errorf("lowered boundary Kind = %v, want BoundaryKindGroup",
				doc1.Systems[0].Boundaries[0].Kind)
		}

		// Print the IR to .sir and re-parse it.
		printed, err := sirena.Print(doc1)
		if err != nil {
			t.Fatalf("Print: %v", err)
		}
		doc2, err := sirena.Parse(printed)
		if err != nil {
			t.Fatalf("re-Parse failed:\nprinted output:\n%s\nerror: %v", string(printed), err)
		}

		// The group boundary must survive the round-trip.
		if len(doc2.Systems) == 0 || len(doc2.Systems[0].Boundaries) == 0 {
			t.Fatalf("re-parsed document has no boundaries; printed:\n%s", string(printed))
		}
		if doc2.Systems[0].Boundaries[0].Kind != sirena.BoundaryKindGroup {
			t.Errorf("round-tripped boundary Kind = %v, want BoundaryKindGroup",
				doc2.Systems[0].Boundaries[0].Kind)
		}
	})
}

// structurallyEqual returns "" when a and b are equal ignoring all Range
// fields (which legitimately shift across a reprint) and any unexported
// fields on Document (parseDiagnostics, which the parser owns and which
// callers cannot construct). Otherwise it returns a human-readable diff
// produced by go-cmp.
func structurallyEqual(a, b *sirena.Document) string {
	return cmp.Diff(a, b,
		cmpopts.IgnoreTypes(sirena.Range{}),
		cmpopts.IgnoreUnexported(sirena.Document{}),
	)
}
