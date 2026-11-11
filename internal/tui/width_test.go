package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestFastWidthMatchesLipgloss(t *testing.T) {
	cases := []string{
		"",
		"hello",
		"   ",
		"\x1b[38;2;255;0;0mred\x1b[0m",
		"\x1b[1m bold \x1b[0m plain",
		"\x1b[48;2;10;20;30m█▀ ",
		"CSI heavy \\x1b[48;2;255;0;0mX\\x1b[0m end",
		"tab\there",
		"\x1b[38;5;123m256-color\x1b[0m",
		"\x1b]8;;https://example.invalid\a link \x1b]8;;\a",
		"mixed \x1b[38;2;1;2;3mASCII only\x1b[0m done",
		"\x1b[38;2;200;100;50m▀▀▀\x1b[0m",
		"日本語テキスト",
		"emoji 🎉 width",
		"\x1b[38;2;255;255;255m✦\x1b[0m",
	}
	for _, c := range cases {
		want := lipgloss.Width(c)
		if got := fastWidth(c); got != want {
			t.Errorf("fastWidth(%q) = %d, lipgloss = %d", c, got, want)
		}
	}
}
