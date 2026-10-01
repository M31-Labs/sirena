# Sirena

A modernized diagram language and renderer. Arch/systems diagrams as the wedge, with a multi-file project model and a pure-Go layout engine — designed to be mdpp's native diagram surface. No JavaScript toolchain, no headless browser; pure Go end to end.

**Status:** SVG and native GoSX Scene3D export, with 12 native diagram families, deterministic layouts, and reusable SVG glyphs.

Install from source with Go 1.26 or newer:

```sh
git clone https://github.com/M31-Labs/sirena.git
cd sirena
go install ./cmd/sirena
```

The executable is installed in `$(go env GOPATH)/bin` (or `GOBIN` when set).
Add that directory to your `PATH`, then run `sirena --help`.

Native graphics: `sirena render --scene3d` exports GoSX Scene3D props JSON with
typed node shapes, readable labels, routed directional relationships, boundary
frames, and optional Selena materials. The renderer uses GoSX v0.57.1; parsing
uses gotreesitter v0.55.1. The existing SVG renderer remains available.

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
`--material` and `--targets` require `--shader`; the CLI and renderer reject
shaderless selections rather than silently leaving default materials in place.
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
the Scene3D adapter caps all emitted scene objects and labels at 2000, including
nodes, edges, summaries, and nested boundary frames. Keyframes are capped at 128.


### Diagram families and semantic presentation beats

`render --diagram` supports **architecture, sequence, radial, state, class, er,
swimlane, timeline, mindmap, bar, pie, and gantt**. The same declaration identities and relationships
survive SVG and Scene3D export. Pie uses SVG; the other families can use native 3D. These are native Sirena layouts; Mermaid
import remains limited to flowcharts.

- `state`: rounded states with `state: "initial"` / `state: "final"` markers.
- `class` / `er`: measured record compartments. Use `fields: "id: UUID; name: string"`
  and optionally `methods: "save(); validate()"`; relationship labels carry roles
  and cardinalities, such as `"1 to many"`.
- `swimlane`: `lane: "Operations"` groups tasks; declaration order defines the
  progression across lanes.
- `timeline`: numeric `start: 2 duration: 5` values share one time unit. Bar widths
  preserve duration ratios; names remain outside bars. Dates are not parsed.

- `mindmap`: a parent-to-child forest with measured boxes. Cycles and multiple parents
  report clear errors; declaration order makes branching deterministic.
- `bar`: `value: -18` produces a signed horizontal bar chart with a shared zero axis.
- `pie`: non-negative `value` metadata produces proportional sectors and a measured
  legend with values and percentages. At most 100 slices; no relationships.
- `gantt`: `start: "2026-10-01" end: "2026-10-04"` uses actual UTC calendar days
  and an exclusive end date. Dependency relationships remain visible.

Copy the runnable sources in `examples/diagrams/`. Non-architecture layouts
require flat views and reject nested boundaries rather than dropping them.

Scene3D steps accept semantic actions as well as explicit transform patches:

```json
[
  {"label":"Overview"},
  {"label":"Request","reveal":["browser","api"],"trace":["browser->api"]},
  {"label":"Worker","focus":["worker"]},
  {"label":"Overview"}
]
```

```sh
sirena render --scene3d --steps examples/scene3d/choreography.json \
  -o request.scene.json examples/scene3d/request.sir
```

`focus` emphasizes selected node identities, `reveal` lists the complete visible
set (and shows relationships between visible endpoints), and `trace` highlights
relationships. Targets accept names or stable `sid` values. Repeated relationships
need their unique edge ID (`edge:0`, etc.) instead of an ambiguous `from->to` alias.
Each beat is an absolute pose; omitted actions restore the original state, so
links, reverse navigation, and replay do not depend on earlier steps. Timelines
are bounded to 128 frames / 4 MiB.

SVG nodes expose `data-sirena-id` and `data-morph-id` for selection and shared
transitions in hosts such as gosx-slides. `sirena version` reports the CLI release.

## Diagram modes and presentation tours

Choose `--diagram sequence` for ordered interactions and lifelines, or `--diagram radial` for a centered dependency diagram. Source `label` metadata is honored by layout and SVG as well as Scene3D; layout measures the actual bundled font, and SVG exposes accessible labels and directional arrows. Inline SVG styles are scoped so diagrams can use independent themes.

Native Scene3D tours use `--tour nodes` or `--tour relationships`. They create bounded, absolute presentation keyframes automatically. `--motion-style spin|float`, `--motion-speed`, and float `--motion-distance` control native movement without a client animation loop. See [runnable examples](examples/diagrams/README.md) for authoring and the current flat-view and Mermaid ingestion limits.

```sh
sirena render --diagram sequence -o sequence.svg examples/diagrams/sequence.sir
sirena render --diagram radial --scene3d --tour nodes --motion-style float \
  -o radial.scene.json examples/diagrams/radial.sir
```

Run `sirena render --help` to discover every control. Node labels have collision
priority over relationship captions and follow native float motion. Native labels
use a single line capped at 320 pixels; choose concise labels and smaller views
for presentation-sized diagrams. GoSX respects reduced-motion preferences.
Boundary headers reserve space above their children, and SVG viewports include
routed relationships and measured captions to prevent clipping.

## Measured SVG performance

Repeated labels share content-addressed glyph definitions through SVG `<use>`.
The font outlines, accessible text, deterministic byte output, and theme scoping
remain intact. This avoids duplicating the full outline of every repeated letter.

Run the rendering benchmark on your machine:

```sh
go test ./render/svg -run '^$' -bench BenchmarkDiagramPipeline -benchmem
```

It reports time, allocations, and SVG bytes separately for 20, 100, and 500-node
mindmaps, bar charts, sequences, and class diagrams. Parsing and layout are outside
this rendering measurement. Timings vary with hardware; use the same fixtures and
machine when comparing releases. Native 3D tours support mindmap, bar, and Gantt
with `--tour nodes --motion-style float`. Pie is SVG-only in this release.

A same-machine comparison against v0.4.0 (20 iterations, Intel Core Ultra 9 285)
reduced the 500-node class SVG from 6,599,478 to 1,979,600 bytes (70.0%), rendering
allocations from 18.8 MB to 6.25 MB per operation (66.8%), and rendering time from
10.88 ms to 7.95 ms (26.9%). The 500-actor sequence SVG shrank 69.2%. These are
fixture measurements, not an end-to-end comparison with other diagram tools.
