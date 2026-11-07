// Package theme reads the active Omarchy theme (colors.toml, with the
// terminal config files as a fallback) and exposes a palette plus the
// fallback defaults. It never touches anything outside the state dir given
// to Load. Colors defined here are the only hardcoded colors in mtm;
// internal/tui builds all its lipgloss styles from Palette.
package theme

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Palette is the semantic color set the TUI styles from. All values are
// hex strings ("#rrggbb") or terminal color numbers when a terminal config
// provided them; lipgloss accepts either.
type Palette struct {
	Name       string
	Mode       string // "dark" | "light"
	Background string
	DarkerBg   string
	DarkBg     string
	LightBg    string
	Foreground string
	BrightFg   string
	Muted      string
	Accent     string
	Selection  string
	Red        string
	Green      string
	Yellow     string
	Blue       string
	Magenta    string
	Cyan       string
	Orange     string
}

// Defaults is the built-in palette used when the Omarchy theme files are
// missing or unreadable.
func Defaults() Palette {
	return Palette{
		Name:       "mtm-default",
		Mode:       "dark",
		Background: "#14282d",
		DarkerBg:   "#0c191e",
		DarkBg:     "#102126",
		LightBg:    "#253c41",
		Foreground: "#e8dfc4",
		BrightFg:   "#fff2d5",
		Muted:      "#a1afa9",
		Accent:     "#d3ae78",
		Selection:  "#3d5960",
		Red:        "#d78f79",
		Green:      "#a8b887",
		Yellow:     "#dfc181",
		Blue:       "#91afc5",
		Magenta:    "#b5a0ba",
		Cyan:       "#86b8b5",
		Orange:     "#d7a06c",
	}
}

// StateDir returns ~/.local/state/omarchy, honoring OMARCHY_STATE_HOME.
func StateDir() string {
	if d := os.Getenv("OMARCHY_STATE_HOME"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "state", "omarchy")
	}
	return filepath.Join(home, ".local", "state", "omarchy")
}

func themeDir(stateDir string) string { return filepath.Join(stateDir, "current", "theme") }
func nameFile(stateDir string) string { return filepath.Join(stateDir, "current", "theme.name") }

// Load reads the active theme under stateDir. It always returns a usable
// palette: on any failure it falls back to the defaults (and a non-nil
// error describing what happened).
func Load() (Palette, error) { return LoadFrom(StateDir()) }

// LoadFrom is Load with an explicit state dir (used by tests).
func LoadFrom(stateDir string) (Palette, error) {
	name, _ := readName(stateDir)
	colors := filepath.Join(themeDir(stateDir), "colors.toml")
	if data, err := os.ReadFile(colors); err == nil {
		if p, ok := parseColorsTOML(data, name); ok {
			return p, nil
		}
	}
	// Terminal-config fallback: best-effort background/foreground/selection.
	p := Defaults()
	if name != "" {
		p.Name = name
	}
	if bg, fg, sel, ok := readTerminalFallback(themeDir(stateDir)); ok {
		p.Background, p.Foreground = bg, fg
		if sel != "" {
			p.Selection = sel
		}
		return p, fmt.Errorf("colors.toml missing or unreadable — used terminal config fallback")
	}
	return p, fmt.Errorf("theme files missing under %s — used built-in defaults", themeDir(stateDir))
}

func readName(stateDir string) (string, error) {
	data, err := os.ReadFile(nameFile(stateDir))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

var tomlLine = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(?:"([^"]*)"|([^\s]+))`)

