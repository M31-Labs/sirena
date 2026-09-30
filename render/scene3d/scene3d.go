// Package scene3d projects Sirena's positioned diagrams into GoSX Scene3D.
// Layout stays on the server; browsers receive bounded native scene payloads.
package scene3d

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"m31labs.dev/gosx/scene"
	"m31labs.dev/sirena"
)

const maxSceneItems = 2000

type Options struct {
	Shader   []byte
	Material string
	// Targets selects node identities for the shader. Empty selects all nodes.
	Targets []string
	Motion  bool
	Steps   []Step
}

// Step is an absolute keyframe. Missing fields restore the original layout,
// making direct seeks and backwards navigation independent of prior steps.
type Step struct {
	Label   string  `json:"label"`
	Patches []Patch `json:"patches,omitempty"`
}
type Patch struct {
	Target string   `json:"target"`
	X      *float64 `json:"x,omitempty"`
	Y      *float64 `json:"y,omitempty"`
	Z      *float64 `json:"z,omitempty"`
	Scale  *float64 `json:"scale,omitempty"`
}
type Frame struct {
	Label    string          `json:"label"`
	Commands []scene.Command `json:"commands"`
}
type Timeline struct {
	Version int     `json:"version"`
	Frames  []Frame `json:"frames"`
}

