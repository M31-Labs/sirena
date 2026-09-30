package scene3d

import (
	"fmt"
	"m31labs.dev/sirena"
	"sort"
)

// Semantic beats expand to independent absolute poses. A seek never depends on
// whether earlier reveal/focus/trace beats have been visited.
func choreographySteps(lr *sirena.LayoutResult, steps []Step) ([]Step, error) {
	ids := map[string]string{}
	var nodes []string
	for _, p := range lr.NodePlacements {
		if p == nil || p.Node == nil {
			continue
		}
		id := p.Node.Name
		if sid, ok := p.Node.Metadata["sid"].(sirena.String); ok && sid.Value != "" {
			id = sid.Value
		}
		ids[p.Node.Name] = id
		ids[id] = id
		nodes = append(nodes, id)
	}
	edgeEnds := map[string][2]string{}
	ambiguous := map[string]bool{}
	for i, r := range lr.EdgeRoutes {
		if r == nil || r.Edge == nil {
			continue
		}
		id := fmt.Sprintf("edge:%d", i)
		if sid, ok := r.Edge.Metadata["sid"].(sirena.String); ok && sid.Value != "" {
			id = sid.Value
		}
		ids[id] = id
		alias := r.Edge.From + "->" + r.Edge.To
		if _, ok := ids[alias]; ok {
			ambiguous[alias] = true
		}
		ids[alias] = id
		edgeEnds[id] = [2]string{ids[r.Edge.From], ids[r.Edge.To]}
	}
	resolve := func(name string) (string, error) {
		id, ok := ids[name]
		if !ok || ambiguous[name] {
			return "", fmt.Errorf("sirena scene3d: unknown or ambiguous choreography target %q; use a unique declaration name or sid", name)
		}
		return id, nil
	}
	result := make([]Step, len(steps))
	for i, original := range steps {
		step := original
		step.Patches = append([]Patch(nil), original.Patches...)
		patches := map[string]Patch{}
		for _, patch := range step.Patches {
			if _, ok := patches[patch.Target]; ok {
				return nil, fmt.Errorf("sirena scene3d: duplicate step target %q", patch.Target)
			}
			patches[patch.Target] = patch
		}
		scale, z := 1.2, .4
		for _, name := range step.Focus {
			id, err := resolve(name)
			if err != nil {
				return nil, err
			}
			p := patches[id]
			p.Target = id
			p.Scale = &scale
			p.Z = &z
			patches[id] = p
		}
		if step.Reveal != nil {
			visible := map[string]bool{}
			for _, name := range step.Reveal {
				id, err := resolve(name)
				if err != nil {
					return nil, err
				}
				visible[id] = true
			}
			for _, id := range nodes {
				p := patches[id]
				p.Target = id
				v := visible[id]
				p.Visible = &v
				patches[id] = p
			}
			for id, ends := range edgeEnds {
				p := patches[id]
				p.Target = id
				v := visible[id] || (visible[ends[0]] && visible[ends[1]])
				p.Visible = &v
				patches[id] = p
			}
		}
		for _, name := range step.Trace {
			id, err := resolve(name)
			if err != nil {
				return nil, err
			}
			if _, ok := edgeEnds[id]; !ok {
				return nil, fmt.Errorf("sirena scene3d: trace target %q must be a relationship", name)
			}
			p := patches[id]
			p.Target = id
			color := "#ffd27e"
			p.Color = &color
			patches[id] = p
		}
		step.Patches = nil
		// Timeline builder sorts targets; this order only needs to be deterministic.
		for _, id := range sortedPatchIDs(patches) {
			step.Patches = append(step.Patches, patches[id])
		}
		step.Focus = nil
		step.Reveal = nil
		step.Trace = nil
		result[i] = step
	}
	return result, nil
}

func sortedPatchIDs(patches map[string]Patch) []string {
	var ids []string
	for id := range patches {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
