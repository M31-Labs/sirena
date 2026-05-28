package sirena_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"m31labs.dev/sirena"
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
