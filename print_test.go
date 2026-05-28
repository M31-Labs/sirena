package sirena_test

import (
	"strings"
	"testing"

	"m31labs.dev/sirena"
)

// --- Task 13a: skeleton + elements --------------------------------------

func TestPrint_Element_Minimal(t *testing.T) {
	doc, err := sirena.Parse([]byte(`service api`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := sirena.Print(doc)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}
	want := "service api\n"
	if string(out) != want {
		t.Errorf("got %q, want %q", string(out), want)
	}
}

func TestPrint_Element_WithMetadata(t *testing.T) {
	doc, err := sirena.Parse([]byte(`service api { label: "API" }`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := sirena.Print(doc)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}
	want := "service api {\n  label: \"API\"\n}\n"
	if string(out) != want {
		t.Errorf("got %q\nwant %q", string(out), want)
	}
}

func TestPrint_Format_Idempotent(t *testing.T) {
	src := []byte(`service api { label: "API" }`)
	once, err := sirena.Format(src)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	twice, err := sirena.Format(once)
	if err != nil {
		t.Fatalf("Format twice: %v", err)
	}
	if string(once) != string(twice) {
		t.Errorf("Format not idempotent:\n%s\n---\n%s", once, twice)
	}
}

// --- Task 13b: boundaries, edges, views, overrides ----------------------

func TestPrint_Boundary_Nested(t *testing.T) {
	src := []byte(`boundary trust "pci" {
  service api
  database db
}`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := sirena.Print(doc)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}
	want := "boundary trust \"pci\" {\n  service api\n  database db\n}\n"
	if string(out) != want {
		t.Errorf("got %q\nwant %q", string(out), want)
	}
}

func TestPrint_Boundary_Empty(t *testing.T) {
	src := []byte(`boundary trust "pci" {}`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := sirena.Print(doc)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}
	want := "boundary trust \"pci\" {}\n"
	if string(out) != want {
		t.Errorf("got %q\nwant %q", string(out), want)
	}
}

func TestPrint_Boundary_DoubleNested(t *testing.T) {
	src := []byte(`boundary trust "outer" {
  boundary trust "inner" {
    service api
  }
}`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := sirena.Print(doc)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}
	want := "boundary trust \"outer\" {\n  boundary trust \"inner\" {\n    service api\n  }\n}\n"
	if string(out) != want {
		t.Errorf("got %q\nwant %q", string(out), want)
	}
}

func TestPrint_Edge_AllDirections(t *testing.T) {
	cases := []struct{ in, want string }{
		{`api -> db: reads`, "api -> db: reads\n"},
		{`api <- worker`, "api <- worker\n"},
		{`api <-> peer`, "api <-> peer\n"},
		{`api -> db`, "api -> db\n"},
	}
	for _, c := range cases {
		doc, err := sirena.Parse([]byte(c.in))
		if err != nil {
			t.Fatalf("Parse %q: %v", c.in, err)
		}
		out, err := sirena.Print(doc)
		if err != nil {
			t.Fatalf("Print %q: %v", c.in, err)
		}
		if string(out) != c.want {
			t.Errorf("in=%q: got %q want %q", c.in, string(out), c.want)
		}
	}
}

func TestPrint_Edge_WithLabel(t *testing.T) {
	src := []byte(`api -> db: reads "user records"`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := sirena.Print(doc)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}
	want := "api -> db: reads \"user records\"\n"
	if string(out) != want {
		t.Errorf("got %q\nwant %q", string(out), want)
	}
}

func TestPrint_Edge_Qualified(t *testing.T) {
	src := []byte(`api -> platform.kafka: publishes`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := sirena.Print(doc)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}
	want := "api -> platform.kafka: publishes\n"
	if string(out) != want {
		t.Errorf("got %q\nwant %q", string(out), want)
	}
}

func TestPrint_View_FullCoverage(t *testing.T) {
	src := []byte(`view "data-plane" {
  title: "Data Plane"
  preset: sociotechnical
}`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := sirena.Print(doc)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `view "data-plane" {`) {
		t.Errorf("missing view header: %q", s)
	}
	if !strings.Contains(s, `title: "Data Plane"`) {
		t.Errorf("missing title: %q", s)
	}
	if !strings.Contains(s, `preset: sociotechnical`) {
		t.Errorf("missing preset: %q", s)
	}
}

func TestPrint_View_WithBudget(t *testing.T) {
	src := []byte(`view "v" {
  budget {
    nodes: 30
    edges_per_node: 6
    depth: 4
    boundary_fanout: 12
    label_chars: 64
  }
}`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := sirena.Print(doc)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}
	want := "view \"v\" {\n  budget {\n    nodes: 30\n    edges_per_node: 6\n    depth: 4\n    boundary_fanout: 12\n    label_chars: 64\n  }\n}\n"
	if string(out) != want {
		t.Errorf("got %q\nwant %q", string(out), want)
	}
}

func TestPrint_View_FullRoundTrip(t *testing.T) {
	src := []byte(`view "payments-data-plane" {
  title: "Payments — data plane"
  preset: sociotechnical
  include: [
    boundary "pci"
    service "auth-svc"
    edges: incoming to "api"
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
    boundary_fanout: 12
    label_chars: 64
  }
}`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := sirena.Print(doc)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}
	// Re-parse to confirm round-trip integrity.
	doc2, err := sirena.Parse(out)
	if err != nil {
		t.Fatalf("Re-parse: %v\nout=%s", err, out)
	}
	if len(doc2.Views) != 1 {
		t.Fatalf("Views: %d", len(doc2.Views))
	}
	v := doc2.Views[0]
	if v.Name != "payments-data-plane" {
		t.Errorf("Name: %q", v.Name)
	}
	if v.Title != "Payments — data plane" {
		t.Errorf("Title: %q", v.Title)
	}
	if v.Preset != "sociotechnical" {
		t.Errorf("Preset: %q", v.Preset)
	}
	if v.Theme != "earth-default" {
		t.Errorf("Theme: %q", v.Theme)
	}
	if len(v.Include) != 3 {
		t.Errorf("Include: %d", len(v.Include))
	}
	if v.Budget == nil || v.Budget.Nodes != 30 {
		t.Errorf("Budget: %+v", v.Budget)
	}
	if v.Layout == nil || v.Layout.Direction != "top-down" {
		t.Errorf("Layout: %+v", v.Layout)
	}
	if v.Layout != nil && v.Layout.Pin["api"] != "top" {
		t.Errorf("Layout.Pin[api]: %q", v.Layout.Pin["api"])
	}
}

func TestPrint_Override_AllForms(t *testing.T) {
	cases := []struct{ in, contains string }{
		{`service api @override`, "service api @override\n"},
		{`service api @override(label, tags)`, "service api @override(label, tags)\n"},
		{`service api @override(except: source_ref)`, "service api @override(except: source_ref)\n"},
	}
	for _, c := range cases {
		doc, err := sirena.Parse([]byte(c.in))
		if err != nil {
			t.Fatalf("Parse %q: %v", c.in, err)
		}
		out, err := sirena.Print(doc)
		if err != nil {
			t.Fatalf("Print %q: %v", c.in, err)
		}
		if string(out) != c.contains {
			t.Errorf("in=%q: got %q want %q", c.in, string(out), c.contains)
		}
	}
}

func TestPrint_Override_FieldsSorted(t *testing.T) {
	src := []byte(`service api @override(zeta, alpha, mu)`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := sirena.Print(doc)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}
	want := "service api @override(alpha, mu, zeta)\n"
	if string(out) != want {
		t.Errorf("got %q\nwant %q", string(out), want)
	}
}

func TestPrint_Override_WithMetadata(t *testing.T) {
	src := []byte(`service api @override(label) {
  label: "API"
}`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := sirena.Print(doc)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}
	want := "service api @override(label) {\n  label: \"API\"\n}\n"
	if string(out) != want {
		t.Errorf("got %q\nwant %q", string(out), want)
	}
}

func TestPrint_Metadata_SortedKeys(t *testing.T) {
	src := []byte(`service api {
  z_last: "z"
  a_first: "a"
  m_middle: "m"
}`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := sirena.Print(doc)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}
	s := string(out)
	a := strings.Index(s, "a_first")
	m := strings.Index(s, "m_middle")
	z := strings.Index(s, "z_last")
	if a < 0 || m < 0 || z < 0 || a > m || m > z {
		t.Errorf("keys not sorted lexically:\n%s", s)
	}
}

func TestPrint_Metadata_AllValueTypes(t *testing.T) {
	src := []byte(`service api {
  label: "API"
  count: 42
  status: active
  tags: [stateless, hot]
}`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := sirena.Print(doc)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}
	want := "service api {\n  count: 42\n  label: \"API\"\n  status: active\n  tags: [stateless, hot]\n}\n"
	if string(out) != want {
		t.Errorf("got %q\nwant %q", string(out), want)
	}
}

func TestPrint_Number_FloatVsInt(t *testing.T) {
	src := []byte(`service api {
  i: 42
  f: 3.14
}`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := sirena.Print(doc)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "i: 42\n") {
		t.Errorf("expected integer-style 42: %q", s)
	}
	if !strings.Contains(s, "f: 3.14\n") {
		t.Errorf("expected 3.14: %q", s)
	}
}

func TestPrint_Import(t *testing.T) {
	src := []byte(`import "../shared/platform.sir"
service api`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := sirena.Print(doc)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}
	want := "import \"../shared/platform.sir\"\n\nservice api\n"
	if string(out) != want {
		t.Errorf("got %q\nwant %q", string(out), want)
	}
}

func TestPrint_TopLevel_BlankLineSeparation(t *testing.T) {
	src := []byte(`service api
database db`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := sirena.Print(doc)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}
	want := "service api\n\ndatabase db\n"
	if string(out) != want {
		t.Errorf("got %q\nwant %q", string(out), want)
	}
}

// --- Task 13c: string escaping (safety) ---------------------------------

func TestPrint_StringEscaping_QuoteAndBackslash(t *testing.T) {
	src := []byte(`service api { label: "a\"; rm -rf /" }`)
	doc, err := sirena.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	out, err := sirena.Print(doc)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}
	doc2, err := sirena.Parse(out)
	if err != nil {
		t.Fatalf("Parse reprinted: %v\nout=%s", err, out)
	}

	got1 := doc.Systems[0].Elements[0].Metadata["label"].(sirena.String).Value
	got2 := doc2.Systems[0].Elements[0].Metadata["label"].(sirena.String).Value
	if got1 != got2 {
		t.Errorf("escape round-trip lost data:\noriginal: %q\nreprint:  %q", got1, got2)
	}
	if got1 != `a"; rm -rf /` {
		t.Errorf("original parse value not what we expected: %q", got1)
	}
}

func TestPrint_StringEscaping_ControlChars(t *testing.T) {
	// Build a Document programmatically — the parser may not accept literal control chars,
	// but the printer must escape them safely if they appear in the IR.
	doc := &sirena.Document{
		Systems: []*sirena.SystemDecl{{
			Elements: []*sirena.Element{{
				Kind: sirena.ElementKindService,
				Name: "api",
				Metadata: map[string]sirena.Value{
					"label": sirena.String{Value: "line1\nline2\ttab"},
				},
			}},
		}},
	}
	out, err := sirena.Print(doc)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `\n`) || !strings.Contains(s, `\t`) {
		t.Errorf("control chars not escaped: %q", s)
	}
	// Verify reparse recovers the original.
	doc2, err := sirena.Parse(out)
	if err != nil {
		t.Fatalf("reparse: %v\nout=%s", err, s)
	}
	got := doc2.Systems[0].Elements[0].Metadata["label"].(sirena.String).Value
	if got != "line1\nline2\ttab" {
		t.Errorf("round-trip lost: %q", got)
	}
}

