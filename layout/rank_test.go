package layout

import (
	"reflect"
	"testing"

	"m31labs.dev/sirena"
)

func elem(name string) *sirena.Element {
	return &sirena.Element{Kind: sirena.ElementKindService, Name: name}
}

func fwd(from, to string) *sirena.Edge {
	return &sirena.Edge{From: from, To: to, Kind: sirena.EdgeKindCalls, Direction: sirena.DirForward}
}

func TestRank_SingleNode(t *testing.T) {
	r, err := rank([]*sirena.Element{elem("solo")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r["solo"] != 0 {
		t.Errorf("solo rank = %d, want 0", r["solo"])
	}
}

func TestRank_Chain(t *testing.T) {
	els := []*sirena.Element{elem("A"), elem("B"), elem("C")}
	edges := []*sirena.Edge{fwd("A", "B"), fwd("B", "C")}
	r, err := rank(els, edges)
	if err != nil {
		t.Fatal(err)
	}
	if r["A"] != 0 || r["B"] != 1 || r["C"] != 2 {
		t.Errorf("chain ranks = %v, want A=0 B=1 C=2", r)
	}
}

func TestRank_Diamond(t *testing.T) {
	els := []*sirena.Element{elem("A"), elem("B"), elem("C"), elem("D")}
	edges := []*sirena.Edge{fwd("A", "B"), fwd("A", "C"), fwd("B", "D"), fwd("C", "D")}
	r, err := rank(els, edges)
	if err != nil {
		t.Fatal(err)
	}
	if r["D"] != 2 {
		t.Errorf("diamond D rank = %d, want 2 (longest path, not sum)", r["D"])
	}
}

func TestRank_Cycle_Broken(t *testing.T) {
	els := []*sirena.Element{elem("A"), elem("B")}
	edges := []*sirena.Edge{fwd("A", "B"), fwd("B", "A")}
	r, err := rank(els, edges)
	if err != nil {
		t.Fatalf("cycle rank errored: %v", err)
	}
	for _, n := range []string{"A", "B"} {
		if r[n] < 0 || r[n] > 1 {
			t.Errorf("%s rank = %d, want 0 or 1 after deterministic cycle break", n, r[n])
		}
	}
}

func TestRank_Deterministic(t *testing.T) {
	els := []*sirena.Element{elem("A"), elem("B"), elem("C"), elem("D")}
	edges := []*sirena.Edge{fwd("A", "B"), fwd("A", "C"), fwd("B", "D"), fwd("C", "D")}
	r1, _ := rank(els, edges)
	r2, _ := rank(els, edges)
	if !reflect.DeepEqual(r1, r2) {
		t.Errorf("rank not deterministic: %v vs %v", r1, r2)
	}
}
