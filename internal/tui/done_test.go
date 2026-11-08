package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

// strikeSegments walks the raw SGR stream and returns the text runs that
// carry strikethrough (SGR 9, cleared by 29 or a full reset 0).
// Everything between escape sequences is a segment.
func strikeSegments(raw string) []string {
	var out []string
	var cur strings.Builder
	strike := false
	i := 0
	for i < len(raw) {
		if raw[i] != '\x1b' {
			cur.WriteByte(raw[i])
			i++
			continue
		}
		// CSI ... <final letter>
		j := i + 1
		for j < len(raw) && (raw[j] < 'a' || raw[j] > 'z') && (raw[j] < 'A' || raw[j] > 'Z') {
			j++
		}
		j++
		seq := raw[i:j]
		if strike && cur.Len() > 0 {
			out = append(out, cur.String())
		}
		cur.Reset()
		// Parse SGR params (only ...m sequences carry attributes).
		if len(seq) >= 2 && seq[len(seq)-1] == 'm' {
			params := strings.Split(seq[2:len(seq)-1], ";")
			for _, p := range params {
				switch p {
				case "9":
					strike = true
				case "29", "0", "":
					strike = false
				}
			}
		}
		i = j
	}
	if strike && cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// renderDoneFrame renders one view with the "All" smart filter selected —
// the default Home filter hides done tasks — at a fixed clock and theme.
func renderDoneFrame(t *testing.T, view string, w, h int) string {
	t.Helper()
	s := fixtures["doneTasks"](t)
	defer func() { _ = s.Close() }()
	m := New(dumpConfig(), s, sync.New(s.Root))
	lipgloss.SetColorProfile(termenv.TrueColor)
	m.palette = theme.Defaults()
	m.styles = newStyles(m.palette)
	m.homeCache = nil
	m.glamourRenderer = nil
	for i, it := range m.sidebarItems {
		if it.Kind == "smart" && it.Key == "all" {
			m.sidebarSel = i
			break
		}
	}
	m.refreshTasks()
	switch view {
	case "list":
		m.view = viewList
	case "board":
		m.view = viewBoard
		m.resetBoardCursor()
		m.boardCol = 2 // the done column (statuses: todo, doing, done)
	case "calendar":
		m.view = viewCalendar
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	m = updated.(Model)
	return m.View()
}

// TestStrikethroughSkipsSpacesAndPadding pins the raw SGR stream: the
// strike attribute never covers a space — not between words, not in the
// padded tail of a title cell, not on blank runs. Regression for "done
// strike-through covered padding".
func TestStrikethroughSkipsSpacesAndPadding(t *testing.T) {
	for _, view := range []string{"list", "board", "calendar"} {
		t.Run(view, func(t *testing.T) {
			frame := renderDoneFrame(t, view, 120, 40)
			segs := strikeSegments(frame)
			for _, seg := range segs {
				if strings.Contains(seg, " ") {
					t.Errorf("struck segment contains a space: %q", seg)
				}
				if strings.TrimSpace(seg) == "" {
					t.Errorf("blank segment under strike: %q", seg)
				}
			}
			// The done fixture title is multi-word and known: "Write the
			// plan now". Every word must appear struck.
			joined := strings.Join(segs, "")
			for _, word := range []string{"Write", "plan", "now"} {
				if !strings.Contains(joined, word) {
					t.Errorf("expected struck %q in frame; struck text was %q", word, joined)
				}
			}
		})
	}
}

// TestDoneTasksNeverShowOverdueOrUrgency asserts done rows never carry
// overdue/urgency foregrounds: the due slot of a done task reads
// "done Nd ago" muted. Regression for "done tasks showed overdue".
func TestDoneTasksNeverShowOverdueOrUrgency(t *testing.T) {
	frame := renderDoneFrame(t, "list", 120, 40)
	plain := StripANSI(frame)
	for _, line := range strings.Split(plain, "\n") {
		if strings.Contains(strings.ToLower(line), "overdue") && strings.Contains(line, "Write the plan") {
			t.Errorf("done task row shows overdue: %q", line)
		}
	}
	if !strings.Contains(plain, "done today") {
		t.Fatalf("done task row is missing the muted completion marker; got:\n%s", plain)
	}
}
