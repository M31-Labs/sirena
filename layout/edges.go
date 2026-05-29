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

const (
	stubLen      = 12.0 // length of the perpendicular exit stub at each port
	detourMargin = 16.0 // clearance past an obstacle when detouring
)

// routeEdges produces an orthogonal polyline route for every edge that
// has assigned ports. For each edge, the obstacle set is every placed
// node except the edge's own endpoints, so a route may detour around
// intervening nodes. Edges are processed in deterministic order; the
// obstacle math is order-independent, so output reproduces across runs.
func routeEdges(placements []*sirena.NodePlacement, ports map[*sirena.Edge]edgePorts, edges []*sirena.Edge) []*sirena.EdgeRoute {
	ordered := append([]*sirena.Edge(nil), edges...)
	sort.SliceStable(ordered, func(i, j int) bool { return edgeLess(ordered[i], ordered[j]) })

	var routes []*sirena.EdgeRoute
	for _, e := range ordered {
		ep, ok := ports[e]
		if !ok {
			continue
		}
		s, d := edgeEndpoints(e)
		var obstacles []sirena.Rect
		for _, p := range placements {
			if p.Node == nil || p.Node.Name == s || p.Node.Name == d {
				continue
			}
			obstacles = append(obstacles, p.Bounds)
		}
		routes = append(routes, &sirena.EdgeRoute{
			Edge:       e,
			Points:     routeEdge(ep.Source, ep.Target, obstacles),
			SourcePort: ep.Source,
			TargetPort: ep.Target,
		})
	}
	return routes
}

// routeEdge builds one orthogonal polyline from a source port to a target
// port: it exits each port with a perpendicular stub, connects the stub
// ends with an L (mixed orientation), straight line, or Z (channel)
// depending on the two sides, detours around obstacles when a straight
// run would cross one, and simplifies collinear points.
func routeEdge(sp, tp sirena.Port, obstacles []sirena.Rect) []sirena.Point {
	sa, ta := sp.Anchor, tp.Anchor
	s := pushOut(sa, sp.Side, stubLen)
	t := pushOut(ta, tp.Side, stubLen)

	srcH := sp.Side == sirena.PortSideLeft || sp.Side == sirena.PortSideRight
	tgtH := tp.Side == sirena.PortSideLeft || tp.Side == sirena.PortSideRight

	var mid []sirena.Point
	switch {
	case srcH && tgtH:
		mid = routeHH(s, t, obstacles)
	case !srcH && !tgtH:
		mid = routeVV(s, t, obstacles)
	case srcH && !tgtH:
		mid = []sirena.Point{{X: t.X, Y: s.Y}} // L corner
	default:
		mid = []sirena.Point{{X: s.X, Y: t.Y}} // L corner
	}

	pts := make([]sirena.Point, 0, len(mid)+4)
	pts = append(pts, sa, s)
	pts = append(pts, mid...)
	pts = append(pts, t, ta)
	return simplify(pts)
}

// routeHH connects two horizontally-exiting stubs. When they share a Y
// and the run is clear, no intermediate points are needed (a straight
// line). When blocked, it detours through a clear horizontal channel.
// Otherwise it bends through a vertical channel at the midpoint X.
func routeHH(s, t sirena.Point, obstacles []sirena.Rect) []sirena.Point {
	if s.Y == t.Y {
		if segClear(s, t, obstacles) {
			return nil
		}
		yCh := clearYChannel(min(s.X, t.X), max(s.X, t.X), s.Y, obstacles)
		return []sirena.Point{{X: s.X, Y: yCh}, {X: t.X, Y: yCh}}
	}
	xCh := (s.X + t.X) / 2
	return []sirena.Point{{X: xCh, Y: s.Y}, {X: xCh, Y: t.Y}}
}

