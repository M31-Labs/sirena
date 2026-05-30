# Sirena Bake — GitHub Action

The `M31-Labs/sirena` repository is itself the action. Add it to any workflow with
`uses: M31-Labs/sirena@v1`.

## What it does

For each Markdown file that matches `paths`, the action:

1. Finds every ` ```mermaid ` and ` ```sirena ` fence.
2. Renders each diagram to an SVG file (committed alongside the Markdown).
3. Rewrites the fence to a source-preserving block:
   - An inline `![](diagram.N.svg)` reference — GitHub renders it natively.
   - A collapsed `<details>` block that preserves the original source.

The action **does not commit**. Leave that to your workflow so you control the commit
message, author, and push target.

## Inputs

| Input | Required | Default | Description |
|---|---|---|---|
| `paths` | yes | — | Space- or newline-separated globs of Markdown files, e.g. `"docs/**/*.md README.md"` |
| `theme` | no | `earth-default` | Sirena theme name |
| `infer` | no | `false` | When `"true"`, promote Mermaid shapes/labels to typed Sirena kinds |
| `working-directory` | no | `.` | Directory to run `sirena bake` from |

## Exit codes

| Code | Meaning |
|---|---|
| 0 | All diagrams baked successfully |
| 1 | One or more diagrams failed to render |
| 2 | Misuse — bad flag, unknown theme, or no files matched |

## Minimal usage

```yaml
- uses: M31-Labs/sirena@v1
  with:
    paths: "docs/**/*.md README.md"
```

## Full example with commit-back

```yaml
on: [push]

jobs:
  bake:
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: M31-Labs/sirena@v1
        with:
          paths: "docs/**/*.md README.md"
          theme: earth-default
          infer: "false"

      - name: Commit SVGs
        run: |
          git config user.name  "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git add -A
          if git diff --cached --quiet; then
            echo "Nothing changed."
          else
            git commit -m "chore(docs): bake diagram SVGs [skip ci]"
            git push
          fi
```

## How SVGs render on GitHub

GitHub renders `![](path/to/file.svg)` inline on any Markdown page — repository
READMEs, wiki pages, pull request descriptions, and `docs/` directories served by
GitHub Pages. The generated SVGs use only static elements (no `<script>`) so they
pass GitHub's content-security policy.

## Versioning

Pin to a tag (`M31-Labs/sirena@v1`) for stability. The action bundles the sirena
binary at build time — no internet access or `go install` required at run time.
