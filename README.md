# Sirena

A modernized diagram language and renderer. Arch/systems diagrams as the wedge, with a multi-file project model and a pure-Go layout engine — designed to be mdpp's native diagram surface. No JavaScript toolchain, no headless browser; pure Go end to end.

**Status:** pre-v0.1, in active development. Public release is gated on the v0.1 ship plan. See `hypha://m31labs/sirena` (M31 Labs internal) for the design spec and ongoing plan.

Native graphics: `sirena render --scene3d` exports GoSX Scene3D props JSON with
typed node shapes, readable labels, routed directional relationships, boundary
frames, and optional Selena materials. The renderer uses GoSX v0.57.1; parsing
uses gotreesitter v0.55.1. The existing SVG and replay renderers remain available.

```sh
go run ./cmd/sirena render --scene3d \
  --shader examples/scene3d/material.sel --targets api,worker \
  --steps examples/scene3d/steps.json \
  -o examples/scene3d/request.scene.json examples/scene3d/request.sir
```

Use the JSON in gosx-slides: `<Scene3D Src="scenes/request.scene.json" />`.
`--motion` adds slow node rotation; hidden surfaces pause through GoSX.
`--material` selects a named Selena material, and `--targets` selects stable
node identities (`sid` metadata, falling back to declaration names).
Shaders are compiled to GLSL and WGSL and shared through the scene shader library.
Mermaid flowcharts can also use `--scene3d --infer`.

Keyframes are a JSON array of `{ "label": "Focus API", "patches": [{ "target":
"api", "z": 1.4, "scale": 1.35 }] }`. Supported pose fields are `x`, `y`, `z`,
and positive `scale`. Each frame is absolute relative to the original layout;
omitted fields restore that layout, so backward and direct seeks agree. Labels
follow positioned nodes; edges retain their original routes. The exported
`slideSteps` version 1 transport contains native GoSX commands for each frame.
The first frame is the initial state; subsequent frames are presentation steps.

Defaults cap graphics at 30 FPS, 1.5 device pixel ratio, and two million pixels
with adaptive quality. Large systems should select a view before rendering:
the Scene3D adapter caps nodes plus edges at 2000 and keyframes at 128.