// routeVV connects two vertically-exiting stubs, mirroring routeHH.
func routeVV(s, t sirena.Point, obstacles []sirena.Rect) []sirena.Point {
	if s.X == t.X {
		if segClear(s, t, obstacles) {
			return nil
		}
		xCh := clearXChannel(min(s.Y, t.Y), max(s.Y, t.Y), s.X, obstacles)
		return []sirena.Point{{X: xCh, Y: s.Y}, {X: xCh, Y: t.Y}}
	}
	yCh := (s.Y + t.Y) / 2
	return []sirena.Point{{X: s.X, Y: yCh}, {X: t.X, Y: yCh}}
}

// pushOut moves a point outward from a node by d along the port's side
// normal.
func pushOut(p sirena.Point, side sirena.PortSide, d float64) sirena.Point {
	switch side {
	case sirena.PortSideTop:
		return sirena.Point{X: p.X, Y: p.Y - d}
	case sirena.PortSideBottom:
		return sirena.Point{X: p.X, Y: p.Y + d}
	case sirena.PortSideLeft:
		return sirena.Point{X: p.X - d, Y: p.Y}
	default: // right
		return sirena.Point{X: p.X + d, Y: p.Y}
	}
}

// segCrossesRect reports whether the axis-aligned segment a→b passes
// through r's interior. Grazing an edge does not count (half-open with a
// small epsilon), so a route may run flush against a node.
func segCrossesRect(a, b sirena.Point, r sirena.Rect) bool {
	const eps = 1e-9
	loX, hiX := min(a.X, b.X), max(a.X, b.X)
	loY, hiY := min(a.Y, b.Y), max(a.Y, b.Y)
	return loX < r.Max.X-eps && hiX > r.Min.X+eps &&
		loY < r.Max.Y-eps && hiY > r.Min.Y+eps
}

// segClear reports whether the segment a→b crosses no obstacle.
func segClear(a, b sirena.Point, obstacles []sirena.Rect) bool {
	for _, r := range obstacles {
		if segCrossesRect(a, b, r) {
			return false
		}
	}
	return true
}

// clearYChannel returns a Y clear of every obstacle overlapping the
// x-range [x1, x2], routing just past the nearer side (above or below
// preferY). With no overlapping obstacle it returns preferY.
func clearYChannel(x1, x2, preferY float64, obstacles []sirena.Rect) float64 {
	lo, hi := math.Inf(1), math.Inf(-1)
	any := false
	for _, o := range obstacles {
		if o.Max.X <= x1 || o.Min.X >= x2 {
			continue
		}
		any = true
		lo = min(lo, o.Min.Y)
		hi = max(hi, o.Max.Y)
	}
	if !any {
		return preferY
	}
	below, above := hi+detourMargin, lo-detourMargin
	if math.Abs(below-preferY) <= math.Abs(above-preferY) {
		return below
	}
	return above
}

// clearXChannel is the vertical-channel analogue of clearYChannel.
func clearXChannel(y1, y2, preferX float64, obstacles []sirena.Rect) float64 {
	lo, hi := math.Inf(1), math.Inf(-1)
	any := false
	for _, o := range obstacles {
		if o.Max.Y <= y1 || o.Min.Y >= y2 {
			continue
		}
		any = true
		lo = min(lo, o.Min.X)
		hi = max(hi, o.Max.X)
	}
	if !any {
		return preferX
	}
	right, left := hi+detourMargin, lo-detourMargin
	if math.Abs(right-preferX) <= math.Abs(left-preferX) {
		return right
	}
	return left
}

// simplify removes consecutive duplicate points and collinear midpoints,
// leaving the minimal polyline with the same shape.
func simplify(pts []sirena.Point) []sirena.Point {
	var dedup []sirena.Point
	for _, p := range pts {
		if len(dedup) == 0 || dedup[len(dedup)-1] != p {
			dedup = append(dedup, p)
		}
	}
	if len(dedup) <= 2 {
		return dedup
	}
	out := []sirena.Point{dedup[0]}
	for i := 1; i < len(dedup)-1; i++ {
		a, b, c := dedup[i-1], dedup[i], dedup[i+1]
		if (a.X == b.X && b.X == c.X) || (a.Y == b.Y && b.Y == c.Y) {
			continue // b is redundant on a straight run
		}
		out = append(out, b)
	}
	return append(out, dedup[len(dedup)-1])
}
