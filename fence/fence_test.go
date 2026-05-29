package fence

import (
	"errors"
	"strings"
	"testing"

	"m31labs.dev/sirena"
)

func hasDiag(diags []sirena.Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}

// TestRender_InlineSynthView covers the common case: a fence body of bare
// declarations with no explicit view. Render synthesizes an all-elements
// view and produces SVG.
func TestRender_InlineSynthView(t *testing.T) {
	src := []byte("service api\ndatabase db\napi -> db: reads \"user records\"\n")
	res, err := Render(src, Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasPrefix(string(res.SVG), "<?xml") || !strings.Contains(string(res.SVG), "<svg") {
		t.Fatalf("expected SVG output, got %d bytes: %.80s", len(res.SVG), res.SVG)
	}
	for _, d := range res.Diagnostics {
		if d.Severity == sirena.SeverityError {
			t.Errorf("unexpected error diagnostic: %+v", d)
		}
	}
}

// TestRender_Interactive returns the sentinel until the island path ships.
func TestRender_Interactive(t *testing.T) {
	_, err := Render([]byte("service api\n"), Options{Interactive: true})
	if !errors.Is(err, ErrInteractiveNotAvailable) {
		t.Fatalf("err = %v, want ErrInteractiveNotAvailable", err)
	}
}

// TestRender_Empty warns and renders nothing when there is no body and no
// view reference.
func TestRender_Empty(t *testing.T) {
	res, err := Render(nil, Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if res.SVG != nil {
		t.Errorf("expected no SVG, got %d bytes", len(res.SVG))
	}
	if !hasDiag(res.Diagnostics, CodeFenceEmpty) {
		t.Errorf("missing %s diagnostic; got %+v", CodeFenceEmpty, res.Diagnostics)
	}
}

// TestRender_NoRenderableContent warns when the body parses but declares
// nothing to draw.
func TestRender_NoRenderableContent(t *testing.T) {
	res, err := Render([]byte("\n  \n"), Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if res.SVG != nil {
		t.Errorf("expected no SVG, got %d bytes", len(res.SVG))
	}
	if !hasDiag(res.Diagnostics, CodeFenceNoView) {
		t.Errorf("missing %s diagnostic; got %+v", CodeFenceNoView, res.Diagnostics)
	}
}

// TestRender_BodyOverridesViewRef confirms a non-empty body beats a view=
// reference: a warning fires and the body still renders.
func TestRender_BodyOverridesViewRef(t *testing.T) {
	res, err := Render([]byte("service api\n"), Options{ViewRef: "views/whatever.view.sir"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !hasDiag(res.Diagnostics, CodeFenceBodyOverrides) {
		t.Errorf("missing %s diagnostic; got %+v", CodeFenceBodyOverrides, res.Diagnostics)
	}
	if !strings.Contains(string(res.SVG), "<svg") {
		t.Errorf("expected body to render despite view=, got %d bytes", len(res.SVG))
	}
}

// TestRender_ThemeApplied confirms a named theme is accepted and an unknown
// theme surfaces as a hard error.
func TestRender_ThemeApplied(t *testing.T) {
	if _, err := Render([]byte("service api\n"), Options{Theme: "earth-default"}); err != nil {
		t.Fatalf("known theme: %v", err)
	}
	if _, err := Render([]byte("service api\n"), Options{Theme: "no-such-theme"}); err == nil {
		t.Error("expected error for unknown theme")
	}
}
