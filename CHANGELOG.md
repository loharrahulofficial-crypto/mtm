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

### Notes
- The iCal export is local-naive by design: without `ical.timezone` set
  it emits floating local times (no TZID); DTSTAMP stays UTC.
- `mtm ical --serve` defaults to `127.0.0.1`; non-loopback requires an
  explicit `--bind` and prints a loud warning; with `--token` the feed
  moves to `/<token>/mtm.ics`.
- JSON export is stable via `schema_version: 1`; ideas and milestones are
  reserved for a later schema bump.
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
