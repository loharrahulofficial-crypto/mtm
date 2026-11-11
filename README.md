# mtm — Multi-Task Manager

A personal terminal task manager for Omarchy (Arch Linux + Hyprland), built for
college life: courses, exams, assignments, projects and notes.

One Go binary, two interfaces over one shared core:

- **CLI** for quick commands (`mtm add "..."`)
- **TUI** for interactive use (`mtm`)

Tasks are stored as one Markdown file per task, inside a git repo that can
auto-sync between devices. No database.

> **Status: Phase 5 complete (v1-ready).** Phases 1–5 ship: the file store and
> CLI core, smart natural-language add, the TUI, academic features (courses,
> exams, weighted grades, Pomodoro), stats and weekly review, export
> (markdown/CSV/JSON), iCal export and serving, reminders, systemd timers,
> Waybar integration, and shell completions. Ideas/milestones and the dev-log
> landed in the project-tools phase.

## Install

```sh
git clone <this repo> && cd mtm
make install        # installs to ~/.local: bin/mtm, the man page, completions
```

`make install` installs:

- `mtm` → `~/.local/bin/mtm`
- the man page → `~/.local/share/man/man1/` (`man mtm`)
- bash/zsh/fish completions → `~/.local/share/{bash-completion/completions,zsh/site-functions,fish/vendor_completions.d}`

Prefix is configurable (`make install PREFIX=/usr/local`). Uninstall with
`make uninstall`. For a plain binary: `make build` (`bin/mtm`) or
`go install github.com/loharrahulofficial-crypto/mtm/cmd/mtm@latest`.

Requires Go ≥ 1.25 and git on PATH.

## Quick start

```sh
mtm init                                   # creates the data repo + default config
mtm add "buy milk"                         # natural language: dates, !priority, #tags, @project
mtm add "submit report friday !high #urgent @college/dbms"
mtm today                                  # what's due today
mtm                                       # the TUI (0 Home · 1 List · 2 Board · 3 Calendar · 4 Courses · 5 Stats)
mtm done 3db991                            # mark done
mtm summary                                # a two-line status block for your shell
mtm stats                                  # stats for the week
mtm review                                 # the weekly review ritual
```

## Commands, grouped

### Tasks
`mtm add`, `list`, `today`, `week`, `overdue`, `show`, `edit`, `done`, `rm`,
`status`, `sub`, `note`, `search`, `archive`, `view`, `views`, `init`

### Academics — courses, exams and grades
`mtm course` (list/show/edit), `exam`, `grade`, `grades`, `hours`,
`timer`, `pomo`

### Stats and review
`mtm stats` (`--week|--month|--semester|--since DATE`), `review`
(interactive weekly ritual), `summary` (shell status line)

### Incubator and projects
`mtm idea` / `ideas` (capture, list, rate, promote to a project),
`mtm milestone` (`add|list|show|edit|done`; link tasks with `mtm add … --milestone <id>`),
`mtm log` / `logs` (per-project dev-log),
`mtm project link` (read-only GitHub stats: commit sparkline, last commit,
streak; open issue/PR counts via `gh` when authenticated, cached 15 min)

### Import and export
`mtm export` (`--format md|csv|json [--out FILE] [--project X] [--course X] [--since DATE]`),
`mtm ical` (`--out FILE`, `--serve [--bind host:port] [--token SECRET]`,
`--include tasks,exams,milestones`, `--alarm 30m`)

### Shell and bars
`mtm summary`, `mtm waybar` (Waybar `custom/` module JSON), `mtm completion bash|zsh|fish`

### System
`mtm remind` (one-shot reminder check; run it from a timer),
`mtm systemd install|uninstall|status` (user sync + reminder timers),
`mtm version`, `mtm sync`, `mtm tui`, `mtm doctor` (read-only health checks,
`--json` for scripting). `make demo` builds a throwaway 73-task demo sandbox

Run `mtm <command> -h` for a command's own reference and examples.

