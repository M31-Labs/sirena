package svg

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"

	"m31labs.dev/sirena"
	"m31labs.dev/sirena/render/svg/font"
)

const (
	canvasMargin = 16.0 // padding around the layout bounds
	labelSize    = 14.0 // label font size in user units
)

// Render turns a positioned LayoutResult into deterministic SVG bytes. A
// nil theme falls back to the default. Output ordering is fixed
// (boundaries by depth then position, edges, nodes, summaries) and labels
// are emitted as glyph paths, so the same layout always renders to
// identical bytes.
func Render(lr *sirena.LayoutResult, theme *Theme) ([]byte, error) {
	if lr == nil {
		return nil, fmt.Errorf("svg: nil layout result")
	}
	if theme == nil {
		t, err := ThemeForName(DefaultThemeName)
		if err != nil {
			return nil, err
		}
		theme = t
	}

	bounds := svgBounds(lr)
	vbX := bounds.Min.X - canvasMargin
	vbY := bounds.Min.Y - canvasMargin
	w := bounds.Width() + 2*canvasMargin
	h := bounds.Height() + 2*canvasMargin

	var b svgBuffer
	b.glyphs = make(map[string]string)
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="%s %s %s %s" width="%s" height="%s" data-sirena-theme="%s">`+"\n",
		num(vbX), num(vbY), num(w), num(h), num(w), num(h), themeScope(theme))
	writeStyle(&b, theme)
	markerPrefix := "sirena-arrow-" + themeScope(theme)
	writeMarkers(&b, lr.EdgeRoutes, markerPrefix)

	if lr.Diagram == "gantt" {
		writeCalendarGrid(&b, lr)
	}
	writeLifelines(&b, lr.Lifelines)
	writeBoundaries(&b, lr.BoundaryPlacements)
	if lr.Diagram == "sankey" {
		writeFlows(&b, lr)
	} else {
		writeEdges(&b, lr.EdgeRoutes, markerPrefix)
	}
	if lr.Plot != nil {
		writePlot(&b, lr)
	} else if lr.Radar != nil {
		writeRadar(&b, lr)
	} else if lr.Diagram == "bar" || lr.Diagram == "pie" {
		writeCharts(&b, lr)
	} else {
		writeNodes(&b, lr.NodePlacements, lr.Diagram, lr.ChartLabelX)
	}
	if lr.Diagram == "gantt" {
		writeCalendar(&b, lr)
	}

	writeSummaries(&b, lr.SummaryPlacements)

	b.writeGlyphs()
	b.WriteString("</svg>\n")
	return b.Bytes(), nil
}

// Include routed geometry and measured captions in the viewport. Architecture
// layout bounds describe boxes; a relationship caption can extend beyond them.
func svgBounds(lr *sirena.LayoutResult) sirena.Rect {
	bounds := lr.Bounds
	if lr.Diagram == "gantt" {
		bounds.Max.X = max(bounds.Max.X, lr.ChartBaseline+690)
	}
	include := func(p sirena.Point) {
		bounds.Min.X, bounds.Min.Y = min(bounds.Min.X, p.X), min(bounds.Min.Y, p.Y)
		bounds.Max.X, bounds.Max.Y = max(bounds.Max.X, p.X), max(bounds.Max.Y, p.Y)
	}
	for _, edge := range lr.EdgeRoutes {
		if edge == nil {
			continue
		}
		for _, point := range edge.Points {
			include(point)
		}
		if label := edge.Label; label != nil && label.Text != "" {
			half := labelHalf(label.Text)
			include(sirena.Point{X: label.Anchor.X - half, Y: label.Anchor.Y - labelSize})
			include(sirena.Point{X: label.Anchor.X + half, Y: label.Anchor.Y + labelSize})
		}
	}
	return bounds
}

// writeStyle emits the :root token declarations (sorted) followed by the
// fixed class-rule set.
func writeStyle(b *svgBuffer, theme *Theme) {
	b.WriteString("<style>\n")
	selector := `svg[data-sirena-theme="` + themeScope(theme) + `"]`
	b.WriteString(selector + " { background: var(--sirena-bg);\n")
	names := make([]string, 0, len(theme.Tokens))
	for k := range theme.Tokens {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		fmt.Fprintf(b, "  %s: %s;\n", k, escapeText(theme.Tokens[k]))
	}
	b.WriteString("}\n")
	for _, line := range strings.Split(classRules, "\n") {
		if strings.TrimSpace(line) != "" {
			b.WriteString(selector + " " + line + "\n")
		}
	}
	b.WriteString("</style>\n")
}

// boundaryAtDepth pairs a boundary placement with its nesting depth for
// back-to-front ordering.
type boundaryAtDepth struct {
	bp    *sirena.BoundaryPlacement
	depth int
}

func writeLifelines(b *svgBuffer, lines []sirena.LifelinePlacement) {
	for _, line := range lines {
		fmt.Fprintf(b, `<path class="lifeline" d="M%s %sL%s %s" fill="none" stroke="var(--sirena-stroke)" stroke-width="1" stroke-dasharray="4 5"/>`, num(line.From.X), num(line.From.Y), num(line.To.X), num(line.To.Y))
	}
}

func writeBoundaries(b *svgBuffer, bps []*sirena.BoundaryPlacement) {
	var flat []boundaryAtDepth
	var walk func([]*sirena.BoundaryPlacement, int)
	walk = func(list []*sirena.BoundaryPlacement, depth int) {
		for _, bp := range list {
			flat = append(flat, boundaryAtDepth{bp, depth})
			walk(bp.Children, depth+1)
		}
	}
	walk(bps, 0)

	sort.SliceStable(flat, func(i, j int) bool {
		if flat[i].depth != flat[j].depth {
			return flat[i].depth < flat[j].depth
		}
		if flat[i].bp.Bounds.Min.X != flat[j].bp.Bounds.Min.X {
			return flat[i].bp.Bounds.Min.X < flat[j].bp.Bounds.Min.X
		}
		return flat[i].bp.Bounds.Min.Y < flat[j].bp.Bounds.Min.Y
	})

	for _, f := range flat {
		kind := "kind-unknown"
		name := "region"
		if f.bp.Boundary != nil {
			kind = "kind-" + f.bp.Boundary.Kind.String()
			name = f.bp.Boundary.DisplayLabel()
		}
		r := f.bp.Bounds
		fmt.Fprintf(b, `<g class="boundary %s">`, kind)
		fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s" rx="6"/>`,
			num(r.Min.X), num(r.Min.Y), num(r.Width()), num(r.Height()))
		// Boundary label sits just inside the top-left corner.
		writeLabel(b, name, sirena.Point{X: r.Min.X + canvasMargin + labelHalf(name), Y: r.Min.Y + labelSize})
		b.WriteString("</g>\n")
	}
}

