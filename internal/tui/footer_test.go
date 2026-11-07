package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// Bug report: the footer truncated "? help" to "? hel" on a 120-wide
// terminal. The footer must always fit its width and the help hint must
// stay intact, whatever the middle message says.
func TestFooterNeverTruncatesHelp(t *testing.T) {
	statuses := []string{
		"",
		"theme: 1.6.0 forven",
		"sync paused: conflicts need manual resolution — resolve them",
		"status: project \"college/dbms\": something long went wrong — run mtm sync --status to review notifications",
		strings.Repeat("long message ", 30),
		"filter: tag:finance",
	}
	for _, width := range []int{60, 78, 80, 100, 120, 140, 200} {
		for _, st := range statuses {
			m, _ := newTestModel(t, nil)
			m.width, m.height = width, 40
			m.statusMsg = st
			foot := m.renderFooterBar(width)
			if got := lipgloss.Width(foot); got != width {
				t.Errorf("width %d msg %q: footer width = %d, want %d", width, trim(st), got, width)
			}
			if got := lipgloss.Height(foot); got != 1 {
				t.Errorf("width %d msg %q: footer wrapped to %d lines", width, trim(st), got)
			}
			if w := lipgloss.Width(m.footerRight(width)); w > 0 && !strings.Contains(foot, "help") {
				t.Errorf("width %d msg %q: help hint missing: %q", width, trim(st), last(foot, 30))
			}
			if strings.Contains(foot, "hel\n") || !containsHint(foot) {
				t.Errorf("width %d msg %q: help hint truncated: %q", width, trim(st), last(foot, 20))
			}
		}
	}
}

// At tiny widths nothing can fit; the help hint may be dropped entirely but
// the footer still must be a single line no wider than the terminal.
func TestFooterNarrowTerminal(t *testing.T) {
	for _, width := range []int{20, 30, 40} {
		m, _ := newTestModel(t, nil)
		m.width, m.height = width, 40
		m.statusMsg = "theme: ok"
		foot := m.renderFooterBar(width)
		if w := lipgloss.Width(foot); w > width {
			t.Errorf("width %d: footer = %d cells", width, w)
		}
	}
}

func containsHint(foot string) bool {
	// The styled hint renders as "? help" possibly split by escape codes;
	// a truncated "? hel" is the failure signature.
	return strings.Contains(foot, "help")
}

func trim(s string) string {
	if len(s) > 24 {
		return s[:24]
	}
	return s
}

func last(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[len(r)-n:])
	}
	return s
}