// Build returns a full Scene3D props document accepted by gosx-slides <Scene3D>.
// It leaves the layout and IR untouched. Scene labels retain readable node and
// relationship text; arrowheads preserve reverse and bidirectional relations.
func Build(lr *sirena.LayoutResult, opts Options) ([]byte, error) {
	if len(opts.Shader) == 0 && (opts.Material != "" || len(opts.Targets) > 0) {
		return nil, fmt.Errorf("sirena scene3d: material and targets require shader source")
	}
	if lr == nil || lr.Bounds.Width() <= 0 || lr.Bounds.Height() <= 0 {
		return nil, fmt.Errorf("sirena scene3d: a non-empty layout is required")
	}
	var custom scene.Material
	if len(opts.Shader) > 0 {
		material, _, err := scene.CompileSelenaMaterial(opts.Shader, scene.SelenaMaterialOptions{Material: opts.Material, Standard: scene.StandardMaterial{Color: "#a8f2da"}})
		if err != nil {
			return nil, err
		}
		custom = material
	}
	// Fixed world scale, with aspect ratio preserved and centered at the origin.
	scale := 12 / math.Max(lr.Bounds.Width(), lr.Bounds.Height()*1.7)
	center := lr.Bounds.Center()
	point := func(p sirena.Point) scene.Vector3 { return scene.Vec3((p.X-center.X)*scale, (center.Y-p.Y)*scale, 0) }
	var nodes []scene.Node
	emit := func(items ...scene.Node) error {
		if len(nodes)+len(items) > maxSceneItems {
			return fmt.Errorf("sirena scene3d: select a view with at most %d scene objects and labels", maxSceneItems)
		}
		nodes = append(nodes, items...)
		return nil
	}
	known := map[string]bool{}
	selected := map[string]bool{}
	for _, id := range opts.Targets {
		selected[id] = true
	}
	add := func(id string) error {
		if id == "" || known[id] {
			return fmt.Errorf("sirena scene3d: duplicate or empty identity %q; assign unique sid metadata", id)
		}
		known[id] = true
		return nil
	}
	identity := func(n *sirena.Element) string {
		if value, ok := n.Metadata["sid"].(sirena.String); ok && value.Value != "" {
			return value.Value
		}
		return n.Name
	}
	for _, placement := range lr.NodePlacements {
		if placement == nil || placement.Node == nil {
			continue
		}
		n := placement.Node
		id := identity(n)
		if err := add(id); err != nil {
			return nil, err
		}
		width, height := placement.Bounds.Width()*scale, placement.Bounds.Height()*scale
		geometry := scene.Geometry(scene.BoxGeometry{Width: width, Height: height, Depth: .35})
		color := "#88cab9"
		switch n.Kind {
		case sirena.ElementKindDatabase, sirena.ElementKindCache:
			geometry = scene.CylinderGeometry{RadiusTop: width * .42, RadiusBottom: width * .42, Height: height, Segments: 32}
			color = "#b9a5f7"
		case sirena.ElementKindClient, sirena.ElementKindExternal:
			geometry = scene.SphereGeometry{Radius: math.Min(width, height) * .5, Segments: 32}
			color = "#a8d7f2"
		case sirena.ElementKindQueue:
			color = "#edc183"
		}
		material := scene.Material(scene.StandardMaterial{Color: color, Roughness: .35, Metalness: .15})
		if custom != nil && (len(selected) == 0 || selected[id]) {
			material = custom
		}
		mesh := scene.Mesh{ID: id, Geometry: geometry, Material: material, Position: point(placement.Bounds.Center())}
		if opts.Motion {
			mesh.Spin = scene.Euler{Y: .18}
		}
		if err := emit(mesh, scene.Label{ID: "label:" + id, Target: id, Text: elementLabel(n), Shift: scene.Vec3(0, -height*.65, .5), Color: "#edf8f5", Background: "#101923", Font: "600 16px sans-serif", Collision: "shift"}); err != nil {
			return nil, err
		}
	}
	for id := range selected {
		if !known[id] {
			return nil, fmt.Errorf("sirena scene3d: unknown shader target %q", id)
		}
	}
	for _, placement := range lr.SummaryPlacements {
		if placement == nil || placement.Summary == nil || placement.Summary.Boundary == nil {
			continue
		}
		id := "summary:" + placement.Summary.Boundary.Name
		if err := add(id); err != nil {
			return nil, err
		}
		pos := point(placement.Bounds.Center())
		if err := emit(scene.Mesh{ID: id, Geometry: scene.BoxGeometry{Width: placement.Bounds.Width() * scale, Height: placement.Bounds.Height() * scale, Depth: .3}, Material: scene.StandardMaterial{Color: "#879ca8"}, Position: pos}, scene.Label{ID: "label:" + id, Target: id, Text: placement.Summary.Label, Shift: scene.Vec3(0, -placement.Bounds.Height()*scale*.65, .5), Color: "#edf8f5"}); err != nil {
			return nil, err
		}
	}
	// Boundaries stay visible as shallow frames behind their contained nodes.
	activeBoundaries := map[*sirena.BoundaryPlacement]bool{}
	var boundaryNodes func([]*sirena.BoundaryPlacement) error
	boundaryNodes = func(placements []*sirena.BoundaryPlacement) error {
		for _, placement := range placements {
			if placement == nil {
				continue
			}
			if activeBoundaries[placement] {
				return fmt.Errorf("sirena scene3d: cyclic boundary placements")
			}
			activeBoundaries[placement] = true
			if placement.Boundary != nil {
				id := "boundary:" + placement.Boundary.Name
				if err := add(id); err != nil {
					return err
				}
				a, b := point(placement.Bounds.Min), point(placement.Bounds.Max)
				points := []scene.Vector3{scene.Vec3(a.X, a.Y, -.7), scene.Vec3(b.X, a.Y, -.7), scene.Vec3(b.X, b.Y, -.7), scene.Vec3(a.X, b.Y, -.7)}
				if err := emit(scene.Mesh{ID: id, Geometry: scene.LinesGeometry{Points: points, Segments: [][2]int{{0, 1}, {1, 2}, {2, 3}, {3, 0}}}, Material: scene.StandardMaterial{Color: "#506c7b", Emissive: 1}}, scene.Label{ID: "label:" + id, Text: boundaryLabel(placement.Boundary), Position: scene.Vec3(a.X+.25, a.Y-.2, -.4), Color: "#a7bdc9", Font: "12px sans-serif"}); err != nil {
					return err
				}
			}
			if err := boundaryNodes(placement.Children); err != nil {
				return err
			}
			delete(activeBoundaries, placement)
		}
		return nil
	}
	if err := boundaryNodes(lr.BoundaryPlacements); err != nil {
		return nil, err
	}
	// Route segments are geometry, rather than straight lines replacing the layout.
	for index, route := range lr.EdgeRoutes {
		if route == nil || route.Edge == nil || len(route.Points) < 2 {
			continue
		}
		id := fmt.Sprintf("edge:%d", index)
		if value, ok := route.Edge.Metadata["sid"].(sirena.String); ok && value.Value != "" {
			id = value.Value
		}
		if err := add(id); err != nil {
			return nil, err
		}
		var points []scene.Vector3
		var segments [][2]int
		for i, p := range route.Points {
			v := point(p)
			v.Z = -.25
			points = append(points, v)
			if i > 0 {
				segments = append(segments, [2]int{i - 1, i})
			}
		}
		arrow := func(tip, previous scene.Vector3) {
			dx, dy := tip.X-previous.X, tip.Y-previous.Y
			length := math.Hypot(dx, dy)
			if length == 0 {
				return
			}
			dx /= length
			dy /= length
			for _, side := range []float64{-1, 1} {
				points = append(points, tip, scene.Vec3(tip.X-dx*.18-dy*.09*side, tip.Y-dy*.18+dx*.09*side, tip.Z))
				segments = append(segments, [2]int{len(points) - 2, len(points) - 1})
			}
		}
		last := len(route.Points) - 1
		if route.Edge.Direction != sirena.DirReverse {
			arrow(points[last], points[last-1])
		}
		if route.Edge.Direction == sirena.DirReverse || route.Edge.Direction == sirena.DirBidirectional {
			arrow(points[0], points[1])
		}
		if err := emit(scene.Mesh{ID: id, Geometry: scene.LinesGeometry{Points: points, Segments: segments, Width: 2}, Material: scene.StandardMaterial{Color: "#658d98", Emissive: 1}}); err != nil {
			return nil, err
		}
		if route.Label != nil && route.Label.Text != "" {
			if err := emit(scene.Label{ID: "label:" + id, Text: route.Label.Text, Position: point(route.Label.Anchor), Color: "#bacdd2", Font: "12px sans-serif", Background: "#101923"}); err != nil {
				return nil, err
			}
		}
	}
	props := scene.Props{AriaLabel: "Sirena diagram", Background: "transparent", CanvasAlpha: scene.Bool(true), Responsive: scene.Bool(true), FillHeight: scene.Bool(true), DragToRotate: scene.Bool(true), MaxFrameRate: 30, MaxDevicePixelRatio: 1.5, MaxPixels: 2_000_000, AdaptiveQuality: scene.Bool(true), Camera: scene.PerspectiveCamera{Position: scene.Vec3(0, 0, 11), FOV: 50, Near: .1, Far: 100}, Environment: scene.Environment{AmbientIntensity: .85}, Graph: scene.NewGraph(nodes...)}
	var payload map[string]any
	if err := json.Unmarshal(props.EngineConfig().Props, &payload); err != nil {
		return nil, err
	}
	if len(opts.Steps) > 0 {
		timeline, err := buildTimeline(props.SceneIR(), opts.Steps)
		if err != nil {
			return nil, err
		}
		payload["slideSteps"] = timeline
	}
	scene.ApplyShaderLib(payload["scene"].(map[string]any))
	return json.MarshalIndent(payload, "", "  ")
}

