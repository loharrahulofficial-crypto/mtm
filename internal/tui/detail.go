package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/dates"
	"github.com/loharrahulofficial-crypto/mtm/internal/gauges"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
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
		// The board owns the detail pane: an empty board shows nothing,
		// never a stale list-cursor task.
		return m.selectedBoardTask()
	}
	if m.view == viewCalendar {
		// The calendar owns the detail pane: an empty day shows a day
		// summary (handled by renderDetail), never a stale task.
		agenda := agendaTasks(*m)
		if m.calAgendaSel >= 0 && m.calAgendaSel < len(agenda) {
			return agenda[m.calAgendaSel]
		}
		return nil
	}
	if m.view == viewHome {
		return m.homeSelectedTask()
	}
	if m.view == viewCourses {
		// The course dashboard is read-only; no task detail.
		return nil
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
	inner := w - 2
	if inner < 10 {
		return renderPanel(m, "", w, h, m.focus == focusDetail, []string{s.muted.Render("pane too narrow")})
	}

	t := m.detailTask()

	bgSurface := lipgloss.Color(m.palette.Surface)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(bgSurface)

	if t == nil && m.view == viewCalendar {
		// Following the calendar cursor: an empty day gets a day summary,
		// never a stale task.
		return renderCenteredBlock(m, w, h, []string{
			lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.BrightText)).Background(bgSurface).Bold(true).Render(dayStartT(m.calDay).Format("Monday, January 2")),
			muted.Render("Nothing due"),
		})
	}
	meta := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bgSurface)
	bright := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.BrightText)).Background(bgSurface).Bold(true)

	if t == nil {
		return renderCenteredBlock(m, w, h, []string{
			muted.Render("Nothing selected"),
			muted.Render("pick a task and it shows up here"),
		})
	}

	isDone := func(p, st string) bool { return m.store.IsDoneStatus(p, st) }
	done := isDone(t.Project, t.Status)
	now := m.clock.Now()

	body := []string{}

	// Title (up to 2 lines, wrapped).
	for _, l := range wrapLines(t.Title, inner-1, 2) {
		body = append(body, bright.Render(" "+l))
	}
	// Project + kind line with course stripe.
	color := theme.CourseColor(m.palette, t.Project, projectColorOverride(m, t.Project))
	stripe := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(bgSurface).Render("▌")
	sub := " " + stripe + " " + t.Project
	if t.Kind != "" && t.Kind != "task" {
		sub += "  ·  " + t.Kind
	}
	if t.Archived {
		sub += "  ·  archived"
	}
	body = append(body, meta.Render(sub))
	body = append(body, "")

	// Chip row: status + priority.
	statusChip := ""
	if done {
		statusChip = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Base)).Background(lipgloss.Color(m.palette.Success)).Bold(true).Render(" done ")
	} else if p, err := m.store.GetProject(t.Project); err == nil && len(p.Statuses) > 0 && strings.EqualFold(t.Status, p.Statuses[0].Name) {
		statusChip = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(lipgloss.Color(m.palette.SurfaceStrong)).Render(" " + strings.ToLower(t.Status) + " ")
	} else {
		statusChip = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Base)).Background(lipgloss.Color(m.palette.Blue)).Bold(true).Render(" " + strings.ToLower(t.Status) + " ")
	}
	body = append(body, " "+statusChip+"  "+s.priorityBadge(t.Priority))
	body = append(body, "")

	// Meta rows: aligned label/value pairs.
	label := func(s string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bgSurface).Width(10).Render(s)
	}
	value := func(s string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(bgSurface).Render(s)
	}
	row := func(l, v string) {
		if strings.TrimSpace(v) == "" {
			return
		}
		body = append(body, " "+label(l)+value(v))
	}
	if t.Due != "" {
		dueLabel, heat := gauges.DueLabel(t.Due, now)
		dc := lipgloss.NewStyle().Background(bgSurface)
		switch heat {
		case gauges.HeatWarm:
			dc = dc.Foreground(lipgloss.Color(m.palette.HeatWarm))
		case gauges.HeatHot:
			dc = dc.Foreground(lipgloss.Color(m.palette.HeatHot)).Bold(true)
		case gauges.HeatOverdue:
			dc = dc.Foreground(lipgloss.Color(m.palette.HeatOverdue)).Bold(true)
		default:
			dc = dc.Foreground(lipgloss.Color(m.palette.Text))
		}
		if done {
			dc = meta
		}
		row("DUE", dc.Render(dueLabel))
	}
	if len(t.Tags) > 0 {
		parts := make([]string, len(t.Tags))
		for i, tag := range t.Tags {
			parts[i] = "#" + tag
		}
		row("TAGS", strings.Join(parts, "  "))
	}
	if t.Recurrence != "" {
		row("REPEATS", t.Recurrence)
	}
	if t.EstimateMinutes > 0 {
		row("ESTIMATE", fmt.Sprintf("%dm", t.EstimateMinutes))
	}
	if t.Reminder != "" {
		row("REMINDER", t.Reminder)
	}
	if t.Created != "" {
		row("CREATED", dates.HumanStamp(t.Created, now.Location()))
	}
	if t.Completed != "" {
		row("COMPLETED", dates.HumanDayStamp(t.Completed, now.Location()))
	}
	if t.Updated != "" {
		row("UPDATED", dates.HumanStamp(t.Updated, now.Location()))
	}

	// Subtask progress.
	doneN, totalN, _ := core.SubtaskProgress(m.store.Tasks(), t.ID, isDone)
	if totalN > 0 {
		body = append(body, "")
		frac := float64(doneN) / float64(totalN)
		frac = easedFrac(m, frac)
		barW := clampInt(inner-16, 8, 18)
		barFill, barTrack := gauges.Bar(frac, barW)
		fill := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(bgSurface).Render(barFill)
		track := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Border)).Background(bgSurface).Render(barTrack)
		body = append(body, " "+label("SUBTASKS")+fill+track+value(fmt.Sprintf("  %d/%d", doneN, totalN)))
	}

	// Notes.
	body = append(body, "")
	body = append(body, " "+s.sectionLabel.Render(letterspace("N O T E S")))
	notesBody := strings.TrimRight(t.Body, "\n")
	if notesBody == "" {
		body = append(body, muted.Render(" No notes yet. Press m to write."))
		return renderPanel(m, "", w, h, m.focus == focusDetail, body)
	}
	notesW := inner - 1
	if notesW < 8 {
		notesW = 8
	}
	rendered := m.glamourRender(notesBody, notesW)
	notesLines := strings.Split(rendered, "\n")
	start := m.detailScroll
	if start > len(notesLines)-1 {
		start = len(notesLines) - 1
	}
	if start < 0 {
		start = 0
	}
	notesH := h - 2 - len(body) // inner height minus lines already written
	if notesH < 2 {
		notesH = 2
	}
	end := start + notesH
	if end > len(notesLines) {
		end = len(notesLines)
	}
	for _, l := range notesLines[start:end] {
		body = append(body, " "+padToWidth(l, notesW, bgSurface))
	}
	return renderPanel(m, "", w, h, m.focus == focusDetail, body)
}

// renderCenteredBlock renders lines centered on a plain surface block with
// no border — used for empty states in the detail pane.
func renderCenteredBlock(m Model, w, h int, lines []string) string {
	surf := lipgloss.Color(m.palette.Surface)
	out := make([]string, 0, h)
	top := (h - len(lines)) / 2
	for i := 0; i < top; i++ {
		out = append(out, "")
	}
	for _, l := range lines {
		pad := (w - lipgloss.Width(l)) / 2
		if pad < 0 {
			pad = 0
		}
		out = append(out, strings.Repeat(" ", pad)+l)
	}
	return paint(strings.Join(out, "\n"), w, h, surf)
}

// homeSelectedTask is defined in homeview.go; the detail pane reuses it in
// Home mode.
