package layout

import (
	"sort"

	"m31labs.dev/sirena"
)

// edgeEndpoints returns the (source, target) element names for an edge,
// accounting for the arrow direction. A reverse arrow (a <- b) flows
// from b to a; forward and bidirectional flow From → To (bidirectional
// picks the written order as its canonical layering direction). The
// names are the verbatim endpoint identifiers; in-scope filtering by the
// caller decides which edges actually constrain a given cell.
func edgeEndpoints(e *sirena.Edge) (src, dst string) {
	if e.Direction == sirena.DirReverse {
		return e.To, e.From
	}
	return e.From, e.To
}

// edgePair is a directed (source, target) name pair after direction
// normalization and in-scope filtering.
type edgePair struct{ src, dst string }

// rank assigns each element an integer layer via longest-path ranking:
// for every in-scope edge u→v, rank(v) > rank(u); sources sit at layer 0
// and the deepest sink at the maximum layer. The returned map is keyed
// by element name and includes every element (isolated nodes rank 0).
//
// Cycles are broken deterministically: when a residual set of nodes
// cannot be topologically ordered, the edge whose (source, target) name
// pair is lexicographically smallest among the residual nodes is dropped
// as a back-edge, and ranking retries until the graph is acyclic.
func rank(elements []*sirena.Element, edges []*sirena.Edge) (map[string]int, error) {
	names := make([]string, 0, len(elements))
	inScope := make(map[string]bool, len(elements))
	for _, e := range elements {
		if !inScope[e.Name] {
			inScope[e.Name] = true
			names = append(names, e.Name)
		}
	}
	sort.Strings(names)

	pairs := inScopePairs(edges, inScope)

	for {
		ranks, residual := kahnLongestPath(names, pairs)
		if len(residual) == 0 {
			return ranks, nil
		}
		// Drop the smallest (src,dst) edge wholly inside the residual
		// set. pairs is sorted, so the first match is the lexicographic
		// minimum.
		idx := -1
		for i, p := range pairs {
			if residual[p.src] && residual[p.dst] {
				idx = i
				break
			}
		}
		if idx < 0 {
			// Residual nodes with no internal edge can't happen for a
			// true cycle, but guard against an infinite loop: settle the
			// residual nodes at layer 0 and finish.
			for n := range residual {
				ranks[n] = 0
			}
			return ranks, nil
		}
		pairs = append(pairs[:idx], pairs[idx+1:]...)
	}
}

// inScopePairs normalizes edges into deduplicated, sorted directed pairs
// whose endpoints are both in scope (self-loops dropped).
func inScopePairs(edges []*sirena.Edge, inScope map[string]bool) []edgePair {
	seen := map[edgePair]bool{}
	var pairs []edgePair
	for _, ed := range edges {
		s, d := edgeEndpoints(ed)
		if s == d || !inScope[s] || !inScope[d] {
			continue
		}
		p := edgePair{s, d}
		if seen[p] {
			continue
		}
		seen[p] = true
		pairs = append(pairs, p)
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].src != pairs[j].src {
			return pairs[i].src < pairs[j].src
		}
		return pairs[i].dst < pairs[j].dst
	})
	return pairs
}

// kahnLongestPath runs a deterministic Kahn topological sweep that
// computes the longest-path rank of every reachable node. It returns the
// rank map plus the set of residual nodes that could not be ordered
// (i.e. participate in a cycle). Ties are broken by processing the
// lexicographically smallest ready node first.
func kahnLongestPath(names []string, pairs []edgePair) (map[string]int, map[string]bool) {
	succ := map[string][]string{}
	indeg := map[string]int{}
	for _, n := range names {
		indeg[n] = 0
	}
	for _, p := range pairs {
		succ[p.src] = append(succ[p.src], p.dst)
		indeg[p.dst]++
	}

	ranks := map[string]int{}
	processed := map[string]bool{}
	ready := map[string]bool{}
	for _, n := range names {
		if indeg[n] == 0 {
			ready[n] = true
			ranks[n] = 0
		}
	}

	for len(ready) > 0 {
		// Pick the lexicographically smallest ready node.
		u := ""
		for n := range ready {
			if u == "" || n < u {
				u = n
			}
		}
		delete(ready, u)
		processed[u] = true

		nexts := append([]string(nil), succ[u]...)
		sort.Strings(nexts)
		for _, v := range nexts {
			if ranks[u]+1 > ranks[v] {
				ranks[v] = ranks[u] + 1
			}
			indeg[v]--
			if indeg[v] == 0 && !processed[v] {
				ready[v] = true
			}
		}
	}

	residual := map[string]bool{}
	for _, n := range names {
		if !processed[n] {
			residual[n] = true
		}
	}
	return ranks, residual
}
