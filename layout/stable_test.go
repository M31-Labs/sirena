package layout

import (
	"encoding/json"
	"reflect"
	"testing"

	"m31labs.dev/sirena"
)

func nestedStoryView(extra bool) *sirena.ResolvedView {
	a, b := elem("api"), elem("db")
	inner := &sirena.Boundary{Kind: sirena.BoundaryKindDeployment, Name: "services", Children: []sirena.Node{a, b}}
	outer := &sirena.Boundary{Kind: sirena.BoundaryKindNetwork, Name: "vpc", Children: []sirena.Node{inner}}
	v := &sirena.ResolvedView{Elements: []*sirena.Element{a, b}, Boundaries: []*sirena.Boundary{outer, inner}, Edges: []*sirena.Edge{fwd("api", "db")}}
	if extra {
		c := elem("worker")
		inner.Children = append(inner.Children, c)
		v.Elements = append(v.Elements, c)
		v.Edges = append(v.Edges, fwd("api", "worker"))
	}
	return v
}

func TestNestedStoryboardReservesActorsAndFrames(t *testing.T) {
	views := []*sirena.ResolvedView{nestedStoryView(false), nestedStoryView(true), nestedStoryView(false)}
	before, _ := json.Marshal(views)
	frames, err := Storyboard(views, sirena.RenderOptions{})
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(views)
	if string(before) != string(after) {
		t.Fatal("storyboard mutated source hierarchy")
	}
	for _, frame := range frames[1:] {
		for i, p := range frames[0].NodePlacements {
			if p.Bounds != frame.NodePlacements[i].Bounds {
				t.Fatalf("actor %s moved", p.Node.Name)
			}
		}
		if frame.BoundaryPlacements[0].Bounds != frames[0].BoundaryPlacements[0].Bounds || frame.BoundaryPlacements[0].Children[0].Bounds != frames[0].BoundaryPlacements[0].Children[0].Bounds {
			t.Fatal("nested frames moved")
		}
	}
	if !reflect.DeepEqual(frames[0].EdgeRoutes, frames[2].EdgeRoutes) {
		t.Fatal("returning to a state changes its route")
	}
}

func TestStableEditPreservesIdentityAndDoesNotMutatePrevious(t *testing.T) {
	v := &sirena.ResolvedView{Elements: []*sirena.Element{elem("a"), elem("b")}, Edges: []*sirena.Edge{fwd("a", "b")}}
	v.Elements[0].Metadata = map[string]sirena.Value{"sid": sirena.String{Value: "stable-api"}}
	previous, err := Compute(v, LayoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(previous)
	renamed := elem("renamed")
	renamed.Metadata = v.Elements[0].Metadata
	next := &sirena.ResolvedView{Elements: []*sirena.Element{elem("new"), v.Elements[1], renamed}, Edges: []*sirena.Edge{fwd("renamed", "b"), fwd("new", "renamed")}}
	lr, err := Compute(next, LayoutOptions{Previous: previous})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range lr.NodePlacements {
		for _, old := range previous.NodePlacements {
			if actorIdentity(p.Node) == actorIdentity(old.Node) && p.Bounds.Center() != old.Bounds.Center() {
				t.Fatalf("existing identity %s moved", p.Node.Name)
			}
		}
	}
	for i, p := range lr.NodePlacements {
		for _, other := range lr.NodePlacements[:i] {
			if p.Bounds.Intersects(other.Bounds) {
				t.Fatal("stable edit introduced an overlap")
			}
		}
	}
	after, _ := json.Marshal(previous)
	if string(before) != string(after) {
		t.Fatal("previous layout changed")
	}
}

func TestStableNestedEditRetainsContainment(t *testing.T) {
	previous, err := Compute(nestedStoryView(false), LayoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	lr, err := Compute(nestedStoryView(true), LayoutOptions{Previous: previous})
	if err != nil {
		t.Fatal(err)
	}
	inner := lr.BoundaryPlacements[0].Children[0].Bounds
	for _, p := range lr.NodePlacements {
		if p.Bounds.Min.X < inner.Min.X || p.Bounds.Max.X > inner.Max.X || p.Bounds.Min.Y < inner.Min.Y+24 || p.Bounds.Max.Y > inner.Max.Y {
			t.Fatal("actor escaped nested frame or overlaps header")
		}
	}
}
