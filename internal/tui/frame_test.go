package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

func mkTasks(n int) []*core.Task {
	out := make([]*core.Task, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, &core.Task{
			Title:    fmt.Sprintf("task %03d", i),
			Status:   "todo",
			Priority: "medium",
			Kind:     core.KindTask,
		})
	}
	return out
}

// Bug report: with 2 tasks the list showed one row while the detail pane
// showed the other as selected — the rendered frame was taller/wider than
// the terminal, pushing the top (selected) row off screen. The frame must
// always be exactly height rows of exactly width cells, and the selected
// row must always be visibly rendered.
func TestFrameExactlyFitsTerminal(t *testing.T) {
	for _, size := range []struct{ w, h int }{{80, 24}, {120, 40}, {200, 50}, {78, 15}, {60, 15}} {
		for _, n := range []int{0, 2, 3, 17, 120} {
			m, _ := newTestModel(t, mkTasks(n))
			upd, _ := m.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
			m = upd.(Model)
			v := m.View()
			lines := strings.Split(v, "\n")
			if len(lines) != size.h {
				t.Errorf("%dx%d n=%d: %d lines, want %d", size.w, size.h, n, len(lines), size.h)
			}
			for i, l := range lines {
				if w := lipglossW(l); w != size.w {
					t.Errorf("%dx%d n=%d line %d: %d cells, want %d", size.w, size.h, n, i, w, size.w)
					break
				}
			}
		}
	}
}

func TestSelectedRowAlwaysVisible(t *testing.T) {
	for _, h := range []int{15, 24, 40} {
		for _, n := range []int{2, 3, 17, 50} {
			m, _ := newTestModel(t, mkTasks(n))
			upd, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: h})
			m = upd.(Model)
			m.sidebarHidden = true        // main pane only: no width noise
			m = press(t, m, "1")          // list view
			m.viewFadeStart = time.Time{} // settle the tab crossfade before inspecting the frame
			vis := m.visibleListRows()
			// Walk to the last row and back with every navigation path.
			for _, key := range []string{"G", "g", "G", "k", "j", "down", "up", "ctrl+d", "G"} {
				m = press(t, m, key)
				if m.cursor >= vis && !(m.scroll <= m.cursor && m.cursor < m.scroll+vis) {
					t.Errorf("h=%d n=%d key=%s: cursor %d not in window [%d,%d)",
						h, n, key, m.cursor, m.scroll, m.scroll+vis)
				}
			}
			// Selected task must appear in the rendered frame.
			sel := m.selectedTask()
			v := m.View()
			if !strings.Contains(v, sel.Title) {
				t.Errorf("h=%d n=%d: selected %q missing from frame", h, n, sel.Title)
			}
		}
	}
}

func lipglossW(s string) int { return lipgloss.Width(s) }

func TestMouseWheelKeepsCursorVisible(t *testing.T) {
	m, _ := newTestModel(t, mkTasks(30))
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	m = updated.(Model)
	m = press(t, m, "1") // list view
	for i := 0; i < 20; i++ {
		m = press(t, m, "down")
	}
	v := m.View()
	if !strings.Contains(v, m.selectedTask().Title) {
		t.Fatalf("selected %q hidden after wheel-walk", m.selectedTask().Title)
	}
}

// Below 60×15 the app refuses to lay out and shows a clean message instead
// of a broken pane salad; the frame is still exactly w×h.
func TestTerminalTooSmallMessage(t *testing.T) {
	m, _ := newTestModel(t, mkTasks(3))
	for _, size := range []struct{ w, h int }{{59, 15}, {60, 14}, {40, 12}, {1, 1}, {8, 4}} {
		upd, _ := m.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
		m2 := upd.(Model)
		v := m2.View()
		lines := strings.Split(v, "\n")
		if len(lines) != size.h {
			t.Errorf("%dx%d: %d lines, want %d", size.w, size.h, len(lines), size.h)
		}
		if size.w >= 15 && !strings.Contains(v, "bigger terminal") {
			t.Errorf("%dx%d: missing too-small message", size.w, size.h)
		}
		for i, l := range lines {
			if w := lipgloss.Width(l); w > size.w {
				t.Errorf("%dx%d line %d: %d cells, want <= %d", size.w, size.h, i, w, size.w)
			}
		}
	}
}
