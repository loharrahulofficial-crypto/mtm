# mtm — Multi-Task Manager

A personal terminal task manager for Omarchy (Arch Linux + Hyprland), built for
college life: courses, exams, assignments, projects and notes.

One Go binary, two interfaces over one shared core:

- **CLI** for quick commands (`mtm add "..."`)
- **TUI** for interactive use (Phase 3)

Tasks are stored as one Markdown file per task, inside a git repo that can
auto-sync between devices. No database.

> **Status: Phase 2 (smart features).** Everything in Phase 1, plus:
> natural-language quick-add, `--due today|week|overdue`, `mtm today|week|overdue`,
> recurring tasks (completing one spawns the next), subtasks with `%` complete,
> `mtm note` / `mtm search`, saved views (`mtm views` / `mtm view <name>`),
> `mtm summary` for shell startup.
> Not yet: TUI, exams/grades/Pomodoro, export, iCal, systemd timer (Phases 3–5).

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

mtm project add college/dbms --statuses backlog,todo,doing,blocked,done --done done
mtm project list
mtm project edit college/dbms            # edit project.yaml in $EDITOR
mtm sync [--status] [--abort] [--json]
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

Every task file looks like:

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
and `completed:` set; archiving to `archive/` arrives with `mtm archive`
(Phase 2+).

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

Layering: `cmd/mtm` → `internal/cli` (Cobra) → `internal/core` (domain) →
`internal/store` (files + index). `internal/parse` is the quick-add parser;
`internal/sync` shells out to git; `internal/config` loads YAML over
defaults; `internal/appname` holds the single source of the app/binary
names.

Commit messages of the data repo are deliberately free of tooling noise
(no session tags): user data belongs to the user.
