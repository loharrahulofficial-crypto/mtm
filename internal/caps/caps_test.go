package caps

import "testing"

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
		{"mlterm", map[string]string{"TERM": "mlterm"}, Terminal{"mlterm", false, true, true, false}},
		{"tmux 256", map[string]string{"TERM": "tmux-256color"}, Terminal{"tmux", false, true, false, false}},
		{"tmux truecolor", map[string]string{"TERM": "tmux-256color", "COLORTERM": "truecolor"}, Terminal{"tmux", true, false, false, false}},
		{"screen", map[string]string{"TERM": "screen"}, Terminal{"screen", false, true, false, false}},
		{"linux console", map[string]string{"TERM": "linux"}, Terminal{"linux", false, true, false, false}},
		{"dumb wins over everything", map[string]string{"TERM": "dumb", "COLORTERM": "truecolor", "GHOSTTY_RESOURCES_DIR": "/x"}, Terminal{"dumb", false, true, false, false}},
		{"xterm 256 no qw hints", map[string]string{"TERM": "xterm-256color"}, Terminal{"unknown", false, true, false, false}},
		{"xterm truecolor via COLORTERM", map[string]string{"TERM": "xterm-256color", "COLORTERM": "24bit"}, Terminal{"unknown", true, false, false, false}},
		{"empty env", map[string]string{}, Terminal{"unknown", false, true, false, false}},
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
