package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

func (m Model) updateListKey(act string) (tea.Model, tea.Cmd) {
	switch act {
	case aUp:
		m.moveSelection(-1)
	case aDown:
		m.moveSelection(+1)
	case aPageUp:
		m.moveSelection(-m.visibleListRows()/2 - 1)
	case aPageDown:
		m.moveSelection(+m.visibleListRows()/2 + 1)
	case aTop:
		m.cursor = 0
		m.ensureCursorVisible()
	case aBottom:
		if len(m.rows) > 0 {
			m.cursor = len(m.rows) - 1
		}
		m.ensureCursorVisible()
	case aLeft:
		if m.focus == focusDetail || m.focus == focusSidebar {
			m.focus = focusMain
			return m, nil
		}
		if sideWidth(m.width, m.sidebarHidden) > 0 {
			m.focus = focusSidebar
		}
	case aRight:
		if m.focus == focusSidebar {
			m.focus = focusMain
			return m, nil
		}
		if detailWidth(m.width, m.detailOn) > 0 {
			m.focus = focusDetail
		}
	default:
		if m2, cmd, handled := m.updateSimpleTaskAction(act); handled {
			return m2, cmd
		}
	}
	return m, nil
}

// subtaskCounts returns parent id -> [done, total] over all tasks.
func subtaskCounts(all []*core.Task, isDone func(p, st string) bool) map[string][2]int {
	counts := map[string][2]int{}
	for _, t := range all {
		if t.Parent == "" {
			continue
		}
		c := counts[t.Parent]
		c[1]++
		if isDone(t.Project, t.Status) {
			c[0]++
		}
		counts[t.Parent] = c
	}
	return counts
}

func priorityPlain(p string) string {
	switch strings.ToLower(p) {
	case "urgent":
		return "[URG]"
	case "high":
		return "[HIGH]"
	case "medium":
		return "[MED]"
	case "low":
		return "[LOW]"
	}
	return "[?]"
}

func duePlain(due string, now time.Time) string {
	if due == "" {
		return "—"
	}
	if d, err := core.ParseDue(due, now.Location()); err == nil {
		if len(due) > len("2006-01-02") {
			return d.Format("01-02 15:04")
		}
		return d.Format("01-02")
	}
	return due
}

func truncatePlain(s string, budget int) string {
	if budget <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= budget {
		return s
	}
	runes := []rune(s)
	for len(runes) > 0 && runewidth.StringWidth(string(runes))+1 > budget {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

func renderList(m Model, w, h int) string {
	s := m.styles
	now := time.Now()
	isDone := func(p, st string) bool { return m.store.IsDoneStatus(p, st) }
	head := subtaskCounts(m.store.Tasks(), isDone)

	visible := h
	start := m.scroll
	if start < 0 {
		start = 0
	}
	end := start + visible
	if end > len(m.rows) {
		end = len(m.rows)
	}

	lines := make([]string, 0, h)
	for i := start; i < end; i++ {
		row := m.rows[i]
		t := row.Task
		done := isDone(t.Project, t.Status)

		indent := strings.Repeat("  ", row.Depth)
		tree := ""
		if row.Depth > 0 {
			tree = "↳ "
		}
		mark := "○"
		if done {
			mark = "✔"
		}
		suffix := ""
		if head[t.ID][1] > 0 {
			suffix = fmt.Sprintf(" [%d/%d]", head[t.ID][0], head[t.ID][1])
		}
		prio := priorityPlain(t.Priority)
		due := duePlain(t.Due, now)
		statusChip := "[" + truncatePlain(t.Status, 10) + "]"
		statusStyle := s.dim
		if done {
			statusStyle = s.statusDone
		} else if strings.EqualFold(t.Status, "doing") || strings.EqualFold(t.Status, "in progress") {
			statusStyle = s.statusDoing
		}

		// Fixed columns: mark + status + priority + due (empty "—" was the
		// stray dash artifact); title takes whatever is left.
		dueCol := ""
		if t.Due != "" {
			dueCol = due
		}
		fixed := runewidth.StringWidth(indent) + runewidth.StringWidth(tree) + 2 /* mark */ + 1 + len(statusChip) + 1 + len(prio) + 1 + runewidth.StringWidth(dueCol) + 1
		titleBudget := w - fixed
		titlePlain := truncatePlain(t.Title+suffix, titleBudget)

		focused := i == m.cursor && m.focus != focusSidebar
		var line string
		if focused {
			plain := indent + tree + mark + " " + statusChip + " " + prio + " " + titlePlain
			gap := w - runewidth.StringWidth(plain) - runewidth.StringWidth(dueCol)
			if gap < 1 {
				gap = 1
			}
			line = s.selected.Width(w).Render(plain + strings.Repeat(" ", gap) + dueCol)
		} else {
			statusMarkStyle := s.accent
			if done {
				statusMarkStyle = s.statusDone
			}
			left := indent + tree + statusMarkStyle.Render(mark) + " " + statusStyle.Render(statusChip) + " " + s.priority(t.Priority).Render(prio) + " "
			if done {
				left += s.done.Render(titlePlain)
			} else {
				left += titlePlain
			}
			if dueCol == "" {
				// No due date: no chip, nothing dangling on the right.
				lines = append(lines, s.base.Render(left))
				continue
			}
			gap := w - lipgloss.Width(left) - lipgloss.Width(dueCol)
			if gap < 1 {
				gap = 1
			}
			line = left + strings.Repeat(" ", gap) + s.dueBadge(t.Due, done, now)
		}
		lines = append(lines, line)
	}
	if len(m.rows) == 0 {
		msg := "no tasks"
		if m.searchQuery != "" {
			msg = fmt.Sprintf("nothing matches %q — esc clears the filter", m.searchQuery)
		} else if m.sidebarSelection() != nil && m.sidebarSelection().Kind == "smart" {
			msg = "nothing here — nice"
		}
		lines = append(lines, s.dim.Render("  "+msg))
	}
	for len(lines) < h {
		lines = append(lines, strings.Repeat(" ", w))
	}
	return strings.Join(lines[:h], "\n")
}
