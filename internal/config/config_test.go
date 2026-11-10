package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixedXDG redirects every mtm root to temp dirs; required since Load
// resolves all directories through internal/paths with the sandbox guard.
func fixedXDG(t *testing.T) {
	t.Helper()
	base := t.TempDir()
	for _, pair := range [][2]string{
		{"XDG_CONFIG_HOME", filepath.Join(base, "config")},
		{"XDG_DATA_HOME", filepath.Join(base, "data")},
		{"XDG_CACHE_HOME", filepath.Join(base, "cache")},
		{"XDG_STATE_HOME", filepath.Join(base, "state")},
	} {
		t.Setenv(pair[0], pair[1])
	}
	t.Setenv("HOME", base)
}

func TestDefaults(t *testing.T) {
	fixedXDG(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultProject != "inbox" || !cfg.Sync.Auto || cfg.Pomodoro.WorkMinutes != 25 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if !strings.HasSuffix(cfg.DataRepo, filepath.Join("mtm")) {
		t.Fatalf("data repo = %q", cfg.DataRepo)
	}
}

func TestFileOverridesDefaults(t *testing.T) {
	fixedXDG(t)
	xdg := os.Getenv("XDG_CONFIG_HOME")
	dir := filepath.Join(xdg, "mtm")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "default_project: college/dbms\nsync:\n  auto: false\n  debounce_seconds: 10\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultProject != "college/dbms" || cfg.Sync.Auto || cfg.Sync.DebounceSeconds != 10 {
		t.Fatalf("override failed: %+v", cfg)
	}
	// Unspecified keys keep defaults.
	if cfg.Pomodoro.WorkMinutes != 25 {
		t.Fatalf("default lost: %+v", cfg.Pomodoro)
	}
}

func TestSortConfigValidation(t *testing.T) {
	fixedXDG(t)
	xdg := os.Getenv("XDG_CONFIG_HOME")
	write := func(content string) {
		t.Helper()
		dir := filepath.Join(xdg, "mtm")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("sort: [due, title]\n")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Sort) != 2 || cfg.Sort[0] != "due" || cfg.Sort[1] != "title" {
		t.Fatalf("sort = %v", cfg.Sort)
	}
	if err := os.Remove(filepath.Join(xdg, "mtm", "config.yaml")); err != nil {
		t.Fatal(err)
	}
	write("sort: [urgency]\n")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for unknown sort key")
	}
}

func TestCorruptConfigFails(t *testing.T) {
	fixedXDG(t)
	xdg := os.Getenv("XDG_CONFIG_HOME")
	dir := filepath.Join(xdg, "mtm")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("\tbad: [yaml"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("expected error for corrupt config")
	}
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	if got := ExpandPath("~/foo/bar"); got != filepath.Join(home, "foo/bar") {
		t.Errorf("~/foo/bar -> %q", got)
	}
	if got := ExpandPath("/abs/path"); got != "/abs/path" {
		t.Errorf("/abs/path -> %q", got)
	}
	if got := ExpandPath("relative"); got != "relative" {
		t.Errorf("relative -> %q", got)
	}
}

func TestEditorCmdFallback(t *testing.T) {
	fixedXDG(t)
	t.Setenv("EDITOR", "")
	t.Setenv("VISUAL", "")
	cfg := Default()
	if got := cfg.EditorCmd(); got != "vi" {
		t.Errorf("EditorCmd() = %q, want vi", got)
	}
	cfg.Editor = "micro"
	if got := cfg.EditorCmd(); got != "micro" {
		t.Errorf("EditorCmd() = %q, want micro", got)
	}
}

