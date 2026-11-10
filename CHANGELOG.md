# Changelog

All notable changes to mtm. Format based on Keep a Changelog; versions
follow semver. `v1.0.0` is not tagged yet — see the note at the bottom.

## [Unreleased] — 2026-11-09

### Added
- **Phase 1 — file store and CLI core**: Markdown task files, the git
  data repo, index cache, projects with custom statuses and colors,
  archive, search, saved views, natural-language quick-add with dates,
  priorities, tags, projects and estimates; recurrence; subtasks.
- **Phase 2 — smart CLI**: `mtm view` (saved views), smart add, `mtm
  pomo`, `mtm timer` with stale-session capping, summary, `mtm done`
  spawns the next recurrence.
- **Phase 3 — TUI**: the Study Desk dashboard; views Home, List, Board
  (done-filtering), Calendar, Courses; Focus mode; Pomodoro; rich
  keybindings with config overrides; live Omarchy theming; help overlay.
- **Phase 4 — academics**: courses on projects, exams, syllabus import,
  weighted grades with needed-on-remaining math, hours rollups,
  per-project list bars, shared timer engine with Pomodoro and
  notify-send.
- **Phase 5 — stats, export, iCal, reminders**:
  - `mtm stats` (`--week|--month|--semester|--since DATE`): completed,
    on-time rate, hours by course, busiest day, estimate accuracy,
    per-course breakdown; `stats.streak_min_minutes` (default 15)
  - `mtm review`: interactive weekly ritual — finished, slipped, next 7
    days; one-by-one reschedule/archive/keep; never mutates without a
    keypress
  - Streaks with a flame indicator on Home and a Stats TUI view (key 5)
  - `mtm export` (`--format md|csv|json`), top-level `schema_version: 1`
    documented in the README; `mtm ical` (floating local times or
    `ical.timezone`), stable task-id UIDs, optional `--alarm`, and a
    read-only loopback-only `--serve` endpoint (`--token` optional)
  - `mtm remind` (deduped, quiet hours supported) and
    `mtm systemd install|uninstall|status` (user sync + reminder timers;
    sync refuses while paused)
  - `mtm waybar`, `mtm completion bash|zsh|fish`
  - `ui.density: comfortable|compact` for the board
  - `mtm version` (embedded version/commit/build date), `make man`
    (man pages via Cobra docs), `make install`
- **Phase 6 — project tools**:
  - Ideas incubator (`ideas/`, one Markdown file per item): `mtm idea`,
    `mtm ideas`, `mtm idea show|edit|status|rate|note|rm|promote`;
    `mtm idea promote` creates a project with the standard workflow, an
    initial milestone and starter tasks from `templates/<name>.md`, records
    `promoted_to`, keeps the idea file; new TUI view 6 (cards grouped by
    status; `a` capture, `r` rate, `p` promote, `s` cycle status, `m`/`M`
    notes) plus a small Home band
  - Milestones (`projects/<p>/milestones/`): `mtm milestone
    add|list|show|edit|done`; tasks link via `milestone:`; progress derives
    from tasks and subtasks
  - Project page in the TUI (`enter` on a sidebar project or `:roadmap`):
    Overview (description, combined task+milestone progress, velocity
    sparkline with 12-week window and an estimated finish date or an honest
    "not enough data" state, hours logged), Tasks, Roadmap (12-week
    timeline, month header, today marker, milestone bars filled by progress,
    at-risk/overdue colours, linked tasks below) and Dev-log
  - Dev-log (`projects/<p>/devlog/<YYYY-MM-DD>-<id>.md`): `mtm log
    <project> [text]`, `--with-timer` (attaches the running timer's
    elapsed time), `--from-commits` (prefilled with recent commits),
    `devlog.suggest_from_commits` config; `mtm logs [--since 7d]`
  - GitHub link (`mtm project link <project> [--repo owner/name] [--path
    ~/Projects/foo]`): read-only — local git log for a 12-week commit
    sparkline, last commit, last 5 subjects and a commit streak (sidebar and
    Home chips); with `--repo` and an authenticated `gh`, open issue/PR
    counts and the latest few, cached in the cache dir for 15 minutes,
    never in the data repo, never writing to the linked repo
  - `mtm export` now fills the reserved `ideas`/`milestones` arrays and adds
    a `devlog` array; JSON top-level `schema_version` bumps 1 → 2
  - `mtm ical` gains `--include milestones` (all-day events on milestone
    target dates)
  - `mtm summary` shows the open-idea count (when non-zero) and the next
    milestone with days remaining; still <50 ms, no network
  - **Safety — sandboxed seeding and repo markers**: `mtm demo-seed`
    refuses to run unless `MTM_SANDBOX=1` *and* every mtm root plus the
    target resolve under a temp dir (the same admission `tui --dump`
    now uses); `mtm init` writes a `.mtm-data` marker (UUID, creation
    date, `kind: real`) and the demo seeder writes `kind: demo`,
    refusing to touch a `kind: real` repo or anything under the real
    home; `mtm doctor` (`--json`) runs read-only health checks: git
    repo, marker, remote, git identity, frontmatter on every task
    file, demo-corpus task IDs/commit messages in a real repo,
    paused/conflicted sync, stale or corrupt timer state

### Notes
- The iCal export is local-naive by design: without `ical.timezone` set
  it emits floating local times (no TZID); DTSTAMP stays UTC.
- `mtm ical --serve` defaults to `127.0.0.1`; non-loopback requires an
  explicit `--bind` and prints a loud warning; with `--token` the feed
  moves to `/<token>/mtm.ics`.
- JSON export is stable via `schema_version: 2`; the reserved
  `ideas`/`milestones` arrays are populated and a `devlog` array was added.
  Version 1 consumers must tolerate the new top-level keys.
- The sync timer only ever runs plain `mtm sync`, which refuses while the
  data repo is conflicted or paused.

## Tagging v1.0.0

`v1.0.0` is deliberately not tagged yet. When you confirm the release:

```sh
git tag -a v1.0.0 -m "v1.0.0"
git push --tags
```

`git describe --tags` then feeds `make` builds (`VERSION`), and
`man mtm` shows `v1.0.0` as the source.