func buildTimeline(ir scene.SceneIR, steps []Step) (Timeline, error) {
	if len(steps) > 128 {
		return Timeline{}, fmt.Errorf("sirena scene3d: at most 128 keyframes")
	}
	objects := map[string]scene.ObjectIR{}
	labels := map[string]scene.LabelIR{}
	for _, label := range ir.Labels {
		labels[label.ID] = label
	}
	for _, obj := range ir.Objects {
		objects[obj.ID] = obj
	}
	targets := map[string]bool{}
	for _, step := range steps {
		for _, patch := range step.Patches {
			if _, ok := objects[patch.Target]; !ok {
				return Timeline{}, fmt.Errorf("sirena scene3d: unknown step target %q", patch.Target)
			}
			for _, v := range []*float64{patch.X, patch.Y, patch.Z, patch.Scale} {
				if v != nil && (math.IsNaN(*v) || math.IsInf(*v, 0)) {
					return Timeline{}, fmt.Errorf("sirena scene3d: non-finite keyframe")
				}
			}
			if patch.Scale != nil && *patch.Scale <= 0 {
				return Timeline{}, fmt.Errorf("sirena scene3d: scale must be positive")
			}
			targets[patch.Target] = true
		}
	}
	ids := make([]string, 0, len(targets))
	for id := range targets {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	timeline := Timeline{Version: 1}
	for _, step := range steps {
		frame := Frame{Label: step.Label, Commands: []scene.Command{}}
		patches := map[string]Patch{}
		for _, patch := range step.Patches {
			if _, ok := patches[patch.Target]; ok {
				return Timeline{}, fmt.Errorf("sirena scene3d: duplicate step target %q", patch.Target)
			}
			patches[patch.Target] = patch
		}
		for _, id := range ids {
			obj := objects[id]
			patch := patches[id]
			x, y, z := obj.X, obj.Y, obj.Z
			if patch.X != nil {
				x = *patch.X
			}
			if patch.Y != nil {
				y = *patch.Y
			}
			if patch.Z != nil {
				z = *patch.Z
			}
			sx, sy, sz := obj.ScaleX, obj.ScaleY, obj.ScaleZ
			if sx == 0 {
				sx = 1
			}
			if sy == 0 {
				sy = 1
			}
			if sz == 0 {
				sz = 1
			}
			if patch.Scale != nil {
				sx *= *patch.Scale
				sy *= *patch.Scale
				sz *= *patch.Scale
			}
			frame.Commands = append(frame.Commands, scene.Command{Kind: scene.CommandSetTransform, ObjectID: id, Data: map[string]any{"x": x, "y": y, "z": z, "rotationX": obj.RotationX, "rotationY": obj.RotationY, "rotationZ": obj.RotationZ, "scaleX": sx, "scaleY": sy, "scaleZ": sz}})
			if label, ok := labels["label:"+id]; ok {
				label.X += x - obj.X
				label.Y += y - obj.Y
				label.Z += z - obj.Z
				frame.Commands = append(frame.Commands, scene.Command{Kind: scene.CommandRemoveObject, ObjectID: label.ID}, scene.CreateLabelCommand(label))
			}
		}
		timeline.Frames = append(timeline.Frames, frame)
	}
	return timeline, nil
}

func elementLabel(n *sirena.Element) string {
	if label, ok := n.Metadata["label"].(sirena.String); ok && label.Value != "" {
		return label.Value
	}
	return n.Name
}
func boundaryLabel(b *sirena.Boundary) string {
	if label, ok := b.Metadata["label"].(sirena.String); ok && label.Value != "" {
		return label.Value
	}
	return b.Name
}
