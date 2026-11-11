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
	"strconv"
	"strings"
	"sync"
	"time"
)

// Palette is the semantic color set the TUI styles from. All values are
// hex strings ("#rrggbb") or terminal color numbers when a terminal config
// provided them; lipgloss accepts either. Raw Omarchy values live in the
// first block; the semantic roles below are computed by Derive() at load
// time and are the only roles the TUI should use.
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
	Brown      string

	// Semantic roles (derived).
	Base          string
	Surface       string
	SurfaceRaised string
	SurfaceStrong string
	Border        string
	Text          string
	BrightText    string
	MutedSoft     string
	AccentSoft    string
	Success       string
	Warning       string
	Danger        string

	HeatWarm    string
	HeatHot     string
	HeatOverdue string

	CoursePalette []string
	Heatmap       []string
}

// Defaults is the built-in palette used when the Omarchy theme files are
// missing or unreadable.
func Defaults() Palette {
	p := Palette{
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
		Brown:      "#b79b7f",
	}
	p.Derive()
	return p
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

var (
	diagMu sync.Mutex
	diags  []string
)

// recordThemeDiagnostic appends one human-readable diagnostic (deduped).
func recordThemeDiagnostic(s string) {
	diagMu.Lock()
	defer diagMu.Unlock()
	for _, d := range diags {
		if d == s {
			return
		}
	}
	diags = append(diags, s)
}

// Diagnostics returns the diagnostics recorded since the last reset.
func Diagnostics() []string {
	diagMu.Lock()
	defer diagMu.Unlock()
	return append([]string{}, diags...)
}

// ResetDiagnostics clears the diagnostics list.
func ResetDiagnostics() {
	diagMu.Lock()
	defer diagMu.Unlock()
	diags = nil
}

const hexDigits = "0123456789abcdef"

func isHexN(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !strings.ContainsRune(hexDigits, rune(strings.ToLower(s)[i])) {
			return false
		}
	}
	return true
}

// NormalizeHexColor parses the accepted color spellings and returns
// canonical #rrggbb:
//
//	#rgb, #rrggbb, 0xRGB, 0xRRGGBB, rgb:r/g/b with 1–4 hex digits per
//	component (X11-style, proportionally scaled)
//
// Anything else reports ok=false.
func NormalizeHexColor(v string) (string, bool) {
	v = strings.TrimSpace(v)
	lower := strings.ToLower(v)
	scale := func(part string) (int, bool) {
		if len(part) < 1 || len(part) > 4 || !isHexN(strings.ToLower(part), len(part)) {
			return 0, false
		}
		maxV := uint64(1)<<(uint(4*len(part))) - 1
		x, err := strconv.ParseUint(strings.ToLower(part), 16, 64)
		if err != nil {
			return 0, false
		}
		return int((x*255 + maxV/2) / maxV), true
	}
	switch {
	case strings.HasPrefix(v, "#"):
		body := v[1:]
		switch len(body) {
		case 3:
			if !isHexN(strings.ToLower(body), 3) {
				return "", false
			}
			return "#" + strings.Repeat(string(strings.ToLower(body)[0]), 2) +
				strings.Repeat(string(strings.ToLower(body)[1]), 2) +
				strings.Repeat(string(strings.ToLower(body)[2]), 2), true
		case 6:
			if !isHexN(strings.ToLower(body), 6) {
				return "", false
			}
			return "#" + strings.ToLower(body), true
		}
		return "", false
	case strings.HasPrefix(lower, "0x"):
		body := v[2:]
		switch len(body) {
		case 3:
			if !isHexN(strings.ToLower(body), 3) {
				return "", false
			}
			return "#" + strings.Repeat(string(strings.ToLower(body)[0]), 2) +
				strings.Repeat(string(strings.ToLower(body)[1]), 2) +
				strings.Repeat(string(strings.ToLower(body)[2]), 2), true
		case 6:
			if !isHexN(strings.ToLower(body), 6) {
				return "", false
			}
			return "#" + strings.ToLower(body), true
		}
		return "", false
	case strings.HasPrefix(lower, "rgb:"):
		parts := strings.Split(v[4:], "/")
		if len(parts) != 3 {
			return "", false
		}
		out := make([]int, 3)
		for i, part := range parts {
			c, ok := scale(strings.TrimSpace(part))
			if !ok {
				return "", false
			}
			out[i] = c
		}
		return fmt.Sprintf("#%02x%02x%02x", out[0], out[1], out[2]), true
	}
	return "", false
}

func normalizeOrDefault(v, fallback, role string) string {
	norm, ok := NormalizeHexColor(v)
	if ok {
		return norm
	}
	recordThemeDiagnostic(fmt.Sprintf("role %s malformed, using fallback (got %q)", role, v))
	return fallback
}

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
		if n, ok := NormalizeHexColor(bg); ok {
			p.Background = n
		}
		if n, ok := NormalizeHexColor(fg); ok {
			p.Foreground = n
		}
		if sel != "" {
			if n, ok := NormalizeHexColor(sel); ok {
				p.Selection = n
			}
		}
		p.Derive()
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
	if v, ok := m["mode"]; ok {
		p.Mode = strings.TrimSpace(v) // mode is not a color
	}
	set := func(key string, dst *string) {
		if v, ok := m[key]; ok {
			if norm, ok := NormalizeHexColor(v); ok {
				*dst = norm
				return
			}
			recordThemeDiagnostic(fmt.Sprintf("role %s malformed, using fallback (got %q)", key, v))
			// keep the Defaults() value for this role
		}
	}
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
	set("brown", &p.Brown)
	// A theme without at least background+foreground is not usable.
	if _, ok := m["background"]; !ok {
		return p, false
	}
	if _, ok := m["foreground"]; !ok {
		return p, false
	}
	p.Derive()
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