func writeNodes(b *svgBuffer, nps []*sirena.NodePlacement, diagram string, labelX float64) {
	sorted := append([]*sirena.NodePlacement(nil), nps...)
	sort.SliceStable(sorted, func(i, j int) bool { return rectLess(sorted[i].Bounds, sorted[j].Bounds) })
	for _, np := range sorted {
		kind := "kind-unknown"
		name := ""
		if np.Node != nil {
			kind = "kind-" + np.Node.Kind.String()
			name = np.Node.DisplayLabel()
		}
		r := np.Bounds
		identity := ""
		if np.Node != nil {
			identity = np.Node.Name
			if sid, ok := np.Node.Metadata["sid"].(sirena.String); ok && sid.Value != "" {
				identity = sid.Value
			}
		}
		fmt.Fprintf(b, `<g class="node %s" data-sirena-id="%s" data-morph-id="%s">`, kind, html.EscapeString(identity), html.EscapeString(identity))
		radius := 4.0
		if diagram == "state" {
			radius = r.Height() / 2
		}
		fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s" rx="%s"/>`,
			num(r.Min.X), num(r.Min.Y), num(r.Width()), num(r.Height()), num(radius))
		if diagram == "state" {
			writeStateMarker(b, np)
		}
		if diagram == "class" || diagram == "er" {
			writeCompartments(b, np)
		} else if diagram == "timeline" || diagram == "gantt" {
			writeLabel(b, name, sirena.Point{X: labelX - labelHalf(name), Y: r.Center().Y})
		} else {
			center := r.Center()
			if diagram == "state" {
				center.X += 8
			}
			writeLabel(b, name, center)
		}
		b.WriteString("</g>\n")
	}
}

func writeSummaries(b *svgBuffer, sps []*sirena.SummaryPlacement) {
	sorted := append([]*sirena.SummaryPlacement(nil), sps...)
	sort.SliceStable(sorted, func(i, j int) bool { return rectLess(sorted[i].Bounds, sorted[j].Bounds) })
	for _, sp := range sorted {
		label := ""
		if sp.Summary != nil {
			label = sp.Summary.Label
		}
		r := sp.Bounds
		b.WriteString(`<g class="summary">`)
		fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s" rx="4"/>`,
			num(r.Min.X), num(r.Min.Y), num(r.Width()), num(r.Height()))
		writeLabel(b, label, r.Center())
		b.WriteString("</g>\n")
	}
}

