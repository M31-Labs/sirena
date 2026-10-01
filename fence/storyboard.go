package fence

import (
	"fmt"
	"m31labs.dev/sirena"
	"m31labs.dev/sirena/layout"
	"m31labs.dev/sirena/render/svg"
)

// Storyboard renders self-contained states with stable slots and chart domains.
// Any invalid state aborts the sequence. Themes and strict budgets match Render.
func Storyboard(sources [][]byte, opts Options) ([][]byte, error) {
	if len(sources) < 2 || len(sources) > 32 {
		return nil, fmt.Errorf("sirena: storyboard needs 2–32 states")
	}
	if opts.Interactive || opts.ViewRef != "" || opts.WorkspaceRoot != "" {
		return nil, fmt.Errorf("sirena: storyboard requires self-contained, non-interactive states")
	}
	var views []*sirena.ResolvedView
	for i, src := range sources {
		ws, err := sirena.NewFenceWorkspace(src, sirena.FenceOptions{})
		if err != nil {
			return nil, err
		}
		doc := ws.Files[0].Document
		if doc == nil {
			return nil, fmt.Errorf("sirena: state %d has no document", i)
		}
		result := Result{Diagnostics: append(ws.ResolveDiagnostics(), doc.Diagnostics()...)}
		rv, ok := resolveView(ws, doc, &result)
		for _, d := range result.Diagnostics {
			if d.Severity == sirena.SeverityError {
				return nil, fmt.Errorf("state %d: %s", i, d.Message)
			}
		}
		if !ok {
			return nil, fmt.Errorf("sirena: state %d has no view", i)
		}
		views = append(views, rv)
	}
	frames, err := layout.Storyboard(views, sirena.RenderOptions{Diagram: opts.Diagram, StrictBudget: opts.StrictBudget})
	if err != nil {
		return nil, err
	}
	theme, err := svg.ThemeForName(themeOrDefault(opts.Theme))
	if err != nil {
		return nil, err
	}
	var output [][]byte
	for _, lr := range frames {
		data, err := svg.Render(lr, theme)
		if err != nil {
			return nil, err
		}
		output = append(output, data)
	}
	return output, nil
}