## Setup

```sh
mtm init                 # creates ~/.local/share/mtm as a git repo + default config at ~/.config/mtm/config.yaml
mtm init --remote <url>  # also wire a git remote (e.g. a private GitHub repo for sync)
```

The data repo auto-commits every change (`add: Buy milk`, `done: ...`).
With a remote configured, each change also pulls `--rebase` and pushes
(best effort: works offline, commits pile up locally). Conflicts are **never
auto-resolved** — sync pauses, `mtm sync --status` shows the state and the
conflicting files; fix them, commit, then `mtm sync`, or `mtm sync --abort`
to drop the pull entirely.

## Multi-device setup (sync over SSH)

`mtm` syncs the data repo (`~/.local/share/mtm`) through any git remote.
The recommended setup is a private GitHub repo reached over a dedicated SSH
host alias with its own deploy key, so `mtm` never touches your personal
GitHub credentials.

```sshconfig
# ~/.ssh/config
Host github-mtm
  HostName github.com
  User git
  IdentityFile ~/.ssh/mtm_data
  IdentitiesOnly yes
```

One-time setup per **account**:

1. `ssh-keygen -t ed25519 -f ~/.ssh/mtm_data -N ""`
2. Add `~/.ssh/mtm_data.pub` as a deploy key with **write access** on the
   private repo that holds the data (repo → Settings → Deploy keys)
3. verify: `ssh -T github-mtm` (expect "Hi <user>/<repo>! You've successfully authenticated…")

Then on each **machine**:

```sh
# first machine (the one with the data already)
git -C ~/.local/share/mtm remote add origin git@github-mtm:<user>/mtm-data.git
git -C ~/.local/share/mtm push -u origin main

# second machine
git clone git@github-mtm:<user>/mtm-data.git ~/.local/share/mtm
mtm init                 # configure as usual; index rebuilds on first open
```

All devices share one data repo; the cache (`~/.cache/mtm/index.json`) is
per-machine and rebuilt automatically.

## Reminders and systemd

`mtm remind` is a one-shot check for tasks whose `reminder` time passed,
plus — when `reminders.due_soon_minutes` is set — tasks due within that
window. It sends `notify-send` and records each task+occurrence in
`~/.local/state/mtm/reminders.json`, so every reminder fires at most once.
Notifications never fire inside `reminders.quiet_hours` (e.g. `22:00-07:00`);
quiet-hours-held reminders fire after the window ends.

```sh
mtm remind --dry-run     # preview, send/record nothing
```

`mtm systemd install` writes two user timers (no changes to the data repo):

- `mtm-sync.timer` → runs `mtm sync` every 15 minutes (plain `mtm sync`; it
  refuses while the data repo is conflicted/paused, per D-003)
- `mtm-remind.timer` → runs `mtm remind` every 5 minutes

and enables them with `systemctl --user enable --now`. Uninstall with
`mtm systemd uninstall`; inspect with `mtm systemd status`. `--dry-run`
prints the plan without writing anything; `MTM_SYSTEMD_USER_DIR` overrides
the unit directory (tests use it; the real `~/.config/systemd/user` is never
touched unless you ask).

## Ideas incubator, milestones and dev-log

Half-formed ideas live in `ideas/` until they prove themselves:

```sh
mtm idea "local-first sync" --tag sync --rating 4
mtm ideas                            # list (seed/exploring/parked live up top)
mtm idea note ide0001 "check CRDT vs oplog"
mtm idea status ide0001 exploring    # seed → exploring → parked → dropped
mtm idea promote ide0001             # → project with todo/doing/done, an initial
                                      #   milestone and a linked starter task list
mtm idea promote ide0001 --template web  # starter tasks from templates/web.md
```

Projects carry milestones; tasks link to them with `--milestone`:

