package scene3d

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"m31labs.dev/gosx/scene"
	"m31labs.dev/gosx/scene/schema"
	"m31labs.dev/sirena"
)

var updateRecords = flag.Bool("update-records", false, "update Scene3D record export goldens")

func recordLayout(t *testing.T, kind, source string) *sirena.LayoutResult {
	t.Helper()
	doc, err := sirena.Parse([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	lr, _, err := sirena.Render(sirena.AllElementsView(doc), sirena.RenderOptions{Diagram: kind})
	if err != nil {
		t.Fatal(err)
	}
	return lr
}

func recordIR(t *testing.T, lr *sirena.LayoutResult, opts Options) (scene.SceneIR, string, []byte) {
	t.Helper()
	data, err := Build(lr, opts)
	if err != nil {
		t.Fatal(err)
	}
	var props struct {
		Scene     scene.SceneIR `json:"scene"`
		AriaLabel string        `json:"ariaLabel"`
	}
	if err := json.Unmarshal(data, &props); err != nil {
		t.Fatal(err)
	}
	report := schema.ValidateJSON(mustJSON(props.Scene), schema.Options{})
	if !report.Valid {
		t.Fatalf("invalid SceneIR: %+v", report.Diagnostics)
	}
	return props.Scene, props.AriaLabel, data
}

func TestRecordExportGolden(t *testing.T) {
	for _, kind := range []string{"class", "er"} {
		t.Run(kind, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join("..", "..", "examples", "diagrams", kind+".sir"))
			if err != nil {
				t.Fatal(err)
			}
			lr := recordLayout(t, kind, string(src))
			before := mustJSON(lr)
			ir, aria, got := recordIR(t, lr, Options{})
			_, _, again := recordIR(t, lr, Options{})
			if !bytes.Equal(got, again) || !bytes.Equal(before, mustJSON(lr)) {
				t.Fatal("non-deterministic export or mutated layout")
			}
			for _, np := range lr.NodePlacements {
				found := false
				for _, obj := range ir.Objects {
					if obj.ID == np.Node.Name {
						found = obj.Kind == "box"
					}
				}
				if !found {
					t.Fatalf("%s must retain rectangular record geometry", np.Node.Name)
				}
				if !strings.Contains(aria, np.Node.DisplayLabel()) {
					t.Fatal("missing accessible title")
				}
				for _, key := range []string{"fields", "methods"} {
					value, _ := np.Node.Metadata[key].(sirena.String)
					for _, row := range strings.Split(value.Value, ";") {
						row = strings.TrimSpace(row)
						if row == "" {
							continue
						}
						found := false
						for _, label := range ir.Labels {
							if label.Text == row {
								found = label.MaxWidth == 320 && label.MaxLines == 1 && label.Overflow == "ellipsis" && label.Collision == "allow"
							}
						}
						if !found || !strings.Contains(aria, row) {
							t.Fatalf("missing readable or accessible row %q", row)
						}
					}
				}
			}
			if !strings.Contains(aria, "1 to many") {
				t.Fatal("relationship cardinality omitted")
			}
			path := filepath.Join("testdata", kind+".golden.json")
			if *updateRecords {
				if err := os.MkdirAll("testdata", 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, got, 0644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatal("record export differs from golden; regenerate with -update-records")
			}
		})
	}
}

func TestRecordAnchorsAndEmptyCompartments(t *testing.T) {
	for _, tc := range []struct {
		name, metadata string
		texts          []string
		ys             []float64
		dividers       []float64
	}{
		{"empty", `fields: "; ;" methods: " ; "`, []string{"Record"}, []float64{20}, nil},
		{"methods only", `methods: " ; run(); ; stop(); "`, []string{"Record", "run()", "stop()"}, []float64{20, 44, 68}, []float64{32}},
		{"both", `fields: " id: UUID ; ; value: int; " methods: " run() ; "`, []string{"Record", "id: UUID", "value: int", "run()"}, []float64{20, 44, 68, 92}, []float64{32, 80}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lr := recordLayout(t, "class", `database record { label: "Record" `+tc.metadata+` }`)
			ir, _, _ := recordIR(t, lr, Options{MotionStyle: "float"})
			np := lr.NodePlacements[0]
			scale := 12 / math.Max(lr.Bounds.Width(), lr.Bounds.Height()*1.7)
			center := lr.Bounds.Center()
			if len(ir.Labels) != len(tc.texts) {
				t.Fatalf("labels=%d", len(ir.Labels))
			}
			for i, label := range ir.Labels {
				if label.Text != tc.texts[i] || math.Abs(label.Y-(center.Y-np.Bounds.Min.Y-tc.ys[i])*scale) > 1e-9 {
					t.Fatalf("wrong row anchor: %+v", label)
				}
				expectedX := np.Bounds.Min.X + 12
				if i == 0 {
					expectedX = np.Bounds.Center().X
				}
				if math.Abs(label.X-(expectedX-center.X)*scale) > 1e-9 {
					t.Fatalf("wrong row x: %+v", label)
				}
				if label.ShiftY == 0 || label.DriftSpeed == 0 {
					t.Fatal("label lost native drift")
				}
			}
			found := false
			for _, obj := range ir.Objects {
				if obj.Kind != "lines" {
					continue
				}
				found = true
				if len(obj.LineSegments) != len(tc.dividers) {
					t.Fatal("wrong compartment count")
				}
				for i, y := range tc.dividers {
					if math.Abs(obj.Y+obj.Points[2*i].Y-(center.Y-np.Bounds.Min.Y-y)*scale) > 1e-9 {
						t.Fatal("divider does not match SVG anchor")
					}
				}
			}
			if found != (len(tc.dividers) > 0) {
				t.Fatal("empty compartment emitted a divider")
			}
		})
	}
}

