package sirena_test

import (
	"strings"
	"testing"

	"m31labs.dev/sirena"
)

// TestValidate_DuplicateName pins SIR-VALIDATE-DUP-NAME: two top-level
// elements with the same name in the same SystemDecl scope is an error.
func TestValidate_DuplicateName(t *testing.T) {
	src := []byte(`service api
service api`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	diags := sirena.Validate(doc)
	if len(diags) != 1 {
		t.Fatalf("want 1 diagnostic, got %d: %+v", len(diags), diags)
	}
	if diags[0].Code != "SIR-VALIDATE-DUP-NAME" {
		t.Errorf("Code: %q", diags[0].Code)
	}
	if diags[0].Severity != sirena.SeverityError {
		t.Errorf("Severity: %v", diags[0].Severity)
	}
}

// TestValidate_DuplicateInBoundary pins that the duplicate-name rule is
// scoped to direct parent: a boundary's two same-named children collide
// without bleeding into the outer SystemDecl scope.
func TestValidate_DuplicateInBoundary(t *testing.T) {
	src := []byte(`boundary trust "pci" {
  service api
  service api
}`)
	doc, _ := sirena.Parse(src)
	diags := sirena.Validate(doc)
	if len(diags) != 1 {
		t.Fatalf("want 1 diagnostic, got %d", len(diags))
	}
	if diags[0].Code != "SIR-VALIDATE-DUP-NAME" {
		t.Errorf("Code: %q", diags[0].Code)
	}
}

// TestValidate_OverrideUnknownField pins SIR-VALIDATE-OVERRIDE-UNKNOWN-FIELD:
// any name in an @override(...) field list that is neither a well-known v0.1
// field nor a metadata key present on the declaration is flagged.
func TestValidate_OverrideUnknownField(t *testing.T) {
	src := []byte(`service api @override(label, frobnicate, tags)`)
	doc, _ := sirena.Parse(src)
	diags := sirena.Validate(doc)
	// Expect one diagnostic for "frobnicate"; label and tags are well-known.
	if len(diags) != 1 {
		t.Fatalf("want 1, got %d: %+v", len(diags), diags)
	}
	if diags[0].Code != "SIR-VALIDATE-OVERRIDE-UNKNOWN-FIELD" {
		t.Errorf("Code: %q", diags[0].Code)
	}
	if !strings.Contains(diags[0].Message, "frobnicate") {
		t.Errorf("Message should mention field name: %q", diags[0].Message)
	}
}

// TestValidate_OverrideKnownMetadataKey pins that a field name present as a
// metadata key on the same declaration is NOT flagged — overrides can target
// user-defined metadata, not just well-known v0.1 fields.
func TestValidate_OverrideKnownMetadataKey(t *testing.T) {
	src := []byte(`service api @override(custom) {
  custom: "value"
}`)
	doc, _ := sirena.Parse(src)
	diags := sirena.Validate(doc)
	if len(diags) != 0 {
		t.Errorf("expected 0 diags, got %d: %+v", len(diags), diags)
	}
}

// TestValidate_SidCollision pins SIR-VALIDATE-SID-COLLISION: the same sid
// reused across two declarations in one document is an error.
func TestValidate_SidCollision(t *testing.T) {
	src := []byte(`service api {
  sid: "01H8XGCOLLIDE"
}
service worker {
  sid: "01H8XGCOLLIDE"
}`)
	doc, _ := sirena.Parse(src)
	diags := sirena.Validate(doc)
	if len(diags) != 1 {
		t.Fatalf("want 1, got %d", len(diags))
	}
	if diags[0].Code != "SIR-VALIDATE-SID-COLLISION" {
		t.Errorf("Code: %q", diags[0].Code)
	}
}

// TestValidate_BadSourceRef pins SIR-VALIDATE-BAD-SOURCE-REF: a source_ref
// string that does not match the canonical code://<lang>/<module>#<symbol>
// form is an error.
func TestValidate_BadSourceRef(t *testing.T) {
	src := []byte(`service api {
  source_ref: "not-a-valid-uri"
}`)
	doc, _ := sirena.Parse(src)
	diags := sirena.Validate(doc)
	if len(diags) != 1 {
		t.Fatalf("want 1, got %d", len(diags))
	}
	if diags[0].Code != "SIR-VALIDATE-BAD-SOURCE-REF" {
		t.Errorf("Code: %q", diags[0].Code)
	}
}

// TestValidate_GoodSourceRef pins that a canonical source_ref passes.
func TestValidate_GoodSourceRef(t *testing.T) {
	src := []byte(`service api {
  source_ref: "code://go/m31labs.dev/api/handler#FooHandler"
}`)
	doc, _ := sirena.Parse(src)
	diags := sirena.Validate(doc)
	if len(diags) != 0 {
		t.Errorf("expected 0 diags, got %+v", diags)
	}
}

// TestValidate_BadPriorSourceRef pins that prior_source_ref is checked with
// the same canonical-URI rule as source_ref.
func TestValidate_BadPriorSourceRef(t *testing.T) {
	src := []byte(`service api {
  prior_source_ref: "garbage"
}`)
	doc, _ := sirena.Parse(src)
	diags := sirena.Validate(doc)
	if len(diags) != 1 {
		t.Fatalf("want 1, got %d", len(diags))
	}
	if diags[0].Code != "SIR-VALIDATE-BAD-SOURCE-REF" {
		t.Errorf("Code: %q", diags[0].Code)
	}
}

// TestValidate_CleanDoc pins the happy path: a syntactically valid document
// with no rule violations produces no diagnostics.
func TestValidate_CleanDoc(t *testing.T) {
	src := []byte(`service api
database db`)
	doc, _ := sirena.Parse(src)
	diags := sirena.Validate(doc)
	if len(diags) != 0 {
		t.Errorf("clean doc should have 0 diags, got %+v", diags)
	}
}
