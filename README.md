# mtm — Multi-Task Manager

A personal terminal task manager for Omarchy (Arch Linux + Hyprland), built for
college life: courses, exams, assignments, projects and notes.

One Go binary, two interfaces over one shared core:

- **CLI** for quick commands (`mtm add "..."`)
- **TUI** for interactive use (Phase 3)

Tasks are stored as one Markdown file per task, inside a git repo that can
auto-sync between devices. No database.

> **Status: Phase 3 (TUI).** Phase 2 smart features, plus: `mtm archive` /
> `mtm list --archived`, default task sorting (`sort:` in config), the full
> interactive TUI (`mtm` with no arguments), Omarchy live theming, background
> sync with a conflict-resolution screen.
> Not yet: exams/grades/Pomodoro, export, iCal, systemd timer (Phases 4–5).

## Install

```sh
git clone <this repo> && cd mtm
make install        # installs to ~/.local/bin/mtm
# or: go install github.com/loharrahulofficial-crypto/mtm/cmd/mtm@latest
```

Requires Go ≥ 1.25 and git on PATH.

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
GitHub credentials. This is the exact layout used on the primary machine:

```sshconfig
# ~/.ssh/config
Host github-mtm
  HostName github.com
  User git
  IdentityFile ~/.ssh/mtm_data
  IdentitiesOnly yes
```

One-time setup per **account** (or per second machine):

```sh
# 1. dedicated keypair (no passphrase so automated sync can use it)
ssh-keygen -t ed25519 -f ~/.ssh/mtm_data -N "" -C "mtm-data deploy key"

# 2. add ~/.ssh/mtm_data.pub as a deploy key with WRITE access on the
#    private repo that holds the data (e.g. <you>/mtm-data):
#    repo → Settings → Deploy keys → Add deploy key (Allow write access)

# 3. verify
ssh -T github-mtm
```

On a **second machine**, one command clones the existing data repo and
wires it up:

```sh
mtm init --remote git@github-mtm:<you>/mtm-data.git
```

`mtm init --remote` clones instead of creating an empty repo whenever the
target directory is missing/empty and the remote has commits. If the target
already holds an initialized repo, it just adds/updates `origin` and pushes
with the upstream set. `mtm sync --status` confirms the link
(`remote: origin (git@github-mtm:...)`, `ahead: 0 commit(s)`).

If both sides were initialized independently (unrelated histories), the
pull is refused with instructions; the safest fix is to delete the local
repo directory and re-run `mtm init --remote` so it clones.

To add the same alias on another machine, copy the `Host github-mtm` block
(steps 1–3 above with a new key, or by copying the existing keypair —
your call on trust boundaries).

## Usage

```sh
# natural language: dates, priority, tags, project, recurrence, estimate
mtm add "submit report friday 6pm !high #college @dbms ~2h"
mtm add "call mum tomorrow"            # date-only due
mtm add "review notes in 2 days"
mtm add "standup every monday 9am"     # recurring (see below)
mtm add buy milk                       # unquoted words are the title

# flags override whatever the parser found
mtm add "Submit report" --due 2026-11-14T23:59 --priority high --tag assignment

mtm list                                 # hides done tasks
mtm list --project college/dbms --tag exam --due week
mtm list --status todo --priority high --all --json
mtm today | mtm week | mtm overdue      # due-date windows
mtm view urgent                          # saved view from config.yaml
mtm views                                # built-in + saved views

mtm show 3db991                          # ids accept unique prefixes; shows subtasks
mtm status 3db991 doing                  # must match the project's statuses
mtm done 3db991                          # spawns next occurrence if recurring
mtm edit 3db991                          # $EDITOR; validated on save
mtm rm 3db991                            # asks for confirmation (--force to skip)
mtm note 3db991 "asked prof about edge cases"   # appends a bullet line
mtm sub 3db991 "draft outline"           # subtask under a task
mtm search dbms                          # full-text: title, notes, tags
mtm search --archived dbms               # also search archived tasks

mtm archive                              # move all done tasks to archive/<project>/ (one commit)
mtm archive --older-than 30d             # only tasks completed >30 days ago
mtm list --archived                      # browse archived tasks

mtm project add college/dbms --statuses backlog,todo,doing,blocked,done --done done
mtm project list
mtm project edit college/dbms            # edit project.yaml in $EDITOR
mtm sync [--status] [--abort] [--json]  # conflicts: fix, then mtm sync --continue
```

### Recurrence

`daily`, `weekly`, `weekly:mon` (…`:tue`…`:sun`), `every N days`, `every N weeks`.
Completing a recurring task (via `done` or `status` → done) automatically
creates the next occurrence with the computed due date. Recurrence is stored
in the `recurrence:` frontmatter field.

### Saved views

Built-in views: `today`, `week`, `overdue`. Add your own to
`~/.config/mtm/config.yaml`:

```yaml
saved_views:
  - name: urgent
    filter: priority:urgent
  - name: dbms
    filter: project:college/dbms due:week
  - name: all-maths
    filter: tag:maths all     # "all" includes done tasks
```

Filter keys: `tag:`, `project:`, `status:`, `priority:` (low|medium|high|urgent),
`due:` (today|week|overdue). Bare token `all` includes done tasks. Run with
`mtm view <name>`, list with `mtm views`.

### TUI

`mtm` with no arguments launches the interactive TUI. `mtm --help` prints
help instead; all subcommands behave as before. The TUI shows a sidebar
(projects with progress, smart views All/Today/Week/Overdue, saved views),
a main pane and an optional detail/notes preview pane (`v` to toggle). On
narrow terminals the sidebar and detail pane collapse automatically.

