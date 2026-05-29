// Package sirena: layout geometry.
//
// geometry.go defines the geometric value types the layout engine
// produces and the SVG renderer consumes: Point, Rect, ports, edge
// routes, and the placement records for nodes, summaries, and
// boundaries.
//
// These types live in the root package on purpose. LayoutResult (in
// render.go) embeds slices of them, and the algorithm package
// `m31labs.dev/sirena/layout` imports the root to read frozen IR
// pointers (*Element, *Edge, *Boundary, *BoundarySummary). If the
// geometric types lived under layout/ instead, the root package could
// not name them without importing layout, which already imports the
// root — a cycle. Keeping the data types here and the algorithms in
// layout/ breaks that cycle cleanly: the dependency points one way,
// layout → root, never back.
//
// All coordinates are float64 in an abstract layout space whose origin
// is the canvas top-left; +X runs right, +Y runs down (SVG
// convention). Rectangles are half-open [Min, Max): the Min corner is
// inside, the Max corner is not. That makes adjacent, edge-sharing
// rectangles non-overlapping, which is what the router and collision
// checks want.
package sirena

import "sort"

// Point is a 2-D coordinate in layout space.
type Point struct {
	X, Y float64
}

// Rect is an axis-aligned rectangle with half-open extent [Min, Max):
// a point is inside when Min.X <= p.X < Max.X and Min.Y <= p.Y < Max.Y.
type Rect struct {
	Min, Max Point
}

// Width is the rectangle's horizontal extent (Max.X - Min.X).
func (r Rect) Width() float64 { return r.Max.X - r.Min.X }

// Height is the rectangle's vertical extent (Max.Y - Min.Y).
func (r Rect) Height() float64 { return r.Max.Y - r.Min.Y }

// Center returns the rectangle's midpoint.
func (r Rect) Center() Point {
	return Point{X: (r.Min.X + r.Max.X) / 2, Y: (r.Min.Y + r.Max.Y) / 2}
}

// Contains reports whether p lies inside r under half-open [Min, Max)
// semantics: the Min edges are inclusive, the Max edges exclusive.
func (r Rect) Contains(p Point) bool {
	return p.X >= r.Min.X && p.X < r.Max.X &&
		p.Y >= r.Min.Y && p.Y < r.Max.Y
}

// Intersects reports whether r and other overlap. Half-open semantics
// mean two rectangles that merely share an edge (r.Max.X == other.Min.X)
// do NOT intersect.
func (r Rect) Intersects(other Rect) bool {
	return r.Min.X < other.Max.X && other.Min.X < r.Max.X &&
		r.Min.Y < other.Max.Y && other.Min.Y < r.Max.Y
}

// PortSide names the side of a node rectangle an edge attaches to.
// The iota order (top, right, bottom, left) is the canonical port-sort
// order; do not reorder without updating SortPorts and the renderer.
type PortSide int

const (
	// PortSideTop is the top edge of a node rectangle.
	PortSideTop PortSide = iota
	// PortSideRight is the right edge.
	PortSideRight
	// PortSideBottom is the bottom edge.
	PortSideBottom
	// PortSideLeft is the left edge.
	PortSideLeft
)

// String returns the side keyword, e.g. "top".
func (s PortSide) String() string {
	switch s {
	case PortSideTop:
		return "top"
	case PortSideRight:
		return "right"
	case PortSideBottom:
		return "bottom"
	case PortSideLeft:
		return "left"
	default:
		return "unknown"
	}
}

// Port is an edge attachment point on one side of a node rectangle.
// Offset runs 0..1 along that side (0 = the side's start corner in
// reading order); Anchor is the absolute layout-space coordinate
// derived from the owning rectangle and the offset.
type Port struct {
	Side   PortSide
	Offset float64
	Anchor Point
}

// EdgeLabel is a positioned label for a routed edge.
type EdgeLabel struct {
	Text   string
	Anchor Point
	Bounds Rect
}

// EdgeRoute is a routed edge: an orthogonal polyline from a source port
// to a target port, with an optional positioned label. Edge is the
// original IR pointer and is read-only.
type EdgeRoute struct {
	Edge       *Edge
	Points     []Point
	SourcePort Port
	TargetPort Port
	Label      *EdgeLabel
}

// IsOrthogonal reports whether every consecutive segment of the polyline
// is axis-aligned (horizontal or vertical). A route with fewer than two
// points is vacuously orthogonal.
func (er EdgeRoute) IsOrthogonal() bool {
	for i := 1; i < len(er.Points); i++ {
		a, b := er.Points[i-1], er.Points[i]
		if a.X != b.X && a.Y != b.Y {
			return false
		}
	}
	return true
}

// NodePlacement positions one element. Node is the original IR pointer
// (read-only). Ports are kept sorted by (Side, Offset) via SortPorts.
type NodePlacement struct {
	Node   *Element
	Bounds Rect
	Ports  []Port
}

// SortPorts orders the placement's ports by side (top, right, bottom,
// left) and, within a side, by ascending offset. The sort is stable.
func (np *NodePlacement) SortPorts() { sortPorts(np.Ports) }

// SummaryPlacement positions a collapsed boundary's summary node.
// Summary is the original IR pointer (read-only).
type SummaryPlacement struct {
	Summary *BoundarySummary
	Bounds  Rect
	Ports   []Port
}

// SortPorts orders the summary placement's ports like NodePlacement.
func (sp *SummaryPlacement) SortPorts() { sortPorts(sp.Ports) }

// BoundaryPlacement positions a boundary region. Boundary is the
// original IR pointer, nil for the implicit top-level region. Bounds is
// the outer bounding box including padding; ChildrenBounds is the inner
// content box; Children are nested boundary placements.
type BoundaryPlacement struct {
	Boundary       *Boundary
	Bounds         Rect
	ChildrenBounds Rect
	Children       []*BoundaryPlacement
}

// sortPorts orders ports by (Side, Offset), stably.
func sortPorts(ports []Port) {
	sort.SliceStable(ports, func(i, j int) bool {
		if ports[i].Side != ports[j].Side {
			return ports[i].Side < ports[j].Side
		}
		return ports[i].Offset < ports[j].Offset
	})
}
