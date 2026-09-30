# Diagram modes and motion

Sequence mode preserves participant declaration order and places each relationship on a separate time row, including repeated and self messages. Radial mode centers the first declared participant and places the others around a ring. Both currently require a flat view; nested boundaries and collapsed summaries produce a clear error.

```sh
sirena render --diagram sequence -o sequence.svg examples/diagrams/sequence.sir
sirena render --diagram radial -o radial.svg examples/diagrams/radial.sir
sirena render --diagram sequence --scene3d --tour relationships -o sequence.scene.json examples/diagrams/sequence.sir
sirena render --diagram radial --scene3d --tour nodes --motion-style float --motion-speed 0.7 --motion-distance 0.1 -o radial.scene.json examples/diagrams/radial.sir
```

Embed the Scene3D JSON in gosx-slides with `<Scene3D Src="sequence.scene.json" />`. Tours start and finish at an overview and focus one participant or relationship at each presentation click. All frames restore their destination's absolute pose, including backwards and direct seeks. No separate keyframe file is required. Use `--steps` for custom poses; it is mutually exclusive with `--tour`.

`--motion` retains spin. `--motion-style spin|float` selects native GoSX motion; speed is radians/second and float distance is world units. Zero speed disables movement. GoSX handles reduced motion and pauses hidden surfaces. Sequence scenes disable drag rotation to preserve time-row readability. Tours are capped at 126 focus steps and 4 MiB of serialized keyframes. Relationship routes retain their original layout while actors are emphasized.

For a named Sirena view, `layout { diagram: sequence }` or `layout { diagram: radial }` selects the mode without a CLI override. This does not add Mermaid sequence syntax ingestion; Mermaid flowcharts can use either layout through `--diagram`.
