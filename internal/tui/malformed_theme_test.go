package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

// malformed_theme_test.go — §6.1 end-to-end: a malformed colors.toml must
// never leak the 255,0,255 sentinel (or #ff00ff) into a dumped frame; the
// malformed roles fall back to the defaults with diagnostics recorded.
// The campfire scene paints Warning/Yellow/Orange heavily, so the
// fallback path is exercised every frame.

func writeMalformedTheme(t *testing.T, dir string) {
	t.Helper()
	themeDir := filepath.Join(dir, "current", "theme")
	if err := os.MkdirAll(themeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	toml := `mode = "dark"
background = "#14282d"
darker_background = "#0c191e"
dark_background = "#102126"
lighter_background = "#253c41"
foreground = "#e8dfc4"
bright_foreground = "#fff2d5"
muted = "#a1afa9"
accent = "not-a-color"
selection = "#3d5960"
warning = "bogus"
yellow = "#dfc18g"
orange = "trailMix"
green = "#a8b887"
blue = "#91afc5"
cyan = "#86b8b5"
red = "#d78f79"
magenta = "#b5a0ba"
brown = "#b79b7f"
`
	if err := os.WriteFile(filepath.Join(themeDir, "colors.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
}

func malformedDumpConfig() config.Config {
	cfg := dumpConfig()
	cfg.UI.Scene = "campfire"
	cfg.UI.AmbientSidebar = true
	return cfg
}

func TestDumpWithMalformedThemeEmitsNoMagenta(t *testing.T) {
	fixedEnv(t)
	lipgloss.SetColorProfile(termenv.TrueColor)
	om := t.TempDir()
	writeMalformedTheme(t, om)
	t.Setenv("OMARCHY_STATE_HOME", om)

	s := emptyFixture(t)
	defer func() { _ = s.Close() }()
	m := BuildDumpModel(malformedDumpConfig(), s, sync.New(s.Root), "home", 160, 45, DumpFixedTime)
	frame := m.View()
	for _, sentinel := range []string{"255;0;255", "ff00ff", "FF00FF"} {
		if strings.Contains(frame, sentinel) {
			t.Fatalf("dumped frame contains sentinel %q", sentinel)
		}
	}
}
