package sirena

import "github.com/odvcencio/gotreesitter/grammargen"

// SirenaGrammar returns the v0.1 sirena grammar.
//
// The grammar covers the minimum surface required by Phase 2, Tasks 3-4:
// element declarations with optional metadata blocks, and typed boundary
// containers that may nest elements and further boundaries. Later tasks
// in Phase 2 extend this with edges, views, and overrides.
//
// Grammar (informal EBNF):
//
//	source_file     → (element_decl | boundary_decl)*
//	element_decl    → kind_keyword IDENT metadata_block?
//	kind_keyword    → "service" | "database" | "queue" | "cache" | "job"
//	                | "external" | "client" | "gateway" | "node"
//	metadata_block  → "{" pair* "}"
//	pair            → IDENT ":" value
//	value           → STRING | NUMBER | IDENT | list
//	list            → "[" value ("," value)* "]"
//	boundary_decl   → "boundary" boundary_kind STRING boundary_block
//	boundary_kind   → "trust" | "network" | "deployment" | "team"
//	boundary_block  → "{" (element_decl | boundary_decl)* "}"
//
// Whitespace (including newlines) and comments are extras. Comments are
// not yet defined; TODO(task-N): comments lands when the comment lint is
// authored.
func SirenaGrammar() *grammargen.Grammar {
	g := grammargen.NewGrammar("sirena")

	// source_file → (element_decl | boundary_decl)*
	g.Define("source_file", grammargen.Repeat(grammargen.Choice(
		grammargen.Sym("element_decl"),
		grammargen.Sym("boundary_decl"),
	)))

	// element_decl → kind_keyword IDENT metadata_block?
	g.Define("element_decl", grammargen.Seq(
		grammargen.Field("kind", grammargen.Sym("kind_keyword")),
		grammargen.Field("name", grammargen.Sym("identifier")),
		grammargen.Optional(grammargen.Field("body", grammargen.Sym("metadata_block"))),
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

	// boundary_decl → "boundary" boundary_kind STRING boundary_block
	g.Define("boundary_decl", grammargen.Seq(
		grammargen.Str("boundary"),
		grammargen.Field("kind", grammargen.Sym("boundary_kind")),
		grammargen.Field("name", grammargen.Sym("string")),
		grammargen.Field("body", grammargen.Sym("boundary_block")),
	))

	// boundary_decl_nested is a structurally identical sibling of
	// boundary_decl, used only inside boundary_block. The duplication is a
	// workaround for a grammargen LALR-state-merging issue that surfaces
	// when the same nonterminal appears in source_file's repeat AND inside
	// its own body's repeat (i.e. direct self-recursion through Repeat at
	// two levels). Using a distinct symbol keeps the parse states from
	// merging; an Alias on the reference rewrites the CST node type back to
	// `boundary_decl` so downstream lowering does not need to special-case.
	g.Define("boundary_decl_nested", grammargen.Seq(
		grammargen.Str("boundary"),
		grammargen.Field("kind", grammargen.Sym("boundary_kind")),
		grammargen.Field("name", grammargen.Sym("string")),
		grammargen.Field("body", grammargen.Sym("boundary_block")),
	))

	// boundary_kind → one of the typed boundary nouns.
	g.Define("boundary_kind", grammargen.Choice(
		grammargen.Str("trust"),
		grammargen.Str("network"),
		grammargen.Str("deployment"),
		grammargen.Str("team"),
	))

	// boundary_block → "{" (element_decl | boundary_decl)* "}"
	//
	// The boundary_decl reference inside the block is routed through the
	// boundary_decl_nested sibling rule (aliased back to "boundary_decl")
	// to avoid the LR table merging boundary_decl's two repeat contexts.
	// See the comment on boundary_decl_nested above.
	g.Define("boundary_block", grammargen.Seq(
		grammargen.Str("{"),
		grammargen.Repeat(grammargen.Choice(
			grammargen.Sym("element_decl"),
			grammargen.Alias(grammargen.Sym("boundary_decl_nested"), "boundary_decl", true),
		)),
		grammargen.Str("}"),
	))

	// metadata_block → "{" pair* "}"
	g.Define("metadata_block", grammargen.Seq(
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
