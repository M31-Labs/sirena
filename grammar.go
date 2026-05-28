package sirena

import "github.com/odvcencio/gotreesitter/grammargen"

// SirenaGrammar returns the v0.1 sirena grammar.
//
// The grammar covers the minimum surface required by Phase 2, Task 3:
// element declarations with optional metadata blocks. Later tasks in
// Phase 2 extend this with boundaries, edges, views, and overrides.
//
// Grammar (informal EBNF):
//
//	source_file   → element_decl*
//	element_decl  → kind_keyword IDENT block?
//	kind_keyword  → "service" | "database" | "queue" | "cache" | "job"
//	              | "external" | "client" | "gateway" | "node"
//	block         → "{" pair* "}"
//	pair          → IDENT ":" value
//	value         → STRING | NUMBER | IDENT | list
//	list          → "[" value ("," value)* "]"
//
// Whitespace (including newlines) and comments are extras. Comments are
// not yet defined; TODO(task-N): comments lands when the comment lint is
// authored.
func SirenaGrammar() *grammargen.Grammar {
	g := grammargen.NewGrammar("sirena")

	// source_file → element_decl*
	g.Define("source_file", grammargen.Repeat(grammargen.Sym("element_decl")))

	// element_decl → kind_keyword IDENT block?
	g.Define("element_decl", grammargen.Seq(
		grammargen.Field("kind", grammargen.Sym("kind_keyword")),
		grammargen.Field("name", grammargen.Sym("identifier")),
		grammargen.Optional(grammargen.Field("body", grammargen.Sym("block"))),
	))

	// kind_keyword → one of the typed element nouns.
	g.Define("kind_keyword", grammargen.Choice(
		grammargen.Str("service"),
		grammargen.Str("database"),
		grammargen.Str("queue"),
		grammargen.Str("cache"),
		grammargen.Str("job"),
		grammargen.Str("external"),
		grammargen.Str("client"),
		grammargen.Str("gateway"),
		grammargen.Str("node"),
	))

	// block → "{" pair* "}"
	g.Define("block", grammargen.Seq(
		grammargen.Str("{"),
		grammargen.Repeat(grammargen.Sym("pair")),
		grammargen.Str("}"),
	))

	// pair → IDENT ":" value
	g.Define("pair", grammargen.Seq(
		grammargen.Field("key", grammargen.Sym("identifier")),
		grammargen.Str(":"),
		grammargen.Field("value", grammargen.Sym("_value")),
	))

	// _value is hidden; the visible variants are string / number /
	// identifier / list, which lower directly to typed sirena.Value
	// concrete types.
	g.Define("_value", grammargen.Choice(
		grammargen.Sym("string"),
		grammargen.Sym("number"),
		grammargen.Sym("identifier"),
		grammargen.Sym("list"),
	))

	// list → "[" value ("," value)* "]"
	g.Define("list", grammargen.Seq(
		grammargen.Str("["),
		grammargen.Optional(grammargen.CommaSep1(grammargen.Sym("_value"))),
		grammargen.Str("]"),
	))

	// identifier → [A-Za-z_][A-Za-z0-9_]*
	// Word rule so kind_keyword's anonymous tokens are matched against it
	// for keyword-vs-identifier disambiguation.
	g.Define("identifier", grammargen.Token(grammargen.Seq(
		grammargen.Pat(`[A-Za-z_]`),
		grammargen.Repeat(grammargen.Pat(`[A-Za-z0-9_]`)),
	)))
	g.SetWord("identifier")

	// string → double-quoted string literal with simple backslash escapes.
	g.Define("string", grammargen.Seq(
		grammargen.Str("\""),
		grammargen.Optional(grammargen.Sym("_string_content")),
		grammargen.Str("\""),
	))
	g.Define("_string_content", grammargen.Repeat1(grammargen.Choice(
		grammargen.Sym("string_content"),
		grammargen.Sym("escape_sequence"),
	)))
	g.Define("string_content", grammargen.ImmToken(grammargen.Prec(1, grammargen.Pat(`[^\\"\n]+`))))
	g.Define("escape_sequence", grammargen.ImmToken(grammargen.Seq(
		grammargen.Str("\\"),
		grammargen.Pat(`["\\nrtbfv0]`),
	)))

	// number → optional sign, integer, optional fraction, optional exponent.
	g.Define("number", grammargen.Token(grammargen.Seq(
		grammargen.Optional(grammargen.Str("-")),
		grammargen.Choice(
			grammargen.Str("0"),
			grammargen.Seq(grammargen.Pat(`[1-9]`), grammargen.Repeat(grammargen.Pat(`[0-9]`))),
		),
		grammargen.Optional(grammargen.Seq(grammargen.Str("."), grammargen.Repeat1(grammargen.Pat(`[0-9]`)))),
		grammargen.Optional(grammargen.Seq(
			grammargen.Pat(`[eE]`),
			grammargen.Optional(grammargen.Pat(`[+\-]`)),
			grammargen.Repeat1(grammargen.Pat(`[0-9]`)),
		)),
	)))

	// Extras: any whitespace, including newlines. Sirena is not line-oriented
	// at this stage; element declarations may span lines inside a block.
	g.SetExtras(grammargen.Pat(`\s`))

	return g
}
