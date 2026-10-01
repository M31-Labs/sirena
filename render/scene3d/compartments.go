package scene3d

import (
	"fmt"
	"strings"

	"m31labs.dev/gosx/scene"
	"m31labs.dev/sirena"
	"m31labs.dev/sirena/render/svg/font"
)

// recordNodes uses the measured card layout and the same anchors as SVG:
// title at 20px, dividers at 32px, and rows at 44px with a 24px stride.
func recordNodes(placement *sirena.NodePlacement, mesh scene.Mesh, scale, scaleY float64) []scene.Node {
	r := placement.Bounds
	label := func(id, text string, x, y float64, title bool) scene.Label {
		// GoSX omits zero-valued anchors on the wire and defaults them to .5.
		// A negligible positive anchor preserves left alignment in both props
		// and presentation commands without changing the shared renderer.
		font, anchor, offset := "14px sans-serif", 1e-9, -10.0
		if title {
			font, anchor, offset = "600 14px sans-serif", .5, 0
		}
		return scene.Label{
			ID: id, Target: mesh.ID, Text: text,
			Position: scene.Vec3((x-r.Center().X)*scale, (r.Center().Y-y)*scaleY, .2),
			Shift:    mesh.Drift, DriftSpeed: mesh.DriftSpeed, DriftPhase: mesh.DriftPhase,
			Color: "#102b32", Background: "transparent", BorderColor: "transparent", Font: font, LineHeight: 16,
			AnchorX: anchor, AnchorY: .5, OffsetX: offset,
			WhiteSpace: "pre", MaxWidth: 320, MaxLines: 1, Overflow: "ellipsis",
			// Records have measured row spacing. Collision avoidance would hide
			// their content when projected rows' padded label boxes overlap.
			Collision: "allow", Priority: 100,
		}
	}
	items := []scene.Node{label("label:"+mesh.ID, placement.Node.DisplayLabel(), r.Center().X, r.Min.Y+20, true)}
	y := r.Min.Y + 40
	var points []scene.Vector3
	var segments [][2]int
	for _, key := range []string{"fields", "methods"} {
		value, _ := placement.Node.Metadata[key].(sirena.String)
		var rows []string
		for _, row := range strings.Split(value.Value, ";") {
			if row = strings.TrimSpace(row); row != "" {
				rows = append(rows, row)
			}
		}
		if len(rows) == 0 {
			continue
		}
		dividerY := (r.Center().Y - (y - 8)) * scaleY
		points = append(points, scene.Vec3(-r.Width()*scale/2, dividerY, .18), scene.Vec3(r.Width()*scale/2, dividerY, .18))
		segments = append(segments, [2]int{len(points) - 2, len(points) - 1})
		for i, row := range rows {
			items = append(items, label(fmt.Sprintf("label:%s:%s:%d", mesh.ID, key, i), row, r.Min.X+12, y+4, false))
			y += 24
		}
	}
	if len(segments) > 0 {
		items = append(items, scene.Mesh{
			ID: "record:" + mesh.ID + ":dividers", Position: mesh.Position,
			Geometry: scene.LinesGeometry{Points: points, Segments: segments, Width: 1},
			Material: scene.StandardMaterial{Color: "#3e5362", Emissive: 1},
			Drift:    mesh.Drift, DriftSpeed: mesh.DriftSpeed, DriftPhase: mesh.DriftPhase,
			Spin: mesh.Spin,
		})
	}
	return items
}

// Native labels use a system font. Reserve a small width margin around the
// bundled font's measured advances, and a full em for unsupported glyphs.
// Truncate runes rather than bytes; the full string stays in AriaLabel.
func recordLabelText(text string, size float64) string {
	text = strings.Join(strings.Fields(text), " ")
	width := func(r rune) float64 {
		if glyph, ok := font.Glyphs[r]; ok {
			return glyph.Advance * size / font.EmSize * 1.1
		}
		return size * 1.1
	}
	used, cut := 0.0, len(text)
	for index, r := range text {
		used += width(r)
		if used > 320-size*1.1 && cut == len(text) {
			cut = index
		}
		if used > 320 {
			return text[:cut] + "…"
		}
	}
	return text
}
