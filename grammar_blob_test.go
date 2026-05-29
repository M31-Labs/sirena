package sirena

import (
	"testing"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammargen"
)

// blobSyncSamples cover the grammar surface that matters for parse-tree
// equivalence: elements with metadata, typed boundaries with nested
// children, every edge arrow form, qualified idents, overrides, and a view
// with layout/budget sub-blocks.
var blobSyncSamples = []string{
	"service api {\n  label: \"API\"\n  tags: [edge, stateless]\n}\n",
	"boundary trust \"pci\" {\n  service api\n  database db\n  api -> db: reads \"rows\"\n}\n",
	"a -> b: calls\nb <- c\na <-> d: flow\n",
	"import \"../shared/platform.sir\"\nplatform.kafka -> api: publishes\n",
	"service api @override(except: label)\n",
	"view dash {\n  title: \"Dashboard\"\n  layout { direction: down }\n  budget { nodes: 30 }\n}\n",
}

// BenchmarkGrammarGenerate measures the LR-table construction cost the blob
// avoids; BenchmarkGrammarLoadBlob measures the cost paid instead. The gap
// is the first-Parse latency the pre-rendered blob saves.
func BenchmarkGrammarGenerate(b *testing.B) {
	for b.Loop() {
		if _, err := grammargen.GenerateLanguage(SirenaGrammar()); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGrammarLoadBlob(b *testing.B) {
	for b.Loop() {
		if _, err := gotreesitter.LoadLanguage(grammarBlob); err != nil {
			b.Fatal(err)
		}
	}
}

// TestGrammarBlobLoads confirms the embedded blob decodes into a usable
// language and parses a basic document without error.
func TestGrammarBlobLoads(t *testing.T) {
	lang, err := gotreesitter.LoadLanguage(grammarBlob)
	if err != nil {
		t.Fatalf("LoadLanguage(grammarBlob): %v", err)
	}
	if lang == nil {
		t.Fatal("LoadLanguage returned nil language")
	}
	if _, err := Parse([]byte("service api\napi -> db: calls\n")); err != nil {
		t.Fatalf("Parse with blob-loaded language: %v", err)
	}
}

// TestGrammarBlobInSync guards against a stale grammar.blob: it regenerates
// the language from SirenaGrammar and asserts the embedded blob parses every
// sample to an identical CST. If this fails, grammar.blob is out of date —
// run `go generate ./...` and commit the result.
func TestGrammarBlobInSync(t *testing.T) {
	fresh, err := grammargen.GenerateLanguage(SirenaGrammar())
	if err != nil {
		t.Fatalf("GenerateLanguage(SirenaGrammar()): %v", err)
	}
	blobLang, err := gotreesitter.LoadLanguage(grammarBlob)
	if err != nil {
		t.Fatalf("LoadLanguage(grammarBlob): %v", err)
	}

	freshParser := gotreesitter.NewParser(fresh)
	blobParser := gotreesitter.NewParser(blobLang)
	for _, src := range blobSyncSamples {
		freshTree, err := freshParser.Parse([]byte(src))
		if err != nil {
			t.Fatalf("fresh parse %q: %v", src, err)
		}
		blobTree, err := blobParser.Parse([]byte(src))
		if err != nil {
			t.Fatalf("blob parse %q: %v", src, err)
		}
		want := freshTree.RootNode().SExpr(fresh)
		got := blobTree.RootNode().SExpr(blobLang)
		freshTree.Release()
		blobTree.Release()
		if got != want {
			t.Errorf("grammar.blob is stale for %q — run `go generate ./...` and commit grammar.blob\n got: %s\nwant: %s", src, got, want)
		}
	}
}
