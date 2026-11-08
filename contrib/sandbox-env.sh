# contrib/sandbox-env.sh — the one sandbox every mtm launcher uses.
# Source this; do not execute it. contrib/demo.sh, contrib/smoke.sh and
# contrib/snap.py all source it, so they cannot drift apart.
#
# Creates a throwaway sandbox and points HOME and every XDG root mtm reads
# inside it, then exports MTM_SANDBOX=1 so the internal/paths guard refuses
# anything that still resolves to the real home. Also exports
# OMARCHY_STATE_HOME (theme.go reads it) and MTM_SANDBOX_DIR (the sandbox
# path itself).
#
# An EXIT trap removes the sandbox on exit. Do NOT set your own EXIT trap
# for the sandbox and do NOT `exec` the mtm binary: exec replaces the shell
# without running the EXIT trap and leaks the sandbox. If MTM_SANDBOX_DIR is
# already set it is reused (snap.py sets it via the wrapper it execs through).
#
# XDG_RUNTIME_DIR is intentionally not redirected: nothing in mtm reads it.
# If that changes, add it here — not in the callers.

repo_root="${repo_root:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"

# Reuse the caller's Go caches: the sandbox HOME is fresh, so without these
# every `go build` re-downloads the module graph — and drops ~100MB of
# read-only (0555/0444) files into the sandbox that the EXIT trap cannot
# remove. Build caches are content-addressed compiler artifacts, not user
# data. Computed from the pre-sandbox environment, so this must run before
# HOME is overridden.
if command -v go >/dev/null 2>&1; then
	export GOCACHE="${GOCACHE:-$(go env GOCACHE)}"
	export GOMODCACHE="${GOMODCACHE:-$(go env GOMODCACHE)}"
fi

if [ -z "${MTM_SANDBOX_DIR:-}" ]; then
	MTM_SANDBOX_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mtm-sandbox.XXXXXX")"
fi
export MTM_SANDBOX_DIR
export MTM_SANDBOX=1

export HOME="$MTM_SANDBOX_DIR/home"
export XDG_DATA_HOME="$HOME/.local/share"
export XDG_CONFIG_HOME="$HOME/.config"
export XDG_CACHE_HOME="$HOME/.cache"
export XDG_STATE_HOME="$HOME/.local/state"
export OMARCHY_STATE_HOME="$HOME/.local/state/omarchy"

mkdir -p "$HOME" "$XDG_DATA_HOME" "$XDG_CONFIG_HOME" "$XDG_CACHE_HOME" "$XDG_STATE_HOME" "$OMARCHY_STATE_HOME"

trap 'rm -rf -- "$MTM_SANDBOX_DIR"' EXIT
