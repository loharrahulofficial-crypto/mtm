# mtm — Multi-Task Manager

A personal terminal task manager for Omarchy (Arch Linux + Hyprland), built for
college life: courses, exams, assignments, projects and notes.

One Go binary, two interfaces over one shared core:

- **CLI** for quick commands (`mtm add "..."`)
- **TUI** for interactive use (`mtm`)

Tasks are stored as one Markdown file per task, inside a git repo that can
auto-sync between devices. No database.

> **Status: v1.0.0 released.** Phase 5 is complete: the file store and CLI
> core, smart natural-language add, the TUI, academic features (courses, exams,
> weighted grades, Pomodoro), stats and weekly review, export
> (markdown/CSV/JSON), iCal export and serving, reminders, systemd timers,
> Waybar integration, shell completions, ideas/milestones/dev-log, and the
> ambient scene engine. See [Commands](#commands) and
> [Configuration reference](#configuration-reference).

## Requirements

- **Go ≥ 1.27** to build from source (`go.mod` pins `go 1.27`), and **git** on
  `PATH`. A prebuilt binary needs neither — only `git`.
- **Linux.** Developed on Omarchy (Arch + Hyprland); the TUI, notifications and
  desktop integrations assume a Linux desktop session.
- A **truecolor terminal** for the ambient scenes. Detection is automatic and
  falls back to xterm-256 (see `ui.truecolor`); Ghostty, kitty, WezTerm,
  alacritty, foot and tmux all work.
- **Nerd Font** (optional). `ui.icons: nerd` is the default; set `unicode` or
  `none` if you do not use one.
- `notify-send` (optional) — desktop reminders. Without it `mtm remind` still
  records state and prints, but sends nothing.
- `gh` (optional) — read-only open issue/PR counts for linked repositories.
- `$EDITOR` (optional) — `mtm edit`, `mtm log` and `mtm idea` fall back to
  `$VISUAL`, then `vi`.

Runtime needs nothing beyond the `mtm` binary and `git`; `mtm init` creates the
data repo and default config.

## Install

```sh
git clone git@github.com:loharrahulofficial-crypto/mtm.git && cd mtm
make install        # installs to ~/.local: bin/mtm, the man page, completions
```

`make install` installs:

- `mtm` → `~/.local/bin/mtm`
- the man page → `~/.local/share/man/man1/` (`man mtm`)
- bash/zsh/fish completions →
  `~/.local/share/{bash-completion/completions,zsh/site-functions,fish/vendor_completions.d}`

Prefix is configurable (`make install PREFIX=/usr/local`). Uninstall with
`make uninstall`. For a plain binary: `make build` (`bin/mtm`) or
`go install github.com/loharrahulofficial-crypto/mtm/cmd/mtm@latest`.

## Quick start

```sh
mtm init                                   # creates the data repo + default config
mtm add "buy milk"                         # natural language: dates, !priority, #tags, @project
mtm add "submit report friday !high #urgent @college/dbms"
mtm today                                  # what's due today
mtm                                        # the TUI (0 Home · 1 List · 2 Board · 3 Calendar ·
                                           #   4 Courses · 5 Stats · 6 Ideas)
mtm done 3db991                            # mark done
mtm summary                                # a two-line status block for your shell
mtm stats                                  # stats for the week
mtm review                                 # the weekly review ritual
```

First run? `make demo` launches the TUI against a throwaway 80+ task sandbox —
it never touches your real data.

## How it works

### Architecture

```
cmd/mtm
  ├── internal/cli      Cobra command tree (the CLI)
  └── internal/tui      Bubble Tea model/view (the TUI)
        ↓
  internal/actions      use cases (add, done, grade, sync, remind, …)
        ↓
  internal/core         domain models and rules (Task, Project, grades, dates)
        ↓
  internal/store        one Markdown file per task, git commits, index cache
```

Supporting packages: `config` (YAML config + defaults), `paths` (every
directory, with the sandbox guard), `parse` (natural-language add), `dates`,
`text`, `theme` (live Omarchy colors), `caps` (terminal capabilities),
`ambient` (scene engine), `gauges` (charts/bars), `ical`, `sync`, `doctor`,
`marker` (`.mtm-data`), `demo`, `perf`, `clock`.

The CLI and TUI contain no business logic; both call `internal/actions` and
`internal/core`.

### Storage model

- One task = one Markdown file with YAML frontmatter; everything below the
  fence is free-form notes.
- Files live under `projects/<project>/`, with `ideas/`, per-project
  `milestones/` and `devlog/`, `archive/<project>/` and `templates/` beside
  them. See [Data layout](#data-layout).
- Every change is a git commit (`add: Buy milk`, `done: …`). With a remote, each
  change also pulls `--rebase` and pushes, best effort.
- The index (`~/.cache/mtm/index.json`) is a local cache; delete it and the next
  command rebuilds it from the files.
- Config is `~/.config/mtm/config.yaml`; runtime state (timer, reminders,
  splash stamp) is `~/.local/state/mtm/`. Neither is required to be in the repo.

### Sync and conflicts

With a remote configured, `mtm` auto-syncs after each change (configurable via
`sync.auto`/`sync.debounce_seconds`). Sync **never auto-resolves conflicts**: it
pauses, `mtm sync --status` shows the state and the conflicting files, and you
either fix them and run `mtm sync --continue`, or `mtm sync --abort` to drop the
pull. The paused flag lives in the repo's `.git` directory
(`mtm-sync-state.json`).

### Configuration and environment

Config is plain YAML read over built-in defaults; every key is optional. All
four directories resolve through one table (highest wins):

| directory | override | XDG variable | default |
|---|---|---|---|
| data | `MTM_DATA_DIR` | `XDG_DATA_HOME` | `~/.local/share/mtm` |
| config | `MTM_CONFIG_DIR` | `XDG_CONFIG_HOME` | `~/.config/mtm` |
| cache | — | `XDG_CACHE_HOME` | `~/.cache/mtm` |
| state | `MTM_STATE_DIR` | `XDG_STATE_HOME` | `~/.local/state/mtm` |

Useful environment switches: `MTM_SANDBOX=1` (refuse the real roots; tests and
`make demo` rely on it), `MTM_NO_AMBIENT=1`, `MTM_NO_SPLASH=1`,
`MTM_TRUECOLOR=1|0`, `NO_MOTION=1`, `MTM_SYSTEMD_USER_DIR`,
`MTM_PPROF=cpu:FILE`, `MTM_GH` (override the `gh` binary), `NO_COLOR`.

### The TUI

`mtm` opens the Study Desk dashboard. Views: `0` Home · `1` List · `2` Board ·
`3` Calendar · `4` Courses · `5` Stats · `6` Ideas (all reachable with `tab`).
Panels derive their colors from the active Omarchy theme at runtime, so theme
changes apply immediately. See [The TUI](#the-tui).

## Commands

Run `mtm <command> -h` for any command's own reference and examples. Global
flags on every command: `--no-ambient`, `--no-splash`.

### Tasks

| command | what it does |
|---|---|
| `mtm add <title...>` | quick add; natural language + flags |
| `mtm list` | list tasks (hides done unless `--all`) |
| `mtm today` / `week` / `overdue` | due today / next 7 days / past due |
| `mtm show <id>` | show one task (id or unique prefix) |
| `mtm edit <id>` | edit the task file in `$EDITOR` |
| `mtm done <id>` | mark done (the project's first done status) |
| `mtm rm <id>` | remove a task (`--force` to skip confirmation) |
| `mtm status <id> <status>` | set a status that exists in the project workflow |
| `mtm sub <parent> <title...>` | create a subtask |
| `mtm note <id> <text...>` | append a line of notes |
| `mtm search <query...>` | full-text search over title, notes, tags |
| `mtm archive` | move done tasks to `archive/<project>/` (`--older-than 30d`) |
| `mtm view <name>` / `views` | run a built-in or saved view / list views |
| `mtm init` | create the data repo + default config (`--remote URL`) |

`mtm add` understands dates and times (`tomorrow`, `friday 6pm`, `next monday`,
`in 3 days`, `14 nov`), `!priority` (`!low !med !high !urgent`), `#tags`,
`@project`, `~estimate` (`~2h`, `~45m`) and recurrence (`every week`, `daily`,
`every monday`). Flags always win over parsed values; leftover words become the
title.

```
mtm add "submit report friday 6pm !high #college @dbms"
mtm add "Submit report" --due 2026-11-14 --priority high
mtm add "read chapter 4" --kind study --weight 10 --milestone ms0001
```

`add` flags: `--due`, `--kind task|assignment|exam|study|project`,
`--milestone`, `--priority`, `--project`/`-p`, `--tag`/`-t`, `--weight`.
`list` flags: `--all`, `--archived`, `--due today|week|overdue`, `--json`,
`--priority`, `--project`, `--status`, `--tag`, `--weighted`.

### Academics — courses, exams, grades, time

- `mtm course add|edit|list|show|syllabus` — courses are projects with a course
  block in `project.yaml`. `syllabus` turns a `.md`/`.yaml` syllabus (or a
  `templates/<name>` file) into chapter tasks and subtopic subtasks, reusing
  existing entries on re-run.
- `mtm exam add|list|plan` — exams are `kind:exam` tasks tied to a course.
  `plan` generates one `kind:study` task per exam topic, spread before the exam
  (`--est` minutes per session).
- `mtm grade <id> <score>` — record a score (`--max`, `--weight`).
- `mtm grades` — per-course weighted totals and what you need to hit the target
  (`--course`, `--target`, `--json`).
- `mtm hours` — logged time per course (`--week`, `--course`, `--json`).
- `mtm timer start|status|stop` — stopwatch on a task; `stop` accrues into
  `time_logged_minutes`.
- `mtm pomo [id]` — foreground Pomodoro (25m work / 5m break, long break every
  4th round; configurable). Ctrl-C stops early and logs accrued work minutes.

### Stats and review

- `mtm stats` — completed tasks, on-time rate, hours by course, busiest day,
  estimate accuracy, per-course breakdown, current/longest active-day streaks.
  Scopes: `--week` (default, trailing 7 days), `--month`, `--semester`
  (since `ui.semester_start`), `--since DATE`.
- `mtm review` — interactive weekly ritual: what finished, what slipped, the
  next 7 days; overdue items handled one by one (`r`eschedule, `a`rchive,
  `k`eep, `q`uit).
- `mtm summary` — compact two-line status block for shell startup (< 50 ms).

### Ideas, projects, milestones, dev-log

- `mtm idea [title...]` — capture an idea (`--tag`, `--rating`); with no
  argument, `$EDITOR` opens a scaffold.
- `mtm idea show|edit|status|rate|note|rm|promote <id>` — manage ideas.
  `promote` turns one into a project with an initial milestone and starter tasks
  (`--template <name>`).
- `mtm ideas` — list (`--status`, `--tag`).
- `mtm project add|edit|list|rm|link` — projects and their custom statuses.
  `link --path <repo> [--repo owner/name]` reads a git repo read-only: 12-week
  commit sparkline, last commit, last 5 subjects, commit streak; with `--repo`
  and an authenticated `gh`, open issue/PR counts (cached 15 min).
- `mtm milestone add|list|show|edit|done` — milestones; link tasks with
  `mtm add … --milestone <id>`.
- `mtm log <project> [text...]` / `mtm logs <project>` — per-project dev-log
  (`--with-timer`, `--from-commits`, `--mood`, `--tag`; `logs --since 7d`).

```
mtm idea "local-first sync" --tag sync --rating 4
mtm idea promote ide0001 --template web
mtm milestone add college/dbms "v0.1 — study MVP" --target 2026-12-15
mtm add "normalize the orders schema" --project college/dbms --milestone ms0001
mtm log college/dbms "3NF, then lossless joins"
```

### Import and export

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
  below in [Export schema](#export-schema-version-2). Since version 2 the
  `ideas`, `milestones` and `devlog` arrays are fully populated.
- **CSV** is a flat task table with stable columns (one row per task; subtasks
  included with `parent` set).
- **iCal** events: timed tasks and exams are timed events (1h duration);
  date-only tasks are all-day events. Milestones are all-day events on their
  target date (`--include tasks,exams,milestones`). UIDs derive from task and
  milestone ids, so re-imports update instead of duplicating. Dates stored
  local-naive export as floating local times; set `ical.timezone` to emit
  `TZID` parameters. `--alarm 30m` adds a VALARM to every event.
- The `--serve` endpoint serves exactly `/mtm.ics` (or `/<token>/mtm.ics` with
  `--token`), GET/HEAD only, read-only, regenerated per request. The default
  bind is loopback; non-loopback prints a loud warning.

### Shell, bars and system

- `mtm summary` — shell status line (see below).
- `mtm waybar` — one JSON line for a Waybar `custom/` module.
- `mtm completion bash|zsh|fish` — completion scripts.
- `mtm remind` — one-shot reminder check (`--dry-run`).
- `mtm systemd install|uninstall|status` — user sync + reminder timers.
- `mtm sync` (`--status`, `--continue`, `--abort`, `--json`).
- `mtm doctor` — read-only health checks (`--json`).
- `mtm tui` — launch the TUI explicitly (bare `mtm` does the same on a TTY).
- `mtm version` — embedded version, commit and build date.

## Configuration reference

`~/.config/mtm/config.yaml`. Every key is optional; defaults are shown. Every
value is validated on load — a typo is reported, not silently ignored.

```yaml
data_repo: ~/.local/share/mtm   # where the data lives (git repo)
default_project: inbox          # where new tasks land without a project
editor: ""                      # $EDITOR, else $VISUAL, else vi
date_format: "2006-01-02"       # parsed for completeness; not yet wired into display
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
  animations: true              # caps animations; honors reduced motion
  icons: nerd                   # nerd | unicode | none
  density: comfortable          # comfortable | compact (compact drops the
                                # blank line between a card's title and badges)
  semester_start: ""            # "2026-09-01" — enables the brand-header week chip
  semester_weeks: 16

  # Ambient scenes: master switch + placement.
  ambient: both                 # off | splash | sidebar | home | both
                                # both = splash + sidebar window;
                                # sidebar = framed window below the sidebar
                                # list; home = larger panel in Home free
                                # space, hidden while the window is on;
                                # Focus + reward ride on any non-off value
  ambient_sidebar: true         # framed window below the sidebar list (≥ 8 rows,
                                # collapses on short terminals); toggle in :ambient
  scene: auto                   # auto (by local hour) | random | night |
                                # bonsai | ocean | rain | koi | snow | campfire |
                                # mountains | river | meteor
  splash_seconds: 3.0           # startup splash length (0–30)
  splash: on                    # on | off | once-a-day — once-a-day shows it
                                # only on the first launch of each local day
                                # (stamp: ~/.local/state/mtm/splash.stamp)
  reward_animation: true        # 1.5s scene burst when a task is completed
                                # (rate-limited to 3 per minute)
  ambient_fps: 60               # animation cap (1–60)
  ambient_idle: slow            # slow | pause | off — slow drops Home/sidebar
                                # to ~4 FPS after 30s idle and freezes after
                                # 5 min; Focus/splash exempt while the timer runs
  ambient_render: auto          # auto | halfblock (▀▄ 2x vertical) | braille | ascii |
                                # quadrant (2x2 subcells, square pixels) |
                                # sextant (2x3 subcells, mosaic glyphs)
  ambient_dither: on            # limited terminals only: nearest xterm-256 entry,
                                # optionally ordered-dithered between the two nearest
  rain_lightning: false         # rare lightning in the rain scene
  truecolor: auto               # auto | on | off — 24-bit color; auto detects
                                # from the environment; MTM_TRUECOLOR=1|0 overrides
  reduced_motion: false         # no crossfades/dissolves; also honors NO_MOTION=1
  wordmark: caps                # caps ("MTM") | lower ("mtm") splash wordmark

stats:
  streak_min_minutes: 15        # a day is active with ≥1 completion OR this many
                                # logged minutes; 0 = only completions count

ical:
  timezone: ""                  # IANA name (e.g. "Europe/London"); empty = floating local times

devlog:
  suggest_from_commits: false   # prefill `mtm log <project>` with recent commits

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
~/.local/state/mtm/splash.stamp   # once-a-day splash marker
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
score: 88               # set by `mtm grade`
score_max: 100          # default 100
parent: abc123          # set for subtasks
milestone: ms0001       # set when linked to a milestone
completed: 2026-11-09T18:30   # set when done
---

Free-form Markdown notes here.
```

Ideas, milestones and dev-log entries are Markdown files too:

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
```

TUI panels derive their colors from the active Omarchy theme at runtime;
themes, icons and layout changes apply immediately with no restart.

## The TUI

`mtm` opens the Study Desk dashboard. Views: `0` Home · `1` List · `2` Board ·
`3` Calendar · `4` Courses · `5` Stats · `6` Ideas (all reachable with `tab`).

- `a` add · `A` add subtask · `x` done/un-done
- `/` filter · `ctrl+p` palette · `?` keybindings (scrollable)
- `F` focus mode (Pomodoro on the selected task)
- ambient scenes (auto by time of day, or `ui.scene:`): `alt+s` cycle scenes ·
  `:scene <name>` switch · `:ambient` opens the live preview + settings (save
  writes the managed `ui.*` keys to the config) · `--no-ambient` disables for
  one run · `--no-splash` skips the splash
- board: `h`/`l` move between columns · `D` toggle the Done filter
- calendar: `h`/`l` move the day · `J`/`K` change months
- courses: `g` record a grade · `:exam <course>` in the palette to add an exam
- ideas: `a` capture · `r` rate 1–5 · `p` promote to a project · `s` cycle
  status · `m`/`M` notes
- project page (`enter` on a project in the sidebar, or `:roadmap`):
  `[`/`]` switch tabs (Overview · Tasks · Roadmap · Dev-log) · `h`/`l` pan the
  roadmap timeline · `j`/`k` move · `enter` opens the selected task · `esc`
  closes
- help overlay shows every binding (`?`)

All keybindings are overridable via `keys:` in the config; see `?` inside the
TUI.

### Ambient scenes

Ten deterministic scenes: `night` (starfield, real moon phase, aurora),
`bonsai`, `ocean`, `rain` (cabin), `koi`, `snow`, `campfire`, `mountains`
(dusk valley), `river` (golden hour) and `meteor` (shower). `ui.scene: auto`
picks by local hour: 5–8 mountains, 8–15 ocean, 15–17 river, 17–20 rain,
20–23 night, 23–5 meteor.

Scenes appear as the startup splash, the framed sidebar window, the Focus
backdrop and the task-complete reward burst. `ui.ambient_render` picks the
raster mode: `auto` (halfblock), `halfblock`, `braille`, `ascii`, `quadrant`
(2×2 subcells, square pixels) or `sextant` (2×3 subcells, mosaic glyphs).
`ui.ambient_idle` throttles idle CPU; `ui.ambient_fps` caps the frame rate.

## Sync and multi-device

`mtm` syncs the data repo (`~/.local/share/mtm`) through any git remote. The
recommended setup is a private GitHub repo reached over a dedicated SSH host
alias with its own deploy key, so `mtm` never touches your personal GitHub
credentials.

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

`mtm remind` is a one-shot check for tasks whose `reminder` time passed, plus —
when `reminders.due_soon_minutes` is set — tasks due within that window. It
sends `notify-send` and records each task+occurrence in
`~/.local/state/mtm/reminders.json`, so every reminder fires at most once.
Notifications never fire inside `reminders.quiet_hours` (e.g. `22:00-07:00`);
quiet-hours-held reminders fire after the window ends.

```sh
mtm remind --dry-run     # preview, send/record nothing
```

`mtm systemd install` writes two user timers (no changes to the data repo):

- `mtm-sync.timer` → runs `mtm sync` every 15 minutes (plain `mtm sync`; it
  refuses while the data repo is conflicted/paused)
- `mtm-remind.timer` → runs `mtm remind` every 5 minutes

and enables them with `systemctl --user enable --now`. Uninstall with
`mtm systemd uninstall`; inspect with `mtm systemd status`. `--dry-run` prints
the plan without writing anything; `MTM_SYSTEMD_USER_DIR` overrides the unit
directory (tests use it; the real `~/.config/systemd/user` is never touched
unless you ask).

## Shell integration

### Startup summary

`mtm summary` prints a two-line block (needs `< 50ms`; ~17ms measured):

```text
3 today · 2 overdue · exam in 13d: dbms final exam
inbox 2/5 (40%) · college/dbms 1/4 (25%)
```

Add to your shell rc (`.bashrc` / `.zshrc`):

```sh
mtm summary 2>/dev/null || true
```

### Waybar module

`mtm waybar` prints one JSON line for a Waybar `custom/` module. Text is
bar-sized: today's open task count, plus an exam chip (`✦<days>`) within 30 days
and an active-timer chip. Class priority: `overdue` > `exam` (≤3d) > `timer` >
`normal`. It never exits non-zero and never prints anything but JSON.

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

### Hyprland: floating quick-add

A one-shot terminal for `mtm add`, sized small and focused:

```ini
# ~/.config/hypr/hyprland.conf
bind = $mainMod, T, exec, hyprctl dispatch exec [float;size 800 400;center] "alacritty --class mtm-quick -e mtm add"
windowrulev2 = float, class:^(mtm-quick)$
windowrulev2 = size 800 400, class:^(mtm-quick)$
windowrulev2 = center, class:^(mtm-quick)$
```

(Omarchy ships alacritty; swap in foot if you use it.)

### Shell completions

Installed by `make install`. To do it by hand:

```sh
mtm completion bash > ~/.local/share/bash-completion/completions/mtm
mtm completion zsh  > "${fpath[1]}/_mtm"        # then: autoload -U compinit && compinit
mtm completion fish > ~/.config/fish/completions/mtm.fish
```

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

## Troubleshooting

**The TUI/index looks stale or weird after moving machines.** The index is a
local cache; rebuild it: `rm ~/.cache/mtm/index.json` and run any command — it
reindexes from the files.

**A config value is rejected.** `mtm` validates every key on load and prints
`mtm: config …: <problem>`. Check the spelling against
[Configuration reference](#configuration-reference); unknown `ui.*` values are
errors, not warnings.

**`mtm sync` keeps saying "sync is paused".** A pull conflicted. Fix the files,
`mtm sync --status` shows what's unmerged, then `mtm sync --continue` (or
`--abort` to drop the pull and get writing again). Sync never auto-resolves.

**Suspect demo/fixture data in the real repo, or something feels off.** Run
`mtm doctor` — it checks the git repo, the marker (`.mtm-data`), the remote,
your git identity, every task file's frontmatter, demo-corpus ids/commit
messages (e.g. `dn0055`, `lt0001`, `od0001` — only flagged when the repo marker
says `kind: real`), a paused/conflicted sync, stale timer state, malformed theme
roles, the detected terminal/color mode, the configured ambient render, and
Ghostty window padding. `mtm doctor --json` for scripting; exits non-zero on
any failure. `mtm demo-seed` itself only runs inside a sandbox
(`MTM_SANDBOX=1` + temp dirs) and refuses repos whose marker says `kind: real`.

**The ambient scene does not reach the very edge of a Ghostty window.** Ghostty
pads the window (`window-padding-x`/`window-padding-y`) and by default fills
that padding with the background color, so the scene stops a few pixels short
of the edge. Set `window-padding-color = extend-always` in
`~/.config/ghostty/config` (or `$XDG_CONFIG_HOME/ghostty/config`) and restart
Ghostty. `mtm doctor` warns about this combination (read-only — it never edits
your terminal config). mtm also paints a uniform base row at the very bottom so
the padding mirrors invisibly instead of replicating the footer pill.

**Waybar shows nothing.** Check `mtm waybar` by hand; it prints one JSON line
or an error payload. The module needs `"custom/mtm"` in the waybar modules list
and `"return-type": "json"`.

**`mtm remind` never fires.** Reminders are one-shot per task+occurrence in
`~/.local/state/mtm/reminders.json` — check whether it's already there (delete
the entry to re-fire), and check `quiet_hours` in the config. `--dry-run` shows
what would fire right now.

**A timer shows time I didn't work.** `mtm timer stop` banks the elapsed time
capped at the session; nothing starts by itself. (Focus mode shows the ring at
00:00 until `space`.)

**Where does mtm write outside the repo?** Only `~/.cache/mtm` (index, GitHub
cache) and `~/.local/state/mtm` (timer, reminders, splash stamp). Your tasks
live in the git repo and in nowhere else.

## Development

```sh
make test vet fmt       # go test ./..., go vet, gofmt check
make check              # fmt + vet + test
make build              # bin/mtm
make man                # man pages into ./man (via cobra docs)
make install            # binary + man page + completions into ~/.local
make demo               # throwaway sandbox with example data (never your real data)
make smoke              # full CLI lifecycle in a throwaway sandbox
make perf               # CPU/paint measurements under a pty (sandboxed)
```

Tests and demos are sandboxed: `contrib/sandbox-env.sh` redirects `HOME`, every
XDG root and `MTM_SANDBOX=1` into a temp dir, and `internal/paths` panics if a
sandboxed process would touch the real roots. `mtm tui --dump` renders one
deterministic frame (`--view`, `--size WxH`, `--no-color`, `--scene`, `--render`,
`--ppm FILE`) and only runs inside the sandbox; it backs the golden tests in
`internal/tui/testdata/`.

Layering: `cmd/mtm` → `internal/cli` (Cobra) / `internal/tui` (Bubble Tea) →
`internal/actions` → `internal/core` → `internal/store`. The TUI has no business
logic.

Commit messages in the data repo are deliberately free of tooling noise; user
data belongs to the user.

## Measuring performance (`make perf`)

`make perf` runs the TUI under a pseudo-terminal in a **throwaway sandbox**
(`HOME`, every XDG root and `MTM_SANDBOX=1` pointed at a temp dir) and reports,
per scenario, the user+sys CPU time over the run (from `wait4`), the paint rate
(alt-screen flushes counted as `ESC[H`), bytes per paint, allocations per paint
(from the `--profile` stats file) and allocations per `View()` rebuild — plus
the top 15 functions from a CPU profile (`go tool pprof -top`).

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
| `home-ambient-12fps` | Home + sidebar window (label kept for comparability; the Home/sidebar idle cap is lower) | ≤ 20% of a core |
| `focus-24fps` | Focus mode behind a running Pomodoro | ≤ 30% of a core |
| `idle-ambient-5m` | Ambient on, untouched; defaults to 300 s | ≤ 5% of a core |

The harness answers the terminal's capability queries (`OSC 11` and `DSR`) like
a real terminal, so colours render through the truecolor path instead of an
ASCII fallback. Profiling refuses to run outside the sandbox
(`MTM_PPROF=cpu:/path/to.pprof` or `mtm tui --profile FILE`).

To measure on your own hardware, run it on the machine you care about. The
numbers in this repo's commit history were taken on an i3-1115G4 and cover the
`mtm` process only (the pty reader is off the clock); your numbers will differ
on a faster or busier machine.

## Release

`v1.0.0` is tagged at the release commit. `make build` embeds the version from
`git describe --tags`, so a checkout at the tag builds `mtm v1.0.0`. To cut the
next release, commit, tag and push:

```sh
git tag -a v1.1.0 -m "v1.1.0"
git push --tags
```

See `CHANGELOG.md` for the per-release notes.
