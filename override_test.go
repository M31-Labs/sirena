package sirena_test

import (
	"io"
	"math/rand"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"m31labs.dev/sirena"
)

// fixedClock returns a Clock function that always reports the given Unix
// second. Used so ULID timestamps are deterministic in fixtures.
func fixedClock(secs int64) func() time.Time {
	return func() time.Time { return time.Unix(secs, 0) }
}

// seededEntropy returns a deterministic io.Reader seeded from the given
// integer. ULID generation pulls 10 random bytes after the timestamp; a
// seeded math/rand keeps fixture output byte-stable.
func seededEntropy(seed int64) io.Reader {
	return rand.New(rand.NewSource(seed))
}

// TestEmitSIDs_AssignsToElementsBoundariesEdges drives the headline
// emission contract: every Element / Boundary / Edge in a freshly-built
// Document gets a ULID `sid:` metadata value. The deterministic clock and
// entropy keep the test stable without pinning specific ULID bytes.
func TestEmitSIDs_AssignsToElementsBoundariesEdges(t *testing.T) {
	doc := &sirena.Document{
		Systems: []*sirena.SystemDecl{{
			Elements:   []*sirena.Element{{Kind: sirena.ElementKindService, Name: "api"}},
			Boundaries: []*sirena.Boundary{{Kind: sirena.BoundaryKindTrust, Name: "pci"}},
			Edges:      []*sirena.Edge{{From: "api", To: "db"}},
		}},
	}
	if err := sirena.EmitSIDs(doc, sirena.EmissionOptions{
		Clock:   fixedClock(1_700_000_000),
		Entropy: seededEntropy(42),
	}); err != nil {
		t.Fatal(err)
	}

	if v, ok := doc.Systems[0].Elements[0].Metadata["sid"].(sirena.String); !ok || v.Value == "" {
		t.Errorf("element sid missing: %#v", doc.Systems[0].Elements[0].Metadata["sid"])
	}
	if v, ok := doc.Systems[0].Boundaries[0].Metadata["sid"].(sirena.String); !ok || v.Value == "" {
		t.Errorf("boundary sid missing: %#v", doc.Systems[0].Boundaries[0].Metadata["sid"])
	}
	if v, ok := doc.Systems[0].Edges[0].Metadata["sid"].(sirena.String); !ok || v.Value == "" {
		t.Errorf("edge sid missing: %#v", doc.Systems[0].Edges[0].Metadata["sid"])
	}
}

// TestEmitSIDs_PreservesExisting confirms an existing `sid:` value is left
// untouched by a subsequent EmitSIDs call. The generator may be run more
// than once on the same in-memory Document; pre-existing identity wins.
func TestEmitSIDs_PreservesExisting(t *testing.T) {
	doc := &sirena.Document{
		Systems: []*sirena.SystemDecl{{
			Elements: []*sirena.Element{{
				Kind: sirena.ElementKindService, Name: "api",
				Metadata: map[string]sirena.Value{"sid": sirena.String{Value: "EXISTING"}},
			}},
		}},
	}
	if err := sirena.EmitSIDs(doc, sirena.EmissionOptions{
		Clock: fixedClock(1_700_000_000), Entropy: seededEntropy(42),
	}); err != nil {
		t.Fatal(err)
	}
	sid := doc.Systems[0].Elements[0].Metadata["sid"].(sirena.String).Value
	if sid != "EXISTING" {
		t.Errorf("sid mutated: %q", sid)
	}
}

