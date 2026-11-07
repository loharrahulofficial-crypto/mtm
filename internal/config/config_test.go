package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
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
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
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

func TestCorruptConfigFails(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
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
