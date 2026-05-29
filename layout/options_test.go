package layout

import "testing"

func TestOptions_DefaultPreset(t *testing.T) {
	var o LayoutOptions
	if o.Preset != LayoutPresetLayered {
		t.Errorf("zero LayoutOptions.Preset = %v, want LayoutPresetLayered", o.Preset)
	}
	if got := LayoutPresetLayered.String(); got != "layered" {
		t.Errorf("LayoutPresetLayered.String() = %q, want \"layered\"", got)
	}
	if got := LayoutPresetForce.String(); got != "force" {
		t.Errorf("LayoutPresetForce.String() = %q, want \"force\"", got)
	}
	// Force round-trips: String() output parses back to the same preset.
	if got, ok := PresetForString(LayoutPresetForce.String()); !ok || got != LayoutPresetForce {
		t.Errorf("PresetForString(%q) = %v, %v; want force, true", LayoutPresetForce.String(), got, ok)
	}
}
