package layout

import (
	"math"
	"sort"

	"m31labs.dev/sirena"
)

// edgePorts is the source and target attachment chosen for one edge.
type edgePorts struct {
	Source sirena.Port
	Target sirena.Port
}

// assignPorts picks an attachment port on each edge's source and target
// rectangle. The v0.1 heuristic chooses the side facing the other node's
// center; multiple edges sharing a (node, side) are spread along that
// side at offsets (i+1)/(k+1) so stacked endpoints stay distinct. Edges
// are processed in a deterministic order (source, target, kind,
// direction) so offset assignment reproduces across runs. Edges with an
// unplaced endpoint are omitted from the result.
func assignPorts(placements []*sirena.NodePlacement, edges []*sirena.Edge) map[*sirena.Edge]edgePorts {
	rectOf := make(map[string]sirena.Rect, len(placements))
	for _, p := range placements {
		if p.Node != nil {
			rectOf[p.Node.Name] = p.Bounds
		}
	}

	ordered := append([]*sirena.Edge(nil), edges...)
	sort.SliceStable(ordered, func(i, j int) bool { return edgeLess(ordered[i], ordered[j]) })

	type sideInfo struct {
		srcName, tgtName string
		srcSide, tgtSide sirena.PortSide
		ok               bool
	}
	type gkey struct {
		name string
		side sirena.PortSide
	}
	type epRef struct {
		edge *sirena.Edge
		role int // 0 = source, 1 = target
	}

	info := make(map[*sirena.Edge]sideInfo, len(ordered))
	group := map[gkey][]epRef{}
	for _, e := range ordered {
		s, d := edgeEndpoints(e)
		rs, oks := rectOf[s]
		rd, okd := rectOf[d]
		if !oks || !okd {
			info[e] = sideInfo{ok: false}
			continue
		}
		srcSide, tgtSide := facingSides(rs, rd)
		info[e] = sideInfo{srcName: s, tgtName: d, srcSide: srcSide, tgtSide: tgtSide, ok: true}
		group[gkey{s, srcSide}] = append(group[gkey{s, srcSide}], epRef{e, 0})
		group[gkey{d, tgtSide}] = append(group[gkey{d, tgtSide}], epRef{e, 1})
	}

	// Offsets depend only on each endpoint's index within its group, and
	// groups were filled in deterministic edge order, so the map's
	// iteration order here does not affect the result.
	portOffset := map[*sirena.Edge][2]float64{}
	for _, eps := range group {
		k := len(eps)
		for i, ep := range eps {
			arr := portOffset[ep.edge]
			arr[ep.role] = float64(i+1) / float64(k+1)
			portOffset[ep.edge] = arr
		}
	}

	out := make(map[*sirena.Edge]edgePorts, len(ordered))
	for _, e := range ordered {
		si := info[e]
		if !si.ok {
			continue
		}
		offs := portOffset[e]
		out[e] = edgePorts{
			Source: makePort(rectOf[si.srcName], si.srcSide, offs[0]),
			Target: makePort(rectOf[si.tgtName], si.tgtSide, offs[1]),
		}
	}
	return out
}

// facingSides returns the side of src and of tgt that face each other,
// choosing the dominant axis between their centers.
func facingSides(src, tgt sirena.Rect) (srcSide, tgtSide sirena.PortSide) {
	sc, tc := src.Center(), tgt.Center()
	dx, dy := tc.X-sc.X, tc.Y-sc.Y
	if math.Abs(dx) >= math.Abs(dy) {
		if dx >= 0 {
			return sirena.PortSideRight, sirena.PortSideLeft
		}
		return sirena.PortSideLeft, sirena.PortSideRight
	}
	if dy >= 0 {
		return sirena.PortSideBottom, sirena.PortSideTop
	}
	return sirena.PortSideTop, sirena.PortSideBottom
}

// makePort builds a port on the given side of r at the given offset (0..1
// along that side), computing the absolute anchor point.
func makePort(r sirena.Rect, side sirena.PortSide, offset float64) sirena.Port {
	var a sirena.Point
	switch side {
	case sirena.PortSideTop:
		a = sirena.Point{X: r.Min.X + offset*r.Width(), Y: r.Min.Y}
	case sirena.PortSideBottom:
		a = sirena.Point{X: r.Min.X + offset*r.Width(), Y: r.Max.Y}
	case sirena.PortSideLeft:
		a = sirena.Point{X: r.Min.X, Y: r.Min.Y + offset*r.Height()}
	case sirena.PortSideRight:
		a = sirena.Point{X: r.Max.X, Y: r.Min.Y + offset*r.Height()}
	}
	return sirena.Port{Side: side, Offset: offset, Anchor: a}
}

// edgeLess is the deterministic edge ordering used throughout routing:
// by source name, then target name, then kind, then direction.
func edgeLess(a, b *sirena.Edge) bool {
	if a.From != b.From {
		return a.From < b.From
	}
	if a.To != b.To {
		return a.To < b.To
	}
	if a.Kind != b.Kind {
		return a.Kind < b.Kind
	}
	return a.Direction < b.Direction
}
