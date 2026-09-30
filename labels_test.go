package sirena_test

import (
	"testing"

	"m31labs.dev/sirena"
)

func TestDisplayLabel(t *testing.T) {
	element := &sirena.Element{Name: "mdpp", Metadata: map[string]sirena.Value{
		"label": sirena.String{Value: "Markdown++"},
	}}
	if got := element.DisplayLabel(); got != "Markdown++" {
		t.Fatalf("Element.DisplayLabel() = %q, want Markdown++", got)
	}

	boundary := &sirena.Boundary{Name: "outputs", Metadata: map[string]sirena.Value{
		"label": sirena.String{Value: "Deck outputs"},
	}}
	if got := boundary.DisplayLabel(); got != "Deck outputs" {
		t.Fatalf("Boundary.DisplayLabel() = %q, want Deck outputs", got)
	}
}

func TestDisplayLabelFallsBackToName(t *testing.T) {
	if got := (&sirena.Element{Name: "api"}).DisplayLabel(); got != "api" {
		t.Fatalf("Element.DisplayLabel() = %q, want api", got)
	}
	if got := (&sirena.Boundary{Name: "platform"}).DisplayLabel(); got != "platform" {
		t.Fatalf("Boundary.DisplayLabel() = %q, want platform", got)
	}
}
