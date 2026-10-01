package mermaid

import (
	"bytes"
	"m31labs.dev/sirena"
	"m31labs.dev/sirena/layout"
	"strings"
	"testing"
)

func TestNativeFamilySizeBoundPreservesLegacyRouting(t *testing.T) {
	large := bytes.Repeat([]byte("a"), 4<<20)
	for _, header := range []string{"flowchart LR\n", "graph TD\n"} {
		source := append([]byte(header+"%% "), large...)
		if _, _, err, handled := parseNativeFamilies(source); err != nil || handled {
			t.Fatal("legacy graph intercepted", header, err)
		}
	}
	source := append([]byte("sequenceDiagram\n%% "), large...)
	if _, _, err, handled := parseNativeFamilies(source); err == nil || !handled {
		t.Fatal("native size ceiling bypassed")
	}
}

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
func TestClassMemberSubsetAndRanges(t *testing.T) {
	for _, member := range []string{"garbage", "size", "+size", "+String owner", "+name: string", "+save()", "+get(id: UUID): Item", "+all() Item[]", "+save(id, name: string, UUID key)"} {
		src := []byte("classDiagram\nclass A {\n" + member + "\n}\n")
		doc, diags, err := Parse(src, Options{})
		if err != nil || doc == nil || len(diags) != 0 {
			t.Fatalf("valid member %q rejected: %v %+v", member, err, diags)
		}
	}
	for _, member := range []string{"+save(", "+save())", "+save(!!!)", "+save(id,,key)", "+save(+id)", "<<interface>>", "style A fill:red", "name ??? invalid"} {
		src := []byte("classDiagram\nclass A {\n" + member + "\n}\n")
		doc, diags, err := Parse(src, Options{})
		if err == nil || doc != nil || len(diags) != 1 || string(src[diags[0].Range.Start:diags[0].Range.End]) != member {
			t.Fatalf("unsupported member %q not diagnosed at source: %v %+v", member, err, diags)
		}
	}
}

func TestGeneratedFamilyIdentitiesCannotCollideWithAuthoredNames(t *testing.T) {
	for _, source := range []string{
		"stateDiagram-v2\n[*] --> __sirena_initial\n__sirena_initial --> __sirena_initial_1\n__sirena_initial_1 --> __sirena_final\n__sirena_final --> [*]\n",
		"mindmap\n__sirena_mindmap_2((Root))\n  Plain branch\n",
	} {
		doc, diags, err := Parse([]byte(source), Options{})
		if err != nil || len(diags) != 0 {
			t.Fatal(err, diags)
		}
		rv := sirena.AllElementsView(doc)
		if doc.Diagram == "state" {
			if len(rv.Elements) != 5 || len(rv.Edges) != 4 {
				t.Fatal("pseudo-state merged with authored state")
			}
			if rv.Edges[0].From == rv.Edges[0].To || rv.Edges[3].From == rv.Edges[3].To {
				t.Fatal("distinct transition became self-edge")
			}
		} else if len(rv.Elements) != 2 {
			t.Fatal("anonymous branch collided with authored root")
		}
		for _, element := range rv.Elements {
			if strings.HasPrefix(element.Name, "__sirena_") && !familyName.MatchString(element.Name) {
				t.Fatal("generated identity is not native-printable")
			}
		}
		printed, err := sirena.Print(doc)
		if err != nil {
			t.Fatal(err)
		}
		reparsed, err := sirena.Parse(printed)
		if err != nil || len(reparsed.Diagnostics()) != 0 {
			t.Fatal("generated identities do not round trip", err, string(printed))
		}
		if _, _, err := layout.Render(rv, sirena.RenderOptions{}); err != nil {
			t.Fatal(err)
		}
	}
}
