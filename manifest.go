// Package sirena: workspace manifest (sirena.yaml) parser.
//
// Format note: the workspace manifest is `sirena.yaml`, parsed as YAML
// using gopkg.in/yaml.v3. The pre-v0.1.0 name was `sirena.toml` — a
// misnomer, since the format was always YAML; OpenWorkspace still reads it
// as a deprecated fallback (see workspace.go).
package sirena

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Manifest captures workspace-level defaults declared in sirena.yaml at
// the workspace root (or the deprecated sirena.toml fallback). The file is
// parsed as YAML — see the package-level note above.
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
	// SirenaVersion is the spec version the workspace was authored
	// against, e.g. "0.1". Corresponds to the `sirena_version` YAML key.
	// Empty when the manifest omits the directive — a hand-authored
	// workspace is treated as the current spec minor. OpenWorkspace
	// checks the field against CurrentSpecVersion via CheckVersion and
	// refuses the workspace when the declared minor is newer than this
	// build's.
	SirenaVersion string
	// Layout carries the workspace-default layout block. Nil when the
	// manifest omits the `layout` key.
	Layout *ManifestLayout
	// Exclude is a list of glob patterns that prune files and directories
	// from workspace enumeration. Each pattern is matched (via path.Match)
	// against both the workspace-relative path and the base name of every
	// candidate, so "testdata" prunes any directory of that name at any
	// depth, "*.gen.sir" drops every generated file, and "examples/demo"
	// targets a specific subtree. Corresponds to the `exclude` YAML key.
	// Empty when the manifest omits the directive.
	Exclude []string
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
	SirenaVersion string              `yaml:"sirena_version"`
	Layout        *manifestLayoutYAML `yaml:"layout"`
	Exclude       []string            `yaml:"exclude"`
}

type manifestLayoutYAML struct {
	Direction string `yaml:"direction"`
}

// ParseManifest decodes a sirena.yaml workspace manifest. The file is
// parsed as YAML (see the package-level note). Empty or nil input yields a
// zero-value
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
	m.SirenaVersion = raw.SirenaVersion
	m.Exclude = raw.Exclude
	if raw.Layout != nil {
		m.Layout = &ManifestLayout{Direction: raw.Layout.Direction}
	}
	return m, nil
}
