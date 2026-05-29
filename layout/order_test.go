package layout

import (
	"reflect"
	"testing"

	"m31labs.dev/sirena"
)

func byNameMap(els ...*sirena.Element) map[string]*sirena.Element {
	m := make(map[string]*sirena.Element, len(els))
	for _, e := range els {
		m[e.Name] = e
	}
	return m
}

func layerNames(layer []*sirena.Element) []string {
	out := make([]string, len(layer))
	for i, e := range layer {
		out[i] = e.Name
	}
	return out
}

func TestOrder_TwoLayers_OneCrossing(t *testing.T) {
	a, b, x, y := elem("A"), elem("B"), elem("X"), elem("Y")
	ranks := map[string]int{"A": 0, "B": 0, "X": 1, "Y": 1}
	edges := []*sirena.Edge{fwd("A", "Y"), fwd("B", "X")}

	layers := order(ranks, edges, byNameMap(a, b, x, y))
	if len(layers) != 2 {
		t.Fatalf("got %d layers, want 2", len(layers))
	}
	got := layerNames(layers[1])
	want := []string{"Y", "X"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("layer 1 order = %v, want %v (zero crossings)", got, want)
	}
}

func TestOrder_StableOnNoEdges(t *testing.T) {
	a, b, c := elem("a"), elem("b"), elem("c")
	ranks := map[string]int{"a": 0, "b": 0, "c": 0}

	layers := order(ranks, nil, byNameMap(a, b, c))
	got := layerNames(layers[0])
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("no-edge order = %v, want %v (name-sorted, stable)", got, want)
	}
}

func TestOrder_Deterministic(t *testing.T) {
	a, b, x, y := elem("A"), elem("B"), elem("X"), elem("Y")
	ranks := map[string]int{"A": 0, "B": 0, "X": 1, "Y": 1}
	edges := []*sirena.Edge{fwd("A", "Y"), fwd("B", "X")}
	bn := byNameMap(a, b, x, y)

	l1 := order(ranks, edges, bn)
	l2 := order(ranks, edges, bn)
	for i := range l1 {
		if !reflect.DeepEqual(layerNames(l1[i]), layerNames(l2[i])) {
			t.Errorf("order not deterministic at layer %d: %v vs %v", i, layerNames(l1[i]), layerNames(l2[i]))
		}
	}
}
