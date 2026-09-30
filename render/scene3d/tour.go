package scene3d

import (
	"fmt"
	"m31labs.dev/sirena"
)

func tourSteps(lr *sirena.LayoutResult, kind string) ([]Step, error) {
	if kind != "nodes" && kind != "relationships" {
		return nil, fmt.Errorf("sirena scene3d: tour must be nodes or relationships")
	}
	ids := map[string]string{}
	for _, p := range lr.NodePlacements {
		if p == nil || p.Node == nil {
			continue
		}
		id := p.Node.Name
		if sid, ok := p.Node.Metadata["sid"].(sirena.String); ok && sid.Value != "" {
			id = sid.Value
		}
		ids[p.Node.Name] = id
	}
	scale, z := 1.2, .4
	steps := []Step{{Label: "Overview"}}
	add := func(label string, names []string) error {
		if len(steps) >= 127 {
			return fmt.Errorf("sirena scene3d: tours support at most 126 focus steps; select a smaller view")
		}
		step := Step{Label: label}
		seen := map[string]bool{}
		for _, name := range names {
			id, ok := ids[name]
			if !ok {
				return fmt.Errorf("sirena scene3d: tour participant %q is not positioned", name)
			}
			if !seen[id] {
				step.Patches = append(step.Patches, Patch{Target: id, Scale: &scale, Z: &z})
				seen[id] = true
			}
		}
		steps = append(steps, step)
		return nil
	}
	if kind == "nodes" {
		for _, p := range lr.NodePlacements {
			if p == nil || p.Node == nil {
				continue
			}
			if err := add(p.Node.DisplayLabel(), []string{p.Node.Name}); err != nil {
				return nil, err
			}
		}
	} else {
		for _, route := range lr.EdgeRoutes {
			if route == nil || route.Edge == nil {
				continue
			}
			edge := route.Edge
			label := edge.Label
			if label == "" {
				label = edge.From + " → " + edge.To
			}
			if err := add(label, []string{edge.From, edge.To}); err != nil {
				return nil, err
			}
		}
	}
	if len(steps) == 1 {
		return nil, fmt.Errorf("sirena scene3d: tour has no %s to focus", kind)
	}
	return append(steps, Step{Label: "Overview"}), nil
}
