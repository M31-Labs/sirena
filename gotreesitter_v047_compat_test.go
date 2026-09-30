package sirena

import "testing"

// TestGotreesitterV047Compatibility covers the two grammar shapes that
// changed behavior when Sirena moved from gotreesitter v0.20 to v0.47:
// escaped string literals and comma-delimited view selectors.
func TestGotreesitterV047Compatibility(t *testing.T) {
	src := []byte("service api { label: \"quote \\\" inside\" }\n" +
		"view \"overview\" { include: [service \"api\", database \"db\"] }\n")
	doc, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if doc == nil || len(doc.Systems) != 1 || len(doc.Systems[0].Elements) != 1 {
		t.Fatalf("unexpected document shape: %+v", doc)
	}
	label, ok := doc.Systems[0].Elements[0].Metadata["label"].(String)
	if !ok {
		t.Fatalf("label metadata type = %T, want sirena.String", doc.Systems[0].Elements[0].Metadata["label"])
	}
	if label.Value != `quote " inside` {
		t.Fatalf("decoded label = %q, want %q", label.Value, `quote " inside`)
	}
	if len(doc.Views) != 1 || len(doc.Views[0].Include) != 2 {
		t.Fatalf("view selectors = %#v, want two selectors", doc.Views)
	}
	if got := doc.Views[0].Include[0].Target; got != "api" {
		t.Errorf("first selector target = %q, want api", got)
	}
	if got := doc.Views[0].Include[1].Target; got != "db" {
		t.Errorf("second selector target = %q, want db", got)
	}
}
