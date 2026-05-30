package mermaid

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"m31labs.dev/sirena"
)

// -update regenerates all construct goldens.
var updateGoldens = flag.Bool("update", false, "regenerate construct goldens")

// irProjection is a stable, Range-insensitive JSON projection of a Document.
// It captures exactly what the mapping table produces: element names/kinds/
// labels/shapes, edges from/to/kind/label, and boundaries name/kind/label/
// child names.
type irProjection struct {
	Elements   []elemProj     `json:"elements,omitempty"`
	Edges      []edgeProj     `json:"edges,omitempty"`
	Boundaries []boundaryProj `json:"boundaries,omitempty"`
}

type elemProj struct {
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	Label string `json:"label,omitempty"`
	Shape string `json:"shape,omitempty"`
}

type edgeProj struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Kind  string `json:"kind"`
	Label string `json:"label,omitempty"`
}

type boundaryProj struct {
	Name     string          `json:"name"`
	Kind     string          `json:"kind"`
	Label    string          `json:"label,omitempty"`
	Children []boundaryChild `json:"children,omitempty"`
}

// boundaryChild holds either an element or a nested boundary.
type boundaryChild struct {
	Type     string          `json:"type"` // "element" or "boundary"
	Name     string          `json:"name"`
	Kind     string          `json:"kind,omitempty"`
	Label    string          `json:"label,omitempty"`
	Shape    string          `json:"shape,omitempty"`
	Children []boundaryChild `json:"children,omitempty"`
}

// projectDoc converts a *sirena.Document to an irProjection.
func projectDoc(doc *sirena.Document) irProjection {
	var proj irProjection
	for _, sys := range doc.Systems {
		for _, e := range sys.Elements {
			proj.Elements = append(proj.Elements, projectElem(e))
		}
		for _, b := range sys.Boundaries {
			proj.Boundaries = append(proj.Boundaries, projectBoundary(b))
		}
		for _, edge := range sys.Edges {
			proj.Edges = append(proj.Edges, edgeProj{
				From:  edge.From,
				To:    edge.To,
				Kind:  edge.Kind.String(),
				Label: edge.Label,
			})
		}
	}
	return proj
}

func projectElem(e *sirena.Element) elemProj {
	p := elemProj{
		Name: e.Name,
		Kind: e.Kind.String(),
	}
	if v, ok := e.Metadata["label"]; ok {
		if sv, ok := v.(sirena.String); ok {
			p.Label = sv.Value
		}
	}
	if v, ok := e.Metadata["shape"]; ok {
		if iv, ok := v.(sirena.Ident); ok {
			p.Shape = iv.Value
		}
	}
	return p
}

func projectBoundary(b *sirena.Boundary) boundaryProj {
	bp := boundaryProj{
		Name: b.Name,
		Kind: b.Kind.String(),
	}
	if v, ok := b.Metadata["label"]; ok {
		if sv, ok := v.(sirena.String); ok {
			bp.Label = sv.Value
		}
	}
	for _, child := range b.Children {
		bp.Children = append(bp.Children, projectBoundaryChild(child))
	}
	return bp
}

func projectBoundaryChild(n sirena.Node) boundaryChild {
	switch v := n.(type) {
	case *sirena.Element:
		ep := projectElem(v)
		return boundaryChild{
			Type:  "element",
			Name:  ep.Name,
			Kind:  ep.Kind,
			Label: ep.Label,
			Shape: ep.Shape,
		}
	case *sirena.Boundary:
		bp := projectBoundary(v)
		bc := boundaryChild{
			Type:     "boundary",
			Name:     bp.Name,
			Kind:     bp.Kind,
			Label:    bp.Label,
			Children: make([]boundaryChild, 0),
		}
		for _, cc := range bp.Children {
			bc.Children = append(bc.Children, cc)
		}
		return bc
	default:
		return boundaryChild{Type: "unknown"}
	}
}

// marshalGolden returns deterministic indented JSON bytes.
func marshalGolden(proj irProjection) ([]byte, error) {
	b, err := json.MarshalIndent(proj, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// TestConstruct_IRGoldens runs the per-construct golden tests.
// Each *.mmd file in testdata/construct/ has a corresponding *.ir.json golden.
// Run with -update to regenerate goldens.
func TestConstruct_IRGoldens(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "construct", "*.mmd"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no construct files found under testdata/construct/")
	}

	for _, f := range files {
		f := f
		base := strings.TrimSuffix(filepath.Base(f), ".mmd")
		t.Run(base, func(t *testing.T) {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatalf("read %s: %v", f, err)
			}

			doc, diags, err := Parse(src, Options{})
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			// Require no fatal diagnostics.
			for _, d := range diags {
				if d.Code == "SIR-MERMAID-NOT-A-GRAPH" || d.Code == "SIR-MERMAID-PARSE" {
					t.Errorf("unexpected fatal diagnostic: %s: %s", d.Code, d.Message)
				}
			}
			if doc == nil {
				t.Fatal("Parse returned nil doc")
			}

			proj := projectDoc(doc)
			got, err := marshalGolden(proj)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			goldenPath := filepath.Join("testdata", "construct", base+".ir.json")

			if *updateGoldens {
				if werr := os.WriteFile(goldenPath, got, 0o644); werr != nil {
					t.Fatalf("write golden: %v", werr)
				}
				t.Logf("updated golden: %s", goldenPath)
				return
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden %s: %v (run with -update to generate)", goldenPath, err)
			}

			if string(got) != string(want) {
				t.Errorf("golden mismatch for %s:\ngot:\n%s\nwant:\n%s", base, got, want)
			}
		})
	}
}
