package sirena_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"m31labs.dev/sirena"
)

// TestEvaluateBudget_NoBudgetReturnsNil pins the cheap fast-path: a view
// without a budget block must yield a nil report so callers can treat
// "no breaches" and "no budget declared" identically.
func TestEvaluateBudget_NoBudgetReturnsNil(t *testing.T) {
	rv := buildResolvedView(t, map[string]string{
		"sys.sir": `service api
service db`,
		"v.view.sir": `view "v" { include: [service "api"] }`,
	}, "v")
	if report := sirena.EvaluateBudget(rv); report != nil {
		t.Errorf("no budget should yield nil report, got %+v", report)
	}
}

// TestEvaluateBudget_NoBreachReturnsNil keeps the asymmetric contract:
// even with a budget block, if every cap holds the report stays nil.
// The renderer treats nil as "go ahead, no warnings".
func TestEvaluateBudget_NoBreachReturnsNil(t *testing.T) {
	rv := buildResolvedView(t, map[string]string{
		"sys.sir": `service api`,
		"v.view.sir": `view "v" {
  include: [service "api"]
  budget { nodes: 10 }
}`,
	}, "v")
	if report := sirena.EvaluateBudget(rv); report != nil {
		t.Errorf("budget under cap should yield nil report, got %+v", report)
	}
}

// TestEvaluateBudget_NodesBreach drives the headline nodes-cap check:
// 15 services with a cap of 10 must yield a single nodes breach with the
// declared limit and observed count populated.
func TestEvaluateBudget_NodesBreach(t *testing.T) {
	var services []string
	for i := 0; i < 15; i++ {
		services = append(services, fmt.Sprintf("service svc%d", i))
	}
	sys := strings.Join(services, "\n")

	var includes []string
	for i := 0; i < 15; i++ {
		includes = append(includes, fmt.Sprintf("    service \"svc%d\"", i))
	}
	view := fmt.Sprintf("view \"v\" {\n  include: [\n%s\n  ]\n  budget { nodes: 10 }\n}", strings.Join(includes, "\n"))

	rv := buildResolvedView(t, map[string]string{
		"sys.sir":    sys,
		"v.view.sir": view,
	}, "v")
	report := sirena.EvaluateBudget(rv)
	if report == nil {
		t.Fatal("expected breach")
	}
	if len(report.Breaches) < 1 {
		t.Errorf("Breaches: %d", len(report.Breaches))
	}
	var hit bool
	for _, b := range report.Breaches {
		if b.Field == "nodes" {
			if b.Limit != 10 {
				t.Errorf("Limit: %d", b.Limit)
			}
			if b.Actual != 15 {
				t.Errorf("Actual: %d", b.Actual)
			}
			hit = true
		}
	}
	if !hit {
		t.Error("no nodes breach reported")
	}
}

// TestEvaluateBudget_NodesBreachSuggestsCollapse confirms a nodes breach
// over a view that includes a boundary yields a CollapseBoundary
// suggestion targeting that boundary — the cheapest curation move.
func TestEvaluateBudget_NodesBreachSuggestsCollapse(t *testing.T) {
	var children []string
	for i := 0; i < 15; i++ {
		children = append(children, fmt.Sprintf("  service svc%d", i))
	}
	sys := fmt.Sprintf("boundary trust \"big\" {\n%s\n}", strings.Join(children, "\n"))
	view := `view "v" {
  include: [boundary "big"]
  budget { nodes: 5 }
}`
	rv := buildResolvedView(t, map[string]string{
		"sys.sir":    sys,
		"v.view.sir": view,
	}, "v")
	report := sirena.EvaluateBudget(rv)
	if report == nil {
		t.Fatal("expected breach")
	}
	var has bool
	for _, s := range report.Suggestions {
		if s.Kind == sirena.SuggestionCollapseBoundary {
			has = true
		}
	}
	if !has {
		t.Errorf("expected CollapseBoundary suggestion; got %+v", report.Suggestions)
	}
}

// TestEvaluateBudget_LabelCharsBreach drives the label-length cap. The
// service's label metadata is longer than the cap so a label_chars breach
// must fire even though no other field is over.
func TestEvaluateBudget_LabelCharsBreach(t *testing.T) {
	rv := buildResolvedView(t, map[string]string{
		"sys.sir": `service api {
  label: "this is a very very very very very very very long label"
}`,
		"v.view.sir": `view "v" {
  include: [service "api"]
  budget { label_chars: 10 }
}`,
	}, "v")
	report := sirena.EvaluateBudget(rv)
	if report == nil {
		t.Fatal("expected breach")
	}
	var hit bool
	for _, b := range report.Breaches {
		if b.Field == "label_chars" {
			hit = true
		}
	}
	if !hit {
		t.Error("no label_chars breach")
	}
}

// TestRender_PermissiveReturnsReport drives the permissive Render path:
// breaches surface as a non-nil report but the call still succeeds. The
// LayoutResult is a Plan 2 placeholder (nil) for now.
func TestRender_PermissiveReturnsReport(t *testing.T) {
	rv := buildResolvedView(t, map[string]string{
		"sys.sir": `service api
service db`,
		"v.view.sir": `view "v" {
  include: [service "api", service "db"]
  budget { nodes: 1 }
}`,
	}, "v")
	layout, report, err := sirena.Render(rv, sirena.RenderOptions{StrictBudget: false})
	if err != nil {
		t.Errorf("permissive Render shouldn't error: %v", err)
	}
	if layout != nil {
		t.Errorf("layout placeholder; expected nil")
	}
	if report == nil {
		t.Error("expected non-nil report on breach")
	}
}

// TestRender_StrictReturnsError drives strict mode: breaches turn into a
// sentinel error and the report rides alongside so callers can surface
// the rationale to the author.
func TestRender_StrictReturnsError(t *testing.T) {
	rv := buildResolvedView(t, map[string]string{
		"sys.sir": `service api
service db`,
		"v.view.sir": `view "v" {
  include: [service "api", service "db"]
  budget { nodes: 1 }
}`,
	}, "v")
	_, report, err := sirena.Render(rv, sirena.RenderOptions{StrictBudget: true})
	if err == nil {
		t.Error("strict Render with breach should error")
	}
	if !errors.Is(err, sirena.ErrBudgetExceeded) {
		t.Errorf("err: %v", err)
	}
	if report == nil {
		t.Error("report should accompany the strict error")
	}
}

// TestRender_NoBudgetSuccess pins the no-budget path: even strict mode
// returns no error and no report when the view declares no budget.
func TestRender_NoBudgetSuccess(t *testing.T) {
	rv := buildResolvedView(t, map[string]string{
		"sys.sir":    `service api`,
		"v.view.sir": `view "v" { include: [service "api"] }`,
	}, "v")
	layout, report, err := sirena.Render(rv, sirena.RenderOptions{StrictBudget: true})
	if err != nil {
		t.Error("no budget no breach no error")
	}
	if report != nil {
		t.Errorf("no budget no report: %+v", report)
	}
	_ = layout
}
