#!/bin/sh
# entrypoint.sh — Sirena Bake GitHub Action entrypoint
#
# Positional args (from action.yml):
#   $1 = paths              space/newline-separated globs of Markdown files
#   $2 = theme              sirena theme name  (default: earth-default)
#   $3 = infer              "true" | "false"   (default: false)
#   $4 = working-directory  run directory      (default: .)
#
# Exit codes mirror sirena bake:
#   0  all diagrams baked successfully
#   1  one or more render errors
#   2  misuse (bad flags, unknown theme, no files)
set -e

INPUT_PATHS="$1"
THEME="${2:-earth-default}"
INFER="${3:-false}"
WORKDIR="${4:-.}"

if [ -z "$INPUT_PATHS" ]; then
  echo "::error::sirena-bake: 'paths' input is required" >&2
  exit 2
fi

cd "$WORKDIR"

# Expand globs.  $INPUT_PATHS may be space- or newline-separated.
# Replace newlines with spaces for uniform word-splitting.
PATHS_FLAT=$(printf '%s' "$INPUT_PATHS" | tr '\n' ' ')

# Build the sirena argument list using positional parameters so every
# flag and filename is a distinct word — avoids quoting pitfalls with
# plain string concatenation.
set -- --theme "$THEME"
[ "$INFER" = "true" ] && set -- "$@" --infer

FILE_COUNT=0
for GLOB in $PATHS_FLAT; do
  for F in $GLOB; do
    if [ -f "$F" ]; then
      set -- "$@" "$F"
      FILE_COUNT=$((FILE_COUNT + 1))
    fi
  done
done

if [ "$FILE_COUNT" -eq 0 ]; then
  echo "::warning::sirena-bake: no Markdown files matched paths: ${PATHS_FLAT}"
  exit 0
fi

echo "::group::Sirena Bake"
printf 'theme:   %s\n' "$THEME"
printf 'infer:   %s\n' "$INFER"
printf 'workdir: %s\n' "$WORKDIR"
printf 'files:   %d matched\n' "$FILE_COUNT"
echo ""

set +e
sirena bake "$@"
EXIT_CODE=$?
set -e

echo "::endgroup::"

case "$EXIT_CODE" in
  0) ;;
  1) echo "::error::sirena-bake: one or more diagrams failed to render" ;;
  2) echo "::error::sirena-bake: misuse — bad flags or unknown theme" ;;
  *) echo "::error::sirena-bake: unexpected exit code $EXIT_CODE" ;;
esac

exit "$EXIT_CODE"
