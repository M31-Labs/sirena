package svg

import (
	"bytes"
	"fmt"
	"m31labs.dev/sirena"
	"strings"
)

func writeCompartments(b *bytes.Buffer, np *sirena.NodePlacement) {
	r := np.Bounds
	writeLabel(b, np.Node.DisplayLabel(), sirena.Point{X: r.Center().X, Y: r.Min.Y + 20})
	y := r.Min.Y + 40
	for _, key := range []string{"fields", "methods"} {
		value, _ := np.Node.Metadata[key].(sirena.String)
		var rows []string
		for _, row := range strings.Split(value.Value, ";") {
			if row = strings.TrimSpace(row); row != "" {
				rows = append(rows, row)
			}
		}
		if len(rows) == 0 {
			continue
		}
		fmt.Fprintf(b, `<path d="M%s %sH%s" fill="none" stroke="var(--sirena-stroke)"/>`, num(r.Min.X), num(y-8), num(r.Max.X))
		for _, row := range rows {
			writeLabel(b, row, sirena.Point{X: r.Min.X + 12 + labelHalf(row), Y: y + 4})
			y += 24
		}
	}
}

func writeStateMarker(b *bytes.Buffer, np *sirena.NodePlacement) {
	value, _ := np.Node.Metadata["state"].(sirena.String)
	if value.Value != "initial" && value.Value != "final" {
		return
	}
	x, y := np.Bounds.Min.X+14, np.Bounds.Center().Y
	fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="4" fill="var(--sirena-fg)"/>`, num(x), num(y))
	if value.Value == "final" {
		fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="7" fill="none" stroke="var(--sirena-stroke)"/>`, num(x), num(y))
	}
}
