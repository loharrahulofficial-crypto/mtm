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
	"gopkg.in/yaml.v3"
)

type Config struct {
	DataRepo       string `yaml:"data_repo"`
	DefaultProject string `yaml:"default_project"`
	Editor         string `yaml:"editor"`
	DateFormat     string `yaml:"date_format"`
	WeekStart      string `yaml:"week_start"`
	// Sort is the task-list sort order: done|due|priority|title, applied in
	// order. Empty means the built-in default (not-done, due, priority, title).
	Sort       []string       `yaml:"sort"`
	Sync       SyncConfig     `yaml:"sync"`
	Pomodoro   PomodoroConfig `yaml:"pomodoro"`
	SavedViews []SavedView    `yaml:"saved_views"`
	// Keys overrides TUI key bindings: action name -> key(s), space-separated.
	Keys map[string]string `yaml:"keys,omitempty"`
	UI   UIConfig          `yaml:"ui"`
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

// UIConfig carries the "ui:" presentation block (Study Desk design language).
type UIConfig struct {
	// Animations caps every animation at 400ms and honors reduce-motion
	// preferences. Default true.
	Animations bool `yaml:"animations"`
	// Icons selects the glyph set: nerd (Nerd Font, default) | unicode | none.
	Icons string `yaml:"icons"`
	// SemesterStart anchors the brand header's "Week N of M". Empty hides it.
	SemesterStart string `yaml:"semester_start,omitempty"` // 2006-01-02
	SemesterWeeks int    `yaml:"semester_weeks,omitempty"` // default 16
}

func Default() Config {
	return Config{
		DataRepo:       DataHome(),
		DefaultProject: "inbox",
		Editor:         "", // resolved from $EDITOR/$VISUAL/vi at use site
		DateFormat:     "2006-01-02",
		WeekStart:      "monday",
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
		UI: UIConfig{
			Animations:    true,
			Icons:         "nerd",
			SemesterWeeks: 16,
		},
	}
}

// Dir returns the config directory, honoring XDG_CONFIG_HOME.
func Dir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "mtm")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", "mtm")
	}
	return filepath.Join(home, ".config", "mtm")
}

// Path returns the config file path.
func Path() string { return filepath.Join(Dir(), "config.yaml") }

// DataHome returns the default data directory, honoring XDG_DATA_HOME.
func DataHome() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "mtm")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "share", "mtm")
	}
	return filepath.Join(home, ".local", "share", "mtm")
}

// CacheDir returns the cache directory, honoring XDG_CACHE_HOME.
func CacheDir() string {
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "mtm")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".cache", "mtm")
	}
	return filepath.Join(home, ".cache", "mtm")
}

// CachePath returns the index cache file path (D-006: outside the data repo).
func CachePath() string { return filepath.Join(CacheDir(), "index.json") }

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
	if cfg.UI.SemesterStart != "" {
		if _, err := time.Parse("2006-01-02", cfg.UI.SemesterStart); err != nil {
			return cfg, fmt.Errorf("config %s: ui.semester_start must be 2006-01-02, got %q", Path(), cfg.UI.SemesterStart)
		}
	}
	if cfg.UI.SemesterWeeks < 0 {
		return cfg, fmt.Errorf("config %s: ui.semester_weeks must be >= 0, got %d", Path(), cfg.UI.SemesterWeeks)
	}
	cfg.DataRepo = ExpandPath(cfg.DataRepo)
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
