package mermaid

import (
	"m31labs.dev/sirena"
	"m31labs.dev/sirena/layout"
	"strings"
	"testing"
)

func TestNativeFamilySemantics(t *testing.T) {
	cases := []struct {
		source, kind string
		nodes, edges int
	}{{"sequenceDiagram\nparticipant A as Browser\nA->>B: Request\nB-->>A: Response\n", "sequence", 2, 2}, {"stateDiagram-v2\nstate \"Ready to build\" as Ready\n[*] --> Ready\nReady --> [*]: stop\n", "state", 3, 2}, {"classDiagram\nclass A {\n+name: string\n+save()\n}\nA \"1\" --> \"many\" B: uses\n", "class", 2, 1}, {"mindmap\nroot((Plan))\n  a[Build]\n    b(Test)\n  c[Ship]\n", "mindmap", 4, 3}}
	for _, c := range cases {
		doc, diags, err := Parse([]byte(c.source), Options{})
		if err != nil || len(diags) != 0 {
			t.Fatalf("%s: %v %+v", c.kind, err, diags)
		}
		rv := sirena.AllElementsView(doc)
		if sirena.DiagramName(rv) != c.kind || len(rv.Elements) != c.nodes || len(rv.Edges) != c.edges {
			t.Fatal("family, actors or relationships lost")
		}
		if _, _, err := layout.Render(rv, sirena.RenderOptions{}); err != nil {
			t.Fatal(err)
		}
		if c.kind == "sequence" && (rv.Elements[0].DisplayLabel() != "Browser" || rv.Edges[1].Label != "Response" || rv.Edges[1].Metadata["arrow"].(sirena.String).Value != "-->>") {
			t.Fatal("message semantics lost")
		}
		if c.kind == "class" && (!strings.Contains(rv.Elements[0].Metadata["fields"].(sirena.String).Value, "name") || !strings.Contains(rv.Edges[0].Label, "many")) {
			t.Fatal("class data lost")
		}
	}
}
func TestUnsupportedFamilySyntaxIsDiagnosedAtSource(t *testing.T) {
	src := []byte("%% test\nsequenceDiagram\n A->>B: hi\n loop retry\n end\n")
	doc, diags, err := Parse(src, Options{})
	if err == nil || doc != nil || len(diags) != 1 || diags[0].Code != "SIR-MERMAID-UNSUPPORTED" || !strings.Contains(string(src[diags[0].Range.Start:diags[0].Range.End]), "loop retry") {
		t.Fatalf("invalid syntax silently lost: %v %+v", err, diags)
	}
}
