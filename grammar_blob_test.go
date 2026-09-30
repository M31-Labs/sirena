package sirena

import (
	"crypto/sha256"
	"reflect"
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
// language blob from SirenaGrammar, compares every serialized grammar field,
// and asserts the embedded blob parses every sample to an identical CST.
// If this fails, grammar.blob is out of date —
// run `go generate ./...` and commit the result.
func TestGrammarBlobInSync(t *testing.T) {
	freshBlob, err := grammargen.Generate(SirenaGrammar())
	if err != nil {
		t.Fatalf("Generate(SirenaGrammar()): %v", err)
	}
	fresh, err := gotreesitter.LoadLanguage(freshBlob)
	if err != nil {
		t.Fatalf("LoadLanguage(regenerated): %v", err)
	}
	blobLang, err := gotreesitter.LoadLanguage(grammarBlob)
	if err != nil {
		t.Fatalf("LoadLanguage(grammarBlob): %v", err)
	}

	// Compare decoded serialized fields: legacy blobs lack the current version
	// header, and gob type IDs and nil/empty slices can change encoded bytes.
	// Private loader identity and lazy caches are not grammar table contents.
	if fields := grammarBlobTableDifferences(fresh, blobLang); len(fields) > 0 {
		t.Fatalf("grammar.blob is stale in fields %v (embedded sha256=%x, regenerated sha256=%x) — run go generate ./...",
			fields, sha256.Sum256(grammarBlob), sha256.Sum256(freshBlob))
	}
	t.Logf("all serialized grammar fields match (embedded sha256=%x, regenerated sha256=%x)",
		sha256.Sum256(grammarBlob), sha256.Sum256(freshBlob))

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

// grammarBlobTableDifferences compares all fields the blob encoder can observe.
func grammarBlobTableDifferences(fresh, embedded *gotreesitter.Language) []string {
	want, got := reflect.ValueOf(fresh).Elem(), reflect.ValueOf(embedded).Elem()
	var differences []string
	for i := 0; i < want.NumField(); i++ {
		field := want.Type().Field(i)
		if field.IsExported() && !reflect.DeepEqual(want.Field(i).Interface(), got.Field(i).Interface()) {
			differences = append(differences, field.Name)
		}
	}
	return differences
}

func TestGrammarBlobTablesRejectStaleContents(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*gotreesitter.Language)
	}{
		{"SymbolNames", func(lang *gotreesitter.Language) { lang.SymbolNames = []string{"stale_symbol"} }},
		{"ParseActions", func(lang *gotreesitter.Language) {
			lang.ParseActions = []gotreesitter.ParseActionEntry{{Actions: []gotreesitter.ParseAction{{State: 65535}}}}
		}},
		{"LexStates", func(lang *gotreesitter.Language) { lang.LexStates = []gotreesitter.LexState{{Default: 65535}} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fresh, err := gotreesitter.LoadLanguage(grammarBlob)
			if err != nil {
				t.Fatal(err)
			}
			embedded, err := gotreesitter.LoadLanguage(grammarBlob)
			if err != nil {
				t.Fatal(err)
			}
			tc.mutate(embedded)
			if got := grammarBlobTableDifferences(fresh, embedded); !reflect.DeepEqual(got, []string{tc.name}) {
				t.Fatalf("stale %s differences = %v", tc.name, got)
			}
		})
	}
}
