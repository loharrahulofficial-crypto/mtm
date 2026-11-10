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
func Detect(getenv func(string) string) Terminal {
	t := Terminal{Program: "unknown", LimitedPalette: true}
	get := func(k string) string { return strings.TrimSpace(getenv(k)) }
	lower := func(k string) string { return strings.ToLower(get(k)) }

	termProgram := lower("TERM_PROGRAM")
	term := lower("TERM")

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
		t = Terminal{Program: "mlterm", Sixel: true}
	case strings.HasPrefix(term, "tmux"):
		t = Terminal{Program: "tmux"}
	case strings.HasPrefix(term, "screen"):
		t = Terminal{Program: "screen"}
	case term == "linux":
		t = Terminal{Program: "linux"}
	case term == "dumb":
		t = Terminal{Program: "dumb", TrueColor: false}
	}

	// COLORTERM upgrades to truecolor when the palette was not pinned
	// down to something smaller.
	ct := lower("COLORTERM")
	if ct == "truecolor" || ct == "24bit" {
		t.TrueColor = true
	}

	// A 256-color xterm without truecolor hints is palette-limited.
	t.LimitedPalette = !t.TrueColor
	return t
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
