package mermaid

import (
	gt "github.com/odvcencio/gotreesitter"
	"m31labs.dev/sirena"
)

// lowerer carries all state needed to walk the Mermaid CST and produce a
// *sirena.Document. It is constructed by Parse and used for a single Parse
// call; it is not safe for concurrent use.
type lowerer struct {
	lang  *gt.Language
	src   []byte // normalized source the CST was parsed from
	smap  srcMap // clean-offset → original-offset remap (Task A2)
	opts  Options
	diags []sirena.Diagnostic
	fatal error
}

// lowerRoot walks the CST root and returns a *sirena.Document.
// Phase B/C/D populate sys.Elements / sys.Edges / sys.Boundaries.
func (l *lowerer) lowerRoot(root *gt.Node) *sirena.Document {
	sys := &sirena.SystemDecl{}
	// Phase B/C/D populate sys.Elements / sys.Edges / sys.Boundaries.
	_ = root
	return &sirena.Document{Systems: []*sirena.SystemDecl{sys}}
}
