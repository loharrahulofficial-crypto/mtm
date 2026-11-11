// Package caps detects terminal capabilities from the environment: the
// program, 24-bit color, limited palettes, and graphics protocol support.
// Nothing here changes rendering behavior by itself — it exists so a
// graphics renderer can plug in later, and so the ambient pipeline knows
// when a terminal cannot show what it just drew.
package caps

import (
	"os"
	"strings"
	"sync"
)

// Terminal describes what the ambient renderer may rely on.
type Terminal struct {
	Program        string // ghostty | kitty | wezterm | alacritty | foot | tmux | screen | ...
	TrueColor      bool   // 24-bit color
	LimitedPalette bool   // no 24-bit color (256 or fewer)
	Sixel          bool   // Sixel raster protocol
	KittyGraphics  bool   // kitty graphics protocol (APC)
}

// Detect inspects getenv (usually os.Getenv) and reports capabilities.
// The default is truecolor: only positive evidence of a limited terminal
// (TERM=linux, TERM=dumb, NO_COLOR set, Apple Terminal, or TERM=screen*
// without COLORTERM) reports a limited palette. No hints at all — a
// keybind launch with a minimal env, for example — means truecolor.
func Detect(getenv func(string) string) Terminal {
	t := Terminal{Program: "unknown", TrueColor: true}
	get := func(k string) string { return strings.TrimSpace(getenv(k)) }
	lower := func(k string) string { return strings.ToLower(get(k)) }

	termProgram := lower("TERM_PROGRAM")
	term := lower("TERM")
	ct := lower("COLORTERM")

	// Dumb terminals get nothing, no matter what else is set.
	if term == "dumb" {
		return Terminal{Program: "dumb", LimitedPalette: true}
	}

	switch {
	case get("GHOSTTY_RESOURCES_DIR") != "" || termProgram == "ghostty":
		t = Terminal{Program: "ghostty", TrueColor: true, KittyGraphics: true}
	case get("KITTY_WINDOW_ID") != "" || termProgram == "kitty" || term == "xterm-kitty":
		t = Terminal{Program: "kitty", TrueColor: true, KittyGraphics: true}
	case termProgram == "wezterm":
		t = Terminal{Program: "wezterm", TrueColor: true, KittyGraphics: true, Sixel: true}
	case termProgram == "alacritty" || term == "alacritty":
		t = Terminal{Program: "alacritty", TrueColor: true}
	case termProgram == "iterm.app" || termProgram == "iterm2":
		t = Terminal{Program: "iterm", TrueColor: true}
	case termProgram == "apple_terminal":
		t = Terminal{Program: "apple_terminal"}
	case term == "foot" || term == "foot-extra":
		t = Terminal{Program: "foot", TrueColor: true, Sixel: true}
	case term == "mlterm":
		t = Terminal{Program: "mlterm", TrueColor: true, Sixel: true}
	case strings.HasPrefix(term, "tmux"):
		// Product decision (pinned by caps_test.go): tmux is assumed
		// truecolor-capable; limited tmux outers get 24-bit quantized by
		// the scene's palette path instead of by caps.
		t = Terminal{Program: "tmux", TrueColor: true}
	case strings.HasPrefix(term, "screen"):
		t = Terminal{Program: "screen"}
	case term == "linux":
		t = Terminal{Program: "linux"}
	}

	// COLORTERM upgrades to truecolor when the palette was not pinned
	// down to something smaller.
	if (ct == "truecolor" || ct == "24bit") && get("NO_COLOR") == "" {
		t.TrueColor = true
	}

	// Positive evidence of a limited palette, strongest first.
	if get("NO_COLOR") != "" ||
		term == "linux" ||
		termProgram == "apple_terminal" ||
		(strings.HasPrefix(term, "screen") && ct == "") {
		t.TrueColor = false
	}

	t.LimitedPalette = !t.TrueColor
	return t
}

// Effective resolves the effective color mode: MTM_TRUECOLOR (1|0) wins,
// then the ui.truecolor config value (auto|on|off), else detection. It
// reports whether truecolor is enabled and where the decision came from.
func Effective(uiSetting string, getenv func(string) string) (bool, string) {
	if v := strings.TrimSpace(getenv("MTM_TRUECOLOR")); v != "" {
		switch v {
		case "1":
			return true, "MTM_TRUECOLOR=1"
		case "0":
			return false, "MTM_TRUECOLOR=0"
		}
		// invalid values are ignored, not fatal
	}
	switch strings.ToLower(strings.TrimSpace(uiSetting)) {
	case "on":
		return true, "ui.truecolor=on"
	case "off":
		return false, "ui.truecolor=off"
	}
	d := Detect(getenv)
	if d.TrueColor {
		return true, "auto (detected " + d.Program + ")"
	}
	return false, "auto (detected " + d.Program + ")"
}

var (
	currentOnce sync.Once
	current     Terminal
)

// Current returns the detected terminal, cached for the process lifetime.
func Current() Terminal {
	currentOnce.Do(func() { current = Detect(os.Getenv) })
	return current
}
