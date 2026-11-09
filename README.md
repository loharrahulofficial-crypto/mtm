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
> Waybar integration, and shell completions. Not yet: Phase 6 ideas/milestones.

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
mtm add "buy milk"                         # natural language: dates, !priority, +tags, project:
mtm add "submit report friday !high +urgent project:college/dbms"
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

### Import and export
`mtm export` (`--format md|csv|json [--out FILE] [--project X] [--course X] [--since DATE]`),
`mtm ical` (`--out FILE`, `--serve [--bind host:port] [--token SECRET]`,
`--include tasks,exams`, `--alarm 30m`)

### Shell and bars
`mtm summary`, `mtm waybar` (Waybar `custom/` module JSON), `mtm completion bash|zsh|fish`

### System
`mtm remind` (one-shot reminder check; run it from a timer),
`mtm systemd install|uninstall|status` (user sync + reminder timers),
`mtm version`, `mtm sync`, `mtm tui`, `mtm demo` (throwaway demo sandbox)

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

## Import and export

```sh
mtm export                     # markdown report to stdout (grouped by course)
mtm export --format json -o export.json
mtm export --format csv --project college/dbms
mtm ical                       # RFC 5545 calendar to stdout
mtm ical --out ~/mtm.ics
mtm ical --serve --bind 127.0.0.1:8765          # read-only endpoint
mtm ical --serve --token secret --bind 0.0.0.0   # remote read access (warns loudly)
```

- **Markdown** is a readable report grouped by course: grades, exams, task
  lists (nested subtasks), hours.
- **JSON** carries a top-level `schema_version` (currently `1`), documented
  below in *Export schema*. Ideas/milestones land in a later schema bump.
- **CSV** is a flat task table with stable columns (one row per task;
  subtasks included with `parent` set).
- **iCal** events: timed tasks and exams are timed events (1h duration);
  date-only tasks are all-day events. UIDs derive from task ids, so
  re-imports update instead of duplicating. Dates stored local-naive export
  as floating local times; set `ical.timezone` to emit `TZID` parameters.
  `--alarm 30m` adds a VALARM to every event.
- The `--serve` endpoint serves exactly `/mtm.ics` (or `/<token>/mtm.ics`
  with `--token`), GET/HEAD only, read-only, regenerated per request. The
  default bind is loopback; non-loopback prints a loud warning.

## Export schema (version 1)

```json
{
  "schema_version": 1,
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
  "ideas": [],             // reserved for a later schema bump (Phase 6)
  "milestones": []         // reserved for a later schema bump (Phase 6)
}
```

Round-trip by `mtm export --format json`; the tests validate the structure
against this schema.

## Configuration reference

`~/.config/mtm/config.yaml`. Every key is optional; defaults are shown.
Run `mtm init` to see a fully-commented default file.

```yaml
data_repo: ~/.local/share/mtm   # where the data lives (git repo)
default_project: inbox          # where new tasks land without a project:
editor: ""                      # $EDITOR, else $VISUAL, else vi
date_format: "2006-01-02"
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

stats:
  streak_min_minutes: 15        # a day is active with ≥1 completion OR this many logged minutes
                                # 0 = only completions count

ical:
  timezone: ""                  # IANA name (e.g. "Europe/London"); empty = floating local times

reminders:
  due_soon_minutes: 0           # >0: also notify for tasks due within N minutes
  quiet_hours: ""               # "22:00-07:00" (spans midnight) — empty disables
```

## Data layout

```
~/.local/share/mtm/                # the data repo (syncs)
├── projects/<project>/<id>-<slug>.md
├── projects/<project>/project.yaml   # statuses, color, course block
├── archive/<project>/            # mtm archive moves done tasks here
└── .git/

~/.cache/mtm/index.json           # index; never committed; safe to delete
~/.local/state/mtm/timer.json     # timer/Pomodoro state (outside the repo)
~/.local/state/mtm/timer.lock
~/.local/state/mtm/reminders.json # sent reminders (dedup; outside the repo)
~/.local/state/mtm/mtm-sync-state.json  # sync paused flag (lives in .git/)
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
completed: 2026-11-09T18:30   # set when done
---
Free-form Markdown notes here.
```

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
`3` Calendar · `4` Courses · `5` Stats (all reachable with `tab`).

- `a` add · `A` add subtask · `x` done/un-done
- `/` filter · `ctrl+p` palette · `?` keybindings
- `F` focus mode (pomodoro on the selected task)
- board: `h`/`l` move between columns · `D` toggle the Done filter
- calendar: `h`/`l` move the day · `J`/`K` change months
- courses: `g` record a grade · `e` add an exam

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
