#!/usr/bin/env bash
# Sandboxed end-to-end smoke test for the mtm binary.
#
# Runs the usual lifecycle against a throwaway data repo in a temp dir.
# HOME and every XDG root point inside the temp dir and MTM_SANDBOX=1 is
# exported (via contrib/sandbox-env.sh), so the internal/paths guard
# refuses anything that still resolves to the real ~/.local/share/mtm,
# ~/.config/mtm, ~/.cache/mtm or ~/.local/state/mtm (it panics instead).
# The sandbox is deleted on exit.
#
# Usage: make smoke   (builds bin/mtm first, or run contrib/smoke.sh [binary])
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
binary="${1:-$repo_root/bin/mtm}"
if [ ! -x "$binary" ]; then
  (cd "$repo_root" && go build -o bin/mtm ./cmd/mtm)
  binary="$repo_root/bin/mtm"
fi

source "$repo_root/contrib/sandbox-env.sh"
sandbox="$MTM_SANDBOX_DIR"

export GIT_AUTHOR_NAME=smoke GIT_AUTHOR_EMAIL=smoke@example.invalid
export GIT_COMMITTER_NAME=smoke GIT_COMMITTER_EMAIL=smoke@example.invalid
export GIT_TERMINAL_PROMPT=0

echo "smoke: sandbox at $sandbox"

"$binary" init

add_out="$("$binary" add "smoke test the thing tomorrow !high")"
echo "$add_out"
id="$(awk '{print $2}' <<<"$add_out")"
[ -n "$id" ] || { echo "smoke: could not parse task id from add output" >&2; exit 1; }

"$binary" list
"$binary" today
"$binary" done "$id"
"$binary" summary

# The dump path must work inside the sandbox too (it refuses otherwise).
"$binary" tui --dump --view list --no-color --size 80x24 >/dev/null

echo "smoke ok — real data untouched (sandbox deleted on exit)"
