package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

type detailData struct {
	Task      *core.Task
	Project   string
	SubDone   int
	SubTotal  int
	Created   string
	Completed string
}

func (m *Model) detailTask() *core.Task {
	if m.view == viewBoard {
		if t := m.selectedBoardTask(); t != nil {
			return t
		}
	}
	if m.view == viewCalendar {
		agenda := agendaTasks(*m)
		if m.calAgendaSel >= 0 && m.calAgendaSel < len(agenda) {
			return agenda[m.calAgendaSel]
		}
	}
	return m.selectedTask()
}

func (m *Model) glamourRender(body string, width int) string {
	// Fixed dark style: WithAutoStyle() queries the terminal (OSC 11) and blocks.
	if m.glamourRenderer == nil || m.glamourBodyID != body+fmt.Sprint(width) {
		r, err := glamour.NewTermRenderer(
			glamour.WithStandardStyle("dark"),
			glamour.WithWordWrap(width),
		)
		if err != nil {
			return body
		}
		rendered, err := r.Render(body)
		if err != nil {
			return body
		}
		m.glamourRenderer = r
		m.glamourBody = rendered
		m.glamourBodyID = body + fmt.Sprint(width)
		m.glamourWidth = width
	}
	return m.glamourBody
}

func (m Model) renderDetail(w, h int) string {
	s := m.styles
	inner := w - 3 // border + padding
	if inner < 10 {
		return ""
	}

	t := m.detailTask()
	var b strings.Builder

	if t == nil {
		b.WriteString(s.dim.Render("no task selected"))
		return paneWithBorder(m, b.String(), w, h)
	}

	isDone := func(p, st string) bool { return m.store.IsDoneStatus(p, st) }
	done := isDone(t.Project, t.Status)
	now := time.Now()

	// Title.
	title := truncatePlain(t.Title, inner)
	b.WriteString(s.accent.Width(inner).Render(" "+title) + "\n")
	meta := fmt.Sprintf(" %s · %s", t.ID, t.Project)
	if t.Kind != "" && t.Kind != "task" {
		meta += " · " + t.Kind
	}
	if t.Archived {
		meta += " · archived"
	}
	b.WriteString(s.dim.Width(inner).Render(meta) + "\n\n")

	// Badges row.
	badges := s.statusDone.Render("✔ " + t.Status)
	if !done {
		p, err := m.store.GetProject(t.Project)
		if err == nil && strings.EqualFold(t.Status, p.DoneStatus()) {
			badges = s.statusDone.Render("✔ " + t.Status)
		} else if p != nil && len(p.Statuses) > 0 && strings.EqualFold(t.Status, p.Statuses[0].Name) {
			badges = s.dim.Render("○ " + t.Status)
		} else {
			badges = s.statusDoing.Render("◐ " + t.Status)
		}
	}
	b.WriteString(" " + badges + "  " + s.priorityBadge(t.Priority) + "\n\n")

	// Meta rows.
	row := func(label, value string) {
		if value == "" {
			return
		}
		b.WriteString(s.dim.Render(padRight(label, 10)) + value + "\n")
	}
	row("due", s.dueBadge(t.Due, done, now)+dueSuffix(t, now))
	if len(t.Tags) > 0 {
		row("tags", strings.Join(func() []string {
			out := make([]string, len(t.Tags))
			for i, tag := range t.Tags {
				out[i] = "#" + tag
			}
			return out
		}(), " "))
	}
	if t.Recurrence != "" {
		row("repeats", t.Recurrence)
	}
	if t.EstimateMinutes > 0 {
		row("estimate", fmt.Sprintf("%dm", t.EstimateMinutes))
	}
	if t.Reminder != "" {
		row("reminder", t.Reminder)
	}
	if t.Created != "" {
		row("created", t.Created)
	}
	if t.Completed != "" {
		row("completed", t.Completed)
	}
	row("updated", t.Updated)

	// Subtask progress bar.
	doneN, totalN, _ := core.SubtaskProgress(m.store.Tasks(), t.ID, isDone)
	if totalN > 0 {
		b.WriteString("\n")
		pct := int(float64(doneN) / float64(totalN) * 100)
		b.WriteString(s.dim.Render(padRight("subtasks", 10)) + progressBarGlyph(pct, 16) + fmt.Sprintf(" %d/%d\n", doneN, totalN))
	}

	// Notes.
	b.WriteString("\n" + s.paneTitle.Width(inner).Render("notes") + "\n")
	body := strings.TrimRight(t.Body, "\n")
	if body == "" {
		b.WriteString(s.dim.Render(" (empty — m to write, M in $EDITOR)") + "\n")
	} else {
		rendered := m.glamourRender(body, inner-2)
		lines := strings.Split(rendered, "\n")
		// Scroll the notes region with detailScroll.
		start := m.detailScroll
		if start > len(lines)-1 {
			start = len(lines) - 1
		}
		if start < 0 {
			start = 0
		}
		notesH := h - strings.Count(b.String(), "\n") - 2
		if notesH < 2 {
			notesH = 2
		}
		end := start + notesH
		if end > len(lines) {
			end = len(lines)
		}
		for _, l := range lines[start:end] {
			b.WriteString(l + "\n")
		}
	}
	return paneWithBorder(m, b.String(), w, h)
}

func dueSuffix(t *core.Task, now time.Time) string {
	if t.Due == "" {
		return ""
	}
	d, err := core.ParseDue(t.Due, now.Location())
	if err != nil {
		return ""
	}
	if len(t.Due) > len("2006-01-02") {
		if d.Before(now) {
			return "  (overdue)"
		}
		return ""
	}
	day := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	days := int(day.Sub(today).Hours() / 24)
	switch {
	case days < 0:
		return fmt.Sprintf("  (%dd overdue)", -days)
	case days == 0:
		return "  (today)"
	case days == 1:
		return "  (tomorrow)"
	default:
		return fmt.Sprintf("  (in %dd)", days)
	}
}

func progressBarGlyph(pct, width int) string {
	filled := pct * width / 100
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// paneWithBorder renders content in a pane with a left border and padding.
func paneWithBorder(m Model, content string, w, h int) string {
	pane := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color(m.palette.Muted)).
		Padding(0, 1).
		Width(w).
		Height(h)
	return pane.Render(content)
}
