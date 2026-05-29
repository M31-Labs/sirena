package layout

import (
	"math"
	"sort"

	"m31labs.dev/sirena"
)

// Metrics carries the spacing constants and text-measurement function
// the coordinate assignment and skeleton solver use. NodeWidth defaults
// to the placeholder glyph-count estimate; Phase F swaps in the bundled
// font's real advance widths by setting WidthOf.
type Metrics struct {
	NodeSpacing  float64 // minimum horizontal gap between nodes in a layer
	LayerSpacing float64 // vertical gap between layers
	Height       float64 // node height (uniform in v0.1)
	WidthOf      func(label string) float64
}

// DefaultMetrics returns the tuned v0.1 layout metrics.
func DefaultMetrics() Metrics {
	return Metrics{
		NodeSpacing:  defaultNodeSpacing,
		LayerSpacing: defaultLayerGap,
		Height:       defaultNodeHeight,
	}
}

// NodeWidth returns a node's rendered width for the given label.
func (m Metrics) NodeWidth(label string) float64 {
	if m.WidthOf != nil {
		return m.WidthOf(label)
	}
	return nodeWidth(label)
}

// NodeHeight returns the uniform node height.
func (m Metrics) NodeHeight() float64 {
	if m.Height > 0 {
		return m.Height
	}
	return defaultNodeHeight
}

// TextWidth measures a string's rendered width with no minimum-width
// floor (unlike NodeWidth). Used for edge labels, which may be narrower
// than a node box.
func (m Metrics) TextWidth(s string) float64 {
	if m.WidthOf != nil {
		return m.WidthOf(s)
	}
	return defaultGlyphWidth * float64(len([]rune(s)))
}

// assignCoords assigns each element an absolute Rect using the
// Brandes-Köpf horizontal coordinate algorithm (Brandes & Köpf 2002):
// four vertical-alignment + horizontal-compaction passes (the up/down ×
// left/right combinations) whose results are balanced into a single
// coordinate per node. Y-coordinates derive from the layer index times
// (height + LayerSpacing). The result is keyed by element name.
//
// v0.1 has no dummy nodes (long edges are not split), so there are no
// inner segments and therefore no type-1 conflicts to mark; the
// alignment step's no-crossing guard (a monotone position cursor per
// layer) is sufficient. The whole procedure is deterministic: every
// iteration walks the ordered layer slices, and adjacency lists are
// position-sorted.
func assignCoords(layers [][]*sirena.Element, edges []*sirena.Edge, metrics Metrics) map[string]sirena.Rect {
	layerNames := make([][]string, len(layers))
	layerIdx := map[string]int{}
	width := map[string]float64{}
	var allNames []string
	for li, layer := range layers {
		for _, e := range layer {
			layerNames[li] = append(layerNames[li], e.Name)
			layerIdx[e.Name] = li
			width[e.Name] = metrics.NodeWidth(e.Name)
			allNames = append(allNames, e.Name)
		}
	}
	sort.Strings(allNames)
	if len(allNames) == 0 {
		return map[string]sirena.Rect{}
	}

	// Symmetric adjacency restricted to consecutive layers.
	adj := map[string]map[string]bool{}
	addAdj := func(a, b string) {
		if adj[a] == nil {
			adj[a] = map[string]bool{}
		}
		adj[a][b] = true
	}
	for _, e := range edges {
		s, d := edgeEndpoints(e)
		ls, oks := layerIdx[s]
		ld, okd := layerIdx[d]
		if !oks || !okd {
			continue
		}
		if abs(ls-ld) == 1 {
			addAdj(s, d)
			addAdj(d, s)
		}
	}
	adjList := map[string][]string{}
	for n, set := range adj {
		ns := make([]string, 0, len(set))
		for m := range set {
			ns = append(ns, m)
		}
		sort.Strings(ns)
		adjList[n] = ns
	}

	ns := metrics.NodeSpacing
	passes := []map[string]float64{
		runPass(layerNames, adjList, width, ns, false),                                     // up, left
		runPass(reverseEachLayer(layerNames), adjList, width, ns, true),                    // up, right
		runPass(reverseLayerOrder(layerNames), adjList, width, ns, false),                  // down, left
		runPass(reverseEachLayer(reverseLayerOrder(layerNames)), adjList, width, ns, true), // down, right
	}
	centers := balance(passes, allNames)

	// Recenter so the leftmost left-edge sits at x=0.
	minLeft := math.Inf(1)
	for _, n := range allNames {
		if l := centers[n] - width[n]/2; l < minLeft {
			minLeft = l
		}
	}
	if math.IsInf(minLeft, 1) {
		minLeft = 0
	}

	h := metrics.NodeHeight()
	out := make(map[string]sirena.Rect, len(allNames))
	for _, n := range allNames {
		cx := centers[n] - minLeft
		y := float64(layerIdx[n]) * (h + metrics.LayerSpacing)
		out[n] = sirena.Rect{
			Min: sirena.Point{X: cx - width[n]/2, Y: y},
			Max: sirena.Point{X: cx + width[n]/2, Y: y + h},
		}
	}
	return out
}