// TestReconcile_BySID covers Task 26a: the prior `.gen.sir` had `sid:X +
// source_ref:foo`, the new emission carries the same `sid:X` but a
// different `source_ref:bar`. The resolver preserves the sid AND writes
// the prior source_ref into `prior_source_ref` so reviewers can see the
// rename history.
func TestReconcile_BySID(t *testing.T) {
	prior := []*sirena.Element{{
		Kind: sirena.ElementKindService, Name: "api",
		Metadata: map[string]sirena.Value{
			"sid":        sirena.String{Value: "01HSAME"},
			"source_ref": sirena.String{Value: "code://go/foo#A"},
		},
	}}
	newEls := []*sirena.Element{{
		Kind: sirena.ElementKindService, Name: "api",
		Metadata: map[string]sirena.Value{
			"sid":        sirena.String{Value: "01HSAME"},
			"source_ref": sirena.String{Value: "code://go/foo#B"},
		},
	}}
	resolutions := sirena.Reconcile(prior, newEls, sirena.EmissionOptions{})
	if len(resolutions) != 1 {
		t.Fatalf("resolutions: %d", len(resolutions))
	}
	r := resolutions[0]
	if r.MatchedVia != sirena.IdentityMatchBySID {
		t.Errorf("MatchedVia: %v", r.MatchedVia)
	}
	if r.SID != "01HSAME" {
		t.Errorf("SID: %q", r.SID)
	}
	if r.PriorSourceRef != "code://go/foo#A" {
		t.Errorf("PriorSourceRef: %q", r.PriorSourceRef)
	}
}

// TestReconcile_SIDPrecedenceOverSourceRef covers the precedence half of
// Task 26a: when a sid match exists AND a different prior element has a
// matching source_ref, the sid match wins.
func TestReconcile_SIDPrecedenceOverSourceRef(t *testing.T) {
	prior := []*sirena.Element{
		{Kind: sirena.ElementKindService, Name: "api",
			Metadata: map[string]sirena.Value{
				"sid":        sirena.String{Value: "01HMATCH"},
				"source_ref": sirena.String{Value: "code://go/a#A"},
			}},
		{Kind: sirena.ElementKindService, Name: "worker",
			Metadata: map[string]sirena.Value{
				"sid":        sirena.String{Value: "01HOTHER"},
				"source_ref": sirena.String{Value: "code://go/b#B"},
			}},
	}
	newEls := []*sirena.Element{{
		Kind: sirena.ElementKindService, Name: "api",
		Metadata: map[string]sirena.Value{
			"sid":        sirena.String{Value: "01HMATCH"},
			"source_ref": sirena.String{Value: "code://go/b#B"},
		},
	}}
	resolutions := sirena.Reconcile(prior, newEls, sirena.EmissionOptions{})
	if resolutions[0].MatchedVia != sirena.IdentityMatchBySID {
		t.Errorf("sid should win: %v", resolutions[0].MatchedVia)
	}
}

// TestReconcile_ByExactSourceRef covers Task 26b: no sid match, exact
// source_ref match. The resolver carries the prior sid forward and writes
// NO prior_source_ref because the source_ref didn't change.
func TestReconcile_ByExactSourceRef(t *testing.T) {
	prior := []*sirena.Element{{
		Kind: sirena.ElementKindService, Name: "api",
		Metadata: map[string]sirena.Value{
			"sid":        sirena.String{Value: "01HPRIOR"},
			"source_ref": sirena.String{Value: "code://go/foo#A"},
		},
	}}
	newEls := []*sirena.Element{{
		Kind: sirena.ElementKindService, Name: "api",
		Metadata: map[string]sirena.Value{
			// No sid; regenerator hasn't filled it yet.
			"source_ref": sirena.String{Value: "code://go/foo#A"},
		},
	}}
	resolutions := sirena.Reconcile(prior, newEls, sirena.EmissionOptions{})
	r := resolutions[0]
	if r.MatchedVia != sirena.IdentityMatchBySourceRef {
		t.Errorf("MatchedVia: %v", r.MatchedVia)
	}
	if r.SID != "01HPRIOR" {
		t.Errorf("SID carried forward: %q", r.SID)
	}
	if r.PriorSourceRef != "" {
		t.Errorf("PriorSourceRef should be empty: %q", r.PriorSourceRef)
	}
}

