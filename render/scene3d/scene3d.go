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
	Targets        []string
	Motion         bool
	MotionStyle    string
	MotionSpeed    *float64
	MotionDistance *float64
	Tour           string
	Steps          []Step
}

// Step is an absolute keyframe. Missing fields restore the original layout,
// making direct seeks and backwards navigation independent of prior steps.
type Step struct {
	Label   string   `json:"label"`
	Patches []Patch  `json:"patches,omitempty"`
	Focus   []string `json:"focus,omitempty"`
	Reveal  []string `json:"reveal,omitempty"`
	Trace   []string `json:"trace,omitempty"`
}
type Patch struct {
	Target  string   `json:"target"`
	Visible *bool    `json:"visible,omitempty"`
	Color   *string  `json:"color,omitempty"`
	X       *float64 `json:"x,omitempty"`
	Y       *float64 `json:"y,omitempty"`
	Z       *float64 `json:"z,omitempty"`
	Scale   *float64 `json:"scale,omitempty"`
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
	if lr != nil && (lr.Diagram == "pie" || lr.Diagram == "sankey") {
		return nil, fmt.Errorf("sirena scene3d: %s currently requires SVG export; use bar, line, scatter or radar for native 3D data", lr.Diagram)
	}
	if opts.MotionStyle != "" && opts.MotionStyle != "spin" && opts.MotionStyle != "float" {
		return nil, fmt.Errorf("sirena scene3d: motion style must be spin or float")
	}

	if (opts.MotionSpeed != nil || opts.MotionDistance != nil) && !opts.Motion && opts.MotionStyle == "" {
		return nil, fmt.Errorf("sirena scene3d: motion controls require motion or a motion style")
	}
	for _, value := range []*float64{opts.MotionSpeed, opts.MotionDistance} {
		if value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0 || *value > 100) {
			return nil, fmt.Errorf("sirena scene3d: motion speed and distance must be finite and between 0 and 100")
		}
	}
	if opts.MotionDistance != nil && opts.MotionStyle != "float" {
		return nil, fmt.Errorf("sirena scene3d: motion distance requires float style")
	}
	if opts.Tour != "" && len(opts.Steps) > 0 {
		return nil, fmt.Errorf("sirena scene3d: tour and explicit steps are mutually exclusive")
	}
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
	// Center the server layout at the origin. Very wide records need a separate
	// vertical scale: a capped label must not collapse all measured rows into
	// a few pixels because the original untruncated text measured very wide.
	scale := 12 / math.Max(lr.Bounds.Width(), lr.Bounds.Height()*1.7)
	scaleY := scale
	if (lr.Diagram == "class" || lr.Diagram == "er") && lr.Bounds.Width() > lr.Bounds.Height()*6 {
		scaleY = 6 / lr.Bounds.Height()
	}
	center := lr.Bounds.Center()
	point := func(p sirena.Point) scene.Vector3 { return scene.Vec3((p.X-center.X)*scale, (center.Y-p.Y)*scaleY, 0) }
	var nodes []scene.Node
	// Record decorations share their owner's presentation lifetime.
	attachments := map[string][]string{}
	accessible := "Sirena diagram"
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
		width, height := placement.Bounds.Width()*scale, placement.Bounds.Height()*scaleY
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
		if lr.Diagram == "bar" || lr.Diagram == "gantt" || lr.Diagram == "timeline" || lr.Diagram == "class" || lr.Diagram == "er" {
			geometry = scene.BoxGeometry{Width: width, Height: height, Depth: .35}
		}
		if lr.Plot != nil || lr.Radar != nil {
			geometry = scene.SphereGeometry{Radius: .045, Segments: 16}
		}
		material := scene.Material(scene.StandardMaterial{Color: color, Roughness: .35, Metalness: .15})
		if custom != nil && (len(selected) == 0 || selected[id]) {
			material = custom
		}
		mesh := scene.Mesh{ID: id, Geometry: geometry, Material: material, Position: point(placement.Bounds.Center())}

		if opts.Motion || opts.MotionStyle != "" {
			speed := .18
			if opts.MotionStyle == "float" {
				speed = .7
			}
			if opts.MotionSpeed != nil {
				speed = *opts.MotionSpeed
			}
			if speed > 0 {
				if opts.MotionStyle == "float" {
					distance := .1
					if opts.MotionDistance != nil {
						distance = *opts.MotionDistance
					}
					if distance > 0 {
						mesh.Drift = scene.Vec3(0, distance, 0)
						mesh.DriftSpeed = speed
						mesh.DriftPhase = float64(len(nodes)) * .2
					}
				} else {
					mesh.Spin = scene.Euler{Y: speed}
				}
			}
		}

		text := elementLabel(n)
		if lr.Plot != nil {
			x, _ := n.Metadata["x"].(sirena.Number)
			y, _ := n.Metadata["y"].(sirena.Number)
			text += fmt.Sprintf(": (%g, %g)", x.Value, y.Value)
		}
		if lr.Diagram == "bar" || lr.Radar != nil {
			if value, ok := n.Metadata["value"].(sirena.Number); ok {
				text = fmt.Sprintf("%s: %g", text, value.Value)
			}
		}
		if lr.Diagram == "gantt" {
			start, _ := n.Metadata["start"].(sirena.String)
			end, _ := n.Metadata["end"].(sirena.String)
			text += " · " + start.Value + " → " + end.Value
		}
		if lr.Diagram == "class" || lr.Diagram == "er" {
			if err := emit(mesh); err != nil {
				return nil, err
			}
			for _, item := range recordNodes(placement, mesh, scale, scaleY) {
				var childID string
				switch child := item.(type) {
				case scene.Label:
					childID = child.ID
					accessible += "; " + child.Text
					child.Text = recordLabelText(child.Text, 14)
					item = child
				case scene.Mesh:
					childID = child.ID
				}
				if err := add(childID); err != nil {
					return nil, err
				}
				attachments[id] = append(attachments[id], childID)
				if err := emit(item); err != nil {
					return nil, err
				}
			}
			continue
		}
		label := nodeLabel(id, text, height)
		// GoSX resolves target anchors when lowering the scene. Mirror native drift
		// so the label keeps following its mesh during browser animation.
		label.Shift, label.DriftSpeed, label.DriftPhase = mesh.Drift, mesh.DriftSpeed, mesh.DriftPhase
		if err := emit(mesh, label); err != nil {
			return nil, err
		}
	}
	if lr.Plot != nil || lr.Radar != nil {
		var series []sirena.ChartSeries
		if lr.Plot != nil {
			series = lr.Plot.Series
		}
		if lr.Radar != nil {
			series = lr.Radar.Series
		}
		if lr.Diagram != "scatter" {
			for i, s := range series {
				var points []scene.Vector3
				var segments [][2]int
				for j, p := range s.Points {
					points = append(points, point(p))
					if j > 0 {
						segments = append(segments, [2]int{j - 1, j})
					}
				}
				if lr.Radar != nil && len(points) > 2 {
					segments = append(segments, [2]int{len(points) - 1, 0})
				}
				id := fmt.Sprintf("chart:series:%d", i)
				if err := add(id); err != nil {
					return nil, err
				}
				if err := emit(scene.Mesh{ID: id, Geometry: scene.LinesGeometry{Points: points, Segments: segments, Width: 2}, Material: scene.StandardMaterial{Color: "#a8d7f2", Emissive: 1}}); err != nil {
					return nil, err
				}
			}
		}
		var axes []sirena.Point
		if p := lr.Plot; p != nil {
			axes = []sirena.Point{p.Bounds.Min, {X: p.Bounds.Min.X, Y: p.Bounds.Max.Y}, {X: p.Bounds.Min.X, Y: p.Bounds.Max.Y}, p.Bounds.Max}
			if err := emit(scene.Label{ID: "chart:domain", Text: fmt.Sprintf("x: %g … %g · y: %g … %g", p.XMin, p.XMax, p.YMin, p.YMax), Position: point(sirena.Point{X: p.Bounds.Center().X, Y: p.Bounds.Max.Y + 36}), Color: "#bacdd2", Font: "12px sans-serif", AnchorX: .5, AnchorY: .5}); err != nil {
				return nil, err
			}
		}
		if r := lr.Radar; r != nil {
			for i, label := range r.Axes {
				angle := -math.Pi/2 + float64(i)*2*math.Pi/float64(len(r.Axes))
				p := sirena.Point{X: r.Center.X + r.Radius*math.Cos(angle), Y: r.Center.Y + r.Radius*math.Sin(angle)}
				axes = append(axes, r.Center, p)
				if err := emit(scene.Label{ID: fmt.Sprintf("chart:axis:%d", i), Text: label, Position: point(p), Color: "#bacdd2", Font: "12px sans-serif", AnchorX: .5, AnchorY: .5, OffsetY: -16}); err != nil {
					return nil, err
				}
			}
		}
		var points []scene.Vector3
		var segments [][2]int
		for i, p := range axes {
			points = append(points, point(p))
			if i%2 == 1 {
				segments = append(segments, [2]int{i - 1, i})
			}
		}
		if err := emit(scene.Mesh{ID: "chart:axes", Geometry: scene.LinesGeometry{Points: points, Segments: segments, Width: 1}, Material: scene.StandardMaterial{Color: "#658d98", Emissive: 1}}); err != nil {
			return nil, err
		}
	}
	for _, line := range lr.Lifelines {
		if line.Actor == nil {
			return nil, fmt.Errorf("sirena scene3d: lifeline has no actor")
		}
		id := "lifeline:" + identity(line.Actor)
		if err := add(id); err != nil {
			return nil, err
		}
		if err := emit(scene.Mesh{ID: id, Geometry: scene.LinesGeometry{Points: []scene.Vector3{point(line.From), point(line.To)}, Segments: [][2]int{{0, 1}}, Width: 1}, Material: scene.StandardMaterial{Color: "#3e5362", Emissive: 1}}); err != nil {
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
		if err := emit(scene.Mesh{ID: id, Geometry: scene.BoxGeometry{Width: placement.Bounds.Width() * scale, Height: placement.Bounds.Height() * scale, Depth: .3}, Material: scene.StandardMaterial{Color: "#879ca8"}, Position: pos}, nodeLabel(id, placement.Summary.Label, placement.Bounds.Height()*scale)); err != nil {
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
			text := route.Label.Text
			collision := "avoid"
			anchor := route.Label.Anchor
			if lr.Diagram == "class" || lr.Diagram == "er" {
				accessible += "; " + route.Label.Text
				text = recordLabelText(text, 12)
				collision = "allow"
			}
			offsetY := -14.0
			first, last := route.Points[0], route.Points[len(route.Points)-1]
			if lr.Diagram == "sequence" {
				offsetY = 2
			} else if math.Abs(last.X-first.X) > math.Abs(last.Y-first.Y) {
				// Horizontal relationships pass below the elevated node labels.
				offsetY = 14
			}
			if scaleY != scale && math.Abs(last.X-first.X) > math.Abs(last.Y-first.Y) {
				// Extremely wide source records squeeze the gutter when fitted.
				// Put their caption above the cards so it cannot cover record rows.
				anchor.Y, offsetY = lr.Bounds.Min.Y-8, 0
			}
			if err := emit(scene.Label{ID: "label:" + id, Text: text, Position: point(anchor), Color: "#bacdd2", Font: "12px sans-serif", Background: "#101923", AnchorX: .5, AnchorY: .5, OffsetY: offsetY, LineHeight: 16, WhiteSpace: "pre", MaxWidth: 320, MaxLines: 1, Overflow: "ellipsis", Collision: collision}); err != nil {
				return nil, err
			}
		}
	}
	props := scene.Props{AriaLabel: accessible, Background: "transparent", CanvasAlpha: scene.Bool(true), Responsive: scene.Bool(true), FillHeight: scene.Bool(true), DragToRotate: scene.Bool(lr.Diagram != "sequence"), MaxFrameRate: 30, MaxDevicePixelRatio: 1.5, MaxPixels: 2_000_000, AdaptiveQuality: scene.Bool(true), Camera: scene.PerspectiveCamera{Position: scene.Vec3(0, 0, 11), FOV: 50, Near: .1, Far: 100}, Environment: scene.Environment{AmbientIntensity: .85}, Graph: scene.NewGraph(nodes...)}
	var payload map[string]any
	if err := json.Unmarshal(props.EngineConfig().Props, &payload); err != nil {
		return nil, err
	}
	if len(opts.Steps) > 0 {
		steps, err := choreographySteps(lr, opts.Steps)
		if err != nil {
			return nil, err
		}
		timeline, err := buildTimeline(props.SceneIR(), steps, attachments)
		if err != nil {
			return nil, err
		}
		payload["slideSteps"] = timeline
	}
	if opts.Tour != "" {
		steps, err := tourSteps(lr, opts.Tour)
		if err != nil {
			return nil, err
		}
		timeline, err := buildTimeline(props.SceneIR(), steps, attachments)
		if err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(timeline)
		if err != nil {
			return nil, err
		}
		if len(encoded) > 4<<20 {
			return nil, fmt.Errorf("sirena scene3d: tour exceeds 4 MiB; select a smaller view")
		}
		payload["slideSteps"] = timeline
	}
	if timeline, ok := payload["slideSteps"]; ok {
		data, err := json.Marshal(timeline)
		if err != nil {
			return nil, err
		}
		if len(data) > 4<<20 {
			return nil, fmt.Errorf("sirena scene3d: presentation keyframes exceed 4 MiB; select a smaller view or fewer steps")
		}
	}
	scene.ApplyShaderLib(payload["scene"].(map[string]any))
	return json.MarshalIndent(payload, "", "  ")
}

