package mermaid

import (
	_ "embed"
	"sync"

	gt "github.com/odvcencio/gotreesitter"
	"m31labs.dev/sirena"
)

//go:embed mermaid.bin
var mermaidGrammarBlob []byte

var (
	mermaidLangOnce sync.Once
	mermaidLang     *gt.Language
)

func mermaidLanguage() *gt.Language {
	mermaidLangOnce.Do(func() {
		lang, err := gt.LoadLanguage(mermaidGrammarBlob)
		if err != nil {
			panic("sirena/ingest/mermaid: failed to load embedded Mermaid grammar: " + err.Error())
		}
		mermaidLang = lang
	})
	return mermaidLang
}

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
	lang := mermaidLanguage()
	pool := gt.NewParserPool(lang)
	tree, err := pool.Parse(clean)
	if err != nil {
		return nil, preDiags, err
	}
	// Guard against pool.Parse returning a nil tree or nil root node (e.g. for
	// completely empty input). Mirror the pattern in emit/goimport.go and
	// parse.go: treat a nil root as NOT-A-GRAPH without dereferencing the node.
	if tree == nil || tree.RootNode() == nil {
		diag := sirena.Diagnostic{
			Code:     "SIR-MERMAID-NOT-A-GRAPH",
			Severity: sirena.SeverityError,
			Message:  "Input is not a Mermaid flowchart (no diagram_flow node found)",
			Range:    sirena.Range{},
		}
		return nil, append(preDiags, diag), notAGraphError
	}
	// smap maps clean-src byte offsets back to original-src offsets so every
	// CST-derived diagnostic Range points at the user's actual source.
	l := &lowerer{lang: lang, src: clean, smap: smap, opts: opts, diags: preDiags}
	doc := l.lowerRoot(tree.RootNode()) // Phase B/C/D/E
	if opts.Infer && doc != nil {
		applyInference(doc) // Phase F: opt-in promotion pass
	}
	return doc, l.diags, l.fatal
}