func TestUIConfigDefaults(t *testing.T) {
	fixedXDG(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.UI.Animations || cfg.UI.Icons != "nerd" || cfg.UI.SemesterWeeks != 16 {
		t.Fatalf("ui defaults wrong: %+v", cfg.UI)
	}
}

func TestUIConfigParsingAndValidation(t *testing.T) {
	fixedXDG(t)
	xdg := os.Getenv("XDG_CONFIG_HOME")
	write := func(content string) {
		t.Helper()
		dir := filepath.Join(xdg, "mtm")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("ui:\n  animations: false\n  icons: unicode\n  semester_start: \"2026-09-01\"\n  semester_weeks: 18\n")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UI.Animations || cfg.UI.Icons != "unicode" || cfg.UI.SemesterStart != "2026-09-01" || cfg.UI.SemesterWeeks != 18 {
		t.Fatalf("ui parse wrong: %+v", cfg.UI)
	}
	write("ui:\n  icons: bogus\n")
	if _, err := Load(); err == nil {
		t.Fatal("invalid icons accepted")
	}
	write("ui:\n  icons: NERD\n")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UI.Icons != "nerd" {
		t.Fatalf("icons not normalized: %q", cfg.UI.Icons)
	}
	write("ui:\n  semester_start: \"07/11/2026\"\n")
	if _, err := Load(); err == nil {
		t.Fatal("invalid semester_start accepted")
	}
	write("ui:\n  density: bogus\n")
	if _, err := Load(); err == nil {
		t.Fatal("invalid density accepted")
	}
	write("ui:\n  density: COMPACT\n")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UI.Density != "compact" {
		t.Fatalf("density not normalized: %q", cfg.UI.Density)
	}
	write("ui:\n  density: comfortable\n")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UI.Density != "comfortable" {
		t.Fatalf("density = %q, want comfortable", cfg.UI.Density)
	}
}

func TestBoardDoneDaysDefaultAndOverride(t *testing.T) {
	fixedXDG(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Board.DoneDays != 14 {
		t.Fatalf("default board.done_days = %d, want 14", cfg.Board.DoneDays)
	}
	dir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "mtm")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("board:\n    done_days: 30\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Board.DoneDays != 30 {
		t.Fatalf("board.done_days = %d, want 30", cfg.Board.DoneDays)
	}
}

func TestHeatmapWeeksDefaultOverrideAndClamp(t *testing.T) {
	fixedXDG(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Heatmap.Weeks != 26 {
		t.Fatalf("default heatmap.weeks = %d, want 26", cfg.Heatmap.Weeks)
	}
	write := func(content string) {
		t.Helper()
		dir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "mtm")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("heatmap:\n    weeks: 52\n")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Heatmap.Weeks != 52 {
		t.Fatalf("heatmap.weeks = %d, want 52", cfg.Heatmap.Weeks)
	}
	write("heatmap:\n    weeks: 80\n")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Heatmap.Weeks != 52 {
		t.Fatalf("heatmap.weeks = %d, want clamped 52", cfg.Heatmap.Weeks)
	}
	write("heatmap:\n    weeks: -1\n")
	if _, err := Load(); err == nil {
		t.Fatal("negative heatmap.weeks accepted")
	}
}

func TestAmbientDitherValidation(t *testing.T) {
	fixedXDG(t)
	dir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "mtm")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	load := func() (Config, error) { return Load() }

	// default is on
	write("default_project: inbox\n")
	cfg, err := load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UI.AmbientDither != "on" {
		t.Fatalf("default ambient_dither = %q, want on", cfg.UI.AmbientDither)
	}
	// off accepted
	write("ui:\n  ambient_dither: off\n")
	cfg, err = load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UI.AmbientDither != "off" {
		t.Fatalf("ambient_dither = %q, want off", cfg.UI.AmbientDither)
	}
	// bogus rejected
	write("ui:\n  ambient_dither: maybe\n")
	if _, err := load(); err == nil {
		t.Fatal("ambient_dither: maybe was accepted")
	}
}

func TestUIAmbientEnumAndSidebarWindow(t *testing.T) {
	fixedXDG(t)
	dir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "mtm")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	load := func() (Config, error) { return Load() }

	// default: both, sidebar window on
	write("default_project: inbox\n")
	cfg, err := load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UI.Ambient != "both" || !cfg.UI.AmbientSidebar {
		t.Fatalf("defaults = ambient %q ambient_sidebar %v, want both/true", cfg.UI.Ambient, cfg.UI.AmbientSidebar)
	}

	for _, v := range []string{"off", "splash", "sidebar", "home", "both", "BOTH"} {
		write("ui:\n  ambient: " + v + "\n")
		cfg, err = load()
		if err != nil {
			t.Fatalf("ambient %q rejected: %v", v, err)
		}
	}

	// home keeps its meaning (no remap to both)
	write("ui:\n  ambient: home\n")
	cfg, err = load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UI.Ambient != "home" {
		t.Fatalf("ambient home migrated to %q — home must stay home", cfg.UI.Ambient)
	}

	// bogus rejected
	write("ui:\n  ambient: corner\n")
	if _, err := load(); err == nil {
		t.Fatal("ambient: corner was accepted")
	}

	// empty string defaults to both
	write("ui:\n  ambient: \"\"\n")
	cfg, err = load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UI.Ambient != "both" {
		t.Fatalf("empty ambient = %q, want both", cfg.UI.Ambient)
	}

	// ui.ambient_sidebar false round-trips
	write("ui:\n  ambient_sidebar: false\n")
	cfg, err = load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UI.AmbientSidebar {
		t.Fatal("ambient_sidebar: false was not honored")
	}
}
