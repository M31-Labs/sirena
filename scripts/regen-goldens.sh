#!/usr/bin/env bash
# Regenerate every conformance case's golden files by running the sirena
# binary against each case's input.sir.
#
# Run from the repo root after any intentional behavioral change to the
# parser or printer. Never hand-edit goldens.

set -euo pipefail

GO_BIN="${SIRENA_GO_BIN:-$HOME/go/bin/go1.26.0}"

REPO_ROOT=$(git rev-parse --show-toplevel)
cd "$REPO_ROOT"

# Build the binary fresh.
"$GO_BIN" build -o ./bin/sirena ./cmd/sirena

CORPUS=testdata/conformance
if [ ! -d "$CORPUS" ]; then
  echo "no corpus dir at $CORPUS" >&2
  exit 1
fi

count=0
for case_dir in "$CORPUS"/*/; do
  case_dir=${case_dir%/}
  case_id=$(basename "$case_dir")
  input="$case_dir/input.sir"
  if [ ! -f "$input" ]; then
    echo "skip $case_id (no input.sir)" >&2
    continue
  fi
  ./bin/sirena parse --json "$input" > "$case_dir/ir.golden.json"
  ./bin/sirena fmt "$input"        > "$case_dir/fmt.golden.sir"
  count=$((count + 1))
done

echo "regenerated $count case(s)"
