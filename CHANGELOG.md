# Changelog

All notable changes to mtm. Format based on Keep a Changelog; versions
follow semver. `v1.0.0` is not tagged yet — see the note at the bottom.

## [Unreleased] — 2026-11-10

### Added

- **Phase 4b-B — ambient scenes** (`internal/ambient`): deterministic seeded
  animations in the TUI — night sky (magnitude-ramped parallax starfield, a
  moon in its real phase computed locally, shooting stars, a noise-ribboned
  aurora), procedural bonsai (recursive branching with seeded vigor, grows
  across a pomodoro; idle shows the swaying full tree), ocean (parallax wave
  crests, drifting clouds, sun or moon by local hour, specular glint path,
  gulls, seeded fish leaps), rain and cabin (braille-streak rain, ripples,
  pine silhouettes, a lit flickering window, chimney smoke and fog, optional
  lightning off by default), plus three extras — koi pond (segment-jointed
  fish), snowfall (three parallax flake depths) and campfire (breathing
  radial glow, sparks, smoke)
- **Three new cozy scenes** (2026-11-11): `mountains` — a dusk valley in
  the Firewatch idiom: one bold warm gradient, a breathing sun halo,
  cloud masses melting into the sky, four ridge layers dissolving into
  haze with atmospheric perspective, snow stipple and slow gliding
  birds for the fine grids; the sun sinks along the pomodoro arc and
  break holds it high. `river` — a golden-hour valley: a winding river
  widening toward the viewer carrying the sky's color, pulsing flow
  dashes, a flickering sun glitter column, swaying bank grass and
  fireflies; the sun tracks the pomodoro arc the same way. `meteor` — a
  meteor-shower night: a denser starfield than night, a faint milky-way
  wash, thin bright streaks with long fading tails several in flight at
  once and a brighter bolide every third wave, over a pine ridge (no
  progress tie-in). With them `ui.scene: auto` rotates by local hour:
  5–8 mountains, 8–15 ocean, 15–17 river, 17–20 rain, 20–23 night,
  23–5 meteor; `make demo` seeds `ambient_render: sextant` in its
  sandbox config
- **Where they appear** (`ui.ambient: off|splash|sidebar|home|both`, default both):
  startup splash (default 3s, any key skips; skipped on non-TTY,
  `--no-splash`, or when disabled), a framed ambient window at the bottom
  of the sidebar filling only the free rows under the course list (the new
  default surface; minimum 8 rows, collapses on short terminals; toggle
  `ui.ambient_sidebar: false` or in `:ambient`), and behind the Focus
  surface (progress follows the Pomodoro; pausing freezes the scene; breaks
  switch to a calm phase), and a 1.5s task-complete reward burst
  (`ui.reward_animation: true`, rate-limited to 3 per minute). `home` keeps
  the larger optional panel in Home free space (hidden while the sidebar
  window is on), `sidebar` is the window only, `both` is splash + window

- **Focus backdrop (v2)**: the scene fills the whole Focus frame (was:
  revealed only behind blank pads) under a soft dark scrim blended from the
  scene palette. The scrim alpha is computed per theme: it rises until
  BrightText vs the brightest scene role passes WCAG 4.5 (light themes
  scrim toward white instead). The timer renders on top of the scene.
  Frame budget is per-surface: ≤ 24 FPS on Focus/splash, ≤ 8 FPS behind
  Home/sidebar
- **Controls**: `alt+s` cycles scenes; `:scene <name>` switches (koi, snow,
  campfire …); `:ambient` opens a live full-screen preview with a mood
  picker (`:ambient` also reachable from the palette and `:scene list`);
  `--no-ambient` disables all surfaces for one run
- **`ui.splash: on|off|once-a-day`** (default on): `--no-splash` and
  `MTM_NO_SPLASH=1` suppress the splash for a run; `once-a-day` shows it
  only on the first launch of each local day (stamp:
  `~/.local/state/mtm/splash.stamp`). The first frame is always the plain
  base-colored frame — the splash fades in from it, and any key skips
  immediately (~350 ms tail, never blocked input)
- **`ui.truecolor: auto|on|off`** (default auto): `MTM_TRUECOLOR=1|0`
  overrides for a run. auto = truecolor unless the terminal is positively
  limited (`TERM=linux`, `TERM=dumb`, `NO_COLOR`, Apple Terminal,
  `TERM=screen*` without `COLORTERM`). Limited terminals emit the nearest
  xterm-256 entry (cube + gray ramp, perceptual distance) instead of
  24-bit color; `ui.ambient_dither` ordered-dithers between the two
  nearest palette entries only. `mtm doctor` reports the detected
  terminal, color mode and override source (`terminal` check)
