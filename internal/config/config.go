// Package config loads ~/.config/mtm/config.yaml over built-in defaults.
package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/paths"
	"gopkg.in/yaml.v3"
)

type Config struct {
	DataRepo       string        `yaml:"data_repo"`
	DefaultProject string        `yaml:"default_project"`
	Editor         string        `yaml:"editor"`
	DateFormat     string        `yaml:"date_format"`
	WeekStart      string        `yaml:"week_start"`
	Board          BoardConfig   `yaml:"board"`
	Heatmap        HeatmapConfig `yaml:"heatmap"`
	// Sort is the task-list sort order: done|due|priority|title, applied in
	// order. Empty means the built-in default (not-done, due, priority, title).
	Sort       []string       `yaml:"sort"`
	Sync       SyncConfig     `yaml:"sync"`
	Pomodoro   PomodoroConfig `yaml:"pomodoro"`
	SavedViews []SavedView    `yaml:"saved_views"`
	// Keys overrides TUI key bindings: action name -> key(s), space-separated.
	Keys      map[string]string `yaml:"keys,omitempty"`
	UI        UIConfig          `yaml:"ui"`
	Stats     StatsConfig       `yaml:"stats"`
	ICal      ICalConfig        `yaml:"ical"`
	Reminders RemindersConfig   `yaml:"reminders"`
	Devlog    DevlogConfig      `yaml:"devlog"`
}

type DevlogConfig struct {
	// SuggestFromCommits prefills the `mtm log <project>` $EDITOR scaffold
	// with recent commits when no text is given (off by default).
	SuggestFromCommits bool `yaml:"suggest_from_commits"`
}

type SyncConfig struct {
	Auto            bool   `yaml:"auto"`
	DebounceSeconds int    `yaml:"debounce_seconds"`
	Remote          string `yaml:"remote"`
}

type PomodoroConfig struct {
	WorkMinutes       int `yaml:"work_minutes"`
	ShortBreakMinutes int `yaml:"short_break_minutes"`
	LongBreakMinutes  int `yaml:"long_break_minutes"`
	LongEvery         int `yaml:"long_every"`
}

type SavedView struct {
	Name   string `yaml:"name"`
	Filter string `yaml:"filter"`
}

// BoardConfig carries the "board:" block (board view behavior).
type StatsConfig struct {
	// StreakMinMinutes is the per-day minimum of logged time that keeps a
	// study streak alive (in addition to completing ≥1 task). Default 15;
	// 0 disables minute-based activity.
	StreakMinMinutes int `yaml:"streak_min_minutes"`
}

type RemindersConfig struct {
	// DueSoonMinutes, when > 0, fires a notification for open tasks whose
	// due time is within this many minutes. 0 disables due-soon alerts
	// (only explicit task reminders fire).
	DueSoonMinutes int `yaml:"due_soon_minutes"`
	// QuietHours suppresses all reminder notifications inside the window,
	// e.g. "22:00-07:00" (local time; windows may span midnight). Empty
	// — or "00:00-00:00" — disables quiet hours.
	QuietHours string `yaml:"quiet_hours,omitempty"`
}

type ICalConfig struct {
	// Timezone is an IANA name (e.g. "Europe/London") for DTSTART/DTEND
	// parameters; empty means floating local times (no TZID). Times stored
	// local-naive are never shifted.
	Timezone string `yaml:"timezone,omitempty"`
}

type BoardConfig struct {
	// DoneDays limits the board's Done column to items completed within
	// the last N days. Default 14; 0 or negative shows everything.
	DoneDays int `yaml:"done_days"`
}

// HeatmapConfig carries the "heatmap:" block (the completion heatmap).
type HeatmapConfig struct {
	// Weeks is how many weeks of history the heatmap shows, counting back
	// from the current week. Default 26; clamped to [4, 52].
	Weeks int `yaml:"weeks"`
}

