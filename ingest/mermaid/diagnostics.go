package mermaid

import (
	"errors"
	"fmt"
	"strings"

	gt "github.com/odvcencio/gotreesitter"
	"m31labs.dev/sirena"
)

// mkDiag builds a sirena.Diagnostic whose Range is mapped through l.smap so
// it points at the original (pre-normalize) source bytes, not the cleaned
// bytes that the CST was built from.
func (l *lowerer) mkDiag(code string, sev sirena.Severity, msg string, node *gt.Node) sirena.Diagnostic {
	start := l.smap.orig(int(node.StartByte()))
	end := l.smap.orig(int(node.EndByte()))
	return sirena.Diagnostic{
		Code:     code,
		Severity: sev,
		Message:  msg,
		Range:    sirena.Range{Start: start, End: end},
	}
}

// checkNotAGraph scans all named children of root for a diagram_flow node.
// Returns the diagram_flow node if found, nil otherwise. When nil is returned
// the caller should set l.fatal and append a SIR-MERMAID-NOT-A-GRAPH
// diagnostic, then return a nil document.
func checkForDiagramFlow(root *gt.Node, lang *gt.Language) *gt.Node {
	for i := 0; i < root.NamedChildCount(); i++ {
		c := root.NamedChild(i)
		if c.Type(lang) == "diagram_flow" {
			return c
		}
	}
	return nil
}

// checkParseErrors scans root's named children for ERROR nodes. When the root
// itself has errors (tree-sitter error recovery pushed broken tokens to root
// level) but a diagram_flow IS present, the valid stmts inside diagram_flow
// are still lowered — but we emit one SIR-MERMAID-PARSE warning covering the
// root span to signal that some source was silently dropped.
//
// The returned boolean is true when at least one root-level ERROR was found.
func (l *lowerer) checkParseErrors(root *gt.Node) bool {
	if !root.HasError() {
		return false
	}
	// Emit a single PARSE diagnostic covering the whole root span.
	d := l.mkDiag(
		"SIR-MERMAID-PARSE",
		sirena.SeverityWarning,
		"Mermaid flowchart contains parse errors; one or more statements were skipped",
		root,
	)
	l.diags = append(l.diags, d)
	return true
}

// checkUnsupportedStmt emits SIR-MERMAID-UNSUPPORTED for a named child of
// diagram_flow whose type starts with "flow_stmt_" but is not one of the
// statement kinds the lowerer handles (flow_stmt_vertice, flow_stmt_subgraph).
// Returns true if the node was an unhandled flow_stmt_* type.
func (l *lowerer) checkUnsupportedStmt(node *gt.Node) bool {
	typ := node.Type(l.lang)
	if !strings.HasPrefix(typ, "flow_stmt_") {
		return false
	}
	switch typ {
	case "flow_stmt_vertice", "flow_stmt_subgraph":
		return false // handled by the lowerer
	}
	d := l.mkDiag(
		"SIR-MERMAID-UNSUPPORTED",
		sirena.SeverityWarning,
		fmt.Sprintf("Mermaid statement type %q is not supported and was skipped", typ),
		node,
	)
	l.diags = append(l.diags, d)
	return true
}

// notAGraphError is returned by Parse when the input is not a flowchart.
var notAGraphError = errors.New("SIR-MERMAID-NOT-A-GRAPH: input is not a Mermaid flowchart")
