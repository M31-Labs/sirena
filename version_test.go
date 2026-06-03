package sirena_test

import (
	"strings"
	"testing"

	"m31labs.dev/sirena"
)

// TestCheckVersion_SameMinor confirms that a sirena_version matching the
// current spec minor is accepted with no diagnostic. This is the
// overwhelming-majority case for any workspace produced by the build that
// also consumes it.
func TestCheckVersion_SameMinor(t *testing.T) {
	d, ok := sirena.CheckVersion(sirena.CurrentSpecVersion)
	if !ok {
		t.Error("same minor should be accepted")
	}
	if d.Code != "" {
		t.Errorf("no diagnostic expected: %+v", d)
	}
}

// TestCheckVersion_OlderMinor confirms an older minor is accepted (we can
// always read forward) and surfaces an upgrade-hint diagnostic so callers
// can decide whether to rewrite the file on disk. The diagnostic carries a
// non-empty Code so consumers can filter it.
func TestCheckVersion_OlderMinor(t *testing.T) {
	d, ok := sirena.CheckVersion("0.0")
	if !ok {
		t.Error("older minor should be accepted")
	}
	if d.Code == "" {
		t.Error("expected diagnostic for older minor")
	}
	if !strings.Contains(d.Message, "upgrade") && !strings.Contains(d.Message, "0.0") {
		t.Errorf("message should mention upgrade or version: %q", d.Message)
	}
}

// TestCheckVersion_NewerMinor confirms that a file authored against a
// newer minor of the spec is refused with SIR-VERSION-AHEAD (severity
// Error) and a migration hint pointing the operator at the upgrade path.
// Reading newer files is unsafe because the IR may have grown fields the
// current build doesn't understand.
func TestCheckVersion_NewerMinor(t *testing.T) {
	d, ok := sirena.CheckVersion("0.2")
	if ok {
		t.Error("newer minor should be refused")
	}
	if d.Code != "SIR-VERSION-AHEAD" {
		t.Errorf("Code: %q", d.Code)
	}
	if d.Severity != sirena.SeverityError {
		t.Errorf("Severity: %v", d.Severity)
	}
	if !strings.Contains(d.Message, "upgrade") && !strings.Contains(d.Message, "sirena") {
		t.Errorf("message should mention upgrade or sirena: %q", d.Message)
	}
}

// TestCheckVersion_Empty asserts hand-authored .sir files without any
// sirena_version directive are accepted silently. Authors who write .sir
// by hand shouldn't be pestered for a version directive; only generated
// files carry one.
func TestCheckVersion_Empty(t *testing.T) {
	d, ok := sirena.CheckVersion("")
	if !ok {
		t.Error("empty version should be accepted")
	}
	if d.Code != "" {
		t.Errorf("no diagnostic expected for empty: %+v", d)
	}
}

// TestCheckVersion_Malformed asserts a version that does not parse as
// major.minor is cautiously refused as if it were newer. A misspelled or
// corrupt version is more dangerous than no version at all because it
// signals a generator we don't recognize.
func TestCheckVersion_Malformed(t *testing.T) {
	d, ok := sirena.CheckVersion("not-a-version")
	if ok {
		t.Error("malformed should be refused (cautious)")
	}
	if d.Code != "SIR-VERSION-AHEAD" {
		t.Errorf("Code: %q", d.Code)
	}
}

// TestCompareMinorVersions_Cases sweeps the major.minor comparison so a
// future relax of the parser surfaces obviously in tests rather than as a
// silently-wrong accept/refuse decision in OpenWorkspace.
func TestCompareMinorVersions_Cases(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.0", "0.1", -1},
		{"0.1", "0.1", 0},
		{"0.2", "0.1", 1},
		{"1.0", "0.9", 1},
		{"0.9", "1.0", -1},
		{"0.10", "0.2", 1}, // numeric not lexicographic
	}
	for _, c := range cases {
		got := sirena.CompareMinorVersions(c.a, c.b)
		if got != c.want {
			t.Errorf("CompareMinorVersions(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// TestOpenWorkspace_RefusesNewerSirenaVersion drives the headline refuse
// behavior: a workspace manifest declaring a newer minor than this build
// understands fails OpenWorkspace with a clear SIR-VERSION-AHEAD-tagged
// error. This is the bright-line "stop the world" case — proceeding could
// silently drop fields the IR doesn't yet understand.
func TestOpenWorkspace_RefusesNewerSirenaVersion(t *testing.T) {
	root := fixtureWorkspace(t, map[string]string{
		"sirena.yaml": "sirena_version: \"0.2\"\n",
		"x.sir":       "service api",
	})
	_, err := sirena.OpenWorkspace(root)
	if err == nil {
		t.Fatal("expected error for newer minor")
	}
	if !strings.Contains(err.Error(), "SIR-VERSION-AHEAD") {
		t.Errorf("error should mention SIR-VERSION-AHEAD: %v", err)
	}
}

// TestOpenWorkspace_AcceptsOlderSirenaVersion confirms a workspace
// manifest declaring an older minor opens cleanly and the SirenaVersion
// field records the declared value verbatim (no in-memory rewrite of the
// manifest itself). The upgrade-hint diagnostic surfaces via
// ResolveDiagnostics.
func TestOpenWorkspace_AcceptsOlderSirenaVersion(t *testing.T) {
	root := fixtureWorkspace(t, map[string]string{
		"sirena.yaml": "sirena_version: \"0.0\"\n",
		"x.sir":       "service api",
	})
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	if ws.Manifest == nil || ws.Manifest.SirenaVersion != "0.0" {
		t.Errorf("Manifest.SirenaVersion: %+v", ws.Manifest)
	}
	// Older minor surfaces the upgrade-hint diagnostic via
	// ResolveDiagnostics so a CLI/IDE can prompt the user.
	var found bool
	for _, d := range ws.ResolveDiagnostics() {
		if strings.Contains(d.Message, "0.0") || strings.Contains(d.Message, "upgrade") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected upgrade-hint diagnostic from ResolveDiagnostics; got %+v", ws.ResolveDiagnostics())
	}
}

// TestOpenWorkspace_AcceptsSameSirenaVersion confirms the headline path:
// a workspace manifest pinned to the current spec minor opens cleanly
// with no version-related diagnostic.
func TestOpenWorkspace_AcceptsSameSirenaVersion(t *testing.T) {
	root := fixtureWorkspace(t, map[string]string{
		"sirena.yaml": "sirena_version: \"" + sirena.CurrentSpecVersion + "\"\n",
		"x.sir":       "service api",
	})
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	if ws.Manifest == nil || ws.Manifest.SirenaVersion != sirena.CurrentSpecVersion {
		t.Errorf("Manifest.SirenaVersion: %+v", ws.Manifest)
	}
	for _, d := range ws.ResolveDiagnostics() {
		if d.Code == "SIR-VERSION-AHEAD" {
			t.Errorf("unexpected version diagnostic: %+v", d)
		}
	}
}

// TestOpenWorkspace_NoSirenaVersion confirms a manifest without any
// sirena_version directive is treated as the current minor (hand-authored
// workspace path) with no diagnostic surfaced.
func TestOpenWorkspace_NoSirenaVersion(t *testing.T) {
	root := fixtureWorkspace(t, map[string]string{
		"sirena.yaml": "theme: earth-default\n",
		"x.sir":       "service api",
	})
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	if ws.Manifest == nil || ws.Manifest.SirenaVersion != "" {
		t.Errorf("expected empty SirenaVersion: %+v", ws.Manifest)
	}
}
