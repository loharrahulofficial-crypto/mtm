# mtm — Multi-Task Manager

A personal terminal task manager for Omarchy (Arch Linux + Hyprland), built for
college life: courses, exams, assignments, projects and notes.

One Go binary, two interfaces over one shared core:

- **CLI** for quick commands (`mtm add "..."`)
- **TUI** for interactive use (Phase 3)

Tasks are stored as one Markdown file per task, inside a git repo that can
auto-sync between devices. No database.

> **Status: Phase 1 (MVP).** Implemented: init, add, list, show, edit, done,
> rm, status, project management (custom statuses), tags, priorities, git
> auto-commit and sync with conflict detection, in-memory index + cache.
> Not yet: natural-language dates, recurrence, search, TUI, exams, grades,
> stats, export, iCal, systemd timer (Phases 2–5).

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

## Usage

```sh
mtm add "Submit DBMS assignment 2" --priority high --tag exam --due 2026-11-14T23:59
mtm add "buy milk"                       # default project (inbox), priority medium
mtm list                                 # hides done tasks
mtm list --project college/dbms --tag exam --status todo --priority high
mtm list --all --json                    # scripting
mtm show 3db991                          # ids accept unique prefixes
mtm status 3db991 doing                  # must match the project's statuses
mtm done 3db991                          # sets the project's done status
mtm edit 3db991                          # $EDITOR; validated on save
mtm rm 3db991                            # asks for confirmation (--force to skip)

mtm project add college/dbms --statuses backlog,todo,doing,blocked,done --done done
mtm project list
mtm project edit college/dbms            # edit project.yaml in $EDITOR
mtm sync [--status] [--abort] [--json]
```

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

`mtm summary` for shell rc files lands in Phase 2 and will read the cached
index to stay under 50ms.

## Development

```sh
make test vet fmt       # go test ./..., go vet, gofmt check
make build              # bin/mtm
```

Layering: `cmd/mtm` → `internal/cli` (Cobra) → `internal/core` (domain) →
`internal/store` (files + index). `internal/sync` shells out to git;
`internal/config` loads YAML over defaults; `internal/appname` holds the
single source of the app/binary names.

Commit messages of the data repo are deliberately free of tooling noise
(no session tags): user data belongs to the user.