func parseColorsTOML(data []byte, name string) (Palette, bool) {
	m := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}
		if mm := tomlLine.FindStringSubmatch(line); mm != nil {
			v := mm[2]
			if v == "" {
				v = mm[3]
			}
			m[strings.ToLower(mm[1])] = strings.TrimSpace(v)
		}
	}
	p := Defaults()
	if name != "" {
		p.Name = name
	}
	set := func(key string, dst *string) {
		if v, ok := m[key]; ok {
			*dst = strings.TrimSpace(v)
		}
	}
	set("mode", &p.Mode)
	set("background", &p.Background)
	set("darker_background", &p.DarkerBg)
	set("dark_background", &p.DarkBg)
	set("lighter_background", &p.LightBg)
	set("foreground", &p.Foreground)
	set("bright_foreground", &p.BrightFg)
	set("muted", &p.Muted)
	set("accent", &p.Accent)
	set("selection", &p.Selection)
	set("red", &p.Red)
	set("green", &p.Green)
	set("yellow", &p.Yellow)
	set("blue", &p.Blue)
	set("magenta", &p.Magenta)
	set("cyan", &p.Cyan)
	set("orange", &p.Orange)
	// A theme without at least background+foreground is not usable.
	if _, ok := m["background"]; !ok {
		return p, false
	}
	if _, ok := m["foreground"]; !ok {
		return p, false
	}
	return p, true
}

var hexRe = regexp.MustCompile(`#[0-9a-fA-F]{6}\b`)

// readTerminalFallback scrapes background/foreground/selection from the
// terminal config files present in the theme dir (alacritty.toml, kitty.conf,
// foot.ini, ghostty.conf). Best-effort; returns false if nothing is found.
func readTerminalFallback(dir string) (bg, fg, sel string, ok bool) {
	tryFile := func(path string, parse func([]byte) (string, string, string, bool)) bool {
		data, err := os.ReadFile(filepath.Join(dir, path))
		if err != nil {
			return false
		}
		var found bool
		bg, fg, sel, found = parse(data)
		return found
	}
	parseAlacritty := func(data []byte) (string, string, string, bool) {
		var b, f, s string
		sections := strings.Split(string(data), "[")
		for _, sec := range sections {
			switch {
			case strings.HasPrefix(sec, "colors.primary"):
				b = firstHex(sec, "background")
				f = firstHex(sec, "foreground")
			case strings.HasPrefix(sec, "colors.selection"):
				s = firstHex(sec, "background")
			}
		}
		return b, f, s, b != "" && f != ""
	}
	parseConf := func(data []byte) (string, string, string, bool) {
		var b, f, s string
		for _, line := range strings.Split(string(data), "\n") {
			lower := strings.ToLower(strings.TrimSpace(line))
			if lower == "" || strings.HasPrefix(lower, "#") || strings.HasPrefix(lower, ";") {
				continue
			}
			hex := hexRe.FindString(line)
			switch {
			case strings.HasPrefix(lower, "background") && hex != "":
				b = hex
			case strings.HasPrefix(lower, "foreground") && hex != "":
				f = hex
			case (strings.HasPrefix(lower, "selection") || strings.HasPrefix(lower, "selection_background")) && hex != "":
				s = hex
			}
		}
		return b, f, s, b != "" && f != ""
	}
	for _, pf := range []struct {
		name  string
		parse func([]byte) (string, string, string, bool)
	}{
		{"alacritty.toml", parseAlacritty},
		{"kitty.conf", parseConf},
		{"foot.ini", parseConf},
		{"ghostty.conf", parseConf},
	} {
		if tryFile(pf.name, pf.parse) {
			return bg, fg, sel, true
		}
	}
	return "", "", "", false
}

func firstHex(section, key string) string {
	re := regexp.MustCompile(key + `\s*=\s*"?(#[0-9a-fA-F]{6})"?`)
	if m := re.FindStringSubmatch(section); m != nil {
		return m[1]
	}
	return ""
}

// PollModTime returns a fingerprint of the theme state: the newest mtime
// among the state dir, theme.name and colors.toml (paths follow symlinks).
// Pass the previous fingerprint; a different value means the theme changed.
func PollModTime(stateDir string) time.Time {
	var latest time.Time
	for _, p := range []string{
		filepath.Join(stateDir, "current"),
		nameFile(stateDir),
		filepath.Join(themeDir(stateDir), "colors.toml"),
	} {
		if fi, err := os.Stat(p); err == nil {
			if fi.ModTime().After(latest) {
				latest = fi.ModTime()
			}
		}
	}
	return latest
}