- **`ui.ambient_render`** gains `quadrant`: 2×2 subcells per cell (square
  pixels, 4× halfblock's subcell count), emitted with the 16 Unicode
  quadrant/half-block glyphs; unpainted cells are skipped like ascii/braille.
  `auto` still resolves to `halfblock`; existing modes are byte-identical
- **`ui.ambient_render`** gains `sextant`: 2×3 subcells per cell (2× the
  subcell count of quadrant), emitted with the 60 Unicode 15 sextant
  mosaics (U+1FB00..U+1FB3B) plus '▌'/'▐' (column mosaics Unicode does not
  encode), '█' and ' '. Same luminance-split + SGR discipline as the other
  sub-cell renderers; transitions decode sextant frames at 2x width × 3x
  height so the structure survives; `--render sextant` and the `:ambient`
  picker expose it
- **`ui.ambient*` config**: `ambient`, `scene` (`auto` picks by local hour,
  `random` per launch), `splash_seconds`, `reward_animation`,
  `ambient_fps` (default 60, max 60), `ambient_idle`
  (`slow` (default) | `pause` | `off` — Home/sidebar drops to ~4 FPS after
  30 s without input and pauses on a still frame after 5 minutes, resumes
  instantly on any key or mouse event; Focus exempt while the timer runs),
  `ambient_render`
  (`auto` | `halfblock` | `braille` | `ascii` | `quadrant` | `sextant`), `rain_lightning` (default off)
- The ambient render is a little more cinematic: supersampling 3x at full
  quality (adaptive tier drops to 2, then 1), and scenes now mix colors in
  linear light instead of sRGB, so skies glow and sunsets keep hue.
- Tab switches cross-fade between the previous and next frame over ~150ms
  (all entry points; skipped under `ui.reduced_motion`).
- The ambient cap is now **60 FPS** everywhere (`ui.ambient_fps`,
  honoured up to `max 60`); adaptive quality still steps down (halved
  supersampling, then halved FPS) when frames blow the 30 ms budget
- `mtm doctor --json` now reports `ambient_paint_ms` / `ambient_paint_ns`
  (slowest measured reference scene frame, 100x30) alongside
  `ambient_quality_tier` / `ambient_quality_name`
- `make perf`: pty harness measuring CPU %, paints/s, bytes/paint,
  allocations/paint for Home (ambient off / on), Focus and idle scenarios;
  see README “Measuring performance”
- Static layers cached (perlin tables, palette LUT); the ambient ticker does
  zero work when no surface is visible or the terminal is unfocused
- Hidden verification path: `mtm tui --dump --scene <name> --t <dur>
  --progress <0..1> --render <mode> --phase <work|break|idle>`
- **Splash wordmark, hand-designed** (2026-11-11): the mtm wordmark is
  drawn from hand-laid 5x5/7x7/9x9 bitmaps with a diagonals-meeting-in-
  the-center M (the old stems+leg M read as an H); the set is chosen by
  terminal size, the block sits in the sky band at ~39% of the frame
  height (never on the horizon), and `ui.wordmark: caps|lower` selects
  the caps letterforms ("MTM", default) or the x-height lowercase-style
  ones ("mtm")

- **Composition + full-frame art** (2026-11-11): full-screen surfaces
  (splash, Focus, `--dump --scene`) land the horizon at 62–70% of the
  height. Night gains a still lake below the horizon mirroring the
  moon, aurora and brightest stars (flipped, squashed, dimmed, with
  horizontal ripple displacement and a slow shimmer + glint path) and a
  treeline reflection along the shoreline. Ocean gains deeper water
  depth and a mirrored celestial disc under the horizon. Rain/cabin
  gains a ground gradient, a winding path from the door, and pooling
  base mist. Bonsai gains a lit ground plane with a soft pot shadow.
  Snow gains foreground drifts. Ridges and treeline slabs in every
  scene are vertical gradients, and no scene may carry one flat color
  across more than 20% of the frame height (cell-grid test, all
  scenes at 100x30 and 200x50). Embedded windows (sidebar, Home
  corner, settings preview) keep the aspect-adaptive horizon.

### Fixed

- **Splash text no longer sits on a plate** (2026-11-11): overlay cells
  carry the scene's own per-cell background cell by cell — on the
  sextant render path the scene frame is decoded back to a raster to
  sample it (previously it fell back to a flat `Base` band behind the
  tagline and hint). A soft radial scrim — a smooth quartic darkening
  toward `Base`, exactly zero at the rim — now backs the wordmark,
  tagline and hint; no flat rectangle remains behind any splash text.
  Goldens `night-0200/0900/1500/2200ms` regenerated accordingly
- **Startup/transition stripes and pink flashes** (2026-11-11): transitions
  blend in pixel space now (`FramePixels` raster API; `Crossfade`/`Dissolve`/
  `BlackFrame`/splash head fade all operate on rasters, always emit `▀`);
  text overlays composite after the scene transition. The `255,0,255` debug
  sentinel is gone everywhere — malformed theme roles fall back to defaults
  with diagnostics in `mtm doctor`; the base-color detection for
  already-rendered frames and the SGR decimal→hex path no longer produce
  garbage colors. Scene layout is size-independent (`min(w, h*2)` and
  aspect ratio) — scenes compose at 24x10 through 200x50; important
  elements (moon/sun/tree crown/cabin) are never clipped by more than 10%
  and the horizon stays in [40%, 70%]
- `mtm doctor` demo-data check no longer flags a repo's history — it detects
  demo data in the current tree only (task IDs / demo project names), treats
  a demo-looking commit message as at most an info line, never reports a
  reverted demo commit, and stops false-flagging real repos that contain
  milestones/dev-log entries
- Project page letterspaced section labels keep their word gaps
  (`NEXT MILESTONE`, `VELOCITY · 12 WEEKS`)
- Roadmap: a completed milestone no longer renders a stray track glyph
  inside its bar next to the today marker; milestone days with timestamps
  in frontmatter parse correctly
- Dev-log tab renders entries via Glamour (no raw `###`/`--------`); the
  project page footer shows the tab keys as `[` `]`, not `[ ]`
- Letterspaced labels on the project page keep word gaps via the shared
  sectionLabelText path

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
                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                