func TestRecordLongTextAndRelationshipSemantics(t *testing.T) {
	long := strings.Repeat("Wide Unicode Ω field ", 40)
	lr := recordLayout(t, "er", fmt.Sprintf(`database a { label: %q fields: %q }
 database b { fields: "PK id: UUID" }
 a -> b: flow "owns · 1 to 0..many"`, long, long))
	ir, aria, _ := recordIR(t, lr, Options{})
	for _, text := range []string{long, "owns · 1 to 0..many"} {
		found := false
		for _, label := range ir.Labels {
			if label.Text == recordLabelText(text, 14) || label.Text == recordLabelText(text, 12) {
				found = label.MaxWidth == 320 && label.MaxLines == 1 && label.Overflow == "ellipsis"
			}
		}
		if !found || !strings.Contains(aria, strings.TrimSpace(text)) {
			t.Fatalf("lost full accessible text %q", text)
		}
	}
	short := recordLabelText(long, 14)
	for _, obj := range ir.Objects {
		if obj.Kind == "box" && obj.Height < 1 {
			t.Fatal("long text collapsed the measured compartments")
		}
	}
	if !strings.HasSuffix(short, "…") || !utf8.ValidString(short) || len(short) >= len(long) {
		t.Fatal("long row did not truncate with a UTF-8 ellipsis")
	}
	if got := recordLabelText("one\ntwo\tthree", 14); got != "one two three" {
		t.Fatalf("label must stay on one line: %q", got)
	}
}

func TestRecordSceneItemLimit(t *testing.T) {
	bounds := sirena.Rect{Max: sirena.Point{X: 100, Y: 100}}
	lr := &sirena.LayoutResult{Diagram: "er", Bounds: bounds}
	for i := 0; i < 500; i++ {
		lr.NodePlacements = append(lr.NodePlacements, &sirena.NodePlacement{Bounds: bounds, Node: &sirena.Element{Name: fmt.Sprintf("record-%d", i), Metadata: map[string]sirena.Value{"fields": sirena.String{Value: "id: UUID"}}}})
	}
	ir, _, _ := recordIR(t, lr, Options{})
	if len(ir.Objects)+len(ir.Labels) != maxSceneItems {
		t.Fatal("record decorations not counted")
	}
	lr.NodePlacements[0].Node.Metadata["methods"] = sirena.String{Value: "save()"}
	if _, err := Build(lr, Options{}); err == nil || !strings.Contains(err.Error(), "at most 2000") {
		t.Fatalf("record overflow accepted: %v", err)
	}
}

func TestRecordPresentationLifetime(t *testing.T) {
	lr := recordLayout(t, "class", `service record { fields: "id: UUID" methods: "save()" }`)
	x, y, factor := 2.0, 3.0, 1.5
	hidden := false
	color := "#88ddcc"
	ir, _, data := recordIR(t, lr, Options{Steps: []Step{{Label: "Initial"}, {Label: "Move", Patches: []Patch{{Target: "record", X: &x, Y: &y, Scale: &factor, Color: &color}}}, {Label: "Hide", Patches: []Patch{{Target: "record", Visible: &hidden}}}, {Label: "Reset"}}})
	var props struct {
		SlideSteps Timeline `json:"slideSteps"`
	}
	if err := json.Unmarshal(data, &props); err != nil {
		t.Fatal(err)
	}
	frames := props.SlideSteps.Frames
	if !bytes.Equal(mustJSON(frames[0].Commands), mustJSON(frames[3].Commands)) {
		t.Fatal("reset did not restore all compartments")
	}
	for _, label := range ir.Labels {
		if !strings.Contains(string(mustJSON(frames[2].Commands)), label.ID) {
			t.Fatalf("hidden record retains %s", label.ID)
		}
		found := false
		for _, command := range frames[1].Commands {
			if command.ObjectID != label.ID || command.Kind != scene.CommandCreateObject {
				continue
			}
			var payload struct {
				Props scene.LabelIR `json:"props"`
			}
			if err := json.Unmarshal(mustJSON(command.Data), &payload); err != nil {
				t.Fatal(err)
			}
			moved := payload.Props
			owner := ir.Objects[0]
			if math.Abs(moved.X-(x+(label.X-owner.X)*factor)) > 1e-9 || math.Abs(moved.Y-(y+(label.Y-owner.Y)*factor)) > 1e-9 {
				t.Fatal("row did not follow owner transform")
			}
			found = true
		}
		if !found {
			t.Fatalf("move lost %s", label.ID)
		}
	}
	if !strings.Contains(string(mustJSON(frames[2].Commands)), "record:record:dividers") {
		t.Fatal("hidden record retains dividers")
	}
}
