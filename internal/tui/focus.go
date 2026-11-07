package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/gauges"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

// focusTickMsg fires once per second while focus mode is active.
type focusTickMsg struct{}

// enterFocusTask enters full-screen focus mode pinned to t, starting a
// fresh Pomodoro work timer and its 1-second tick.
func (m Model) enterFocusTask(t *core.Task) (Model, tea.Cmd) {
	work := m.cfg.Pomodoro.WorkMinutes
	if work <= 0 {
		work = 25
	}
	m.focusTotal = time.Duration(work) * time.Minute
	m.focusRemaining = m.focusTotal
	m.focusEnd = m.clock.Now().Add(m.focusTotal)
	m.focusPaused = false
	m.notesID = t.ID // reuse for identity of the focused task
	m.mode = modeFocus
	return m, focusTickCmd()
}

func focusTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return focusTickMsg{} })
}

// updateFocusKey handles keys while focus mode is active.
func (m Model) updateFocusKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// The x key in focus mode completes the focused task via toggleDoneAction.
	switch msg.String() {
	case "esc", "q":
		m.mode = modeNormal
		m.focusPaused = true
		return m, nil
	case " ":
		m.focusPaused = !m.focusPaused
		if m.focusPaused {
			m.focusRemaining = time.Until(m.focusEnd)
			if m.focusRemaining < 0 {
				m.focusRemaining = 0
			}
		} else {
			m.focusEnd = m.clock.Now().Add(m.focusRemaining)
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

// handleFocusTick advances the countdown once per second.
func (m Model) handleFocusTick() (tea.Model, tea.Cmd) {
	if m.mode != modeFocus {
		return m, nil
	}
	if m.focusPaused {
		return m, focusTickCmd()
	}
	remaining := time.Until(m.focusEnd)
	if remaining <= 0 {
		m.focusRemaining = 0
		m.focusPaused = true
		m.statusMsg = "pomodoro done — take a break"
		return m, focusTickCmd()
	}
	m.focusRemaining = remaining
	return m, focusTickCmd()
}

func formatFocusDuration(d time.Duration) string {
	total := int(d.Seconds())
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

	// Timer bar + times.
	elapsed := m.focusTotal - m.focusRemaining
	frac := 0.0
	if m.focusTotal > 0 {
		frac = float64(elapsed) / float64(m.focusTotal)
	}
	barW := clampInt(inner-22, 16, 44)
	barFill, barTrack := gauges.Bar(frac, barW)
	fill := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(lipgloss.Color(m.palette.Base)).Render(barFill)
	track := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Border)).Background(lipgloss.Color(m.palette.Base)).Render(barTrack)
	body = append(body, "")
	body = append(body, "")
	body = append(body, strings.Repeat(" ", pad)+formatFocusDuration(elapsed)+"  "+fill+track+"  "+formatFocusDuration(m.focusRemaining))
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
	if m.focusPaused {
		return "paused"
	}
	return "focusing…"
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
			l += strings.Repeat(" ", pad)
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
