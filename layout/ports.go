package layout

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"m31labs.dev/sirena"
)

// Explicit attachment metadata uses "right" or "right:0.25". An omitted
// offset retains deterministic distribution among relationships on that side.
func parsePort(value sirena.Value) (sirena.PortSide, *float64, error) {
	var text string
	switch v := value.(type) {
	case sirena.String:
		text = v.Value
	case sirena.Ident:
		text = v.Value
	default:
		return 0, nil, fmt.Errorf("port must be a side or a side:offset string")
	}
	name, raw, has := strings.Cut(text, ":")
	var side sirena.PortSide
	switch name {
	case "top":
		side = sirena.PortSideTop
	case "right":
		side = sirena.PortSideRight
	case "bottom":
		side = sirena.PortSideBottom
	case "left":
		side = sirena.PortSideLeft
	default:
		return 0, nil, fmt.Errorf("port side must be top, right, bottom or left")
	}
	if !has {
		return side, nil, nil
	}
	offset, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(offset) || math.IsInf(offset, 0) || offset < 0 || offset > 1 {
		return 0, nil, fmt.Errorf("port offset must be finite and between 0 and 1")
	}
	return side, &offset, nil
}

func explicitPort(edge *sirena.Edge, key string) (sirena.PortSide, *float64, bool) {
	value, ok := edge.Metadata[key]
	if !ok {
		return 0, nil, false
	}
	side, offset, err := parsePort(value)
	return side, offset, err == nil
}

func validatePorts(rv *sirena.ResolvedView) error {
	for _, edge := range rv.Edges {
		if edge == nil {
			continue
		}
		for _, key := range []string{"source_port", "target_port"} {
			if value, ok := edge.Metadata[key]; ok {
				if _, _, err := parsePort(value); err != nil {
					return fmt.Errorf("sirena: %s on %s->%s: %w", key, edge.From, edge.To, err)
				}
			}
		}
	}
	return nil
}
