#!/usr/bin/env bash
# Launch the Study Desk TUI against a throwaway copy of the demo dataset
# (80+ example tasks across three courses, dates relative to today via
# `mtm demo-seed`). Everything — HOME, XDG roots,
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
"$binary" demo-seed "$data/projects"

# Keep the demo data repo a git repo like a real one (sync stays off).
(
  cd "$data"
  git init -q
  git add -A
  git -c user.name=mtm -c user.email=mtm@example.invalid commit -qm "demo: example tasks"
) >/dev/null

# Seed a work-in-progress linked git repo for college/dbms, so the project
# page's GitHub section, the sidebar/Home streak chips and the overview
# sparkline all have something live to show. Commits are dated relative to
# today (12 commits over the last ~24 weeks, two of them yesterday/today
# for the streak) — created inside the sandbox, never outside it.
linkedrepo="$XDG_DATA_HOME/linkedrepo"
mkdir -p "$linkedrepo"
(
  cd "$linkedrepo"
  git init -q -b main
  for i in $(seq 1 12); do
    week_ago=$(date -d "-$((24 - 2 * i)) days" +%F)
    day=""
    if [ "$i" -eq 11 ]; then day=$(date -d "yesterday" +%F); fi
    if [ "$i" -eq 12 ]; then day=$(date +%F); fi
    if [ -z "$day" ]; then day=$week_ago; fi
    echo "demo content $i" > "f$i.txt"
    git add -A
    GIT_AUTHOR_DATE="${day}T10:00:00" GIT_COMMITTER_DATE="${day}T10:00:00" \
      git -c user.name=mtm -c user.email=mtm@example.invalid commit -qm "demo: commit $i"
  done
) >/dev/null
"$binary" project link college/dbms --repo loharrahulofficial-crypto/mtm --path "$linkedrepo" >/dev/null

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
ui:
    ambient_render: sextant
pomodoro:
    work_minutes: 25
    short_break_minutes: 5
    long_break_minutes: 15
    long_every: 4
saved_views: []
EOF
sed -i "s|__DATA__|$data|" "$XDG_CONFIG_HOME/mtm/config.yaml"

echo "mtm demo — 80+ example tasks (dated relative to today) in a throwaway sandbox: $sandbox"
echo "(the sandbox is deleted when mtm exits)"
"$binary" tui
