package sirena

import "github.com/odvcencio/gotreesitter/grammargen"

// SirenaGrammar returns the v0.1 sirena grammar.
//
// The grammar covers the full surface delivered by Phase 2, Tasks 3-9:
// import declarations, element declarations with optional metadata
// blocks, typed boundary containers (with optional metadata blocks) that
// may nest elements and further boundaries, typed directed edges between
// elements with optional metadata blocks and qualified-identifier
// endpoints (`platform.kafka`), `@override` annotations on element /
// boundary / edge declarations (bare, field-listed, or
// except-listed), and view declarations with
// title/preset/include/expand/collapse/theme directives plus optional
// `layout { ... }` and `budget { ... }` sub-blocks.
//
// Grammar (informal EBNF):
//
//	source_file       → (import_decl | element_decl | boundary_decl | edge_decl | view_decl)*
//	import_decl       → "import" STRING
//	element_decl      → kind_keyword IDENT override_annotation? metadata_block?
//	kind_keyword      → "service" | "database" | "queue" | "cache" | "job"
//	                  | "external" | "client" | "gateway" | "node"
//	metadata_block    → "{" pair* "}"
//	pair              → IDENT ":" value
//	value             → STRING | NUMBER | IDENT | list
//	list              → "[" value ("," value)* "]"
//	boundary_decl     → "boundary" boundary_kind STRING override_annotation?
//	                    metadata_block? boundary_block
//	boundary_kind     → "trust" | "network" | "deployment" | "team"
//	boundary_block    → "{" (element_decl | boundary_decl | edge_decl)* "}"
//	edge_decl         → (IDENT | qualified_ident) arrow (IDENT | qualified_ident)
//	                    override_annotation? (":" edge_kind STRING?)? metadata_block?
//	qualified_ident   → IDENT "." IDENT
//	override_annotation → "@override" override_args?
//	override_args     → "(" ("except" ":")? (IDENT ("," IDENT)*)? ")"
//	arrow             → "->" | "<-" | "<->"
//	edge_kind         → "calls" | "reads" | "writes" | "publishes"
//	                  | "subscribes" | "depends_on" | "flow"
//	view_decl         → "view" STRING view_block
//	view_block        → "{" (view_pair | layout_decl | budget_decl)* "}"
//	view_pair         → IDENT ":" view_value
//	view_value        → STRING | NUMBER | IDENT | list | selector_list
//	selector_list     → "[" selector ("," selector)* "]"
//	selector          → boundary_selector | element_selector | edges_selector
//	boundary_selector → "boundary" STRING
//	element_selector  → kind_keyword STRING
//	edges_selector    → "edges" ":" ("incoming" "to" | "outgoing" "from") STRING
//	layout_decl       → "layout" layout_block
//	layout_block      → "{" layout_pair* "}"
//	layout_pair       → IDENT ":" layout_value
//	layout_value      → STRING | NUMBER | IDENT | pin_map
//	pin_map           → "{" pin_pair ("," pin_pair)* "}"
//	pin_pair          → STRING ":" IDENT
//	budget_decl       → "budget" budget_block
//	budget_block      → "{" budget_pair* "}"
//	budget_pair       → IDENT ":" NUMBER
//
// Whitespace (including newlines) and comments are extras. Comments are
// not yet defined; TODO(task-N): comments lands when the comment lint is
// authored.
func SirenaGrammar() *grammargen.Grammar {
	g := grammargen.NewGrammar("sirena")

	// source_file → (import_decl | element_decl | boundary_decl | edge_decl | view_decl)*
	//
	// import_decl and view_decl are top-level only — neither nests inside
	// boundaries — so the LALR-state-merge workaround that applies to
	// boundary_decl / edge_decl (a `_nested` sibling aliased back to the
	// canonical name) is NOT required for either.
	g.Define("source_file", grammargen.Repeat(grammargen.Choice(
		grammargen.Sym("import_decl"),
		grammargen.Sym("element_decl"),
		grammargen.Sym("boundary_decl"),
		grammargen.Sym("edge_decl"),
		grammargen.Sym("view_decl"),
	)))

	// import_decl → "import" STRING
	//
	// The path is captured as a STRING literal; lowerImport strips the
	// quotes via decodeStringLiteral. Resolution against the workspace
	// happens in Phase 5; parse-time just records the directive.
	g.Define("import_decl", grammargen.Seq(
		grammargen.Str("import"),
		grammargen.Field("path", grammargen.Sym("string")),
	))

	// element_decl → kind_keyword IDENT override_annotation? metadata_block?
	//
	// The override_annotation slots between the name and the optional metadata
	// block so authors write `service api @override { ... }` reading
	// left-to-right. The annotation itself is unambiguous — it begins with
	// `@`, which appears nowhere else in the grammar — so LR(1) picks it up
	// with no lookahead games.
	g.Define("element_decl", grammargen.Seq(
		grammargen.Field("kind", grammargen.Sym("kind_keyword")),
		grammargen.Field("name", grammargen.Sym("identifier")),
		grammargen.Optional(grammargen.Field("override", grammargen.Sym("override_annotation"))),
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

	// boundary_decl → "boundary" boundary_kind STRING override_annotation?
	//                 metadata_block? boundary_block
	//
	// The optional metadata_block appears BEFORE the boundary_block (children).
	// Both blocks begin with "{" but are syntactically distinguishable because
	// metadata_block contains `pair` nodes (IDENT ":" value) while
	// boundary_block contains nested declarations starting with kind keywords.
	// override_annotation slots between the boundary name and the optional
	// metadata block; its leading `@` token is unambiguous.
	//
	// Empty-block edge case: `boundary trust "pci" {}` parses with the
	// metadata_block omitted and the single `{}` consumed as the
	// boundary_block (with zero children). The grammar never produces
	// both an empty metadata_block AND an empty boundary_block — LR(1)
	// commits to boundary_block for the only `{...}` pair because the
	// lookahead at `}` is consistent only with boundary_block's closing
	// brace shape.
	g.Define("boundary_decl", grammargen.Seq(
		grammargen.Str("boundary"),
		grammargen.Field("kind", grammargen.Sym("boundary_kind")),
		grammargen.Field("name", grammargen.Sym("string")),
		grammargen.Optional(grammargen.Field("override", grammargen.Sym("override_annotation"))),
		grammargen.Optional(grammargen.Field("metadata", grammargen.Sym("metadata_block"))),
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
		grammargen.Optional(grammargen.Field("override", grammargen.Sym("override_annotation"))),
		grammargen.Optional(grammargen.Field("metadata", grammargen.Sym("metadata_block"))),
		grammargen.Field("body", grammargen.Sym("boundary_block")),
	))

	// boundary_kind → one of the typed boundary nouns.
	g.Define("boundary_kind", grammargen.Choice(
		grammargen.Str("trust"),
		grammargen.Str("network"),
		grammargen.Str("deployment"),
		grammargen.Str("team"),
	))

	// boundary_block → "{" (element_decl | boundary_decl | edge_decl)* "}"
	//
	// The boundary_decl reference inside the block is routed through the
	// boundary_decl_nested sibling rule (aliased back to "boundary_decl")
	// to avoid the LR table merging boundary_decl's two repeat contexts.
	// edge_decl is similarly routed through edge_decl_nested for the same
	// reason — see the comments on boundary_decl_nested and edge_decl_nested.
	g.Define("boundary_block", grammargen.Seq(
		grammargen.Str("{"),
		grammargen.Repeat(grammargen.Choice(
			grammargen.Sym("element_decl"),
			grammargen.Alias(grammargen.Sym("boundary_decl_nested"), "boundary_decl", true),
			grammargen.Alias(grammargen.Sym("edge_decl_nested"), "edge_decl", true),
		)),
		grammargen.Str("}"),
	))

	// edge_decl → ident_or_qualified arrow ident_or_qualified
	//             (":" edge_kind STRING?)? metadata_block?
	//
	// Edge metadata is the optional trailing block; no ambiguity arises
	// because nothing else syntactically follows an edge declaration.
	//
	// `from` and `to` accept either a bare IDENT or a qualified_ident
	// (IDENT "." IDENT) so an imported namespace can be referenced as
	// `platform.kafka`. Lowering picks both apart into the structured
	// Edge.FromRef / Edge.ToRef while preserving the verbatim text in
	// Edge.From / Edge.To.
	g.Define("edge_decl", grammargen.Seq(
		grammargen.Field("from", grammargen.Sym("_ident_or_qualified")),
		grammargen.Field("arrow", grammargen.Sym("arrow")),
		grammargen.Field("to", grammargen.Sym("_ident_or_qualified")),
		grammargen.Optional(grammargen.Field("override", grammargen.Sym("override_annotation"))),
		grammargen.Optional(grammargen.Seq(
			grammargen.Str(":"),
			grammargen.Field("kind", grammargen.Sym("edge_kind")),
			grammargen.Optional(grammargen.Field("label", grammargen.Sym("string"))),
		)),
		grammargen.Optional(grammargen.Field("metadata", grammargen.Sym("metadata_block"))),
	))

	// edge_decl_nested is a structurally identical sibling of edge_decl,
	// used only inside boundary_block. The duplication is the same LALR
	// workaround applied to boundary_decl in Task 4 — when the same
	// nonterminal appears in Repeat at multiple nesting levels, grammargen
	// v0.19.1 silently merges LR states and nested parses fail. An Alias
	// on the reference rewrites the CST node type back to "edge_decl" so
	// downstream lowering does not need to special-case the sibling.
	g.Define("edge_decl_nested", grammargen.Seq(
		grammargen.Field("from", grammargen.Sym("_ident_or_qualified")),
		grammargen.Field("arrow", grammargen.Sym("arrow")),
		grammargen.Field("to", grammargen.Sym("_ident_or_qualified")),
		grammargen.Optional(grammargen.Field("override", grammargen.Sym("override_annotation"))),
		grammargen.Optional(grammargen.Seq(
			grammargen.Str(":"),
			grammargen.Field("kind", grammargen.Sym("edge_kind")),
			grammargen.Optional(grammargen.Field("label", grammargen.Sym("string"))),
		)),
		grammargen.Optional(grammargen.Field("metadata", grammargen.Sym("metadata_block"))),
	))

	// _ident_or_qualified is a hidden choice between a bare identifier
	// and a qualified identifier (IDENT "." IDENT). Used by edge_decl /
	// edge_decl_nested so namespaced endpoints from imports parse
	// cleanly. Hiding the choice (leading underscore) keeps the CST flat
	// — the named child of the `from` / `to` field is either an
	// "identifier" or a "qualified_ident", mirroring the `_value` shape.
	g.Define("_ident_or_qualified", grammargen.Choice(
		grammargen.Sym("qualified_ident"),
		grammargen.Sym("identifier"),
	))

	// qualified_ident → IDENT "." IDENT
	//
	// The two named children are "namespace" (the import-file stem) and
	// "name" (the identifier within that import).
	g.Define("qualified_ident", grammargen.Seq(
		grammargen.Field("namespace", grammargen.Sym("identifier")),
		grammargen.Str("."),
		grammargen.Field("name", grammargen.Sym("identifier")),
	))

	// arrow → "->" | "<-" | "<->"
	//
	// Order matters: "<->" must appear before "<-" so tree-sitter's
	// longest-match lexer picks the trigraph; reordering will silently
	// break bidirectional edges by lexing them as a "<-" followed by a
	// stray ">" that fails the parser at the next state. "->" position
	// is incidental because no other arrow shares its prefix.
	g.Define("arrow", grammargen.Choice(
		grammargen.Str("<->"),
		grammargen.Str("->"),
		grammargen.Str("<-"),
	))

	// edge_kind → one of the typed edge verbs, or the untyped fallback.
	g.Define("edge_kind", grammargen.Choice(
		grammargen.Str("calls"),
		grammargen.Str("reads"),
		grammargen.Str("writes"),
		grammargen.Str("publishes"),
		grammargen.Str("subscribes"),
		grammargen.Str("depends_on"),
		grammargen.Str("flow"),
	))

	// override_annotation → "@override" override_args?
	//
	// `@override` is a single anonymous string token rather than two tokens
	// (`@` + the `override` keyword). The leading `@` appears nowhere else
	// in the grammar, so the lexer routes it unambiguously and the parser
	// does not need precedence hints to tell it apart from a normal
	// identifier or metadata key. The optional args parenthesized group is
	// hoisted into its own rule (`override_args`) so the CST exposes a
	// stable `args` field for the lowering function to consult.
	g.Define("override_annotation", grammargen.Seq(
		grammargen.Str("@override"),
		grammargen.Optional(grammargen.Field("args", grammargen.Sym("override_args"))),
	))

	// override_args → "(" ("except" ":")? (IDENT ("," IDENT)*)? ")"
	//
	// Two shapes are accepted inside the parens:
	//   1. `(field1, field2)`         — OverrideFields mode.
	//   2. `(except: field1, field2)` — OverrideExcept mode.
	// The leading `except` keyword followed by `:` is the discriminator;
	// LR(1) picks one branch by looking at the first two tokens after `(`.
	// The empty-args shape `()` is permitted by the grammar (the field
	// list is Optional) — Phase 10 will lint it as a likely author error
	// because the IR cannot distinguish empty args from OverrideAll.
	//
	// Field names on this rule: `except` (a sentinel node — its presence
	// flips the lowering to OverrideExcept mode), and `fields` (the IDENT
	// list, lowered into Override.Fields).
	g.Define("override_args", grammargen.Seq(
		grammargen.Str("("),
		grammargen.Optional(grammargen.Field("except", grammargen.Sym("override_except"))),
		grammargen.Optional(grammargen.Field("fields", grammargen.Sym("override_field_list"))),
		grammargen.Str(")"),
	))

	// override_except → "except" ":"
	//
	// Hoisting the `except :` pair into its own rule lets `override_args`
	// reference it as a single field-named child. Without this hoist the
	// `except` field would point at just the keyword and the `:` would
	// dangle as an anonymous token, making the CST harder to walk.
	g.Define("override_except", grammargen.Seq(
		grammargen.Str("except"),
		grammargen.Str(":"),
	))

	// override_field_list → IDENT ("," IDENT)*
	//
	// The list is non-empty by construction (CommaSep1). The Optional()
	// wrapper around the `fields` field on override_args is what makes the
	// entire list optional inside the parens.
	g.Define("override_field_list", grammargen.CommaSep1(grammargen.Sym("identifier")))

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

	// view_decl → "view" STRING view_block
	//
	// View declarations are top-level only and start with the dedicated
	// "view" keyword, which lex-promotes against the `identifier` word
	// rule so there is no conflict with arbitrary identifier-keyed pairs.
	g.Define("view_decl", grammargen.Seq(
		grammargen.Str("view"),
		grammargen.Field("name", grammargen.Sym("string")),
		grammargen.Field("body", grammargen.Sym("view_block")),
	))

	// view_block → "{" (view_pair | layout_decl | budget_decl)* "}"
	//
	// `layout` and `budget` are dedicated keywords (Str()-promoted via the
	// `identifier` word rule). At the start of a directive the parser
	// distinguishes them from a generic IDENT-keyed view_pair by their
	// distinct terminal symbol; once past the keyword the disambiguation
	// is unambiguous (a view_pair has ":" as its second token, while
	// layout_decl / budget_decl have "{").
	g.Define("view_block", grammargen.Seq(
		grammargen.Str("{"),
		grammargen.Repeat(grammargen.Choice(
			grammargen.Sym("layout_decl"),
			grammargen.Sym("budget_decl"),
			grammargen.Sym("view_pair"),
		)),
		grammargen.Str("}"),
	))

	// view_pair → IDENT ":" _view_value
	//
	// Distinct from the metadata-block `pair` rule because the value
	// position accepts an additional shape (`selector_list`) that is
	// invalid in element / boundary / edge metadata. Keeping these two
	// pair rules separate avoids polluting `_value` with selector forms.
	g.Define("view_pair", grammargen.Seq(
		grammargen.Field("key", grammargen.Sym("identifier")),
		grammargen.Str(":"),
		grammargen.Field("value", grammargen.Sym("_view_value")),
	))

	// _view_value is the hidden choice for view_pair right-hand sides.
	// `list` and `selector_list` both start with "[" — LR(1) disambiguates
	// at the next token: a selector-leading keyword (boundary / a
	// kind_keyword / edges) routes to selector_list, while any IDENT /
	// STRING / NUMBER routes to the generic value list. `dashed_ident`
	// admits kebab-case atoms like `earth-default` so theme/preset
	// directives can use the source convention without quoting.
	g.Define("_view_value", grammargen.Choice(
		grammargen.Sym("string"),
		grammargen.Sym("number"),
		grammargen.Sym("identifier"),
		grammargen.Sym("dashed_ident"),
		grammargen.Sym("selector_list"),
		grammargen.Sym("list"),
	))

	// selector_list → "[" selector* "]"
	//
	// Selectors are whitespace-separated (newlines are extras) rather than
	// comma-separated — the spec's include block lists items one per line
	// with no punctuation between them. Each selector starts with a
	// distinct terminal (`boundary` / a kind_keyword / `edges`) so the
	// parser does not need a separator to detect item boundaries.
	g.Define("selector_list", grammargen.Seq(
		grammargen.Str("["),
		grammargen.Repeat(grammargen.Sym("_selector")),
		grammargen.Str("]"),
	))

	// _selector is the hidden choice among the three selector forms.
	// Each form starts with a distinct terminal (`boundary` / kind_keyword
	// / `edges`) so LR(1) routes unambiguously.
	g.Define("_selector", grammargen.Choice(
		grammargen.Sym("boundary_selector"),
		grammargen.Sym("edges_selector"),
		grammargen.Sym("element_selector"),
	))

	// boundary_selector → "boundary" STRING
	g.Define("boundary_selector", grammargen.Seq(
		grammargen.Str("boundary"),
		grammargen.Field("target", grammargen.Sym("string")),
	))

	// element_selector → kind_keyword STRING
	//
	// Reuses the same kind_keyword nonterminal element_decl uses, so the
	// element_kind enum mapping logic in lowering applies unchanged.
	g.Define("element_selector", grammargen.Seq(
		grammargen.Field("kind", grammargen.Sym("kind_keyword")),
		grammargen.Field("target", grammargen.Sym("string")),
	))

	// edges_selector → "edges" ":" ("incoming" "to" | "outgoing" "from") STRING
	//
	// `incoming`, `outgoing`, `to`, `from` are keyword-promoted via Str()
	// so the parser rejects misuse at parse time. The two direction
	// branches each carry their own preposition because the spec pairs
	// "incoming" with "to" and "outgoing" with "from".
	g.Define("edges_selector", grammargen.Seq(
		grammargen.Str("edges"),
		grammargen.Str(":"),
		grammargen.Field("direction", grammargen.Choice(
			grammargen.Seq(grammargen.Str("incoming"), grammargen.Str("to")),
			grammargen.Seq(grammargen.Str("outgoing"), grammargen.Str("from")),
		)),
		grammargen.Field("target", grammargen.Sym("string")),
	))

	// layout_decl → "layout" layout_block
	g.Define("layout_decl", grammargen.Seq(
		grammargen.Str("layout"),
		grammargen.Field("body", grammargen.Sym("layout_block")),
	))

	// layout_block → "{" layout_pair* "}"
	g.Define("layout_block", grammargen.Seq(
		grammargen.Str("{"),
		grammargen.Repeat(grammargen.Sym("layout_pair")),
		grammargen.Str("}"),
	))

	// layout_pair → IDENT ":" _layout_value
	g.Define("layout_pair", grammargen.Seq(
		grammargen.Field("key", grammargen.Sym("identifier")),
		grammargen.Str(":"),
		grammargen.Field("value", grammargen.Sym("_layout_value")),
	))

	// _layout_value is the hidden choice for layout_pair right-hand sides.
	// pin_map is the only shape whose first token is "{", so LR(1)
	// disambiguates trivially. `dashed_ident` parallels its use in
	// _view_value so a layout `direction: top-down` lexes as one atom.
	g.Define("_layout_value", grammargen.Choice(
		grammargen.Sym("string"),
		grammargen.Sym("number"),
		grammargen.Sym("identifier"),
		grammargen.Sym("dashed_ident"),
		grammargen.Sym("pin_map"),
	))

	// pin_map → "{" pin_pair ("," pin_pair)* "}"
	g.Define("pin_map", grammargen.Seq(
		grammargen.Str("{"),
		grammargen.Optional(grammargen.CommaSep1(grammargen.Sym("pin_pair"))),
		grammargen.Str("}"),
	))

	// pin_pair → STRING ":" IDENT
	g.Define("pin_pair", grammargen.Seq(
		grammargen.Field("name", grammargen.Sym("string")),
		grammargen.Str(":"),
		grammargen.Field("side", grammargen.Sym("identifier")),
	))

	// budget_decl → "budget" budget_block
	g.Define("budget_decl", grammargen.Seq(
		grammargen.Str("budget"),
		grammargen.Field("body", grammargen.Sym("budget_block")),
	))

	// budget_block → "{" budget_pair* "}"
	g.Define("budget_block", grammargen.Seq(
		grammargen.Str("{"),
		grammargen.Repeat(grammargen.Sym("budget_pair")),
		grammargen.Str("}"),
	))

	// budget_pair → IDENT ":" NUMBER
	g.Define("budget_pair", grammargen.Seq(
		grammargen.Field("key", grammargen.Sym("identifier")),
		grammargen.Str(":"),
		grammargen.Field("value", grammargen.Sym("number")),
	))

	// identifier → [A-Za-z_][A-Za-z0-9_]*
	// Word rule so kind_keyword's anonymous tokens are matched against it
	// for keyword-vs-identifier disambiguation.
	g.Define("identifier", grammargen.Token(grammargen.Seq(
		grammargen.Pat(`[A-Za-z_]`),
		grammargen.Repeat(grammargen.Pat(`[A-Za-z0-9_]`)),
	)))
	g.SetWord("identifier")

	// dashed_ident → identifier ("-" identifier)+
	//
	// A kebab-case atom — used only inside view/layout pair right-hand
	// sides where authors write theme names like `earth-default` and
	// flow hints like `top-down` without quoting. The trailing dash run
	// is Repeat1 (NOT Repeat) so plain `earth` continues to lex as a
	// regular `identifier`; only inputs containing at least one internal
	// dash promote to dashed_ident. Tree-sitter's lexer prefers the
	// longest match, so `earth-default` is preferred over the prefix
	// `earth` when the trailing `-default` is present.
	//
	// The dash-run REQUIRES the next character after `-` to be an
	// identifier head character; an isolated trailing dash (e.g.
	// `earth- `) does NOT promote to dashed_ident. This keeps `api -> db`
	// safe — the lexer never tries to glue the leading `-` of the arrow
	// onto the preceding identifier.
	g.Define("dashed_ident", grammargen.Token(grammargen.Seq(
		grammargen.Pat(`[A-Za-z_]`),
		grammargen.Repeat(grammargen.Pat(`[A-Za-z0-9_]`)),
		grammargen.Repeat1(grammargen.Seq(
			grammargen.Str("-"),
			grammargen.Pat(`[A-Za-z_]`),
			grammargen.Repeat(grammargen.Pat(`[A-Za-z0-9_]`)),
		)),
	)))

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