func TestPrint_StringEscaping_BackslashAlone(t *testing.T) {
	doc := &sirena.Document{
		Systems: []*sirena.SystemDecl{{
			Elements: []*sirena.Element{{
				Kind: sirena.ElementKindService,
				Name: "api",
				Metadata: map[string]sirena.Value{
					"path": sirena.String{Value: `C:\Users\x`},
				},
			}},
		}},
	}
	out, err := sirena.Print(doc)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}
	doc2, err := sirena.Parse(out)
	if err != nil {
		t.Fatalf("reparse: %v\nout=%s", err, string(out))
	}
	got := doc2.Systems[0].Elements[0].Metadata["path"].(sirena.String).Value
	if got != `C:\Users\x` {
		t.Errorf("backslash round-trip lost: %q", got)
	}
}

// --- General round-trip exercise on existing parse-test inputs ----------

func TestPrint_RoundTrip_KnownInputs(t *testing.T) {
	inputs := []string{
		`service api`,
		`boundary trust "pci" {
  service api
  database db
}`,
		`api -> db: reads "user records"`,
		`api <- worker`,
		`api <-> peer`,
		`api -> platform.kafka: publishes`,
		`service api @override`,
		`service api @override(label, tags) {
  label: "API"
  tags: [stateless]
}`,
		`boundary trust "pci" @override(label) {
  service api
}`,
		`import "../shared/platform.sir"`,
	}
	for _, in := range inputs {
		doc, err := sirena.Parse([]byte(in))
		if err != nil {
			t.Errorf("Parse %q: %v", in, err)
			continue
		}
		out, err := sirena.Print(doc)
		if err != nil {
			t.Errorf("Print %q: %v", in, err)
			continue
		}
		// Format must be idempotent for any input.
		twice, err := sirena.Format(out)
		if err != nil {
			t.Errorf("Format(Print(...)) %q: %v\nout=%s", in, err, out)
			continue
		}
		if string(out) != string(twice) {
			t.Errorf("Format not idempotent for %q:\nfirst:  %s\nsecond: %s", in, out, twice)
		}
	}
}
