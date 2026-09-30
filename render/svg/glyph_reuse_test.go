package svg_test

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"testing"

	"m31labs.dev/sirena"
	"m31labs.dev/sirena/layout"
	"m31labs.dev/sirena/render/svg"
)

func TestGlyphReuseResolvesAndReducesRepeatedText(t *testing.T) {
	rv := &sirena.ResolvedView{}
	for i := 0; i < 100; i++ {
		rv.Elements = append(rv.Elements, &sirena.Element{Name: fmt.Sprintf("n%d", i), Metadata: map[string]sirena.Value{"label": sirena.String{Value: "Processing service WWW"}}})
	}
	lr, _, err := layout.Render(rv, sirena.RenderOptions{Diagram: "class"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := svg.Render(lr, nil)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	var refs []string
	d := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if start, ok := tok.(xml.StartElement); ok {
			for _, a := range start.Attr {
				if a.Name.Local == "id" {
					if ids[a.Value] {
						t.Fatal("duplicate glyph definition")
					}
					ids[a.Value] = true
				}
				if a.Name.Local == "href" {
					refs = append(refs, strings.TrimPrefix(a.Value, "#"))
				}
			}
		}
	}
	for _, ref := range refs {
		if !ids[ref] {
			t.Fatalf("unresolved %s", ref)
		}
	}
	if len(refs) < 1000 || len(ids) > 30 {
		t.Fatalf("glyphs not reused: %d uses, %d definitions", len(refs), len(ids))
	}
	if len(data) > 400000 {
		t.Fatalf("repeated text SVG exceeds budget: %d", len(data))
	}
	again, _ := svg.Render(lr, nil)
	if !bytes.Equal(data, again) {
		t.Fatal("unstable bytes")
	}
}
func TestNewChartsAreWellFormed(t *testing.T) {
	for _, kind := range []string{"bar", "pie"} {
		rv := &sirena.ResolvedView{Elements: []*sirena.Element{{Name: "wide", Metadata: map[string]sirena.Value{"label": sirena.String{Value: "A <wide> & readable label"}, "value": sirena.Number{Value: 2}}}, {Name: "other", Metadata: map[string]sirena.Value{"value": sirena.Number{Value: 6}}}}}
		lr, _, err := layout.Render(rv, sirena.RenderOptions{Diagram: kind})
		if err != nil {
			t.Fatal(err)
		}
		data, err := svg.Render(lr, nil)
		if err != nil {
			t.Fatal(err)
		}
		d := xml.NewDecoder(bytes.NewReader(data))
		for {
			_, err := d.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
		}
		if !bytes.Contains(data, []byte("data-sirena-id=\"wide\"")) {
			t.Fatal("semantic identity lost")
		}
	}
}
