// Package lint runs sirena's structural and semantic checks against a
// workspace. Rules in this package are thin wrappers around the diagnostic
// surfaces in the root sirena package (Document.Diagnostics,
// ResolveWorkspace, ValidateOverridesAgainstWorkspace, etc.) plus a few
// rules that are lint-only — kind-specific edge requirements and
// dead-element detection.
//
// Run is the canonical entrypoint; the CLI's `sirena lint` and the LSP
// linter both call into it. The rule registry is unexported so the lint
// pack evolves as a single unit: callers don't pick-and-mix rules, and
// the deterministic union surfaces every relevant finding in one place.
package lint

import (
	"fmt"

	"m31labs.dev/sirena"
)

// RuleFunc is the signature every lint rule satisfies. A rule takes the
// workspace, returns its diagnostics, and never mutates ws — the runner
// is allowed to call rules in any order without coordination, so any
// mutation would be a race waiting to happen.
type RuleFunc func(ws *sirena.Workspace) []sirena.Diagnostic

// rules is the v0.1 lint pack. Order here is the order Run walks them;
// the dedup pass in Run collapses overlapping findings so the relative
// order of two rules touching the same range is immaterial to the
// output. Adding a rule to the pack means appending it here; the runner
// picks it up automatically.
var rules = []RuleFunc{
	QueueWithoutPublisher,
	QueueWithoutSubscriber,
	UnresolvedRefs,
	DeadElement,
	DanglingOverride,
	Versions,
}

// Run executes every rule in the lint package against ws and returns the
// deduplicated union of diagnostics. Deduplication uses
// (Code, Range.Start, Range.End) as the key so a rule fired by two paths
// — e.g. the resolver-backed UnresolvedRefs and a future rule walking the
// same edge — surfaces once.
//
// A nil workspace returns nil rather than panicking; the CLI's happy path
// can forget to construct a workspace (e.g. when a load error precedes
// linting) and still call Run safely.
func Run(ws *sirena.Workspace) []sirena.Diagnostic {
	if ws == nil {
		return nil
	}
	seen := make(map[string]bool)
	var out []sirena.Diagnostic
	for _, rule := range rules {
		for _, d := range rule(ws) {
			key := fmt.Sprintf("%s|%d|%d", d.Code, d.Range.Start, d.Range.End)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, d)
		}
	}
	return out
}
