package layout

import (
	"reflect"
	"testing"

	"m31labs.dev/sirena"
)

func TestExplicitPortsAndObstructedBends(t *testing.T) {
	for _, ports := range []map[string]sirena.Value{
		nil,
		{"source_port": sirena.String{Value: "right:0.25"}, "target_port": sirena.String{Value: "top:0.75"}},
	} {
		placements := []*sirena.NodePlacement{np("src", 0, 0, 80, 40), np("dst", 200, 140, 280, 180), np("obstacle", 95, 5, 165, 80)}
		e := fwd("src", "dst")
		e.Metadata = ports
		route := routeOne(t, placements, e)
		if !route.IsOrthogonal() {
			t.Fatalf("diagonal route: %v", route.Points)
		}
		for _, p := range placements {
			for i := 1; i < len(route.Points); i++ {
				if segCrossesRect(route.Points[i-1], route.Points[i], p.Bounds) {
					t.Fatalf("route crosses %s: %v", p.Node.Name, route.Points)
				}
			}
		}
		if ports != nil && (route.SourcePort.Side != sirena.PortSideRight || route.SourcePort.Offset != .25 || route.TargetPort.Side != sirena.PortSideTop || route.TargetPort.Offset != .75) {
			t.Fatalf("port hints were lost: %+v", route)
		}
		if again := routeOne(t, placements, e); !reflect.DeepEqual(route.Points, again.Points) {
			t.Fatal("routing is nondeterministic")
		}
	}
}

func TestInvalidPortHintsDiagnosed(t *testing.T) {
	for _, value := range []sirena.Value{sirena.String{Value: "sideways"}, sirena.String{Value: "top:NaN"}, sirena.String{Value: "top:1.1"}, sirena.Number{Value: 1}} {
		e := fwd("src", "dst")
		e.Metadata = map[string]sirena.Value{"source_port": value}
		if _, err := Compute(&sirena.ResolvedView{Edges: []*sirena.Edge{e}}, LayoutOptions{}); err == nil {
			t.Fatalf("accepted invalid port: %#v", value)
		}
	}
}

func TestPortHintsFromSirenaSource(t *testing.T) {
	doc, err := sirena.Parse([]byte("service a\nservice b\na -> b: calls \"request\" { source_port: \"bottom:0.25\"; target_port: \"top:0.75\" }\n"))
	if err != nil {
		t.Fatal(err)
	}
	lr, err := Compute(sirena.AllElementsView(doc), LayoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(lr.EdgeRoutes) != 1 || lr.EdgeRoutes[0].SourcePort.Side != sirena.PortSideBottom || lr.EdgeRoutes[0].SourcePort.Offset != .25 || lr.EdgeRoutes[0].TargetPort.Offset != .75 {
		t.Fatalf("source port hints did not reach layout: %+v", lr.EdgeRoutes)
	}
}

func TestParallelCaptionsAvoidActorsAndEachOther(t *testing.T) {
	placements := []*sirena.NodePlacement{np("src", 0, 0, 80, 40), np("dst", 180, 0, 260, 40)}
	var edges []*sirena.Edge
	for range 6 {
		e := fwd("src", "dst")
		e.Label = "request dispatch"
		edges = append(edges, e)
	}
	routes := routeEdges(placements, assignPorts(placements, edges), edges)
	placeLabels(routes, placements, DefaultMetrics())
	for i, r := range routes {
		if r.Label == nil {
			t.Fatal("missing caption")
		}
		for _, p := range placements {
			if r.Label.Bounds.Intersects(p.Bounds) {
				t.Fatal("caption overlaps actor")
			}
		}
		for _, old := range routes[:i] {
			if r.Label.Bounds.Intersects(old.Label.Bounds) {
				t.Fatal("captions overlap")
			}
		}
	}
}

func BenchmarkObstructedRouting(b *testing.B) {
	placements := []*sirena.NodePlacement{np("src", 0, 0, 80, 40), np("dst", 200, 140, 280, 180), np("obstacle", 95, 5, 165, 80)}
	e := fwd("src", "dst")
	ports := assignPorts(placements, []*sirena.Edge{e})
	b.ReportAllocs()
	for b.Loop() {
		routeEdges(placements, ports, []*sirena.Edge{e})
	}
}
