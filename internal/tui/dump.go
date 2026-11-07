package tui

import (
	"fmt"
	"regexp"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/clock"
	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

// DumpFixedTime is the fixed local-naive clock the --dump command and the
// golden tests render with (Saturday 7 Nov 2026, 14:32 UTC). Deterministic
// regardless of the machine's timezone and wall clock.
var DumpFixedTime = time.Date(2026, 11, 7, 14, 32, 0, 0, time.UTC)

// RenderDump renders exactly one deterministic frame of the requested view:
// fixed clock, fixed default theme, nerd icons unless cfg says otherwise.
// Focus pins to the first row of tasks (deterministic); with zero tasks it
// renders its empty state. The frame is exactly height lines × width cells.
// The color profile is pinned to TrueColor so colored output is stable no
// matter where the frame lands (piped dumps, CI, tests).
//
// view: home|list|board|calendar|focus
func RenderDump(cfg config.Config, s *store.Store, repo sync.Repo, view string, w, h int, noColor bool, t time.Time) (string, error) {
	switch view {
	case "home", "list", "board", "calendar", "focus":
	default:
		return "", fmt.Errorf("unknown view %q (want home|list|board|calendar|focus)", view)
	}
	s.Clock = clock.Fixed(t)
	lipgloss.SetColorProfile(termenv.TrueColor)
	m := New(cfg, s, repo)
	switch view {
	case "home":
		m.view = viewHome
	case "list":
		m.view = viewList
	case "board":
		m.view = viewBoard
		m.resetBoardCursor()
	case "calendar":
		m.view = viewCalendar
	case "focus":
		m.view = viewList
		if task := m.selectedTask(); task != nil {
			m2, _ := m.enterFocusTask(task)
			m = m2
		} else {
			m.mode = modeFocus
			m.notesID = ""
		}
	}
	m.palette = theme.Defaults()
	m.styles = newStyles(m.palette)
	m.homeCache = nil       // rebuild with the fixed palette
	m.glamourRenderer = nil // glamour rebuilt lazily at the fixed style
	updated, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	m = updated.(Model)
	frame := m.View()
	if noColor {
		frame = StripANSI(frame)
	}
	return frame, nil
}

var (
	anSICap = regexp.MustCompile("\x1b\\[[0-9;?]*[a-zA-Z]")
	anSIOsc = regexp.MustCompile("\x1b\\][^\x07]*(\x07|\x1b\\\\)")
)

// StripANSI removes SGR/OSC escape sequences from a rendered frame.
func StripANSI(s string) string {
	s = anSIOsc.ReplaceAllString(s, "")
	return anSICap.ReplaceAllString(s, "")
}