func nodeLabel(id, text string, height float64) scene.Label {
	return scene.Label{
		ID: "label:" + id, Target: id, Text: text,
		Position: scene.Vec3(0, height*.65, .5), Priority: 100,
		Color: "#edf8f5", Background: "#101923", Font: "600 16px sans-serif",
		AnchorX: .5, AnchorY: .5, WhiteSpace: "pre", MaxLines: 1, MaxWidth: 320,
		Collision: "avoid",
	}
}

func buildTimeline(ir scene.SceneIR, steps []Step, attachments map[string][]string) (Timeline, error) {
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
	replacements := map[string]bool{}
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
			if patch.Visible != nil || patch.Color != nil {
				replacements[patch.Target] = true
			}
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
			children := attachments[id]
			if len(children) == 0 {
				children = []string{"label:" + id}
			}
			if replacements[id] {
				frame.Commands = append(frame.Commands, scene.Command{Kind: scene.CommandRemoveObject, ObjectID: id})
				if patch.Visible != nil && !*patch.Visible {
					for _, childID := range children {
						if _, ok := labels[childID]; ok {
							frame.Commands = append(frame.Commands, scene.Command{Kind: scene.CommandRemoveObject, ObjectID: childID})
						} else if _, ok := objects[childID]; ok {
							frame.Commands = append(frame.Commands, scene.Command{Kind: scene.CommandRemoveObject, ObjectID: childID})
						}
					}
					continue
				}
				if patch.Color != nil {
					obj.Color = *patch.Color
				}
				frame.Commands = append(frame.Commands, scene.CreateObjectCommand(obj))
			}
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
			// Labels are separate projected objects. Every visible frame recreates
			// the original label, including a reveal after a hidden frame.
			factor := 1.0
			if patch.Scale != nil && len(attachments[id]) > 0 {
				factor = *patch.Scale
			}
			for _, childID := range children {
				if label, ok := labels[childID]; ok {
					label.X = x + (label.X-obj.X)*factor
					label.Y = y + (label.Y-obj.Y)*factor
					label.Z = z + (label.Z-obj.Z)*factor
					frame.Commands = append(frame.Commands, scene.Command{Kind: scene.CommandRemoveObject, ObjectID: label.ID}, scene.CreateLabelCommand(label))
				} else if child, ok := objects[childID]; ok {
					child.X = x + (child.X-obj.X)*factor
					child.Y = y + (child.Y-obj.Y)*factor
					child.Z = z + (child.Z-obj.Z)*factor
					child.ScaleX, child.ScaleY, child.ScaleZ = sx, sy, sz
					frame.Commands = append(frame.Commands, scene.Command{Kind: scene.CommandRemoveObject, ObjectID: childID}, scene.CreateObjectCommand(child))
				}
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
