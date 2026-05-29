// Package sirena: render entrypoint.
//
// render.go is the single, stable public entrypoint that wraps view
// evaluation, budget enforcement, and layout. Render evaluates the
// budget and, when it passes, delegates geometry to the layout engine.
// The signature is stable on purpose so downstream consumers (mdpp, the
// Sirena LSP, the CLI) wire against it without touching call sites as
// the engine grows.
//
// The contract:
//
//	Render(rv, opts) (*LayoutResult, *BudgetReport, error)
//
//	- StrictBudget = true: any breach surfaces as ErrBudgetExceeded
//	  with the populated *BudgetReport; the *LayoutResult is nil.
//	- StrictBudget = false: breaches still produce a non-nil report so
//	  callers can warn, but the call succeeds and layout proceeds.
//	- No budget block or no breaches: report is nil; layout is computed
//	  when the layout engine is linked in (see RegisterLayoutComputer),
//	  otherwise nil; err is nil.
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

// LayoutResult is the positioned IR: the resolved view plus the absolute
// geometry the layout engine assigned to every node, summary, boundary,
// and edge. The geometric value types live in geometry.go; the layout
// algorithms that fill these slices live in m31labs.dev/sirena/layout.
type LayoutResult struct {
	// View is the resolved view this layout was computed from.
	View *ResolvedView
	// Bounds is the overall canvas bounding box enclosing every
	// placement and route.
	Bounds Rect
	// Seed is the deterministic layout seed; it equals ViewHash(View) on
	// the production path. The force preset feeds it to its RNG.
	Seed [32]byte
	// NodePlacements positions every element in the view.
	NodePlacements []*NodePlacement
	// SummaryPlacements positions every collapsed-boundary summary.
	SummaryPlacements []*SummaryPlacement
	// BoundaryPlacements positions every visible boundary region.
	BoundaryPlacements []*BoundaryPlacement
	// EdgeRoutes carries the orthogonal route for every visible edge.
	EdgeRoutes []*EdgeRoute
}

// layoutComputer is the layout engine, injected by the layout package's
// init() via RegisterLayoutComputer. It is nil until the layout package
// is linked into the build. Render delegates geometry to it; the
// indirection exists solely to break the root↔layout import cycle (the
// layout package imports this one for the IR and geometry types, so this
// package must never import layout).
var layoutComputer func(rv *ResolvedView) (*LayoutResult, error)

// RegisterLayoutComputer wires the layout engine into Render. The layout
// package calls this from its init(); application code never calls it
// directly. Passing nil clears the registration (used by tests).
func RegisterLayoutComputer(fn func(rv *ResolvedView) (*LayoutResult, error)) {
	layoutComputer = fn
}

// ErrBudgetExceeded is the sentinel returned by Render when StrictBudget
// is true and the view's projection overflows its declared budget. The
// accompanying *BudgetReport carries the field-level details.
var ErrBudgetExceeded = errors.New("sirena: view budget exceeded; see BudgetReport")

// Render is the canonical entrypoint for turning a ResolvedView into a
// rendered layout.
//
// Behavior:
//   - Calls EvaluateBudget(rv). If StrictBudget is true and there are
//     breaches, returns (nil, report, ErrBudgetExceeded).
//   - Otherwise, when the layout engine is linked in, computes geometry
//     and returns (layout, report, nil). The report is non-nil only when
//     there were breaches (permissive mode lets the caller warn).
//   - When no layout engine is linked, returns (nil, report, nil) so
//     budget-only callers still work.
func Render(rv *ResolvedView, opts RenderOptions) (*LayoutResult, *BudgetReport, error) {
	report := EvaluateBudget(rv)
	if report != nil && opts.StrictBudget {
		return nil, report, ErrBudgetExceeded
	}
	if layoutComputer != nil {
		lr, err := layoutComputer(rv)
		if err != nil {
			return nil, report, err
		}
		return lr, report, nil
	}
	// No layout engine linked: budget-only result, stable nil layout.
	return nil, report, nil
}
