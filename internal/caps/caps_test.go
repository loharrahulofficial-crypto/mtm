package caps

import (
	"strings"
	"testing"
)

func TestDetect(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want Terminal
	}{
		{"ghostty by env dir", map[string]string{"GHOSTTY_RESOURCES_DIR": "/opt/ghostty"}, Terminal{"ghostty", true, false, false, true}},
		{"ghostty by program", map[string]string{"TERM_PROGRAM": "ghostty", "TERM": "xterm-256color"}, Terminal{"ghostty", true, false, false, true}},
		{"kitty by env id", map[string]string{"KITTY_WINDOW_ID": "1"}, Terminal{"kitty", true, false, false, true}},
		{"kitty by term", map[string]string{"TERM": "xterm-kitty"}, Terminal{"kitty", true, false, false, true}},
		{"wezterm", map[string]string{"TERM_PROGRAM": "WezTerm"}, Terminal{"wezterm", true, false, true, true}},
		{"alacritty", map[string]string{"TERM_PROGRAM": "alacritty"}, Terminal{"alacritty", true, false, false, false}},
		{"iterm", map[string]string{"TERM_PROGRAM": "iTerm.app"}, Terminal{"iterm", true, false, false, false}},
		{"apple terminal", map[string]string{"TERM_PROGRAM": "Apple_Terminal"}, Terminal{"apple_terminal", false, true, false, false}},
		{"foot", map[string]string{"TERM": "foot"}, Terminal{"foot", true, false, true, false}},
		{"mlterm", map[string]string{"TERM": "mlterm"}, Terminal{"mlterm", true, false, true, false}},
		{"tmux 256 no hints", map[string]string{"TERM": "tmux-256color"}, Terminal{"tmux", true, false, false, false}},
		{"tmux truecolor", map[string]string{"TERM": "tmux-256color", "COLORTERM": "truecolor"}, Terminal{"tmux", true, false, false, false}},
		{"screen", map[string]string{"TERM": "screen"}, Terminal{"screen", false, true, false, false}},
		{"screen-256color", map[string]string{"TERM": "screen-256color"}, Terminal{"screen", false, true, false, false}},
		{"screen with COLORTERM upgrades", map[string]string{"TERM": "screen", "COLORTERM": "truecolor"}, Terminal{"screen", true, false, false, false}},
		{"linux console", map[string]string{"TERM": "linux"}, Terminal{"linux", false, true, false, false}},
		{"dumb wins over everything", map[string]string{"TERM": "dumb", "COLORTERM": "truecolor", "GHOSTTY_RESOURCES_DIR": "/x"}, Terminal{"dumb", false, true, false, false}},
		{"xterm 256 with no hints is truecolor", map[string]string{"TERM": "xterm-256color"}, Terminal{"unknown", true, false, false, false}},
		{"xterm truecolor via COLORTERM", map[string]string{"TERM": "xterm-256color", "COLORTERM": "24bit"}, Terminal{"unknown", true, false, false, false}},
		{"empty env defaults to truecolor", map[string]string{}, Terminal{"unknown", true, false, false, false}},
		{"keybind launch with minimal env", map[string]string{"TERM": ""}, Terminal{"unknown", true, false, false, false}},
		{"NO_COLOR pins limited", map[string]string{"TERM": "xterm-256color", "NO_COLOR": "1"}, Terminal{"unknown", false, true, false, false}},
		{"NO_COLOR with COLORTERM stays limited", map[string]string{"TERM": "xterm-256color", "NO_COLOR": "1", "COLORTERM": "truecolor"}, Terminal{"unknown", false, true, false, false}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := Detect(func(k string) string { return tc.env[k] })
			if v != tc.want {
				t.Fatalf("Detect() = %+v, want %+v", v, tc.want)
			}
		})
	}
}

func TestCurrentCaches(t *testing.T) {
	a := Current()
	b := Current()
	if a != b {
		t.Fatal("Current() returned different values across calls")
	}
}

func TestEffective(t *testing.T) {
	cases := []struct {
		name    string
		ui      string
		env     map[string]string
		want    bool
		wantSrc string
	}{
		{"env override on", "off", map[string]string{"MTM_TRUECOLOR": "1"}, true, "MTM_TRUECOLOR"},
		{"env override off", "on", map[string]string{"MTM_TRUECOLOR": "0"}, false, "MTM_TRUECOLOR"},
		{"ui on", "on", map[string]string{}, true, "ui.truecolor"},
		{"ui off", "off", map[string]string{}, false, "ui.truecolor"},
		{"ui auto with truecolor hint", "auto", map[string]string{"COLORTERM": "truecolor"}, true, "auto"},
		{"ui auto on linux console", "auto", map[string]string{"TERM": "linux"}, false, "auto"},
		{"ui auto with no hints is truecolor", "auto", map[string]string{}, true, "auto"},
		{"invalid ui value falls back to auto", "sometimes", map[string]string{}, true, "auto"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, src := Effective(tc.ui, func(k string) string { return tc.env[k] })
			if got != tc.want {
				t.Fatalf("Effective(%q) = %v, want %v", tc.ui, got, tc.want)
			}
			if !strings.HasPrefix(src, tc.wantSrc) {
				t.Fatalf("Effective(%q) source = %q, want prefix %q", tc.ui, src, tc.wantSrc)
			}
		})
	}
}