func writeEdges(b *svgBuffer, routes []*sirena.EdgeRoute, marker string) {
	sorted := append([]*sirena.EdgeRoute(nil), routes...)
	sort.SliceStable(sorted, func(i, j int) bool { return edgeRouteLess(sorted[i], sorted[j]) })
	occurrences := map[string]int{}
	for _, er := range sorted {
		if len(er.Points) < 2 {
			continue
		}
		kind := "kind-flow"
		if er.Edge != nil {
			kind = "kind-" + er.Edge.Kind.String()
		}
		id := "edge"
		if er.Edge != nil {
			id = fmt.Sprintf("edge:%s:%s:%d:%d", er.Edge.From, er.Edge.To, er.Edge.Kind, er.Edge.Direction)
		}
		occurrence := occurrences[id]
		occurrences[id]++
		fmt.Fprintf(b, `<g class="edge %s" data-morph-id="%s:%d">`, kind, html.EscapeString(id), occurrence)
		b.WriteString(`<path`)
		if er.Edge != nil {
			markerID := marker + "-" + edgeMarkerKind(er.Edge)
			if er.Edge.Direction == sirena.DirForward || er.Edge.Direction == sirena.DirBidirectional {
				fmt.Fprintf(b, ` marker-end="url(#%s)"`, markerID)
			}
			if er.Edge.Direction == sirena.DirReverse || er.Edge.Direction == sirena.DirBidirectional {
				fmt.Fprintf(b, ` marker-start="url(#%s)"`, markerID)
			}
		}
		b.WriteString(` d="`)
		for i, p := range er.Points {
			cmd := "L"
			if i == 0 {
				cmd = "M"
			}
			fmt.Fprintf(b, "%s%s %s", cmd, num(p.X), num(p.Y))
		}
		b.WriteString(`"/>`)
		if er.Label != nil {
			writeLabel(b, er.Label.Text, er.Label.Anchor)
		}
		b.WriteString("</g>\n")
	}
}

func edgeMarkerKind(edge *sirena.Edge) string {
	if edge == nil || edge.Kind < sirena.EdgeKindCalls || edge.Kind > sirena.EdgeKindFlow {
		return "flow"
	}
	return edge.Kind.String()
}

func writeMarkers(b *svgBuffer, routes []*sirena.EdgeRoute, prefix string) {
	used := map[string]bool{}
	for _, route := range routes {
		if route != nil && route.Edge != nil && len(route.Points) >= 2 {
			used[edgeMarkerKind(route.Edge)] = true
		}
	}
	kinds := make([]string, 0, len(used))
	for kind := range used {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	b.WriteString("<defs>")
	for _, kind := range kinds {
		token := strings.ReplaceAll(kind, "_", "-")
		fmt.Fprintf(b, `<marker id="%s-%s" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0 0L10 5L0 10Z" fill="var(--sirena-edge-stroke-%s)"/></marker>`, prefix, kind, token)
	}
	b.WriteString("</defs>")
}

// writeLabel emits a label as a group of glyph <path> elements centered
// horizontally on center.X with the baseline near center.Y. Each glyph is
// translated to the pen position and scaled from font units to user units.
// The bundled glyph outlines are already in SVG's Y-down orientation (sfnt's
// native output), so no Y flip is applied — scaling by a positive factor
// keeps text upright.
func writeLabel(b *svgBuffer, text string, center sirena.Point) {
	if text == "" {
		return
	}
	id := b.labelID(text)
	fmt.Fprintf(b, `<g class="label" role="img" aria-label="%s"><title>%s</title><use href="#%s" transform="translate(%s %s)"/>`, html.EscapeString(text), html.EscapeString(text), id, num(center.X), num(center.Y))
	b.WriteString(`</g>`)
}

// labelHalf returns half a label's rendered width, used to offset a
// boundary label so its center lands at the intended anchor.
func labelHalf(text string) float64 {
	return font.Measure(text) * (labelSize / font.EmSize) / 2
}

func rectLess(a, b sirena.Rect) bool {
	if a.Min.X != b.Min.X {
		return a.Min.X < b.Min.X
	}
	return a.Min.Y < b.Min.Y
}

func edgeRouteLess(a, b *sirena.EdgeRoute) bool {
	ae, be := a.Edge, b.Edge
	if ae == nil || be == nil {
		return ae != nil // non-nil edges sort before nil
	}
	if ae.From != be.From {
		return ae.From < be.From
	}
	if ae.To != be.To {
		return ae.To < be.To
	}
	if ae.Kind != be.Kind {
		return ae.Kind < be.Kind
	}
	return ae.Direction < be.Direction
}

// num formats a float without exponent or redundant zeros.
func num(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

// classRules is the fixed CSS rule set. It is a constant string so output
// is byte-stable; the per-kind rules map class names to allowlisted
// tokens declared in writeStyle.
const classRules = `.boundary rect { fill: none; stroke: var(--sirena-stroke); stroke-width: 1; stroke-dasharray: 4 3; }
.boundary.kind-trust rect { fill: var(--sirena-boundary-fill-trust); }
.boundary.kind-network rect { fill: var(--sirena-boundary-fill-network); }
.boundary.kind-deployment rect { fill: var(--sirena-boundary-fill-deployment); }
.boundary.kind-team rect { fill: var(--sirena-boundary-fill-team); }
.node rect { stroke: var(--sirena-stroke-strong); stroke-width: 1.5; }
.node.kind-service rect { fill: var(--sirena-element-fill-service); }
.node.kind-database rect { fill: var(--sirena-element-fill-database); }
.node.kind-queue rect { fill: var(--sirena-element-fill-queue); }
.node.kind-cache rect { fill: var(--sirena-element-fill-cache); }
.node.kind-job rect { fill: var(--sirena-element-fill-job); }
.node.kind-external rect { fill: var(--sirena-element-fill-external); }
.node.kind-client rect { fill: var(--sirena-element-fill-client); }
.node.kind-gateway rect { fill: var(--sirena-element-fill-gateway); }
.node.kind-node rect { fill: var(--sirena-element-fill-node); }
.summary rect { fill: var(--sirena-element-fill-node); stroke: var(--sirena-stroke-strong); stroke-width: 1.5; stroke-dasharray: 2 2; }
.edge > path { fill: none; stroke: var(--sirena-edge-stroke-flow); stroke-width: 1.5; }
.edge.kind-calls > path { stroke: var(--sirena-edge-stroke-calls); }
.edge.kind-reads > path { stroke: var(--sirena-edge-stroke-reads); }
.edge.kind-writes > path { stroke: var(--sirena-edge-stroke-writes); }
.edge.kind-publishes > path { stroke: var(--sirena-edge-stroke-publishes); }
.edge.kind-subscribes > path { stroke: var(--sirena-edge-stroke-subscribes); }
.edge.kind-depends_on > path { stroke: var(--sirena-edge-stroke-depends-on); }
.label { fill: var(--sirena-label-fill); stroke: none; }
`

func themeScope(theme *Theme) string {
	data, _ := json.Marshal(theme.Tokens)
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:8])
}

