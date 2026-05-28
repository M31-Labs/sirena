// Package sirena: render entrypoint.
//
// render.go is the single, stable public entrypoint that wraps view
// evaluation, budget enforcement, and (eventually) layout. For
// v0.0.1-internal the layout side is a Plan-2 placeholder: Render
// evaluates the budget and returns a nil *LayoutResult. The signature
// is stable on purpose so downstream consumers (mdpp, the Sirena LSP,
// the CLI) can wire against it now and graft layout in later without
// touching call sites.
//
// The contract:
//
//	Render(rv, opts) (*LayoutResult, *BudgetReport, error)
//
//	- StrictBudget = true: any breach surfaces as ErrBudgetExceeded
//	  with the populated *BudgetReport; the *LayoutResult is nil.
//	- StrictBudget = false: breaches still produce a non-nil report so
//	  callers can warn, but the call succeeds.
//	- No budget block or no breaches: report is nil; layout is nil
//	  (placeholder for Plan 2); err is nil.
//
// View pointers, including the embedded ViewDecl, are not mutated.
package sirena

import "errors"

// RenderOptions configures the render pipeline.
type RenderOptions struct {
	// StrictBudget makes Render return ErrBudgetExceeded when any cap is
	// over budget. Permissive mode (the zero value) still surfaces the
	// report so callers can warn, but does not abort.
	StrictBudget bool
}

// LayoutResult is the positioned IR. Plans 2-6 will fill in geometric
// fields (positions, sizes, edge routes); for v0.0.1-internal it is a
// placeholder that carries only the resolved view so the public API
// surface is already stable.
type LayoutResult struct {
	View *ResolvedView
	// Geometry fields (positions, sizes, edge routes) live in Plan 2.
}

// ErrBudgetExceeded is the sentinel returned by Render when StrictBudget
// is true and the view's projection overflows its declared budget. The
// accompanying *BudgetReport carries the field-level details.
var ErrBudgetExceeded = errors.New("sirena: view budget exceeded; see BudgetReport")

// Render is the canonical entrypoint for turning a ResolvedView into a
// rendered layout. For v0.0.1-internal the layout output is a Plan-2
// placeholder (always nil); the budget side of the pipeline is
// complete.
//
// Behavior:
//   - Calls EvaluateBudget(rv). If the report is nil (no budget or no
//     breaches), Render returns (nil, nil, nil).
//   - If StrictBudget is true and there are breaches, returns (nil,
//     report, ErrBudgetExceeded).
//   - Otherwise (permissive mode), returns (nil, report, nil) so the
//     caller can warn while layout proceeds.
func Render(rv *ResolvedView, opts RenderOptions) (*LayoutResult, *BudgetReport, error) {
	report := EvaluateBudget(rv)
	if report != nil && opts.StrictBudget {
		return nil, report, ErrBudgetExceeded
	}
	// Layout is Plan 2; for v0.0.1-internal we deliberately return a nil
	// *LayoutResult so callers that branch on `layout != nil` get a
	// stable signal that layout hasn't been wired yet.
	return nil, report, nil
}