// TestReconcile_ByFuzzy covers Task 26c: neither sid nor source_ref match,
// but `symbol_kind` and the qualified-name suffix match — the function
// moved files but kept its name. The resolver fuzzy-matches and writes
// prior_source_ref so reviewers can audit the rename.
func TestReconcile_ByFuzzy(t *testing.T) {
	prior := []*sirena.Element{{
		Kind: sirena.ElementKindService, Name: "api",
		Metadata: map[string]sirena.Value{
			"sid":         sirena.String{Value: "01HPRIOR"},
			"source_ref":  sirena.String{Value: "code://go/foo#OldName"},
			"symbol_kind": sirena.String{Value: "function"},
		},
	}}
	newEls := []*sirena.Element{{
		Kind: sirena.ElementKindService, Name: "api",
		Metadata: map[string]sirena.Value{
			// Same suffix #OldName; different module prefix.
			"source_ref":  sirena.String{Value: "code://go/bar/qux#OldName"},
			"symbol_kind": sirena.String{Value: "function"},
		},
	}}
	resolutions := sirena.Reconcile(prior, newEls, sirena.EmissionOptions{})
	r := resolutions[0]
	if r.MatchedVia != sirena.IdentityMatchByFuzzy {
		t.Errorf("MatchedVia: %v", r.MatchedVia)
	}
	if r.SID != "01HPRIOR" {
		t.Errorf("SID: %q", r.SID)
	}
	if r.PriorSourceRef != "code://go/foo#OldName" {
		t.Errorf("PriorSourceRef: %q", r.PriorSourceRef)
	}
}

// TestReconcile_New covers Task 26d: no match at any level. The
// resolution keeps the new element's already-assigned sid and emits no
// prior_source_ref.
func TestReconcile_New(t *testing.T) {
	prior := []*sirena.Element{}
	newEls := []*sirena.Element{{
		Kind: sirena.ElementKindService, Name: "api",
		Metadata: map[string]sirena.Value{
			"sid":        sirena.String{Value: "01HNEW"},
			"source_ref": sirena.String{Value: "code://go/foo#A"},
		},
	}}
	resolutions := sirena.Reconcile(prior, newEls, sirena.EmissionOptions{})
	r := resolutions[0]
	if r.MatchedVia != sirena.IdentityMatchNew {
		t.Errorf("MatchedVia: %v", r.MatchedVia)
	}
	if r.SID != "01HNEW" {
		t.Errorf("SID: %q", r.SID)
	}
	if r.PriorSourceRef != "" {
		t.Errorf("PriorSourceRef should be empty: %q", r.PriorSourceRef)
	}
}

// TestReconcile_CleanRegenerationStripsPriorSourceRef covers Task 26e:
// emission N+1 wrote `prior_source_ref:foo` because of a rename. Emission
// N+2 has the same source_ref as N+1 (no further rename), so the
// resolution carries forward the sid and writes NO prior_source_ref —
// the rename history is recorded once and cleaned on the next clean
// regeneration.
func TestReconcile_CleanRegenerationStripsPriorSourceRef(t *testing.T) {
	prior := []*sirena.Element{{
		Kind: sirena.ElementKindService, Name: "api",
		Metadata: map[string]sirena.Value{
			"sid":              sirena.String{Value: "01HPRIOR"},
			"source_ref":       sirena.String{Value: "code://go/foo#B"},
			"prior_source_ref": sirena.String{Value: "code://go/foo#A"},
		},
	}}
	newEls := []*sirena.Element{{
		Kind: sirena.ElementKindService, Name: "api",
		Metadata: map[string]sirena.Value{
			"source_ref": sirena.String{Value: "code://go/foo#B"},
		},
	}}
	resolutions := sirena.Reconcile(prior, newEls, sirena.EmissionOptions{})
	r := resolutions[0]
	if r.MatchedVia != sirena.IdentityMatchBySourceRef {
		t.Errorf("MatchedVia: %v", r.MatchedVia)
	}
	if r.PriorSourceRef != "" {
		t.Errorf("prior_source_ref should be empty on clean regen, got %q", r.PriorSourceRef)
	}
}