```sh
mtm milestone add college/dbms "v0.1 — study MVP" --target 2026-12-15
mtm add "normalize the orders schema" --project college/dbms --milestone ms0001
mtm milestone list college/dbms      # progress bar per milestone (tasks+subtasks)
mtm milestone done ms0001
```

`mtm log <project>` appends to the project's dev-log
(`projects/<project>/devlog/<YYYY-MM-DD>-<id>.md`):

```sh
mtm log college/dbms "3NF, then lossless joins"
mtm log college/dbms                  # scaffold in $EDITOR (commits prefilled
                                      #   with devlog.suggest_from_commits: true)
mtm log college/dbms --with-timer "evening block"   # attach the running timer
mtm log college/dbms --from-commits   # body prefilled with recent commits
mtm logs college/dbms --since 7d
```

`mtm project link <project> --path ~/Projects/foo [--repo owner/name]` reads
the linked repo read-only: a 12-week commit sparkline, last commit, last 5
subjects and a commit streak (sidebar and Home chips); with `--repo` and an
authenticated `gh` it also shows open issue/PR counts and the latest few
(cached 15 minutes, in the cache dir, never in the data repo).

The project page (`enter` on a project in the sidebar, or `:roadmap` in the
TUI) has four tabs: **Overview** (description, combined progress, velocity,
hours, GitHub section), **Tasks**, **Roadmap** (12-week timeline, milestone
bars with progress, today marker, at-risk/overdue colours) and **Dev-log**.

## Import and export

```sh
mtm export                     # markdown report to stdout (grouped by course)
mtm export --format json --out export.json
mtm export --format csv --project college/dbms
mtm ical                       # RFC 5545 calendar to stdout
mtm ical --out ~/mtm.ics
mtm ical --serve --bind 127.0.0.1:8765          # read-only endpoint
mtm ical --serve --token secret --bind 0.0.0.0   # remote read access (warns loudly)
```

- **Markdown** is a readable report grouped by course: grades, exams, task
  lists (nested subtasks), hours.
- **JSON** carries a top-level `schema_version` (currently `2`), documented
  below in *Export schema*. Since version 2 the `ideas`, `milestones` and
  `devlog` arrays are fully populated.
- **CSV** is a flat task table with stable columns (one row per task;
  subtasks included with `parent` set).
- **iCal** events: timed tasks and exams are timed events (1h duration);
  date-only tasks are all-day events. Milestones are all-day events on their
  target date (`--include tasks,exams,milestones`). UIDs derive from task and
  milestone ids, so re-imports update instead of duplicating. Dates stored
  local-naive export as floating local times; set `ical.timezone` to emit
  `TZID` parameters. `--alarm 30m` adds a VALARM to every event.
- The `--serve` endpoint serves exactly `/mtm.ics` (or `/<token>/mtm.ics`
  with `--token`), GET/HEAD only, read-only, regenerated per request. The
  default bind is loopback; non-loopback prints a loud warning.

## Export schema (version 2)

Version 2 fills the reserved `ideas` and `milestones` arrays and adds a
`devlog` array; `schema_version` bumps 1 → 2 accordingly. Everything else is
unchanged from version 1. Version 1 consumers must tolerate new top-level
keys — upgrade to version 2 for the new sections.

```json
{
  "schema_version": 2,
  "generated_at": "2026-11-09T10:43",
  "filters": { "project": "", "course": "", "since": "" },
  "tasks": [ ... ],        // id, title, project, status, priority, kind,
                           // tags, due, reminder, recurrence, parent,
                           // created, updated, completed, estimate_minutes,
                           // time_logged_minutes, weight, score, score_max, body
  "courses": [ ... ],      // name, code, title, semester, credits,
                           // target_grade, color, done, total, hours_minutes,
                           // grade_standing_pct, grade_current_pct
  "exams": [ ... ],        // task_id, title, course, due, weight,
                           // topics_done, topics_total
  "grades": [ ... ],       // task_id, title, kind, course, weight, score,
                           // score_max, pct, scored, points
  "hours": [ ... ],        // course, logged_minutes, week_logged_minutes
  "totals": { "tasks": 73, "completed": 40, "hours_minutes": 620 },
  "ideas": [],             // since v2: id, title, status, tags, created,
                           // updated, rating, links, promoted_to, body
  "milestones": [],        // since v2: id, title, project, target, status,
                           // description, created, updated, done, total,
                           // frac, body — progress derived from linked tasks
  "devlog": []             // since v2: id, title, project, mood,
                           // time_spent_minutes, tags, created, body
}
```