// runPass runs one Brandes-Köpf vertical-alignment + horizontal-
// compaction in the canonical orientation (process layer 0 downward,
// align to the layer above). Callers flip orientedLayers vertically
// and/or horizontally to obtain the other three directional passes;
// horizFlip negates the result so all four share a coordinate frame.
func runPass(orientedLayers [][]string, adj map[string][]string, width map[string]float64, nodeSpacing float64, horizFlip bool) map[string]float64 {
	pos := map[string]int{}
	oidx := map[string]int{}
	for li, layer := range orientedLayers {
		for i, n := range layer {
			pos[n] = i
			oidx[n] = li
		}
	}

	above := map[string][]string{}
	for _, layer := range orientedLayers {
		for _, v := range layer {
			var nbrs []string
			for _, u := range adj[v] {
				if oidx[u] == oidx[v]-1 {
					nbrs = append(nbrs, u)
				}
			}
			sort.Slice(nbrs, func(i, j int) bool { return pos[nbrs[i]] < pos[nbrs[j]] })
			above[v] = nbrs
		}
	}

	root, align := verticalAlignment(orientedLayers, above, pos)
	x := horizontalCompaction(orientedLayers, root, align, width, nodeSpacing)
	if horizFlip {
		for k := range x {
			x[k] = -x[k]
		}
	}
	return x
}

// verticalAlignment builds the root/align block structure: each node is
// aligned to a median neighbor in the layer above when doing so does not
// cross an already-made alignment (tracked by the monotone position
// cursor r per layer). root[v] is the topmost node of v's block; align
// forms a cyclic linked list around each block.
func verticalAlignment(layers [][]string, above map[string][]string, pos map[string]int) (root, align map[string]string) {
	root = map[string]string{}
	align = map[string]string{}
	for _, layer := range layers {
		for _, v := range layer {
			root[v] = v
			align[v] = v
		}
	}
	for i := 1; i < len(layers); i++ {
		r := -1
		for _, v := range layers[i] {
			nbrs := above[v]
			d := len(nbrs)
			if d == 0 {
				continue
			}
			lower := (d - 1) / 2
			upper := d / 2
			medians := []int{lower}
			if upper != lower {
				medians = append(medians, upper)
			}
			for _, m := range medians {
				if align[v] != v {
					break
				}
				u := nbrs[m]
				if pos[u] > r {
					align[u] = v
					root[v] = root[u]
					align[v] = root[v]
					r = pos[u]
				}
			}
		}
	}
	return root, align
}

// horizontalCompaction places each block as far left as its separation
// constraints allow, using the sink/shift class machinery from Brandes &
// Köpf so independent blocks pack tightly. Coordinates are node centers.
// Separation between in-layer neighbors uses their actual widths.
func horizontalCompaction(layers [][]string, root, align map[string]string, width map[string]float64, nodeSpacing float64) map[string]float64 {
	predInLayer := map[string]string{}
	for _, layer := range layers {
		for i, v := range layer {
			if i > 0 {
				predInLayer[v] = layer[i-1]
			}
		}
	}

	sink := map[string]string{}
	shift := map[string]float64{}
	x := map[string]float64{}
	placed := map[string]bool{}
	for _, layer := range layers {
		for _, v := range layer {
			sink[v] = v
			shift[v] = math.Inf(1)
		}
	}

	sep := func(left, right string) float64 {
		return width[left]/2 + nodeSpacing + width[right]/2
	}

	var placeBlock func(v string)
	placeBlock = func(v string) {
		if placed[v] {
			return
		}
		placed[v] = true
		x[v] = 0
		w := v
		for {
			if u0, ok := predInLayer[w]; ok {
				u := root[u0]
				placeBlock(u)
				if sink[v] == v {
					sink[v] = sink[u]
				}
				if sink[v] != sink[u] {
					if s := x[v] - x[u] - sep(u0, w); s < shift[sink[u]] {
						shift[sink[u]] = s
					}
				} else if cand := x[u] + sep(u0, w); cand > x[v] {
					x[v] = cand
				}
			}
			w = align[w]
			if w == v {
				break
			}
		}
	}

	for _, layer := range layers {
		for _, v := range layer {
			if root[v] == v {
				placeBlock(v)
			}
		}
	}

	out := map[string]float64{}
	for _, layer := range layers {
		for _, v := range layer {
			out[v] = x[root[v]]
			if s := shift[sink[root[v]]]; !math.IsInf(s, 1) {
				out[v] += s
			}
		}
	}
	return out
}

// balance combines the four directional passes into a single coordinate
// per node by normalizing each pass to a minimum of zero and averaging.
// Averaging (rather than the paper's per-node median of the inner two)
// is a deterministic v0.1 simplification; it keeps single chains
// straight and same-layer spacing exact.
func balance(passes []map[string]float64, names []string) map[string]float64 {
	norm := make([]map[string]float64, len(passes))
	for i, p := range passes {
		mn := math.Inf(1)
		for _, n := range names {
			if p[n] < mn {
				mn = p[n]
			}
		}
		m := make(map[string]float64, len(names))
		for _, n := range names {
			m[n] = p[n] - mn
		}
		norm[i] = m
	}
	out := make(map[string]float64, len(names))
	for _, n := range names {
		sum := 0.0
		for _, m := range norm {
			sum += m[n]
		}
		out[n] = sum / float64(len(norm))
	}
	return out
}

// reverseEachLayer returns a copy of layers with each layer's node order
// reversed (layer order preserved). Used to obtain right-biased passes.
func reverseEachLayer(layers [][]string) [][]string {
	out := make([][]string, len(layers))
	for i, layer := range layers {
		r := make([]string, len(layer))
		for j, n := range layer {
			r[len(layer)-1-j] = n
		}
		out[i] = r
	}
	return out
}

// reverseLayerOrder returns a copy of layers with the layer order
// reversed (each layer's node order preserved). Used to obtain
// down-aligned passes.
func reverseLayerOrder(layers [][]string) [][]string {
	out := make([][]string, len(layers))
	for i, layer := range layers {
		out[len(layers)-1-i] = layer
	}
	return out
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
