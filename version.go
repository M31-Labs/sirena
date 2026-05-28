package sirena

import (
	"fmt"
	"strconv"
	"strings"
)

// CurrentSpecVersion is the sirena spec version this build implements.
// It is the canonical "major.minor" string carried in generated workspace
// manifests (`sirena_version:`) and on .gen.sir frontmatter. The minor
// component is the compatibility axis: this build refuses files declaring
// a newer minor and accepts (with an upgrade hint) files declaring an
// older minor.
const CurrentSpecVersion = "0.1"

// CheckVersion compares a declared sirena_version against CurrentSpecVersion
// and returns the canonical Diagnostic + accept/refuse decision pair.
//
// The contract has four cases, ordered by likelihood:
//
//   - declared == "": the caller is a hand-authored .sir without any
//     version directive. Accepted silently — empty diagnostic, bool true.
//   - declared parses to the same minor as CurrentSpecVersion: accepted
//     silently — empty diagnostic, bool true.
//   - declared parses to an older minor: accepted with an upgrade-hint
//     Diagnostic (SIR-VERSION-AHEAD-prefixed code so consumers can filter
//     for "we should rewrite this file on disk"); bool true.
//   - declared parses to a newer minor, or doesn't parse at all: refused
//     with SIR-VERSION-AHEAD (severity Error) and bool false. Refusing
//     malformed inputs is the cautious choice — a generator we don't
//     recognize is more dangerous than a generator we know is too new.
//
// The Diagnostic's Range is zero — the caller is expected to overlay a
// real source range at the call site (frontmatter directive, manifest
// field, etc.) before surfacing to a UI.
func CheckVersion(declared string) (Diagnostic, bool) {
	if declared == "" {
		return Diagnostic{}, true
	}
	cmp, err := compareMinor(declared, CurrentSpecVersion)
	if err != nil {
		// Cautious refuse: a malformed sirena_version is more dangerous
		// than a clearly-newer one because it signals a generator we
		// don't recognize at all.
		return Diagnostic{
			Code:     "SIR-VERSION-AHEAD",
			Severity: SeverityError,
			Message: fmt.Sprintf(
				"sirena_version %q is malformed; upgrade sirena or correct the directive (this build implements %s)",
				declared, CurrentSpecVersion,
			),
		}, false
	}
	switch {
	case cmp > 0:
		return Diagnostic{
			Code:     "SIR-VERSION-AHEAD",
			Severity: SeverityError,
			Message: fmt.Sprintf(
				"sirena_version %q is newer than this build (%s); upgrade sirena to read this file",
				declared, CurrentSpecVersion,
			),
		}, false
	case cmp < 0:
		return Diagnostic{
			Code:     "SIR-VERSION-AHEAD",
			Severity: SeverityWarning,
			Message: fmt.Sprintf(
				"sirena_version %q is older than this build (%s); the IR will be upgraded in memory — rewrite the file to %s on next save",
				declared, CurrentSpecVersion, CurrentSpecVersion,
			),
		}, true
	default:
		return Diagnostic{}, true
	}
}

// CompareMinorVersions returns -1 if a < b, 0 if equal, 1 if a > b under
// the major.minor numeric ordering. Versions are parsed as `<major>.<minor>`
// (any additional dot-segments are ignored). Malformed inputs panic; use
// CheckVersion for the public-facing path that tolerates them.
//
// The exported form is provided so generators can implement their own
// "should I rewrite this file?" policies without re-deriving the
// comparator from CheckVersion's behavior.
func CompareMinorVersions(a, b string) int {
	cmp, err := compareMinor(a, b)
	if err != nil {
		panic(fmt.Sprintf("sirena.CompareMinorVersions: %v", err))
	}
	return cmp
}

// compareMinor is the unexported worker shared by CheckVersion and
// CompareMinorVersions. Returns an error rather than panicking so the
// public CheckVersion path can map malformed inputs to a cautious
// refuse instead of crashing the caller's goroutine.
func compareMinor(a, b string) (int, error) {
	aMaj, aMin, err := splitMajorMinor(a)
	if err != nil {
		return 0, fmt.Errorf("parse %q: %w", a, err)
	}
	bMaj, bMin, err := splitMajorMinor(b)
	if err != nil {
		return 0, fmt.Errorf("parse %q: %w", b, err)
	}
	if aMaj != bMaj {
		if aMaj < bMaj {
			return -1, nil
		}
		return 1, nil
	}
	if aMin == bMin {
		return 0, nil
	}
	if aMin < bMin {
		return -1, nil
	}
	return 1, nil
}

// splitMajorMinor parses `<major>.<minor>` into two ints. Additional
// dot-segments after the minor are tolerated and ignored so future patch
// suffixes (`0.1.3`) compare against `0.1` cleanly. Negative numbers and
// non-numeric segments are rejected.
func splitMajorMinor(v string) (int, int, error) {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("missing minor component")
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("major: %w", err)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("minor: %w", err)
	}
	if major < 0 || minor < 0 {
		return 0, 0, fmt.Errorf("negative version segment")
	}
	return major, minor, nil
}