// TestMergeOverride_All covers Task 27's bare `@override`: the
// hand-authored side wins wholesale and the generator-side fields are
// dropped.
func TestMergeOverride_All(t *testing.T) {
	gen := &sirena.Element{
		Kind: sirena.ElementKindService, Name: "api",
		Metadata: map[string]sirena.Value{
			"label": sirena.String{Value: "Auto"},
			"owner": sirena.String{Value: "auto"},
		},
	}
	hand := &sirena.Element{
		Kind:     sirena.ElementKindService,
		Name:     "api",
		Override: &sirena.Override{Mode: sirena.OverrideAll},
		Metadata: map[string]sirena.Value{
			"label": sirena.String{Value: "Hand"},
		},
	}
	merged := sirena.MergeOverride(gen, hand)
	if got := merged.Metadata["label"].(sirena.String).Value; got != "Hand" {
		t.Errorf("label: %q", got)
	}
	if _, has := merged.Metadata["owner"]; has {
		t.Errorf("OverrideAll drops generator fields; owner was kept")
	}
}

// TestMergeOverride_Fields covers Task 27's `@override(label)`: only the
// named field wins for the hand side; the rest stay generator-owned.
func TestMergeOverride_Fields(t *testing.T) {
	gen := &sirena.Element{
		Kind: sirena.ElementKindService, Name: "api",
		Metadata: map[string]sirena.Value{
			"label": sirena.String{Value: "Auto"},
			"owner": sirena.String{Value: "auto"},
		},
	}
	hand := &sirena.Element{
		Kind:     sirena.ElementKindService,
		Name:     "api",
		Override: &sirena.Override{Mode: sirena.OverrideFields, Fields: []string{"label"}},
		Metadata: map[string]sirena.Value{
			"label": sirena.String{Value: "Hand"},
		},
	}
	merged := sirena.MergeOverride(gen, hand)
	if got := merged.Metadata["label"].(sirena.String).Value; got != "Hand" {
		t.Errorf("label: %q", got)
	}
	if got := merged.Metadata["owner"].(sirena.String).Value; got != "auto" {
		t.Errorf("owner: %q", got)
	}
}

// TestMergeOverride_Except covers Task 27's `@override(except: source_ref)`:
// the hand side wins for every field EXCEPT the named ones; the named
// fields stay generator-owned.
func TestMergeOverride_Except(t *testing.T) {
	gen := &sirena.Element{
		Kind: sirena.ElementKindService, Name: "api",
		Metadata: map[string]sirena.Value{
			"label":      sirena.String{Value: "Auto"},
			"source_ref": sirena.String{Value: "code://go/foo#A"},
		},
	}
	hand := &sirena.Element{
		Kind:     sirena.ElementKindService,
		Name:     "api",
		Override: &sirena.Override{Mode: sirena.OverrideExcept, Fields: []string{"source_ref"}},
		Metadata: map[string]sirena.Value{
			"label":      sirena.String{Value: "Hand"},
			"source_ref": sirena.String{Value: "ignored"},
		},
	}
	merged := sirena.MergeOverride(gen, hand)
	if got := merged.Metadata["label"].(sirena.String).Value; got != "Hand" {
		t.Errorf("label: %q", got)
	}
	if got := merged.Metadata["source_ref"].(sirena.String).Value; got != "code://go/foo#A" {
		t.Errorf("source_ref should be generator's: %q", got)
	}
}

// TestValidateOverridesAgainstWorkspace_Dangling covers Task 27's dangling
// branch: a hand declaration carries an @override but no .gen.sir
// declares a matching name. The resolver emits SIR-OVERRIDE-DANGLING at
// the override's range.
func TestValidateOverridesAgainstWorkspace_Dangling(t *testing.T) {
	root := fixtureWorkspace(t, map[string]string{
		"sirena.toml": "default_preset: default\n",
		"hand.sir":    `service ghost @override(label)`,
		// No .gen.sir declares ghost.
	})
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	diags := sirena.ValidateOverridesAgainstWorkspace(ws)
	var found bool
	for _, d := range diags {
		if d.Code == "SIR-OVERRIDE-DANGLING" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected SIR-OVERRIDE-DANGLING; got %+v", diags)
	}
}

