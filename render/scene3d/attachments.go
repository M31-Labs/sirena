package scene3d

import (
	"fmt"
	"math"

	"m31labs.dev/gosx/scene"
	"m31labs.dev/sirena"
)

type edgeAttachment struct {
	From, To  string
	Count     int
	Direction sirena.Direction
}

func edgeAttachments(lr *sirena.LayoutResult) map[string]edgeAttachment {
	ids := map[string]string{}
	for _, p := range lr.NodePlacements {
		if p == nil || p.Node == nil {
			continue
		}
		id := p.Node.Name
		if sid, ok := p.Node.Metadata["sid"].(sirena.String); ok && sid.Value != "" {
			id = sid.Value
		}
		ids[p.Node.Name] = id
	}
	links := map[string]edgeAttachment{}
	for i, r := range lr.EdgeRoutes {
		if r == nil || r.Edge == nil || len(r.Points) < 2 {
			continue
		}
		id := fmt.Sprintf("edge:%d", i)
		if sid, ok := r.Edge.Metadata["sid"].(sirena.String); ok && sid.Value != "" {
			id = sid.Value
		}
		if ids[r.Edge.From] != "" && ids[r.Edge.To] != "" {
			links[id] = edgeAttachment{ids[r.Edge.From], ids[r.Edge.To], len(r.Points), r.Edge.Direction}
		}
	}
	return links
}

func poseRelationship(edge scene.ObjectIR, label scene.LabelIR, link edgeAttachment, objects map[string]scene.ObjectIR, patches map[string]Patch) (scene.ObjectIR, *scene.LabelIR) {
	if link.Count < 2 || len(edge.Points) < link.Count {
		return edge, nil
	}
	delta := func(point scene.Vector3, id string) scene.Vector3 {
		obj, p := objects[id], patches[id]
		factor := 1.0
		if p.Scale != nil {
			factor = *p.Scale
		}
		x, y, z := obj.X, obj.Y, obj.Z
		if p.X != nil {
			x = *p.X
		}
		if p.Y != nil {
			y = *p.Y
		}
		if p.Z != nil {
			z = *p.Z
		}
		return scene.Vec3(x-obj.X+(point.X-obj.X)*(factor-1), y-obj.Y+(point.Y-obj.Y)*(factor-1), z-obj.Z+(point.Z-obj.Z)*(factor-1))
	}
	from, to := delta(edge.Points[0], link.From), delta(edge.Points[link.Count-1], link.To)
	length := 0.0
	distances := make([]float64, link.Count)
	for i := 1; i < link.Count; i++ {
		a, b := edge.Points[i-1], edge.Points[i]
		length += math.Sqrt((a.X-b.X)*(a.X-b.X) + (a.Y-b.Y)*(a.Y-b.Y) + (a.Z-b.Z)*(a.Z-b.Z))
		distances[i] = length
	}
	points := make(scene.LinePoints, 0, len(edge.Points))
	for i, p := range edge.Points[:link.Count] {
		weight := float64(i) / float64(link.Count-1)
		if length > 0 {
			weight = distances[i] / length
		}
		points = append(points, scene.Vec3(p.X+from.X*(1-weight)+to.X*weight, p.Y+from.Y*(1-weight)+to.Y*weight, p.Z+from.Z*(1-weight)+to.Z*weight))
	}
	segments := make([][2]int, 0, len(edge.LineSegments))
	for i := 1; i < link.Count; i++ {
		segments = append(segments, [2]int{i - 1, i})
	}
	arrow := func(tip, previous scene.Vector3) {
		dx, dy, dz := tip.X-previous.X, tip.Y-previous.Y, tip.Z-previous.Z
		n := math.Sqrt(dx*dx + dy*dy + dz*dz)
		if n == 0 {
			return
		}
		dx /= n
		dy /= n
		dz /= n
		px, py, pz := -dy, dx, 0.0
		pn := math.Hypot(px, py)
		if pn < 1e-9 {
			px, py, pz = 0, -dz, dy
			pn = math.Sqrt(py*py + pz*pz)
		}
		px /= pn
		py /= pn
		pz /= pn
		for _, sign := range []float64{-1, 1} {
			points = append(points, tip, scene.Vec3(tip.X-dx*.18+px*.09*sign, tip.Y-dy*.18+py*.09*sign, tip.Z-dz*.18+pz*.09*sign))
			segments = append(segments, [2]int{len(points) - 2, len(points) - 1})
		}
	}
	if link.Direction != sirena.DirReverse {
		arrow(points[link.Count-1], points[link.Count-2])
	}
	if link.Direction == sirena.DirReverse || link.Direction == sirena.DirBidirectional {
		arrow(points[0], points[1])
	}
	edge.Points, edge.LineSegments = points, segments
	label.X += (from.X + to.X) / 2
	label.Y += (from.Y + to.Y) / 2
	label.Z += (from.Z + to.Z) / 2
	return edge, &label
}

func validateStoryCamera(camera scene.IRCamera) error {
	if camera.Kind != "" && camera.Kind != "perspective" && camera.Kind != "orthographic" {
		return fmt.Errorf("sirena scene3d: camera kind must be perspective or orthographic")
	}
	for _, v := range []float64{camera.X, camera.Y, camera.Z, camera.RotationX, camera.RotationY, camera.RotationZ, camera.FOV, camera.Left, camera.Right, camera.Top, camera.Bottom, camera.Zoom, camera.Near, camera.Far} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("sirena scene3d: non-finite camera")
		}
	}
	if camera.FOV < 0 || camera.FOV >= 180 || camera.Zoom < 0 || camera.Near < 0 || camera.Far < 0 || camera.Near > 0 && camera.Far > 0 && camera.Far <= camera.Near {
		return fmt.Errorf("sirena scene3d: invalid camera projection")
	}
	return nil
}