// UIConfig carries the "ui:" presentation block (Study Desk design language).
type UIConfig struct {
	// Animations caps every animation at 400ms and honors reduce-motion
	// preferences. Default true.
	Animations bool `yaml:"animations"`
	// Icons selects the glyph set: nerd (Nerd Font, default) | unicode | none.
	Icons string `yaml:"icons"`
	// Density controls board card spacing: comfortable (default) keeps the
	// 5-row card (title lines always padded to two rows); compact drops the
	// blank line between a card's title and its badge row when the title
	// fits on one line.
	Density string `yaml:"density,omitempty"`
	// SemesterStart anchors the brand header's "Week N of M". Empty hides it.
	SemesterStart string `yaml:"semester_start,omitempty"` // 2006-01-02
	SemesterWeeks int    `yaml:"semester_weeks,omitempty"` // default 16

	// Ambient scenes (Phase 4b-B). Ambient master switch + where the
	// surface scenes appear: off | splash | sidebar | home | both.
	// "both" = splash + sidebar window (the new default placement).
	// "sidebar" = the framed window below the sidebar list.
	// "home" = the larger optional panel in Home free space (hidden while
	// the sidebar window is on). Focus, reward and splash ride on any
	// non-off value. Old "home" values keep their meaning — no remap.
	Ambient string `yaml:"ambient"`
	// Scene: auto (by local time of day) | night | bonsai | ocean | rain |
	// koi | snow | campfire | random (per launch).
	Scene string `yaml:"scene"`
	// SplashSeconds is the startup splash length (default 3s).
	SplashSeconds float64 `yaml:"splash_seconds"`
	// RewardAnimation shows a 1.5s scene celebration when a task is done.
	RewardAnimation bool `yaml:"reward_animation"`
	// AmbientFPS caps animation frames per second (default 8, max 30).
	AmbientFPS int `yaml:"ambient_fps"`
	// AmbientIdle: slow (default) | pause | off. "slow" drops the
	// Home/sidebar scene to ~4 FPS after 30 s with no input and pauses it
	// on a still frame after 5 minutes (resumes instantly on any key or
	// mouse event). "pause" skips the 4 FPS stage. "off" never throttles.
	// Focus mode and the splash are exempt while the timer runs.
	AmbientIdle string `yaml:"ambient_idle"`
	// AmbientRender: auto | halfblock | braille | ascii.
	AmbientRender string `yaml:"ambient_render"`
	// AmbientDither: on | off (default on). Ordered dithering only ever
	// applies where a limited palette forces it — on truecolor terminals the
	// renderer emits 24-bit color unchanged.
	AmbientDither string `yaml:"ambient_dither"`
	// Splash: on | off | once-a-day (default on). --no-splash /
	// MTM_NO_SPLASH=1 suppress it for a run. once-a-day shows it only on
	// the first launch of each local day.
	Splash string `yaml:"splash"`
	// TrueColor: auto | on | off (default auto). auto = detect from the
	// environment (truecolor unless there is positive evidence of a
	// limited terminal); on/off force 24-bit color or xterm-256 output.
	// MTM_TRUECOLOR=1|0 overrides for the run.
	TrueColor string `yaml:"truecolor"`
	// ReducedMotion honors NO_MOTION=1 too: no crossfades or dissolves,
	// scenes crawl as slow low-amplitude stills (idle breathing only).
	ReducedMotion bool `yaml:"reduced_motion"`
	// AmbientSidebar shows a framed ambient window at the bottom of the
	// sidebar, filling the rows the course list leaves free (minimum 8,
	// collapses to nothing on short terminals). On by default; the Home
	// corner hides while it is on unless ui.ambient opts into the corner.
	AmbientSidebar bool `yaml:"ambient_sidebar"`
	// RainLightning enables rare lightning in the rain scene (off default).
	RainLightning bool `yaml:"rain_lightning"`
}

func Default() Config {
	return Config{
		DataRepo:       DataHome(),
		DefaultProject: "inbox",
		Editor:         "", // resolved from $EDITOR/$VISUAL/vi at use site
		DateFormat:     "2006-01-02",
		WeekStart:      "monday",
		Board:          BoardConfig{DoneDays: 14},
		Heatmap:        HeatmapConfig{Weeks: 26},
		Sort:           append([]string(nil), core.DefaultSortKeys...),
		Sync: SyncConfig{
			Auto:            true,
			DebounceSeconds: 2,
			Remote:          "origin",
		},
		Pomodoro: PomodoroConfig{
			WorkMinutes:       25,
			ShortBreakMinutes: 5,
			LongBreakMinutes:  15,
			LongEvery:         4,
		},
		SavedViews: []SavedView{},
		Stats: StatsConfig{
			StreakMinMinutes: 15,
		},
		Reminders: RemindersConfig{
			DueSoonMinutes: 0,
			QuietHours:     "",
		},
		UI: UIConfig{
			Animations:      true,
			Icons:           "nerd",
			Density:         "comfortable",
			SemesterWeeks:   16,
			Ambient:         "both",
			Scene:           "auto",
			SplashSeconds:   3.0,
			RewardAnimation: true,
			AmbientFPS:      8,
			AmbientIdle:     "slow",
			AmbientRender:   "auto",
			AmbientDither:   "on",
			TrueColor:       "auto",
			Splash:          "on",
			ReducedMotion:   false,
			RainLightning:   false,
			AmbientSidebar:  true,
		},
	}
}