// TestValidateOverridesAgainstWorkspace_Resolved is the negative twin:
// a matching .gen.sir declares the same name, so no SIR-OVERRIDE-DANGLING
// fires.
func TestValidateOverridesAgainstWorkspace_Resolved(t *testing.T) {
	root := fixtureWorkspace(t, map[string]string{
		"sirena.toml":  "default_preset: default\n",
		"hand.sir":     `service api @override(label)`,
		"auto.gen.sir": `service api`,
	})
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	diags := sirena.ValidateOverridesAgainstWorkspace(ws)
	for _, d := range diags {
		if d.Code == "SIR-OVERRIDE-DANGLING" {
			t.Errorf("unexpected SIR-OVERRIDE-DANGLING: %+v", d)
		}
	}
}

// TestReconcileWithOrphans_RetainedWithinGrace covers Task 28a: a prior
// `.gen.sir` declared `service stale { sid:...; source_ref:... }`; the new
// emission omits it. The resolver retains the orphan and emits a
// SIR-ORPHAN warning so reviewers see the dropped declaration.
func TestReconcileWithOrphans_RetainedWithinGrace(t *testing.T) {
	// Build prior element with a known ULID timestamp (day 0).
	day0 := time.Unix(1_700_000_000, 0)
	sid := ulid.MustNew(ulid.Timestamp(day0), seededEntropy(1)).String()

	prior := []*sirena.Element{{
		Kind: sirena.ElementKindService, Name: "stale",
		Metadata: map[string]sirena.Value{
			"sid":        sirena.String{Value: sid},
			"source_ref": sirena.String{Value: "code://go/foo#Stale"},
		},
	}}
	newEls := []*sirena.Element{} // nothing matches

	// Now is day 5 — within 14-day grace.
	day5 := day0.Add(5 * 24 * time.Hour)
	result := sirena.ReconcileWithOrphans(prior, newEls, sirena.ReconcileOptions{
		EmissionOptions: sirena.EmissionOptions{Clock: func() time.Time { return day5 }},
	})

	if len(result.Orphans) != 1 {
		t.Fatalf("Orphans: %d", len(result.Orphans))
	}
	if result.Orphans[0].Status != sirena.OrphanRetained {
		t.Errorf("Status: %v", result.Orphans[0].Status)
	}
	var hasOrphan bool
	for _, d := range result.Diagnostics {
		if d.Code == "SIR-ORPHAN" {
			hasOrphan = true
		}
	}
	if !hasOrphan {
		t.Errorf("expected SIR-ORPHAN; got %+v", result.Diagnostics)
	}
}

// TestReconcileWithOrphans_DeletedPastGrace covers Task 28b: a prior
// orphan whose ULID timestamp is older than the 14-day default grace is
// dropped silently — past the grace window the generator's intentional
// removal wins and the diagnostic stops firing.
func TestReconcileWithOrphans_DeletedPastGrace(t *testing.T) {
	day0 := time.Unix(1_700_000_000, 0)
	sid := ulid.MustNew(ulid.Timestamp(day0), seededEntropy(2)).String()

	prior := []*sirena.Element{{
		Kind: sirena.ElementKindService, Name: "stale",
		Metadata: map[string]sirena.Value{"sid": sirena.String{Value: sid}},
	}}
	newEls := []*sirena.Element{}

	// Now is day 15 — past 14-day grace.
	day15 := day0.Add(15 * 24 * time.Hour)
	result := sirena.ReconcileWithOrphans(prior, newEls, sirena.ReconcileOptions{
		EmissionOptions: sirena.EmissionOptions{Clock: func() time.Time { return day15 }},
	})

	if len(result.Orphans) != 1 {
		t.Fatalf("Orphans: %d", len(result.Orphans))
	}
	if result.Orphans[0].Status != sirena.OrphanDeleted {
		t.Errorf("Status: %v", result.Orphans[0].Status)
	}
	for _, d := range result.Diagnostics {
		if d.Code == "SIR-ORPHAN" {
			t.Errorf("deleted orphans should not emit SIR-ORPHAN; got %+v", d)
		}
	}
}

