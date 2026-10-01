package svg

import (
	"fmt"
	"html"
	"math"
	"strings"

	"m31labs.dev/sirena"
)

func seriesColor(index int) string { return chartColors[index%len(chartColors)] }
func pointString(points []sirena.Point) string {
	var b strings.Builder
	for _, p := range points {
		fmt.Fprintf(&b, "%s,%s ", num(p.X), num(p.Y))
	}
	return strings.TrimSpace(b.String())
}
func chartIdentity(e *sirena.Element) string {
	if sid, ok := e.Metadata["sid"].(sirena.String); ok && sid.Value != "" {
		return sid.Value
	}
	return e.Name
}
func plotSeries(e *sirena.Element) string {
	if v, ok := e.Metadata["series"].(sirena.String); ok {
		return v.Value
	}
	return "Data"
}
func writeLegend(b *svgBuffer, series []sirena.ChartSeries, x, y float64) {
	for i, s := range series {
		row := y + float64(i)*26
		fmt.Fprintf(b, `<rect x="%s" y="%s" width="12" height="12" rx="3" fill="%s"/>`, num(x), num(row-6), seriesColor(i))
		writeLabel(b, s.Name, sirena.Point{X: x + 24 + labelHalf(s.Name), Y: row})
	}
}
func writePlot(b *svgBuffer, lr *sirena.LayoutResult) {
	p := lr.Plot
	r := p.Bounds
	for i := 0; i <= 5; i++ {
		fraction := float64(i) / 5
		x := r.Min.X + fraction*r.Width()
		y := r.Max.Y - fraction*r.Height()
		fmt.Fprintf(b, `<path class="chart-grid" d="M%s %sV%sM%s %sH%s" fill="none" stroke="var(--sirena-stroke)" stroke-opacity=".18"/>`, num(x), num(r.Min.Y), num(r.Max.Y), num(r.Min.X), num(y), num(r.Max.X))
		writeLabel(b, fmt.Sprintf("%.4g", p.XMin+fraction*(p.XMax-p.XMin)), sirena.Point{X: x, Y: r.Max.Y + 24})
		writeLabel(b, fmt.Sprintf("%.4g", p.YMin+fraction*(p.YMax-p.YMin)), sirena.Point{X: r.Min.X - 48, Y: y})
	}
	fmt.Fprintf(b, `<path class="chart-axis" d="M%s %sV%sH%s" fill="none" stroke="var(--sirena-stroke)"/>`, num(r.Min.X), num(r.Min.Y), num(r.Max.Y), num(r.Max.X))
	indices := map[string]int{}
	for i, s := range p.Series {
		indices[s.Name] = i
		if lr.Diagram == "line" {
			fmt.Fprintf(b, `<polyline data-sirena-series="%s" data-morph-id="series:%s" points="%s" fill="none" stroke="%s" stroke-width="3" stroke-linejoin="round"/>`, html.EscapeString(s.Name), html.EscapeString(s.Name), pointString(s.Points), seriesColor(i))
		}
	}
	for _, np := range lr.NodePlacements {
		e := np.Node
		point := np.Bounds.Center()
		x := e.Metadata["x"].(sirena.Number).Value
		y := e.Metadata["y"].(sirena.Number).Value
		label := fmt.Sprintf("%s: (%s, %s)", e.DisplayLabel(), num(x), num(y))
		fmt.Fprintf(b, `<g class="chart-item" data-sirena-id="%s" data-morph-id="%s" role="img" aria-label="%s"><title>%s</title><circle cx="%s" cy="%s" r="5" fill="%s" stroke="var(--sirena-bg)" stroke-width="1.5"/></g>`, html.EscapeString(chartIdentity(e)), html.EscapeString(chartIdentity(e)), html.EscapeString(label), html.EscapeString(label), num(point.X), num(point.Y), seriesColor(indices[plotSeries(e)]))
	}
	writeLegend(b, p.Series, r.Min.X, r.Max.Y+52)
}
func writeRadar(b *svgBuffer, lr *sirena.LayoutResult) {
	r := lr.Radar
	n := len(r.Axes)
	polar := func(index int, radius float64) sirena.Point {
		a := -math.Pi/2 + float64(index)*2*math.Pi/float64(n)
		return sirena.Point{X: r.Center.X + math.Cos(a)*radius, Y: r.Center.Y + math.Sin(a)*radius}
	}
	for ring := 1; ring <= 4; ring++ {
		points := make([]sirena.Point, n)
		for i := range points {
			points[i] = polar(i, r.Radius*float64(ring)/4)
		}
		fmt.Fprintf(b, `<polygon class="chart-grid" points="%s" fill="none" stroke="var(--sirena-stroke)" stroke-opacity=".25"/>`, pointString(points))
		writeLabel(b, num(r.Maximum*float64(ring)/4), sirena.Point{X: r.Center.X + 24, Y: r.Center.Y - r.Radius*float64(ring)/4})
	}
	for i, label := range r.Axes {
		p := polar(i, r.Radius)
		fmt.Fprintf(b, `<path d="M%s %sL%s %s" stroke="var(--sirena-stroke)" stroke-opacity=".25"/>`, num(r.Center.X), num(r.Center.Y), num(p.X), num(p.Y))
		writeLabel(b, label, polar(i, r.Radius+32))
	}
	for i, s := range r.Series {
		fmt.Fprintf(b, `<polygon data-sirena-series="%s" data-morph-id="series:%s" points="%s" fill="%s" fill-opacity=".15" stroke="%s" stroke-width="2.5"/>`, html.EscapeString(s.Name), html.EscapeString(s.Name), pointString(s.Points), seriesColor(i), seriesColor(i))
	}
	indices := map[string]int{}
	for i, s := range r.Series {
		indices[s.Name] = i
	}
	for _, np := range lr.NodePlacements {
		e := np.Node
		p := np.Bounds.Center()
		v := e.Metadata["value"].(sirena.Number).Value
		label := fmt.Sprintf("%s: %s", e.DisplayLabel(), num(v))
		fmt.Fprintf(b, `<g data-sirena-id="%s" data-morph-id="%s" role="img" aria-label="%s"><title>%s</title><circle cx="%s" cy="%s" r="4" fill="%s"/></g>`, html.EscapeString(chartIdentity(e)), html.EscapeString(chartIdentity(e)), html.EscapeString(label), html.EscapeString(label), num(p.X), num(p.Y), seriesColor(indices[plotSeries(e)]))
	}
	writeLegend(b, r.Series, 96, lr.Bounds.Max.Y-float64(len(r.Series))*26)
}
func writeFlows(b *svgBuffer, lr *sirena.LayoutResult) {
	for i, f := range lr.Flows {
		a, z := f.From, f.To
		mid := (a.X + z.X) / 2
		half := f.Width / 2
		id := fmt.Sprintf("flow:%s->%s:%d", f.Edge.From, f.Edge.To, i)
		label := fmt.Sprintf("%s → %s: %s", f.Edge.From, f.Edge.To, num(f.Value))
		fmt.Fprintf(b, `<g data-sirena-id="%s" data-morph-id="%s" role="img" aria-label="%s"><title>%s</title><path d="M%s %sC%s %s %s %s %s %sL%s %sC%s %s %s %s %s %sZ" fill="%s" fill-opacity=".5"/></g>`, html.EscapeString(id), html.EscapeString(id), html.EscapeString(label), html.EscapeString(label), num(a.X), num(a.Y-half), num(mid), num(a.Y-half), num(mid), num(z.Y-half), num(z.X), num(z.Y-half), num(z.X), num(z.Y+half), num(mid), num(z.Y+half), num(mid), num(a.Y+half), num(a.X), num(a.Y+half), seriesColor(i))
	}
}
