// Package fence is the canonical entrypoint for rendering a single sirena
// fence — the body of a ```sirena code block in an mdpp document — to an
// SVG bundle. It lives in its own package (not the root) because rendering
// requires render/svg, which imports the root sirena package; a root-level
// entrypoint would cycle. mdpp wires fence.Render through a dependency-
// injection seam so the mdpp library never imports sirena directly.
//
// The non-interactive SVG path is the v0.1 deliverable. The interactive
// (GoSX island) path depends on render/island, which is not yet shipped;
// Render returns ErrInteractiveNotAvailable when Options.Interactive is set.
package fence

import (
	"errors"
	"fmt"

	"m31labs.dev/sirena"
	_ "m31labs.dev/sirena/layout" // registers the layout engine into sirena.Render
	"m31labs.dev/sirena/render/svg"
)

// ErrInteractiveNotAvailable is returned by Render when Options.Interactive
// is true but the render/island sub-package (Plan 3) is not yet shipped.
var ErrInteractiveNotAvailable = errors.New("sirena/fence: interactive rendering requires the render/island sub-package (not yet shipped)")

// Fence-scoped diagnostic codes. SIR-IMPORT-IN-FENCE (from the workspace
// layer) flows through ResolveDiagnostics; these are emitted by Render.
const (
	CodeFenceEmpty         = "SIR-FENCE-EMPTY"
	CodeFenceNoView        = "SIR-FENCE-NO-VIEW"
	CodeFenceBodyOverrides = "SIR-FENCE-BODY-OVERRIDES-VIEW"
	CodeViewEval           = "SIR-FENCE-VIEW-EVAL"
	CodeBudgetBreach       = "SIR-BUDGET-BREACH"
)

// Options configures a single fence render. WorkspaceRoot and ViewRef mirror
// sirena.FenceOptions; Theme, Interactive, and StrictBudget add render-time
// concerns the workspace layer does not own.
type Options struct {
	// WorkspaceRoot, if non-empty, names a sirena workspace the fence's
	// resolver chains into (workspace-resolving mode). Empty = self-contained.
	WorkspaceRoot string
	// ViewRef, if non-empty, names a view file in the host workspace to
	// render; the fence body is then expected to be empty. A non-empty body
	// wins and ViewRef is ignored (with a warning).
	ViewRef string
	// Theme is the SVG theme name; empty selects the default theme.
	Theme string
	// Interactive requests a GoSX island payload alongside the SVG. Not yet
	// available — Render returns ErrInteractiveNotAvailable.
	Interactive bool
	// StrictBudget makes a budget breach suppress SVG output (the breach is
	// still reported via BudgetReport + a diagnostic).
	StrictBudget bool
}

// Result is the bundle produced by Render. SVG is the canonical artifact and
// is empty when a diagnostic prevented rendering (empty fence, no view, or a
// strict-budget breach). IslandPayload is reserved for the interactive path.
type Result struct {
	SVG           []byte
	IslandPayload []byte
	Diagnostics   []sirena.Diagnostic
	BudgetReport  *sirena.BudgetReport
}

