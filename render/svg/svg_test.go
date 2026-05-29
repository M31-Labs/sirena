package svg

import (
	"crypto/sha256"
	"strings"
	"testing"

	"m31labs.dev/sirena"
)

func twoNodeOneEdge() *sirena.LayoutResult {
	a := &sirena.NodePlacement{
		Node:   &sirena.Element{Kind: sirena.ElementKindService, Name: "API"},
		Bounds: sirena.Rect{Min: sirena.Point{X: 0, Y: 0}, Max: sirena.Point{X: 80, Y: 40}},
	}
	b := &sirena.NodePlacement{
		Node:   &sirena.Element{Kind: sirena.ElementKindDatabase, Name: "db"},
		Bounds: sirena.Rect{Min: sirena.Point{X: 160, Y: 0}, Max: sirena.Point{X: 240, Y: 40}},
	}
	e := &sirena.EdgeRoute{
		Edge:   &sirena.Edge{From: "API", To: "db", Kind: sirena.EdgeKindReads, Direction: sirena.DirForward},
		Points: []sirena.Point{{X: 80, Y: 20}, {X: 160, Y: 20}},
	}
	return &sirena.LayoutResult{
		Bounds:         sirena.Rect{Min: sirena.Point{X: 0, Y: 0}, Max: sirena.Point{X: 240, Y: 40}},
		NodePlacements: []*sirena.NodePlacement{a, b},
		EdgeRoutes:     []*sirena.EdgeRoute{e},
	}
}

func theme(t *testing.T) *Theme {
	t.Helper()
	th, err := ThemeForName("earth-default")
	if err != nil {
		t.Fatalf("theme: %v", err)
	}
	return th
}

func TestRender_TwoNodeOneEdge(t *testing.T) {
	out, err := Render(twoNodeOneEdge(), theme(t))
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	s := string(out)
	if got := strings.Count(s, "<rect"); got != 2 {
		t.Errorf("<rect> count = %d, want 2", got)
	}
	if got := strings.Count(s, `<g class="node`); got != 2 {
		t.Errorf("node group count = %d, want 2", got)
	}
	if got := strings.Count(s, `<g class="edge`); got != 1 {
		t.Errorf("edge group count = %d, want 1", got)
	}
}

func TestRender_EmbedsCSSVariables(t *testing.T) {
	out, _ := Render(twoNodeOneEdge(), theme(t))
	s := string(out)
	if !strings.Contains(s, "<style>") {
		t.Fatal("no <style> block")
	}
	for token := range AllowlistedTokens {
		if !strings.Contains(s, token+":") {
			t.Errorf("style block missing token declaration %s", token)
		}
	}
}

func TestRender_GlyphsAsPaths(t *testing.T) {
	lr := &sirena.LayoutResult{
		Bounds: sirena.Rect{Min: sirena.Point{X: 0, Y: 0}, Max: sirena.Point{X: 80, Y: 40}},
		NodePlacements: []*sirena.NodePlacement{{
			Node:   &sirena.Element{Kind: sirena.ElementKindService, Name: "API"},
			Bounds: sirena.Rect{Min: sirena.Point{X: 0, Y: 0}, Max: sirena.Point{X: 80, Y: 40}},
		}},
	}
	out, _ := Render(lr, theme(t))
	s := string(out)
	if strings.Contains(s, "<text") {
		t.Errorf("output contains <text>; labels must be glyph paths")
	}
	// "API" → three glyph paths inside the node's label group.
	if got := strings.Count(s, "<path"); got < 3 {
		t.Errorf("expected >= 3 glyph paths for \"API\"; got %d", got)
	}
}

func TestRender_ByteEqualAcrossRuns(t *testing.T) {
	a, _ := Render(twoNodeOneEdge(), theme(t))
	b, _ := Render(twoNodeOneEdge(), theme(t))
	if sha256.Sum256(a) != sha256.Sum256(b) {
		t.Errorf("render not byte-stable across runs")
	}
}

func TestRender_SanitizesUserStrings(t *testing.T) {
	lr := &sirena.LayoutResult{
		Bounds: sirena.Rect{Min: sirena.Point{X: 0, Y: 0}, Max: sirena.Point{X: 200, Y: 40}},
		NodePlacements: []*sirena.NodePlacement{{
			Node:   &sirena.Element{Kind: sirena.ElementKindService, Name: "<script>alert(1)</script>"},
			Bounds: sirena.Rect{Min: sirena.Point{X: 0, Y: 0}, Max: sirena.Point{X: 200, Y: 40}},
		}},
	}
	out, _ := Render(lr, theme(t))
	if strings.Contains(string(out), "<script>") {
		t.Errorf("literal <script> reached output")
	}
}

func TestRender_BoundariesBeforeNodes(t *testing.T) {
	child := &sirena.Element{Kind: sirena.ElementKindService, Name: "svc"}
	b := &sirena.Boundary{Kind: sirena.BoundaryKindTrust, Name: "pci", Children: []sirena.Node{child}}
	lr := &sirena.LayoutResult{
		Bounds: sirena.Rect{Min: sirena.Point{X: 0, Y: 0}, Max: sirena.Point{X: 200, Y: 120}},
		BoundaryPlacements: []*sirena.BoundaryPlacement{{
			Boundary:       b,
			Bounds:         sirena.Rect{Min: sirena.Point{X: 0, Y: 0}, Max: sirena.Point{X: 200, Y: 120}},
			ChildrenBounds: sirena.Rect{Min: sirena.Point{X: 16, Y: 16}, Max: sirena.Point{X: 184, Y: 104}},
		}},
		NodePlacements: []*sirena.NodePlacement{{
			Node:   child,
			Bounds: sirena.Rect{Min: sirena.Point{X: 60, Y: 40}, Max: sirena.Point{X: 140, Y: 80}},
		}},
	}
	out := string(mustRender(t, lr, theme(t)))
	bi := strings.Index(out, `<g class="boundary`)
	ni := strings.Index(out, `<g class="node`)
	if bi < 0 || ni < 0 || bi > ni {
		t.Errorf("boundary group (idx %d) must precede node group (idx %d)", bi, ni)
	}
}

func mustRender(t *testing.T, lr *sirena.LayoutResult, th *Theme) []byte {
	t.Helper()
	out, err := Render(lr, th)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	return out
}