// Dir returns the config directory. All mtm directories resolve through
// internal/paths (sandbox guard, MTM_CONFIG_DIR/XDG overrides).
func Dir() string { return paths.ConfigDir() }

// Path returns the config file path.
func Path() string { return paths.ConfigPath() }

// DataHome returns the default data directory.
func DataHome() string { return paths.Data() }

// CacheDir returns the cache directory.
func CacheDir() string { return paths.CacheDir() }

// CachePath returns the index cache file path (D-006: outside the data repo).
func CachePath() string { return paths.CachePath() }

// ExpandPath expands a leading "~" or "~/" to the user's home directory.
func ExpandPath(p string) string {
	if p == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return p
	}
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// Load reads the config file over defaults. A missing file is not an error.
func Load() (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(Path())
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("read config %s: %w", Path(), err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", Path(), err)
	}
	keys, err := core.ParseSortKeys(cfg.Sort)
	if err != nil {
		return cfg, fmt.Errorf("config %s: %w", Path(), err)
	}
	cfg.Sort = keys
	switch icons := strings.ToLower(strings.TrimSpace(cfg.UI.Icons)); icons {
	case "nerd", "unicode", "none":
		cfg.UI.Icons = icons
	case "":
		cfg.UI.Icons = "nerd"
	default:
		return cfg, fmt.Errorf("config %s: ui.icons must be nerd|unicode|none, got %q", Path(), cfg.UI.Icons)
	}
	switch density := strings.ToLower(strings.TrimSpace(cfg.UI.Density)); density {
	case "comfortable", "compact":
		cfg.UI.Density = density
	case "":
		cfg.UI.Density = "comfortable"
	default:
		return cfg, fmt.Errorf("config %s: ui.density must be comfortable|compact, got %q", Path(), cfg.UI.Density)
	}
	if cfg.UI.SemesterStart != "" {
		if _, err := time.Parse("2006-01-02", cfg.UI.SemesterStart); err != nil {
			return cfg, fmt.Errorf("config %s: ui.semester_start must be 2006-01-02, got %q", Path(), cfg.UI.SemesterStart)
		}
	}
	if cfg.UI.SemesterWeeks < 0 {
		return cfg, fmt.Errorf("config %s: ui.semester_weeks must be >= 0, got %d", Path(), cfg.UI.SemesterWeeks)
	}
	if cfg.Heatmap.Weeks < 0 {
		return cfg, fmt.Errorf("config %s: heatmap.weeks must be >= 0, got %d", Path(), cfg.Heatmap.Weeks)
	}
	if cfg.Heatmap.Weeks == 0 {
		cfg.Heatmap.Weeks = 26
	}
	if cfg.Heatmap.Weeks > 52 {
		cfg.Heatmap.Weeks = 52
	}
	if cfg.Stats.StreakMinMinutes < 0 {
		return cfg, fmt.Errorf("config %s: stats.streak_min_minutes must be >= 0 (0 disables minute-based activity), got %d", Path(), cfg.Stats.StreakMinMinutes)
	}
	ambientNorm := strings.ToLower(strings.TrimSpace(cfg.UI.Ambient))
	switch ambientNorm {
	case "off", "splash", "home", "sidebar", "both":
		cfg.UI.Ambient = ambientNorm
	case "":
		cfg.UI.Ambient = "both"
	default:
		return cfg, fmt.Errorf("config %s: ui.ambient must be off|splash|sidebar|home|both, got %q", Path(), cfg.UI.Ambient)
	}
	if strings.TrimSpace(cfg.UI.Scene) == "" {
		cfg.UI.Scene = "auto"
	} else {
		known := map[string]bool{"auto": true, "random": true, "night": true, "bonsai": true, "ocean": true, "rain": true, "koi": true, "snow": true, "campfire": true}
		if !known[strings.ToLower(strings.TrimSpace(cfg.UI.Scene))] {
			return cfg, fmt.Errorf("config %s: ui.scene must be auto|random|night|bonsai|ocean|rain|koi|snow|campfire, got %q", Path(), cfg.UI.Scene)
		}
		cfg.UI.Scene = strings.ToLower(strings.TrimSpace(cfg.UI.Scene))
	}
	switch r := strings.ToLower(strings.TrimSpace(cfg.UI.AmbientRender)); r {
	case "auto", "halfblock", "braille", "ascii":
	case "":
		cfg.UI.AmbientRender = "auto"
	default:
		return cfg, fmt.Errorf("config %s: ui.ambient_render must be auto|halfblock|braille|ascii, got %q", Path(), cfg.UI.AmbientRender)
	}
	if cfg.UI.SplashSeconds <= 0 || cfg.UI.SplashSeconds > 30 {
		return cfg, fmt.Errorf("config %s: ui.splash_seconds must be in (0, 30], got %v", Path(), cfg.UI.SplashSeconds)
	}
	switch r := strings.ToLower(strings.TrimSpace(cfg.UI.AmbientIdle)); r {
	case "slow", "pause", "off":
		cfg.UI.AmbientIdle = r
	case "":
		cfg.UI.AmbientIdle = "slow"
	default:
		return cfg, fmt.Errorf("config %s: ui.ambient_idle must be slow|pause|off, got %q", Path(), cfg.UI.AmbientIdle)
	}
	if cfg.UI.AmbientFPS < 1 || cfg.UI.AmbientFPS > 30 {
		return cfg, fmt.Errorf("config %s: ui.ambient_fps must be in [1, 30], got %d", Path(), cfg.UI.AmbientFPS)
	}
	switch r := strings.ToLower(strings.TrimSpace(cfg.UI.AmbientDither)); r {
	case "on", "off":
	case "":
		cfg.UI.AmbientDither = "on"
	default:
		return cfg, fmt.Errorf("config %s: ui.ambient_dither must be on|off, got %q", Path(), cfg.UI.AmbientDither)
	}
	switch r := strings.ToLower(strings.TrimSpace(cfg.UI.Splash)); r {
	case "on", "off", "once-a-day":
		cfg.UI.Splash = r
	case "":
		cfg.UI.Splash = "on"
	default:
		return cfg, fmt.Errorf("config %s: ui.splash must be on|off|once-a-day, got %q", Path(), cfg.UI.Splash)
	}
	switch r := strings.ToLower(strings.TrimSpace(cfg.UI.TrueColor)); r {
	case "auto", "on", "off":
		cfg.UI.TrueColor = r
	case "":
		cfg.UI.TrueColor = "auto"
	default:
		return cfg, fmt.Errorf("config %s: ui.truecolor must be auto|on|off, got %q", Path(), cfg.UI.TrueColor)
	}
	// --no-ambient disables every surface for the run.
	if os.Getenv("MTM_NO_AMBIENT") == "1" {
		cfg.UI.Ambient = "off"
	}
	if tz := strings.TrimSpace(cfg.ICal.Timezone); tz != "" {
		if _, err := time.LoadLocation(tz); err != nil {
			return cfg, fmt.Errorf("config %s: ical.timezone %q is not a valid IANA name: %v", Path(), tz, err)
		}
		cfg.ICal.Timezone = tz
	}
	if cfg.Reminders.DueSoonMinutes < 0 {
		return cfg, fmt.Errorf("config %s: reminders.due_soon_minutes must be >= 0, got %d", Path(), cfg.Reminders.DueSoonMinutes)
	}
	if _, err := ParseQuietHours(cfg.Reminders.QuietHours); err != nil {
		return cfg, fmt.Errorf("config %s: reminders.quiet_hours: %v", Path(), err)
	}
	cfg.DataRepo = ExpandPath(cfg.DataRepo)
	paths.Guard(cfg.DataRepo)
	return cfg, nil
}

// DataRepoPath returns the expanded data repo path.
func (c Config) DataRepoPath() string { return ExpandPath(c.DataRepo) }

// EditorCmd resolves the editor: config, then $EDITOR, then $VISUAL, then vi.
func (c Config) EditorCmd() string {
	if c.Editor != "" {
		return c.Editor
	}
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	if e := os.Getenv("VISUAL"); e != "" {
		return e
	}
	return "vi"
}

// WriteDefault saves a fully-defaulted config file (used by mtm init).
func WriteDefault() error {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(Path()); err == nil {
		return fmt.Errorf("config already exists at %s", Path())
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	if err := enc.Encode(Default()); err != nil {
		return err
	}
	return os.WriteFile(Path(), buf.Bytes(), 0o600)
}
