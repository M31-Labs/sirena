package mermaid

import (
	gt "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
	"m31labs.dev/sirena"
)

// Options controls optional behaviour of Parse.
type Options struct {
	// Infer promotes unambiguous Mermaid constructs to typed sirena kinds
	// (cylinder→Database, edge label "reads"→EdgeKindReads). Default false.
	Infer bool
}

// Parse lowers a Mermaid flowchart/graph source into a *sirena.Document.
//
// src may use either the modern "flowchart" keyword or the legacy "graph"
// keyword — the normalize pre-pass rewrites "graph" to "flowchart"
// transparently. Styling directives (classDef, style, linkStyle, click) are
// stripped with a SIR-MERMAID-STYLE-DROPPED warning. Semicolon statement
// terminators are neutralized before parsing.
//
// Non-flowchart diagram types return a SIR-MERMAID-NOT-A-GRAPH diagnostic
// and a nil document.
func Parse(src []byte, opts Options) (*sirena.Document, []sirena.Diagnostic, error) {
	clean, preDiags, smap := normalize(src) // Task A2: graph→flowchart, ;→space, strip styling
	lang := grammars.MermaidLanguage()
	pool := gt.NewParserPool(lang)
	tree, err := pool.Parse(clean)
	if err != nil {
		return nil, preDiags, err
	}
	// smap maps clean-src byte offsets back to original-src offsets so every
	// CST-derived diagnostic Range points at the user's actual source.
	l := &lowerer{lang: lang, src: clean, smap: smap, opts: opts, diags: preDiags}
	doc := l.lowerRoot(tree.RootNode()) // Phase B/C/D
	return doc, l.diags, l.fatal
}