// TestReconcileWithOrphans_ConfigurableGrace covers Task 28c: callers can
// shorten the grace period via OverrideOptions. Under a 24-hour grace, a
// day-2 orphan is past grace and gets dropped.
func TestReconcileWithOrphans_ConfigurableGrace(t *testing.T) {
	day0 := time.Unix(1_700_000_000, 0)
	sid := ulid.MustNew(ulid.Timestamp(day0), seededEntropy(3)).String()

	prior := []*sirena.Element{{
		Kind: sirena.ElementKindService, Name: "stale",
		Metadata: map[string]sirena.Value{"sid": sirena.String{Value: sid}},
	}}
	newEls := []*sirena.Element{}

	// 24-hour grace; check day 2.
	day2 := day0.Add(2 * 24 * time.Hour)
	result := sirena.ReconcileWithOrphans(prior, newEls, sirena.ReconcileOptions{
		EmissionOptions:   sirena.EmissionOptions{Clock: func() time.Time { return day2 }},
		OrphanGracePeriod: 24 * time.Hour,
	})

	if len(result.Orphans) != 1 {
		t.Fatalf("Orphans: %d", len(result.Orphans))
	}
	if result.Orphans[0].Status != sirena.OrphanDeleted {
		t.Errorf("expected deleted under 24h grace; got %v", result.Orphans[0].Status)
	}
}

// TestReconcileWithOrphans_NoOrphansWhenAllMatched is the negative twin:
// every prior element matches a new one, so the Orphans slice stays empty
// and no SIR-ORPHAN diagnostic surfaces.
func TestReconcileWithOrphans_NoOrphansWhenAllMatched(t *testing.T) {
	sid := ulid.MustNew(ulid.Timestamp(time.Unix(1_700_000_000, 0)), seededEntropy(4)).String()
	prior := []*sirena.Element{{
		Kind: sirena.ElementKindService, Name: "api",
		Metadata: map[string]sirena.Value{
			"sid":        sirena.String{Value: sid},
			"source_ref": sirena.String{Value: "code://go/foo#A"},
		},
	}}
	newEls := []*sirena.Element{{
		Kind: sirena.ElementKindService, Name: "api",
		Metadata: map[string]sirena.Value{
			"sid":        sirena.String{Value: sid},
			"source_ref": sirena.String{Value: "code://go/foo#A"},
		},
	}}
	result := sirena.ReconcileWithOrphans(prior, newEls, sirena.ReconcileOptions{})
	if len(result.Orphans) != 0 {
		t.Errorf("Orphans should be empty: %+v", result.Orphans)
	}
}

// TestReconcileWithOrphans_MissingSidGetsFullGrace pins the missing-sid
// fallback: when a prior orphan has no `sid:` metadata (or a malformed
// one), the resolver treats FirstSeen as the current clock and grants the
// full grace window. The orphan surfaces as Retained.
func TestReconcileWithOrphans_MissingSidGetsFullGrace(t *testing.T) {
	prior := []*sirena.Element{{
		Kind: sirena.ElementKindService, Name: "stale",
		Metadata: map[string]sirena.Value{"source_ref": sirena.String{Value: "x"}},
	}}
	newEls := []*sirena.Element{}
	result := sirena.ReconcileWithOrphans(prior, newEls, sirena.ReconcileOptions{})
	if len(result.Orphans) != 1 {
		t.Fatalf("Orphans: %d", len(result.Orphans))
	}
	if result.Orphans[0].Status != sirena.OrphanRetained {
		t.Errorf("missing sid should yield retained: %v", result.Orphans[0].Status)
	}
}
