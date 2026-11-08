package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/gauges"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

// timerTickMsg fires once per second, everywhere in the TUI: it advances
// phases through the shared timer engine, auto-finalizes stale sessions,
// and refreshes the cached view the statusline and Focus mode render.
type timerTickMsg struct{}

// enterFocusTask enters full-screen focus mode pinned to t, starting or
// attaching to the shared timer (pomodoro in Focus mode).
func (m Model) enterFocusTask(t *core.Task) (Model, tea.Cmd) {
	m.notesID = t.ID // reuse for identity of the focused task
	m.mode = modeFocus
	m.attachOrStartTimer(t)
	return m, nil
}

// attachOrStartTimer starts a pomodoro for t when no timer is running,
// attaches to the running one, and never starts a second session.
func (m *Model) attachOrStartTimer(t *core.Task) {
	v := actions.LoadTimerView(m.store)
	switch {
	case v.Phase == core.TimerPhaseIdle:
		if _, err := actions.StartTimer(m.store, m.cfg, t.ID, core.TimerModePomodoro); err != nil {
			m.statusMsg = err.Error()
		} else {
			m.statusMsg = "pomodoro started"
		}
	case v.TaskID == t.ID:
		m.statusMsg = "attached to the running timer"
	default:
		title := v.TaskID
		if other, err := m.store.Resolve(v.TaskID); err == nil {
			title = other.Title
		}
		m.statusMsg = "timer already running on " + title
	}
	m.refreshTimer()
}

// refreshTimer re-reads the cached timer view the render path consumes.
func (m *Model) refreshTimer() {
	m.timer = actions.LoadTimerView(m.store)
}

// updateFocusKey handles keys while focus mode is active.
func (m Model) updateFocusKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// The x key in focus mode completes the focused task via toggleDoneAction.
	switch msg.String() {
	case "esc", "q":
		m.mode = modeNormal
		return m, nil
	case " ":
		if m.timer.Phase == core.TimerPhaseIdle {
			if t := m.focusTask(); t != nil {
				if _, err := actions.StartTimer(m.store, m.cfg, t.ID, core.TimerModePomodoro); err != nil {
					m.statusMsg = err.Error()
					return m, nil
				}
				m.statusMsg = "pomodoro started"
				m.refreshTimer()
				return m, nil
			}
			return m, nil
		}
		v, err := actions.TogglePauseTimer(m.store)
		if err != nil {
			m.statusMsg = err.Error()
			return m, nil
		}
		m.timer = v
		if v.Running {
			m.statusMsg = "resumed"
		} else {
			m.statusMsg = "paused"
		}
		return m, nil
	case "x":
		if t := m.focusTask(); t != nil {
			m.mode = modeNormal
			return m, m.toggleDoneAction()
		}
		return m, nil
	}
	return m, nil
}

// focusTask returns the task focus mode is pinned to.
func (m Model) focusTask() *core.Task {
	for i := range m.rows {
		if m.rows[i].Task.ID == m.notesID {
			return m.rows[i].Task
		}
	}
	// Fall back to the live selection (e.g. after a refresh dropped it).
	return m.selectedTask()
}

// formatFocusDuration renders mm:ss for the focus surface.
func formatFocusDuration(d time.Duration) string {
	total := int(d.Round(time.Second).Seconds())
	if total < 0 {
		total = 0
	}
	return fmt.Sprintf("%02d:%02d", total/60, total%60)
}