Views (`1` `2` `3` or `tab`):

1. **List** — tree view with `↳` subtasks, priority/status badges and
   due-date coloring (overdue / today / upcoming).
2. **Kanban board** — columns come from the selected project's custom
   statuses. `h`/`l` select a column, `H`/`L` move the card, or drag with
   the mouse.
3. **Calendar** — month grid with task dots, a day agenda below;
   `h`/`l` (days), `H`/`L` (months), `j`/`k` scrolls the agenda.

Keys: `gg`/`G` top/bottom, `/` live filter (full-text plus `tag:`
`project:` `status:` `priority:` `due:` syntax, same parser as saved
views), `n`/`N` next/prev match, `:` command line (e.g. `:add buy milk
friday !high`, `:done`, `:status doing`, `:due 2026-11-20`), `a` add,
`A` add subtask, `x` done (spawns recurring occurrence), `dd` delete
(confirms), `D` due date, `t` tags, `e` title, `s` status, `p` priority,
`m` notes editor, `M` notes in `$EDITOR`, `E` task file in `$EDITOR`,
`S` sync now, `X` archive selected done task, `?` help overlay, `q` quit.
Vim-style `h j k l` throughout; multi-key sequences (`gg`, `dd`) work
with a short prefix timeout.

Config (`~/.config/mtm/config.yaml`):

```yaml
sort: [done, due, priority, title]   # any order/subset of these keys
keys:                                # action -> key sequence overrides
  quit: q
  toggle_done: x
  add: a
```

Unknown `keys:` actions are ignored with a startup note. Colors come from
the active Omarchy theme (`~/.local/state/omarchy/current/theme/`,
`colors.toml`; terminal configs as fallback) and re-theme live when the
theme changes. If the files are missing, a built-in default palette is
used. Work is committed in small auto-commits and synced in the
background (debounced); a sync conflict opens the conflict screen
automatically for per-file **keep mine / keep theirs / edit merged in
$EDITOR** resolution — nothing is ever auto-resolved.

### Shell startup summary

`mtm summary` prints a two-line block:

```text
3 today · 2 overdue · exam in 13d: dbms final exam
inbox 2/5 (40%) · college/dbms 1/4 (25%)
```

It reads the cached index (`~/.cache/mtm/index.json`) and is fast: ~25ms with
2000 tasks on warm cache (< 50ms target). Add to your shell rc:

```sh
# .bashrc or .zshrc
mtm summary 2>/dev/null || true
```

### Waybar module

`mtm waybar` prints one JSON line for a Waybar `custom/` module. Text is
bar-sized: today's open task count, plus an exam chip (`✦<days>`) when the
nearest open exam is within 30 days and an active-timer chip. Class follows
fixed priority: `overdue` > `exam` (≤3 days) > `timer` > `normal`. It reads
the cached index and runs in ~16ms warm — safe to poll every few seconds.

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

and a block is added to the `modules-right` list. `style.css`:

```css
#custom-mtm.overdue { color: @red; }
#custom-mtm.exam { color: @yellow; }
#custom-mtm.timer { color: @blue; }
#custom-mtm.normal { color: @foreground; }
```

### Hyprland keybind: floating quick-add

A one-shot terminal for `mtm add`, sized small and focused, closes itself
on save:

```ini
# ~/.config/hypr/hyprland.conf
bind = $mainMod, T, exec, hyprctl dispatch exec [float;size 800 400;center] "alacritty --class mtm-quick -e mtm add"
windowrulev2 = float, class:^(mtm-quick)$
windowrulev2 = size 800 400, class:^(mtm-quick)$
windowrulev2 = center, class:^(mtm-quick)$
```

(Omarchy ships alacritty; swap in foot if you use it. `mtm add` asks
questions interactively; add flags like `--due` to skip them.)

### Shell completions

```sh
mtm completion bash > ~/.local/share/bash-completion/completions/mtm
mtm completion zsh  > "${fpath[1]}/_mtm"
mtm completion fish > ~/.config/fish/completions/mtm.fish
# zsh: then autoload -U compinit && compinit
```

### Every task file looks like:

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
created: 2026-11-07T08:00
updated: 2026-11-07T08:00
weight: 15              # grade weight %, for assignments and exams
---
Free-form Markdown notes here.
```

Completed tasks stay in place with `status` set to the project's done status
and `completed:` set. `mtm archive` moves them to `archive/<project>/`
keeping files byte-identical; they leave the index by default and stay
searchable with `--archived`. Nothing is ever deleted.

## Data and config paths

| What | Path |
|------|------|
| Data repo | `~/.local/share/mtm` (`data_repo` in config) |
| Config | `~/.config/mtm/config.yaml` |
| Index cache | `~/.cache/mtm/index.json` (never committed; safe to delete) |
| Sync pause flag | `<data repo .git>/mtm-sync-state.json` |

## Development

```sh
make test vet fmt       # go test ./..., go vet, gofmt check
make build              # bin/mtm
```

Layering: `cmd/mtm` → `internal/cli` (Cobra) / `internal/tui` (Bubble Tea)
→ `internal/actions` (shared operations: quick-add, done/status transitions,
commit+sync) → `internal/core` (domain) → `internal/store` (files + index).
`internal/parse` is the quick-add parser; `internal/sync` shells out to
git; `internal/config` loads YAML over defaults; `internal/appname` holds
the single source of the app/binary names; `internal/theme` reads the
Omarchy palette. The TUI has no business logic — it calls actions/core/
store/sync.

Commit messages of the data repo are deliberately free of tooling noise
(no session tags): user data belongs to the user.