// Render turns a fence body (and/or a view reference) into an SVG bundle.
// It never panics: parse, resolve, view-eval, and budget findings surface as
// Result.Diagnostics. A non-nil error is reserved for hard failures
// (workspace construction, SVG serialization). Interactive requests return
// ErrInteractiveNotAvailable.
func Render(src []byte, opts Options) (Result, error) {
	var result Result

	if opts.Interactive {
		return result, ErrInteractiveNotAvailable
	}

	// A non-empty body overrides a view reference (spec §6.3).
	if len(src) > 0 && opts.ViewRef != "" {
		result.Diagnostics = append(result.Diagnostics, sirena.Diagnostic{
			Code:     CodeFenceBodyOverrides,
			Severity: sirena.SeverityWarning,
			Message:  "fence has both a body and view=; the body wins and view= is ignored",
		})
		opts.ViewRef = ""
	}

	if len(src) == 0 && opts.ViewRef == "" {
		result.Diagnostics = append(result.Diagnostics, sirena.Diagnostic{
			Code:     CodeFenceEmpty,
			Severity: sirena.SeverityWarning,
			Message:  "empty fence body and no view= reference; nothing to render",
		})
		return result, nil
	}

	ws, err := sirena.NewFenceWorkspace(src, sirena.FenceOptions{
		WorkspaceRoot: opts.WorkspaceRoot,
		ViewRef:       opts.ViewRef,
	})
	if err != nil {
		return result, err
	}
	result.Diagnostics = append(result.Diagnostics, ws.ResolveDiagnostics()...)

	doc := ws.Files[0].Document
	if doc == nil {
		loaded, lerr := ws.LoadDocument(ws.Files[0])
		if lerr != nil {
			return result, lerr
		}
		doc = loaded
	}
	result.Diagnostics = append(result.Diagnostics, doc.Diagnostics()...)

	rv, ok := resolveView(ws, doc, &result)
	if !ok {
		return result, nil
	}

	lr, report, rerr := sirena.Render(rv, sirena.RenderOptions{StrictBudget: opts.StrictBudget})
	result.BudgetReport = report
	if report != nil && len(report.Breaches) > 0 {
		result.Diagnostics = append(result.Diagnostics, sirena.Diagnostic{
			Code:     CodeBudgetBreach,
			Severity: sirena.SeverityWarning,
			Message:  budgetMessage(report),
		})
	}
	if errors.Is(rerr, sirena.ErrBudgetExceeded) {
		// Strict mode: breach already reported; suppress SVG, no hard error.
		return result, nil
	}
	if rerr != nil {
		return result, rerr
	}
	if lr == nil {
		return result, nil
	}

	theme, terr := svg.ThemeForName(themeOrDefault(opts.Theme))
	if terr != nil {
		return result, terr
	}
	out, serr := svg.Render(lr, theme)
	if serr != nil {
		return result, serr
	}
	result.SVG = out
	return result, nil
}

// resolveView selects the view to render: the first explicit view
// declaration, or — when the body declares renderable elements but no view —
// a synthesized all-elements view. It reports SIR-FENCE-NO-VIEW (no view, no
// elements) or SIR-FENCE-VIEW-EVAL (evaluation error) into result and returns
// ok=false when nothing should be rendered.
func resolveView(ws *sirena.Workspace, doc *sirena.Document, result *Result) (*sirena.ResolvedView, bool) {
	if len(doc.Views) > 0 {
		rv, err := sirena.EvaluateView(ws, doc.Views[0])
		if err != nil {
			result.Diagnostics = append(result.Diagnostics, sirena.Diagnostic{
				Code:     CodeViewEval,
				Severity: sirena.SeverityError,
				Message:  err.Error(),
			})
			return nil, false
		}
		return rv, true
	}
	if docHasContent(doc) {
		return sirena.AllElementsView(doc), true
	}
	result.Diagnostics = append(result.Diagnostics, sirena.Diagnostic{
		Code:     CodeFenceNoView,
		Severity: sirena.SeverityWarning,
		Message:  "fence body has no view declaration and no renderable elements",
	})
	return nil, false
}

// docHasContent reports whether any system in doc declares an element,
// boundary, or edge worth rendering.
func docHasContent(doc *sirena.Document) bool {
	for _, sys := range doc.Systems {
		if len(sys.Elements) > 0 || len(sys.Boundaries) > 0 || len(sys.Edges) > 0 {
			return true
		}
	}
	return false
}

func themeOrDefault(name string) string {
	if name == "" {
		return svg.DefaultThemeName
	}
	return name
}

func budgetMessage(r *sirena.BudgetReport) string {
	if len(r.Breaches) == 1 {
		b := r.Breaches[0]
		return fmt.Sprintf("view exceeds budget: %s is %d, over the limit of %d", b.Field, b.Actual, b.Limit)
	}
	return fmt.Sprintf("view exceeds budget: %d caps breached", len(r.Breaches))
}
