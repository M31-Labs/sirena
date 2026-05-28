# Sirena conformance corpus

Golden-file test corpus for the sirena parser, printer, and (in later plans) layout/SVG. Each case is a directory.

## Case layout

```
testdata/conformance/<case-id>/
├── input.sir            # the source under test
├── ir.golden.json       # expected IR after Parse (mdpp-shape JSON dump)
└── fmt.golden.sir       # expected output of Format(input)
```

Plan 2 adds `layout.golden.json`; Plan 2 also adds `svg.golden.svg`.

## Case id

`<NNN>-<short-slug>`, lexically sortable. `NNN` is a zero-padded three-digit number; the slug is kebab-case and descriptive.

## Regenerating goldens

```
scripts/regen-goldens.sh
```

Re-runs the sirena binary against every case's `input.sir` and overwrites the golden files. Run after any intentional behavioral change to the parser or printer. Never hand-edit goldens.

## Runner

`_runner_test.go` enumerates cases and asserts each case's goldens match the current implementation. Cases with no goldens are SKIPped (used during early plan development).