Round-trip by `mtm export --format json`; the tests validate the structure
against this schema.

## Configuration reference

`~/.config/mtm/config.yaml`. Every key is optional; defaults are shown.
`mtm init` writes the generated default file (plain YAML, no comments).

```yaml
data_repo: ~/.local/share/mtm   # where the data lives (git repo)
default_project: inbox          # where new tasks land without a project:
editor: ""                      # $EDITOR, else $VISUAL, else vi
date_format: "2006-01-02"       # reserved (not yet wired)
week_start: monday              # or "sunday"

sort:                           # task-list sort, applied in order
  - done                        # open first
  - due
  - priority
  - title

board:
  done_days: 14                 # board Done column window (0 = all)

heatmap:
  weeks: 26                     # completion heatmap width (clamped 4–52)

sync:
  auto: true                    # sync after each change
  debounce_seconds: 2
  remote: origin

pomodoro:
  work_minutes: 25
  short_break_minutes: 5
  long_break_minutes: 15
  long_every: 4

saved_views: []                 # [{name: ..., filter: ...}]

keys: {}                        # TUI key overrides: action -> key(s), e.g. quit: Q

ui:
  animations: true              # 400ms animation cap; honors reduce-motion
  icons: nerd                   # nerd | unicode | none
  density: comfortable          # comfortable | compact (compact drops the
                                # blank line between a card's title and badges)
  semester_start: ""            # "2026-09-01" — enables the brand-header week chip
  semester_weeks: 16
  ambient: both                 # off | splash | sidebar | home | both — ambient
                                # scenes (both = splash + sidebar window;
                                # sidebar = framed window below the sidebar
                                # list; home = larger panel in Home free
                                # space, hidden while the window is on;
                                # Focus + reward ride on any non-off value)
  ambient_sidebar: true         # framed window below the sidebar list (≥ 8 rows,
                                # collapses on short terminals); toggle in :ambient
  scene: auto                   # auto (by local hour) | random | night |
                                # bonsai | ocean | rain | koi | snow | campfire
  splash_seconds: 3.0           # startup splash length
  reward_animation: true        # 1.5s scene burst when a task is completed
                                # (rate-limited to 3 per minute)
                                # tab switches dissolve between the old
                                # and new view over ~150ms (skipped with
                                # reduced motion)
  ambient_fps: 60               # animation cap (max 60)
  ambient_render: auto          # auto | halfblock (▀▄ 2x vertical) | braille | ascii |
                                # quadrant (2x2 subcells, square pixels) |
                                # sextant (2x3 subcells, mosaic glyphs)
  rain_lightning: false         # rare lightning in the rain scene
  truecolor: auto               # auto | on | off — 24-bit color; auto =
                                # truecolor unless the terminal shows
                                # positive evidence of limited palette
                                # (TERM=linux|dumb, NO_COLOR, Apple Terminal,
                                # TERM=screen* without COLORTERM);
                                # MTM_TRUECOLOR=1|0 overrides for a run
  splash: on                    # on | off | once-a-day — once-a-day shows
                                # the splash only on the first launch of
                                # each local day (stamp: ~/.local/state/mtm/
                                # splash.stamp); --no-splash /
                                # MTM_NO_SPLASH=1 suppresses for a run
  ambient_dither: on            # only relevant on limited terminals: nearest
                                # xterm-256 entry per color, optionally
                                # ordered-dithered between the two nearest
                                # (never per-channel)

stats:
  streak_min_minutes: 15        # a day is active with ≥1 completion OR this many logged minutes
                                # 0 = only completions count

ical:
  timezone: ""                  # IANA name (e.g. "Europe/London"); empty = floating local times

devlog:
  suggest_from_commits: false   # prefill `mtm log <project>` $EDITOR scaffold with recent commits

reminders:
  due_soon_minutes: 0           # >0: also notify for tasks due within N minutes
  quiet_hours: ""               # "22:00-07:00" (spans midnight) — empty disables
```

