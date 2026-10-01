package layout

import (
	"fmt"

	"m31labs.dev/sirena"
)

// Clone the included hierarchy into the union without altering source IR.
// Moving an identity between scopes requires an explicit scene pose; stable
// structural stories retain membership while adding/removing actors and edges.
func mergeStoryBoundaries(views []*sirena.ResolvedView, union *sirena.ResolvedView) error {
	boundaries := map[string]*sirena.Boundary{}
	parents := map[string]string{}
	actorParents := map[string]string{}
	assign := func(table map[string]string, name, parent string) error {
		if previous, ok := table[name]; ok && previous != parent {
			return fmt.Errorf("sirena: storyboard identity %q changes boundary membership", name)
		}
		table[name] = parent
		return nil
	}
	for _, v := range views {
		included := map[string]bool{}
		for _, b := range v.Boundaries {
			included[b.Name] = true
			if old, ok := boundaries[b.Name]; ok {
				if old.Kind != b.Kind {
					return fmt.Errorf("sirena: storyboard boundary %q changes kind", b.Name)
				}
				if labelWidth(b.DisplayLabel()) > labelWidth(old.DisplayLabel()) {
					old.Metadata = b.Metadata
				}
			} else {
				copy := *b
				copy.Children = nil
				boundaries[b.Name] = &copy
				union.Boundaries = append(union.Boundaries, &copy)
			}
		}
		localParents := map[string]string{}
		localActors := map[string]string{}
		for _, b := range v.Boundaries {
			for _, child := range b.Children {
				switch c := child.(type) {
				case *sirena.Element:
					localActors[c.Name] = b.Name
				case *sirena.Boundary:
					if included[c.Name] {
						localParents[c.Name] = b.Name
					}
				}
			}
		}
		for _, b := range v.Boundaries {
			if err := assign(parents, b.Name, localParents[b.Name]); err != nil {
				return err
			}
		}
		for _, e := range v.Elements {
			if err := assign(actorParents, e.Name, localActors[e.Name]); err != nil {
				return err
			}
		}
	}
	for _, e := range union.Elements {
		if parent := boundaries[actorParents[e.Name]]; parent != nil {
			parent.Children = append(parent.Children, e)
		}
	}
	for _, b := range union.Boundaries {
		if parent := boundaries[parents[b.Name]]; parent != nil {
			parent.Children = append(parent.Children, b)
		}
	}
	return nil
}
