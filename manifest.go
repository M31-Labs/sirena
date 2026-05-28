// Package sirena: workspace manifest (sirena.toml) parser.
//
// Format note: the workspace manifest filename is conventionally
// `sirena.toml`, but the v0.1 implementation parses it as YAML using
// gopkg.in/yaml.v3. The `.toml` extension is a misnomer carried forward
// from early notes; the extension rename is deferred to Plan 6 so the
// rest of v0.1 can land without touching every reference to the
// filename. Contributors editing this file should treat the format as
// YAML.
package sirena

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Manifest captures workspace-level defaults declared in sirena.toml at
// the workspace root. v0.1 parses the file as YAML despite the .toml
// extension (the rename is deferred to Plan 6 — see the package-level
// note above).
//
// Every field is optional. Workspace loading layers defaults on top of
// a missing or partial manifest; downstream consumers should treat the
// zero value of any field as "use the built-in default".
type Manifest struct {
	// DefaultPreset is the preset name to apply to views that don't
	// declare a `preset:` directive themselves. Corresponds to the
	// `default_preset` YAML key.
	DefaultPreset string
	// Theme is the workspace theme name. Corresponds to the `theme`
	// YAML key.
	Theme string
	// OutputDir is the workspace-relative directory the renderer should
	// write artifacts into. Corresponds to the `output_dir` YAML key.
	OutputDir string
	// Layout carries the workspace-default layout block. Nil when the
	// manifest omits the `layout` key.
	Layout *ManifestLayout
	// Range covers the manifest bytes; useful when the manifest is
	// embedded in a larger document or surfaced in diagnostics. For a
	// standalone file, Start is 0 and End is len(src).
	Range Range
}

// ManifestLayout captures the workspace-default `layout` block. v0.1
// recognises a single field (Direction); future fields are added by
// extending this struct and the private decoder.
type ManifestLayout struct {
	// Direction is the workspace-default flow direction, e.g.
	// "top-down" or "left-right". Corresponds to the `direction` key
	// inside the `layout` YAML mapping.
	Direction string
}

// manifestYAML is the private intermediate decoded by yaml.v3. Splitting
// the wire shape from the public IR lets us evolve the public surface
// without breaking the on-disk format and vice versa.
type manifestYAML struct {
	DefaultPreset string              `yaml:"default_preset"`
	Theme         string              `yaml:"theme"`
	OutputDir     string              `yaml:"output_dir"`
	Layout        *manifestLayoutYAML `yaml:"layout"`
}

type manifestLayoutYAML struct {
	Direction string `yaml:"direction"`
}

// ParseManifest decodes a sirena.toml workspace manifest. The file is
// parsed as YAML in v0.1 despite the `.toml` extension (see the
// package-level note). Empty or nil input yields a zero-value
// *Manifest, not an error — callers loading a possibly-missing file can
// pass the raw bytes through without special-casing absence.
func ParseManifest(src []byte) (*Manifest, error) {
	m := &Manifest{Range: Range{Start: 0, End: len(src)}}
	if len(src) == 0 {
		return m, nil
	}
	var raw manifestYAML
	if err := yaml.Unmarshal(src, &raw); err != nil {
		return nil, fmt.Errorf("sirena: parse manifest: %w", err)
	}
	m.DefaultPreset = raw.DefaultPreset
	m.Theme = raw.Theme
	m.OutputDir = raw.OutputDir
	if raw.Layout != nil {
		m.Layout = &ManifestLayout{Direction: raw.Layout.Direction}
	}
	return m, nil
}
