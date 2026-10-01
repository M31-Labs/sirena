package layout

import (
	"container/heap"
	"math"
	"slices"
	"sort"

	"m31labs.dev/sirena"
)

func routeClear(points []sirena.Point, obstacles []sirena.Rect) bool {
	for i := 1; i < len(points); i++ {
		if !segClear(points[i-1], points[i], obstacles) {
			return false
		}
	}
	return true
}

type routeVisit struct {
	id   int
	g, f float64
}
type routeQueue []routeVisit

func (q routeQueue) Len() int { return len(q) }
func (q routeQueue) Less(i, j int) bool {
	if q[i].f != q[j].f {
		return q[i].f < q[j].f
	}
	if q[i].g != q[j].g {
		return q[i].g > q[j].g
	}
	return q[i].id < q[j].id
}
func (q routeQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *routeQueue) Push(v any)   { *q = append(*q, v.(routeVisit)) }
func (q *routeQueue) Pop() any     { n := len(*q) - 1; v := (*q)[n]; *q = (*q)[:n]; return v }

// Search a lazy rectilinear visibility grid only when the cheap channel route
// is obstructed. Channels include both sides of every actor. The grid is never
// allocated; a bounded A* search stores only visited intersections.
func visibilityRoute(start, end sirena.Point, obstacles []sirena.Rect) ([]sirena.Point, bool) {
	xs, ys := []float64{start.X, end.X}, []float64{start.Y, end.Y}
	for _, r := range obstacles {
		xs = append(xs, r.Min.X-4, r.Max.X+4)
		ys = append(ys, r.Min.Y-4, r.Max.Y+4)
	}
	sort.Float64s(xs)
	sort.Float64s(ys)
	xs = slices.Compact(xs)
	ys = slices.Compact(ys)
	width := len(ys)
	key := func(p sirena.Point) int { return sort.SearchFloat64s(xs, p.X)*width + sort.SearchFloat64s(ys, p.Y) }
	point := func(id int) sirena.Point { return sirena.Point{X: xs[id/width], Y: ys[id%width]} }
	first, target := key(start), key(end)
	distance := func(a, b sirena.Point) float64 { return math.Abs(a.X-b.X) + math.Abs(a.Y-b.Y) }
	cost := map[int]float64{first: 0}
	previous := map[int]int{}
	queue := &routeQueue{{id: first, f: distance(start, end)}}
	heap.Init(queue)
	for visits := 0; queue.Len() > 0 && visits < 20000; visits++ {
		current := heap.Pop(queue).(routeVisit)
		if current.g != cost[current.id] {
			continue
		}
		if current.id == target {
			path := []sirena.Point{end}
			for id := target; id != first; {
				id = previous[id]
				path = append(path, point(id))
			}
			slices.Reverse(path)
			return simplify(path), true
		}
		x, y := current.id/width, current.id%width
		neighbors := []int{}
		if x > 0 {
			neighbors = append(neighbors, current.id-width)
		}
		if x+1 < len(xs) {
			neighbors = append(neighbors, current.id+width)
		}
		if y > 0 {
			neighbors = append(neighbors, current.id-1)
		}
		if y+1 < len(ys) {
			neighbors = append(neighbors, current.id+1)
		}
		a := point(current.id)
		for _, id := range neighbors {
			b := point(id)
			if !segClear(a, b, obstacles) {
				continue
			}
			g := current.g + distance(a, b)
			if old, ok := cost[id]; ok && old <= g {
				continue
			}
			cost[id] = g
			previous[id] = current.id
			heap.Push(queue, routeVisit{id: id, g: g, f: g + distance(b, end)})
		}
	}
	return nil, false
}
