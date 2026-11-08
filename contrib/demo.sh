#!/usr/bin/env bash
# Launch the Study Desk TUI against a throwaway copy of the demo dataset
# (73 example tasks across three courses). Everything — HOME, XDG roots,
# the data repo — lives in a temp dir that is removed on exit, so your real
# tasks are never touched.
#
# Usage: make demo   (builds bin/mtm first, or run contrib/demo.sh directly)
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
binary="${1:-$repo_root/bin/mtm}"

if [ ! -x "$binary" ]; then
  echo "building bin/mtm…"
  (cd "$repo_root" && go build -o bin/mtm ./cmd/mtm)
  binary="$repo_root/bin/mtm"
fi

# Sandbox first: HOME and every XDG root land in a temp dir, MTM_SANDBOX=1,
# EXIT trap deletes the sandbox. No exec at the end of this script — the
# trap must get to run.
source "$repo_root/contrib/sandbox-env.sh"
sandbox="$MTM_SANDBOX_DIR"

data="$XDG_DATA_HOME/mtm"
mkdir -p "$data" "$XDG_CONFIG_HOME/mtm"
cp -a "$repo_root/testdata/demo/projects/." "$data/projects/"

# Keep the demo data repo a git repo like a real one (sync stays off).
(
  cd "$data"
  git init -q
  git add -A
  git -c user.name=mtm -c user.email=mtm@example.invalid commit -qm "demo: example tasks"
) >/dev/null

# Seed a running demo timer on the ER-diagram task before launching, so the
# statusline chip, the Focus view and `mtm summary` all have something live
# to show. It keeps running after the TUI closes (stop with mtm timer stop).
"$binary" timer start lt0001

cat > "$XDG_CONFIG_HOME/mtm/config.yaml" <<'EOF'
data_repo: __DATA__
default_project: college/dbms
editor: ""
date_format: "2006-01-02"
week_start: monday
sort:
    - done
    - due
    - priority
    - title
sync:
    auto: false
    debounce_seconds: 2
    remote: origin
pomodoro:
    work_minutes: 25
    short_break_minutes: 5
    long_break_minutes: 15
    long_every: 4
saved_views: []
EOF
sed -i "s|__DATA__|$data|" "$XDG_CONFIG_HOME/mtm/config.yaml"

echo "mtm demo — 73 example tasks in a throwaway sandbox: $sandbox"
echo "(the sandbox is deleted when mtm exits)"
"$binary" tui
