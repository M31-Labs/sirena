// Command gengrammar serializes the sirena tree-sitter grammar into a
// pre-compiled blob (grammar.blob at the module root) so the runtime loads
// the language with gotreesitter.LoadLanguage instead of paying the LR-table
// construction cost of grammargen.GenerateLanguage on the first Parse.
//
// Regenerate after any change to SirenaGrammar with:
//
//	go generate ./...
//
// The blob is gzipped gob (grammargen.Generate's format), directly loadable
// by gotreesitter.LoadLanguage.
package main

import (
	"log"
	"os"

	"github.com/odvcencio/gotreesitter/grammargen"

	"m31labs.dev/sirena"
)

func main() {
	out := "grammar.blob"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	blob, err := grammargen.Generate(sirena.SirenaGrammar())
	if err != nil {
		log.Fatalf("generate grammar blob: %v", err)
	}
	if err := os.WriteFile(out, blob, 0o644); err != nil {
		log.Fatalf("write %s: %v", out, err)
	}
	log.Printf("wrote %s (%d bytes)", out, len(blob))
}
