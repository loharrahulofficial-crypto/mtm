#!/usr/bin/env bash
# Sandboxed end-to-end smoke test for the mtm binary.
#
# Runs the usual lifecycle against a throwaway data repo in a temp dir.
# HOME and every XDG root point inside the temp dir and MTM_SANDBOX=1 is
# exported, so the internal/paths guard refuses anything that still resolves
# to the real ~/.local/share/mtm, ~/.config/mtm, ~/.cache/mtm or
# ~/.local/state/mtm (it panics instead). The sandbox is deleted on exit.
#
# Usage: make smoke   (builds bin/mtm first, or run contrib/smoke.sh [binary])
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
binary="${1:-$repo_root/bin/mtm}"
if [ ! -x "$binary" ]; then
  (cd "$repo_root" && go build -o bin/mtm ./cmd/mtm)
  binary="$repo_root/bin/mtm"
fi

sandbox="$(mktemp -d "${TMPDIR:-/tmp}/mtm-smoke.XXXXXX")"
cleanup() { rm -rf "$sandbox"; }
trap cleanup EXIT

export HOME="$sandbox/home"
export XDG_CONFIG_HOME="$HOME/.config"
export XDG_DATA_HOME="$HOME/.local/share"
export XDG_CACHE_HOME="$HOME/.cache"
export XDG_STATE_HOME="$HOME/.local/state"
export MTM_SANDBOX=1
export GIT_AUTHOR_NAME=smoke GIT_AUTHOR_EMAIL=smoke@example.invalid
export GIT_COMMITTER_NAME=smoke GIT_COMMITTER_EMAIL=smoke@example.invalid
export GIT_TERMINAL_PROMPT=0
mkdir -p "$HOME"

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
