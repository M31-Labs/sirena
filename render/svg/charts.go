package svg

import (
	"fmt"
	"html"
	"math"
	"time"

	"m31labs.dev/sirena"
)

var chartColors = []string{"#6b9cff", "#ff8a65", "#64d8b4", "#b69cff", "#ffd166", "#ec83b4", "#59cbdc", "#a3c966"}

func writeCharts(b *svgBuffer, lr *sirena.LayoutResult) {
	if lr.Diagram == "bar" {
		fmt.Fprintf(b, `<path class="chart-axis" d="M%s 24V%s" stroke="var(--sirena-stroke)"/>`, num(lr.ChartBaseline), num(lr.Bounds.Max.Y))
	}
	for i, np := range lr.NodePlacements {
		e := np.Node
		v, _ := e.Metadata["value"].(sirena.Number)
		r := np.Bounds
		color := chartColors[i%len(chartColors)]
		identity := e.Name
		if sid, ok := e.Metadata["sid"].(sirena.String); ok && sid.Value != "" {
			identity = sid.Value
		}
		fmt.Fprintf(b, `<g class="chart-item" data-sirena-id="%s" data-morph-id="%s" role="img" aria-label="%s: %s"><title>%s: %s</title>`, html.EscapeString(identity), html.EscapeString(identity), html.EscapeString(e.DisplayLabel()), num(v.Value), html.EscapeString(e.DisplayLabel()), num(v.Value))
		if lr.Diagram == "pie" {
			slice := lr.PieSlices[i]
			if slice.Fraction >= 1 {
				fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="%s" fill="%s"/>`, num(slice.Center.X), num(slice.Center.Y), num(slice.Radius), color)
			} else if slice.Fraction > 0 {
				sx, sy := slice.Center.X+slice.Radius*math.Cos(slice.Start), slice.Center.Y+slice.Radius*math.Sin(slice.Start)
				ex, ey := slice.Center.X+slice.Radius*math.Cos(slice.End), slice.Center.Y+slice.Radius*math.Sin(slice.End)
				large := 0
				if slice.Fraction > .5 {
					large = 1
				}
				fmt.Fprintf(b, `<path d="M%s %sL%s %sA%s %s 0 %d 1 %s %sZ" fill="%s" stroke="var(--sirena-bg)" stroke-width="2"/>`, num(slice.Center.X), num(slice.Center.Y), num(sx), num(sy), num(slice.Radius), num(slice.Radius), large, num(ex), num(ey), color)
			}
			fmt.Fprintf(b, `<rect x="%s" y="%s" width="12" height="12" rx="3" fill="%s"/>`, num(r.Min.X), num(r.Center().Y-6), color)
			label := fmt.Sprintf("%s — %s (%.1f%%)", e.DisplayLabel(), num(v.Value), slice.Fraction*100)
			writeLabel(b, label, sirena.Point{X: r.Min.X + 24 + labelHalf(label), Y: r.Center().Y})
		} else {
			fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s" rx="3" fill="%s"/>`, num(r.Min.X), num(r.Min.Y), num(r.Width()), num(r.Height()), color)
			writeLabel(b, e.DisplayLabel(), sirena.Point{X: lr.ChartLabelX - labelHalf(e.DisplayLabel()), Y: r.Center().Y})
			writeLabel(b, num(v.Value), sirena.Point{X: lr.Bounds.Max.X - 60, Y: r.Center().Y})
		}
		b.WriteString("</g>\n")
	}
}

func calendarTicks(lr *sirena.LayoutResult) []struct {
	x     float64
	label string
} {
	start, err := time.Parse("2006-01-02", lr.ChartStart)
	if err != nil {
		return nil
	}
	end, _ := time.Parse("2006-01-02", lr.ChartEnd)
	days := int(end.Sub(start).Hours() / 24)
	if days <= 0 {
		return nil
	}
	step := max(1, int(math.Ceil(float64(days)/5)))
	var ticks []struct {
		x     float64
		label string
	}
	for day := 0; day < days; day += step {
		ticks = append(ticks, struct {
			x     float64
			label string
		}{lr.ChartBaseline + float64(day)/float64(days)*640, start.AddDate(0, 0, day).Format("01-02")})
	}
	if len(ticks) > 1 && (lr.ChartBaseline+640-ticks[len(ticks)-1].x) < 40 {
		ticks = ticks[:len(ticks)-1]
	}
	ticks = append(ticks, struct {
		x     float64
		label string
	}{lr.ChartBaseline + 640, end.Format("01-02")})
	if len(ticks) > 0 {
		ticks[0].label = start.Format("2006-01-02")
	}
	if start.Year() != end.Year() {
		for i := range ticks {
			day := int(math.Round((ticks[i].x - lr.ChartBaseline) / 640 * float64(days)))
			ticks[i].label = start.AddDate(0, 0, day).Format("2006-01-02")
		}
	}
	return ticks
}
func writeCalendarGrid(b *svgBuffer, lr *sirena.LayoutResult) {
	for _, tick := range calendarTicks(lr) {
		fmt.Fprintf(b, `<path d="M%s 32V%s" stroke="var(--sirena-stroke)" stroke-opacity="0.2" stroke-dasharray="3 5"/>`, num(tick.x), num(lr.Bounds.Max.Y))
	}
}
func writeCalendar(b *svgBuffer, lr *sirena.LayoutResult) {
	for _, tick := range calendarTicks(lr) {
		writeLabel(b, tick.label, sirena.Point{X: tick.x, Y: 16})
	}
}