// Each outline is emitted once per document, instead of once per character.
// Content-derived IDs keep independent diagrams safe when embedded together.
type svgBuffer struct {
	bytes.Buffer
	glyphs      map[string]string
	labels      map[string]string
	labelBodies map[string]string
}

func (b *svgBuffer) glyphID(path string) string {
	if b.glyphs == nil {
		b.glyphs = make(map[string]string)
	}
	if id, ok := b.glyphs[path]; ok {
		return id
	}
	sum := sha256.Sum256([]byte(path))
	id := fmt.Sprintf("sirena-glyph-%x", sum[:12])
	b.glyphs[path] = id
	return id
}

// Repeated labels share their positioned glyph run as well as their outlines.
// Definitions are centered at the origin, keeping instance transforms compact.
func (b *svgBuffer) labelID(text string) string {
	if b.labels == nil {
		b.labels = map[string]string{}
		b.labelBodies = map[string]string{}
	}
	if id, ok := b.labels[text]; ok {
		return id
	}
	sum := sha256.Sum256([]byte(text))
	id := fmt.Sprintf("sirena-label-%x", sum[:12])
	b.labels[text] = id
	scale := labelSize / font.EmSize
	penX := -font.Measure(text) * scale / 2
	var run strings.Builder
	for _, r := range text {
		g := font.Lookup(r)
		if g.Path != "" {
			fmt.Fprintf(&run, `<use href="#%s" transform="translate(%s %s) scale(%s %s)"/>`, b.glyphID(g.Path), num(penX), num(labelSize*.32), num(scale), num(scale))
		}
		penX += g.Advance * scale
	}
	b.labelBodies[text] = run.String()
	return id
}
func (b *svgBuffer) writeGlyphs() {
	paths := make([]string, 0, len(b.glyphs))
	for path := range b.glyphs {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	b.WriteString("<defs>")
	for _, path := range paths {
		fmt.Fprintf(&b.Buffer, `<path id="%s" d="%s"/>`, b.glyphs[path], path)
	}
	texts := make([]string, 0, len(b.labels))
	for text := range b.labels {
		texts = append(texts, text)
	}
	sort.Strings(texts)
	for _, text := range texts {
		fmt.Fprintf(&b.Buffer, `<g id="%s">%s</g>`, b.labels[text], b.labelBodies[text])
	}
	b.WriteString("</defs>\n")
}
