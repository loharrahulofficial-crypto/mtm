package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeState(t *testing.T, dir string, colorsToml string) {
	t.Helper()
	themeDir := filepath.Join(dir, "current", "theme")
	if err := os.MkdirAll(themeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(themeDir, "colors.toml"), []byte(colorsToml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "current", "theme.name"), []byte("outpost\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

const sampleColors = `mode = "dark"
accent = "#d3ae78"
selection = "#3d5960"
muted = "#a1afa9"
background = "#14282d"
darker_background = "#0c191e"
dark_background = "#102126"
lighter_background = "#253c41"
foreground = "#e8dfc4"
red = "#d78f79"
green = "#a8b887"
yellow = "#dfc181"
blue = "#91afc5"
`

func TestLoadColorsTOML(t *testing.T) {
	dir := t.TempDir()
	writeState(t, dir, sampleColors)
	p, err := LoadFrom(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "outpost" {
		t.Errorf("name = %q, want outpost", p.Name)
	}
	if p.Background != "#14282d" || p.Foreground != "#e8dfc4" || p.Accent != "#d3ae78" {
		t.Errorf("colors not parsed: %+v", p)
	}
	if p.DarkerBg != "#0c191e" || p.LightBg != "#253c41" {
		t.Errorf("bg variants not parsed: %+v", p)
	}
	// Unspecified keys fall back to defaults, not empty strings.
	if p.Magenta == "" || p.Orange == "" {
		t.Errorf("default missing for unspecified key: %+v", p)
	}
}

func TestLoadMissingFilesFallsBackToDefaults(t *testing.T) {
	p, err := LoadFrom(t.TempDir())
	if err == nil {
		t.Fatal("expected an error describing the fallback")
	}
	if p.Background != Defaults().Background || p.Foreground != Defaults().Foreground {
		t.Fatalf("fallback palette not defaults: %+v", p)
	}
}

func TestLoadTerminalFallback(t *testing.T) {
	dir := t.TempDir()
	themeDir := filepath.Join(dir, "current", "theme")
	if err := os.MkdirAll(themeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	alacritty := "[colors.primary]\nbackground = \"#101010\"\nforeground = \"#eeeeee\"\n[colors.selection]\nbackground = \"#222233\"\n"
	if err := os.WriteFile(filepath.Join(themeDir, "alacritty.toml"), []byte(alacritty), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "current", "theme.name"), []byte("fromterm\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := LoadFrom(dir)
	if err == nil {
		t.Fatal("expected fallback notice error")
	}
	if p.Background != "#101010" || p.Foreground != "#eeeeee" || p.Selection != "#222233" {
		t.Fatalf("terminal fallback wrong: %+v", p)
	}
	if p.Name != "fromterm" {
		t.Fatalf("name = %q", p.Name)
	}
}

func TestPollModTimeChanges(t *testing.T) {
	dir := t.TempDir()
	writeState(t, dir, sampleColors)
	before := PollModTime(dir)
	if before.IsZero() {
		t.Fatal("expected non-zero fingerprint")
	}
	// Touch a newer mtime on colors.toml (explicit: filesystem granularity
	// can equal the original mtime on fast machines).
	colors := filepath.Join(dir, "current", "theme", "colors.toml")
	if err := os.WriteFile(colors, []byte(sampleColors+"# comment\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(colors, future, future); err != nil {
		t.Fatal(err)
	}
	after := PollModTime(dir)
	if !after.After(before) {
		t.Fatalf("fingerprint did not advance: before %v after %v", before, after)
	}
}

func TestColorsTOMLUnhashedAndHashedValues(t *testing.T) {
	p, ok := parseColorsTOML([]byte("background = #101010\nforeground = \"#202020\"\n"), "t")
	if !ok {
		t.Fatal("expected unhashed values to parse")
	}
	if p.Background != "#101010" {
		t.Fatalf("unquoted hex: %q", p.Background)
	}
	if !strings.HasPrefix(p.Foreground, "#") {
		t.Fatalf("quoted hex: %q", p.Foreground)
	}
}
