package layout

// LayoutPreset selects which layout algorithm Compute runs.
type LayoutPreset int

const (
	// LayoutPresetLayered is the default: a Sugiyama-class layered
	// pipeline (rank → order → coord) per boundary scope with orthogonal
	// edge routing and a constraint-solved top-level skeleton. It is the
	// zero value so LayoutOptions{} selects it.
	LayoutPresetLayered LayoutPreset = iota
	// LayoutPresetForce is a seeded Fruchterman-Reingold fallback. It is
	// available by explicit request only; it never runs by default.
	LayoutPresetForce
)

// String returns the preset's source keyword, e.g. "layered".
func (p LayoutPreset) String() string {
	switch p {
	case LayoutPresetLayered:
		return "layered"
	case LayoutPresetForce:
		return "force"
	default:
		return "unknown"
	}
}

// PresetForString parses a preset keyword back into a LayoutPreset. The
// second result is false for an unrecognized keyword, so callers can
// distinguish "force" from a typo rather than silently defaulting.
func PresetForString(s string) (LayoutPreset, bool) {
	switch s {
	case "layered":
		return LayoutPresetLayered, true
	case "force":
		return LayoutPresetForce, true
	default:
		return LayoutPresetLayered, false
	}
}

// LayoutOptions configures Compute. The zero value is valid and selects
// the layered preset with a seed derived from the view hash.
type LayoutOptions struct {
	// Preset chooses the layout algorithm. The zero value is
	// LayoutPresetLayered.
	Preset LayoutPreset
	// Seed overrides the RNG seed used by the force preset. Nil means
	// derive the seed from sirena.ViewHash(rv), which is the production
	// path; tests set it to pin or perturb force-directed output.
	Seed *[32]byte
}