// renderFocus renders the full-screen focus surface.
func (m Model) renderFocus() string {
	s := m.styles
	base := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(lipgloss.Color(m.palette.Base))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(lipgloss.Color(m.palette.Base))
	bright := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.BrightText)).Background(lipgloss.Color(m.palette.Base)).Bold(true)

	w := m.width
	pad := 4
	inner := w - pad*2
	if inner < 10 {
		pad = 1
		inner = w - 2
	}

	t := m.focusTask()
	var body []string

	meta := s.pillActive.Render(" FOCUS ")
	if t != nil {
		color := theme.CourseColor(m.palette, t.Project, projectColorOverride(m, t.Project))
		meta += "  " + lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(lipgloss.Color(m.palette.Base)).Render("▌"+t.Project)
	}
	body = append(body, "")
	body = append(body, strings.Repeat(" ", pad)+meta)

	if t == nil {
		body = append(body, "")
		body = append(body, strings.Repeat(" ", pad)+bright.Render("Nothing to focus on."))
		body = append(body, strings.Repeat(" ", pad)+muted.Render("Esc to leave — pick a task and press F."))
		return verticalCenter(body, m.height, w, base)
	}

	// Title (up to 3 lines).
	titleW := inner
	lines := wrapLines(t.Title, titleW, 3)
	body = append(body, "")
	for _, l := range lines {
		body = append(body, strings.Repeat(" ", pad)+bright.Render(l))
	}

	// Chips: kind, tags, recurrence.
	chips := []string{}
	if t.Kind != "" && t.Kind != core.KindTask {
		chips = append(chips, t.Kind)
	}
	for i, tag := range t.Tags {
		if i >= 3 {
			chips = append(chips, fmt.Sprintf("+%d", len(t.Tags)-3))
			break
		}
		chips = append(chips, "#"+tag)
	}
	if t.Recurrence != "" {
		chips = append(chips, "↻ "+t.Recurrence)
	}
	if len(chips) > 0 {
		body = append(body, strings.Repeat(" ", pad)+muted.Render(strings.Join(chips, "  ")))
	}

	// Timer bar + times (from the shared timer state).
	v := m.timer
	idle := v.Phase == core.TimerPhaseIdle
	elapsed := v.Total - v.Remaining
	openEnded := !idle && v.Total <= 0 // plain stopwatch: no phase end
	frac := v.Progress
	barW := clampInt(inner-22, 16, 44)
	barFill, barTrack := gauges.Bar(frac, barW)
	fill := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(lipgloss.Color(m.palette.Base)).Render(barFill)
	track := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Border)).Background(lipgloss.Color(m.palette.Base)).Render(barTrack)
	body = append(body, "")
	body = append(body, "")
	if idle {
		body = append(body, strings.Repeat(" ", pad)+muted.Render("no timer running — press space to start a pomodoro"))
	} else if openEnded {
		// Plain stopwatch: big elapsed time, no phase bar.
		big := formatFocusDuration(v.Accrued)
		label := "elapsed"
		if !v.Running {
			label += "  ·  paused"
		}
		body = append(body, strings.Repeat(" ", pad)+bright.Render(big)+"  "+muted.Render(label+"  ·  open stopwatch — :timer stop banks the minutes"))
	} else {
		body = append(body, strings.Repeat(" ", pad)+formatFocusDuration(elapsed)+"  "+fill+track+"  "+formatFocusDuration(v.Remaining))
	}
	body = append(body, strings.Repeat(" ", pad)+muted.Render(pomodoroState(m)))

	// Notes preview.
	body = append(body, "")
	body = append(body, strings.Repeat(" ", pad)+s.sectionLabel.Render(letterspace("N O T E S")))
	mutedNotes := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(lipgloss.Color(m.palette.Base))
	bodyText := strings.TrimRight(t.Body, "\n")
	if bodyText == "" {
		body = append(body, strings.Repeat(" ", pad)+mutedNotes.Render("No notes yet. Press m to write."))
	} else {
		maxLines := m.height - len(body) - 3
		if maxLines < 2 {
			maxLines = 2
		}
		for i, l := range strings.Split(bodyText, "\n") {
			if i >= maxLines {
				body = append(body, strings.Repeat(" ", pad)+mutedNotes.Render("…"))
				break
			}
			body = append(body, strings.Repeat(" ", pad)+mutedNotes.Render(truncatePlain(l, inner)))
		}
	}

	// Bottom hint line.
	for len(body) < m.height-1 {
		body = append(body, "")
	}
	hint := muted.Render(" space pause/resume   x done   esc exit")
	body = append(body, strings.Repeat(" ", pad)+hint)

	return verticalCenter(body, m.height, w, base)
}

func pomodoroState(m Model) string {
	v := m.timer
	switch v.Phase {
	case core.TimerPhaseIdle:
		return "space starts a pomodoro · esc exit"
	case core.TimerPhaseBreak:
		if !v.Running {
			return "break — paused"
		}
		return fmt.Sprintf("break — rest · %d pomodoro(s) done", v.Pomodoros)
	default:
		if !v.Running {
			return "paused"
		}
		if v.Mode == core.TimerModeTimer {
			return "focusing… · open stopwatch"
		}
		return fmt.Sprintf("focusing… · pomodoro %d", v.PhaseIndex)
	}
}

func wrapLines(s string, width, maxLines int) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	cur := ""
	for _, word := range words {
		cand := word
		if cur != "" {
			cand = cur + " " + word
		}
		if lipglossWidth(cand) <= width {
			cur = cand
			continue
		}
		if cur != "" {
			lines = append(lines, cur)
		}
		cur = word
		if len(lines) >= maxLines-1 {
			break
		}
	}
	if cur != "" && len(lines) < maxLines {
		lines = append(lines, cur)
	}
	if len(lines) == maxLines {
		lines[maxLines-1] = truncatePlain(lines[maxLines-1], width)
	}
	return lines
}

func lipglossWidth(s string) int { return lipgloss.Width(s) }

// verticalCenter pads body lines to exactly width cells and vertically
// centers the block within height rows.
func verticalCenter(body []string, height, width int, base lipgloss.Style) string {
	if len(body) > height {
		body = body[:height]
	}
	top := (height - len(body)) / 2
	lines := make([]string, 0, height)
	fill := base.Render(strings.Repeat(" ", width))
	for i := 0; i < top; i++ {
		lines = append(lines, fill)
	}
	for _, l := range body {
		if pad := width - lipgloss.Width(l); pad > 0 {
			l += base.Render(strings.Repeat(" ", pad))
		} else if pad < 0 {
			l = truncatePlain(l, width)
		}
		lines = append(lines, l)
	}
	for len(lines) < height {
		lines = append(lines, fill)
	}
	return strings.Join(lines, "\n")
}
