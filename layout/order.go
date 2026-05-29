package layout

import (
	"sort"

	"m31labs.dev/sirena"
)

// order assigns each layer's nodes a left-to-right order that reduces
// edge crossings, using the median heuristic. It seeds each layer in
// name-sorted order (deterministic), then runs one top-down sweep
// (layers 1..max ordered against the fixed layer above) and one
// bottom-up sweep (layers max-1..0 against the fixed layer below). Ties
// break by name. One sweep each way is the v0.1 contract; multi-sweep
// barycenter + transpose is deferred to v0.2.
//
// The result is indexed by rank: out[r] is the ordered slice of elements
// at layer r. Elements with no resolved entry in byName are skipped.
func order(ranks map[string]int, edges []*sirena.Edge, byName map[string]*sirena.Element) [][]*sirena.Element {
	if len(ranks) == 0 {
		return nil
	}

	maxRank := 0
	for _, r := range ranks {
		if r > maxRank {
			maxRank = r
		}
	}

	layers := make([][]string, maxRank+1)
	inScope := make(map[string]bool, len(ranks))
	for name, r := range ranks {
		layers[r] = append(layers[r], name)
		inScope[name] = true
	}
	for i := range layers {
		sort.Strings(layers[i])
	}

	preds := map[string][]string{}
	succs := map[string][]string{}
	for _, e := range edges {
		s, d := edgeEndpoints(e)
		if s == d || !inScope[s] || !inScope[d] {
			continue
		}
		succs[s] = append(succs[s], d)
		preds[d] = append(preds[d], s)
	}

	// Top-down: order each layer against the (now fixed) layer above.
	for l := 1; l <= maxRank; l++ {
		layers[l] = sortByMedian(layers[l], layers[l-1], preds, ranks, l-1)
	}
	// Bottom-up: order each layer against the (now fixed) layer below.
	for l := maxRank - 1; l >= 0; l-- {
		layers[l] = sortByMedian(layers[l], layers[l+1], succs, ranks, l+1)
	}

	out := make([][]*sirena.Element, len(layers))
	for i, layer := range layers {
		for _, name := range layer {
			if el := byName[name]; el != nil {
				out[i] = append(out[i], el)
			}
		}
	}
	return out
}

// sortByMedian reorders layer by each node's median neighbor position in
// the fixed adjacent layer. neighbors maps a node to its candidate
// neighbors (predecessors for a top-down sweep, successors for
// bottom-up); only neighbors whose rank equals adjRank — i.e. that
// actually live in adj — count. Nodes with no such neighbor keep their
// current index as the sort key so they stay put. Ties break by name.
func sortByMedian(layer, adj []string, neighbors map[string][]string, ranks map[string]int, adjRank int) []string {
	pos := make(map[string]int, len(adj))
	for i, n := range adj {
		pos[n] = i
	}

	keys := make(map[string]float64, len(layer))
	for curIdx, v := range layer {
		var ps []int
		for _, nb := range neighbors[v] {
			if ranks[nb] != adjRank {
				continue
			}
			if p, ok := pos[nb]; ok {
				ps = append(ps, p)
			}
		}
		if len(ps) == 0 {
			keys[v] = float64(curIdx)
			continue
		}
		sort.Ints(ps)
		keys[v] = medianOf(ps)
	}

	sorted := append([]string(nil), layer...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if keys[sorted[i]] != keys[sorted[j]] {
			return keys[sorted[i]] < keys[sorted[j]]
		}
		return sorted[i] < sorted[j]
	})
	return sorted
}

// medianOf returns the median of an already-sorted slice of positions.
// For an even count it averages the two central values (the standard
// median-heuristic definition).
func medianOf(sorted []int) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return float64(sorted[n/2])
	}
	return float64(sorted[n/2-1]+sorted[n/2]) / 2
}