## Data layout

```
~/.local/share/mtm/                # the data repo (syncs)
├── projects/<project>/<id>-<slug>.md
├── projects/<project>/project.yaml   # statuses, color, course block, github/repo_path (read-only link)
├── projects/<project>/milestones/<id>-<slug>.md
├── projects/<project>/devlog/<YYYY-MM-DD>-<id>.md
├── ideas/<id>-<slug>.md               # incubator items (sync-safe, one per file)
├── archive/<project>/            # mtm archive moves done tasks here
├── templates/<name>.md           # optional starter-task templates for `mtm idea promote`
├── .mtm-data                     # repo marker: UUID, creation date, kind: real|demo
└── .git/

~/.cache/mtm/index.json           # index; never committed; safe to delete
~/.cache/mtm/github.json          # gh issue/PR counts cache (15-minute TTL; never in the data repo)
~/.local/state/mtm/timer.json     # timer/Pomodoro state (outside the repo)
~/.local/state/mtm/timer.lock
~/.local/state/mtm/reminders.json # sent reminders (dedup; outside the repo)
~/.local/share/mtm/.git/mtm-sync-state.json  # sync paused flag (inside the data repo's .git directory)
```

Each task is one Markdown file:

```markdown
---
id: 3db991
title: Submit DBMS assignment 2
project: college/dbms
status: doing
priority: high          # low | medium | high | urgent
tags: [assignment, exam]
due: 2026-11-14T23:59   # local-naive; date-only also allowed
kind: assignment        # task | assignment | exam | study | project
estimate_minutes: 90
time_logged_minutes: 45
weight: 15              # grade weight %, for assignments and exams
parent: abc123          # set for subtasks
milestone: ms0001       # set when linked to a milestone
completed: 2026-11-09T18:30   # set when done
---
Free-form Markdown notes here.
```

Ideas and milestones are Markdown files too:

```markdown
---                     # ideas/<id>-<slug>.md
id: ide0001
title: Local-first sync engine
status: exploring        # seed | exploring | parked | promoted | dropped
tags: [sync, go]
created: 2026-10-01T10:00
updated: 2026-11-02T10:00
rating: 5                # 1-5; 0/absent = unrated
links: []
promoted_to: college/dbms   # set by `mtm idea promote`
---

---                     # projects/<project>/milestones/<id>-<slug>.md
id: ms0001
title: v0.1 — study MVP
target: 2026-12-15      # local-naive date
status: active          # planned | active | done | missed
description: Course companion features
created: 2026-10-20T09:00
updated: 2026-11-05T10:00
---

---                     # projects/<project>/devlog/<YYYY-MM-DD>-<id>.md
id: dl00001
title: Week 2 wrap
mood: flow              # optional
time_spent_minutes: 90  # optional (--with-timer attaches the running timer)
tags: [study]
created: 2026-11-04T21:30
---

TUI panels derive their colors from the active Omarchy theme at runtime;
themes, icons and layout scale changes apply immediately with no restart.

## Shell startup summary

`mtm summary` prints a two-line block (needs `< 50ms`; ~17ms measured):

```text
3 today · 2 overdue · exam in 13d: dbms final exam
inbox 2/5 (40%) · college/dbms 1/4 (25%)
```

Add to your shell rc (`.bashrc` / `.zshrc`):

```sh
mtm summary 2>/dev/null || true
```

## Waybar module

`mtm waybar` prints one JSON line for a Waybar `custom/` module. Text is
bar-sized: today's open task count, plus an exam chip (`✦<days>`) within
30 days and an active-timer chip. Class priority: `overdue` > `exam` (≤3d) >
`timer` > `normal`.

`~/.config/waybar/config.jsonc` (Omarchy's Waybar config):

```jsonc
"custom/mtm": {
  "exec": "mtm waybar",
  "return-type": "json",
  "interval": 30,
  "format": "{}",
  "tooltip": true
}
```

and add `"custom/mtm"` to the `modules-right` list. `style.css`:

```css
#custom-mtm.overdue { color: @red; }
#custom-mtm.exam { color: @yellow; }
#custom-mtm.timer { color: @blue; }
#custom-mtm.normal { color: @foreground; }
```

## Hyprland: floating quick-add

A one-shot terminal for `mtm add`, sized small and focused:

```ini
# ~/.config/hypr/hyprland.conf
bind = $mainMod, T, exec, hyprctl dispatch exec [float;size 800 400;center] "alacritty --class mtm-quick -e mtm add"
windowrulev2 = float, class:^(mtm-quick)$
windowrulev2 = size 800 400, class:^(mtm-quick)$
windowrulev2 = center, class:^(mtm-quick)$
```

(Omarchy ships alacritty; swap in foot if you use it.)

## Shell completions

Installed by `make install`. To do it by hand:

```sh
mtm completion bash > ~/.local/share/bash-completion/completions/mtm
mtm completion zsh  > "${fpath[1]}/_mtm"        # then: autoload -U compinit && compinit
mtm completion fish > ~/.config/fish/completions/mtm.fish
```

## The TUI in short

`mtm` opens the Study Desk dashboard. Views: `0` Home · `1` List · `2` Board ·
`3` Calendar · `4` Courses · `5` Stats · `6` Ideas (all reachable with `tab`).

- `a` add · `A` add subtask · `x` done/un-done
- `/` filter · `ctrl+p` palette · `?` keybindings
- `F` focus mode (pomodoro on the selected task)
- ambient scenes (auto by time of day, or `ui.scene:`): `alt+s` cycle scenes
  · `:scene <name>` switch · `:ambient` opens the live preview + mood picker
  · `--no-ambient` disables for one run · `--no-splash` skips the splash
- board: `h`/`l` move between columns · `D` toggle the Done filter
- calendar: `h`/`l` move the day · `J`/`K` change months
- courses: `g` record a grade · `:exam <course>` in the palette to add an exam
- ideas: `a` capture · `r` rate 1–5 · `p` promote to a project · `s` cycle
  status · `m`/`M` notes
- project page (`enter` on a project in the sidebar, or `:roadmap`):
  `[`/`]` switch tabs (Overview · Tasks · Roadmap · Dev-log) · `h`/`l` pan
  the roadmap timeline · `j`/`k` move · `enter` opens the selected task ·
  `esc` closes
- help overlay shows every binding (`?`)

All keybindings are overridable via `keys:` in the config; see `?` inside the
TUI.

## Troubleshooting

**The TUI/index looks stale or weird after moving machines.** The index
is a local cache; rebuild it: `rm ~/.cache/mtm/index.json` and run any
command — it reindexes from the files.

**`mtm sync` keeps saying "sync is paused".** A pull conflicted. Fix the
files, `mtm sync --status` shows what's unmerged, then
`mtm sync --continue` (or `--abort` to drop the pull and get writing again).
Sync never auto-resolves.

**Suspect demo/fixture data in the real repo, or something feels off.**
Run `mtm doctor` — it checks the marker (`.mtm-data`), the git repo, the
remote, your git identity, every task file's frontmatter, demo-corpus
ids/commit messages (e.g. `dn0055`, `lt0001`, `od0001` — only flagged when
the repo marker says `kind: real`), a paused/conflicted sync and stale
timer state. `mtm doctor --json` for scripting; exits non-zero on any
failure. `mtm demo-seed` itself only runs inside a sandbox
(`MTM_SANDBOX=1` + temp dirs) and refuses repos whose marker says
`kind: real`.

**Waybar shows nothing.** Check `mtm waybar` by hand; it prints one JSON
line or an error payload. The module needs `"custom/mtm"` in the waybar
modules list and `"return-type": "json"`.

**`mtm remind` never fires.** Reminders are one-shot per task+occurrence
in `~/.local/state/mtm/reminders.json` — check whether it's already there
(delete the entry to re-fire), and check `quiet_hours` in the config.
`--dry-run` shows what would fire right now.

**A timer shows time I didn't work.** `mtm timer stop` banks the elapsed
time capped at the session; nothing starts by itself. (Focus mode shows
the ring at 00:00 until `space`.)

**Where does mtm write outside the repo?** Only `~/.cache/mtm` (index) and
`~/.local/state/mtm` (timer, reminders). Your tasks live in the git repo
and in nowhere else.

## Development

```sh
make test vet fmt       # go test ./..., go vet, gofmt check
make build              # bin/mtm
make man                # man pages into ./man (via cobra docs)
make install            # binary + man page + completions into ~/.local
make demo               # throwaway sandbox with example data (never your real data)
make smoke              # full CLI lifecycle in a throwaway sandbox
```

## Measuring performance (`make perf`)

`make perf` runs the TUI under a pseudo-terminal in a **throwaway sandbox**
(`HOME`, every XDG root and `MTM_SANDBOX=1` pointed at a temp dir) and
reports, per scenario, the user+sys CPU time over the run (from
`wait4`), the paint rate (alt-screen flushes counted as `ESC[H`), bytes
per paint, allocations per paint (from the `--profile` stats file) and
allocations per `View()` rebuild — plus the top 15 functions from a CPU
profile (`go tool pprof -top`).

```sh
make perf                                   # all four scenarios, ~6.5 min
make perf PERF_ARGS="--duration 10 home-ambient-off"
python3 contrib/perf.py --duration 30 home-ambient-12fps
python3 contrib/perf.py --help              # scenarios, flags, environment
```

The four scenarios (120x40 by default):

| scenario | what it measures | shipped target |
|---|---|---|
| `home-ambient-off` | Home idle, ambient off | ≤ 2% of a core |
| `home-ambient-12fps` | Home + sidebar window (label kept for comparability; the Home/sidebar cap is 8 FPS since the idle-policy change) | ≤ 20% of a core |
| `focus-24fps` | Focus mode behind a running Pomodoro | ≤ 30% of a core |
| `idle-ambient-5m` | Ambient on, untouched; defaults to 300 s | ≤ 5% of a core |

The harness answers the terminal's capability queries (`OSC 11` and
`DSR`) like a real terminal, so colours render through the truecolor
path instead of an ASCII fallback. Profiling refuses to run outside the
sandbox (`MTM_PPROF=cpu:/path/to.pprof` or `mtm tui --profile FILE`).

To measure on your own hardware, run it on the machine you care about.
The numbers in this repo's commit history were taken on an i3-1115G4 and
cover the `mtm` process only (the pty reader is off the clock); your
numbers will differ on a faster or busier machine.

Layering: `cmd/mtm` → `internal/cli` (Cobra) / `internal/tui` (Bubble Tea)
→ `internal/actions` → `internal/core` → `internal/store`.
The TUI has no business logic.

Commit messages in the data repo are deliberately free of tooling noise;
user data belongs to the user.

## Tagging v1.0.0

The `v1.0.0` tag is intentionally **not** applied yet. When you confirm the
release, tag it yourself:

```sh
git tag -a v1.0.0 -m "v1.0.0"
git push --tags
```

(and `git describe --tags` will produce `v1.0.0` for `make` builds.)
                                                                                                                                                                                                                                                                                                                  