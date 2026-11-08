package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

// quickTimerToggle stops the running timer (banking the capped accrual) or
// starts a plain stopwatch on the selected task.
func quickTimerToggle(m Model) (Model, tea.Cmd) {
	view := actions.LoadTimerView(m.store)
	if view.Phase != core.TimerPhaseIdle {
		res, err := actions.StopTimer(m.store, false)
		if err != nil {
			m.statusMsg = err.Error()
			m.refreshTimer()
			return m, nil
		}
		m.refreshTimer()
		m.refreshTasks()
		m.sidebarItems = m.buildSidebar()
		if res.Minutes > 0 {
			return m, m.afterChange(fmt.Sprintf("log: %s on %s", actions.FormatMinutes(res.Minutes), res.Title))
		}
		m.statusMsg = "timer stopped"
		return m, nil
	}
	t := m.selectedTask()
	if t == nil {
		m.statusMsg = "no task selected — a timer needs one"
		return m, nil
	}
	if _, err := actions.StartTimer(m.store, m.cfg, t.ID, core.TimerModeTimer); err != nil {
		m.statusMsg = err.Error()
		return m, nil
	}
	m.refreshTimer()
	m.statusMsg = "timer started on " + t.Title
	return m, nil
}

// quickPomoStart starts a pomodoro on the selected task; on a running timer
// it reports the attachment instead of starting a second session.
func quickPomoStart(m Model) (Model, tea.Cmd) {
	view := actions.LoadTimerView(m.store)
	t := m.selectedTask()
	if view.Phase != core.TimerPhaseIdle {
		title := view.TaskID
		if view.TaskID == "" || (t != nil && view.TaskID == t.ID) {
			title = "this task"
		} else if other, err := m.store.Resolve(view.TaskID); err == nil {
			title = other.Title
		}
		m.statusMsg = "timer already running on " + title
		m.refreshTimer()
		return m, nil
	}
	if t == nil {
		m.statusMsg = "no task selected — a pomodoro needs one"
		return m, nil
	}
	if _, err := actions.StartTimer(m.store, m.cfg, t.ID, core.TimerModePomodoro); err != nil {
		m.statusMsg = err.Error()
		return m, nil
	}
	m.refreshTimer()
	m.statusMsg = "pomodoro started on " + t.Title
	return m, nil
}

// commandGrade parses "grade <id> <score> [--max N] [--weight N]" and records it.
func (m Model) commandGrade(arg string) (tea.Model, tea.Cmd) {
	fields := strings.Fields(arg)
	if len(fields) < 2 {
		m.statusMsg = "usage: :grade <id> <score> [--max N] [--weight N]"
		return m, nil
	}
	id, scoreRaw := fields[0], fields[1]
	score, err := strconv.ParseFloat(scoreRaw, 64)
	if err != nil {
		m.statusMsg = fmt.Sprintf("score %q must be a number", scoreRaw)
		return m, nil
	}
	max := 100.0
	var weight *float64
	for i := 2; i < len(fields); i++ {
		switch fields[i] {
		case "--max":
			i++
			if i >= len(fields) {
				m.statusMsg = "--max needs a value"
				return m, nil
			}
			v, err := strconv.ParseFloat(fields[i], 64)
			if err != nil {
				m.statusMsg = fmt.Sprintf("--max %q must be a number", fields[i])
				return m, nil
			}
			max = v
		case "--weight":
			i++
			if i >= len(fields) {
				m.statusMsg = "--weight needs a value"
				return m, nil
			}
			v, err := strconv.ParseFloat(fields[i], 64)
			if err != nil {
				m.statusMsg = fmt.Sprintf("--weight %q must be a number", fields[i])
				return m, nil
			}
			weight = &v
		default:
			m.statusMsg = fmt.Sprintf("unknown flag %q", fields[i])
			return m, nil
		}
	}
	t, err := actions.RecordGrade(m.store, id, score, max, weight)
	if err != nil {
		m.statusMsg = err.Error()
		return m, nil
	}
	m.statusMsg = fmt.Sprintf("graded %s  %g/%g", t.ID, score, max)
	return m, m.afterChange("grade: " + t.Title)
}

// commandTimer handles ":timer", ":timer start", ":timer stop", ":timer pause".
func (m Model) commandTimer(arg string) (tea.Model, tea.Cmd) {
	switch strings.ToLower(strings.TrimSpace(arg)) {
	case "", "toggle":
		return quickTimerToggle(m)
	case "start":
		v := actions.LoadTimerView(m.store)
		if v.Phase != core.TimerPhaseIdle {
			m.statusMsg = "timer already running — `:timer stop` first"
			m.refreshTimer()
			return m, nil
		}
		if t := m.selectedTask(); t != nil {
			if _, err := actions.StartTimer(m.store, m.cfg, t.ID, core.TimerModeTimer); err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}
			m.statusMsg = "timer started on " + t.Title
			m.refreshTimer()
			return m, nil
		}
		m.statusMsg = "no task selected — a timer needs one"
		return m, nil
	case "stop":
		res, err := actions.StopTimer(m.store, false)
		if err != nil {
			m.statusMsg = err.Error()
			return m, nil
		}
		m.refreshTimer()
		m.refreshTasks()
		m.sidebarItems = m.buildSidebar()
		if res.Minutes > 0 {
			return m, m.afterChange(fmt.Sprintf("log: %s on %s", actions.FormatMinutes(res.Minutes), res.Title))
		}
		m.statusMsg = "timer stopped"
		return m, nil
	case "pause":
		v, err := actions.TogglePauseTimer(m.store)
		if err != nil {
			m.statusMsg = err.Error()
			return m, nil
		}
		m.timer = v
		if v.Running {
			m.statusMsg = "timer resumed"
		} else {
			m.statusMsg = "timer paused"
		}
		return m, nil
	default:
		m.statusMsg = "usage: :timer [start|stop|pause]"
		return m, nil
	}
}

// commandPomo handles ":pomo" — start (or report the running) pomodoro.
func (m Model) commandPomo(arg string) (tea.Model, tea.Cmd) {
	if strings.TrimSpace(arg) != "" {
		m.statusMsg = "usage: :pomo"
		return m, nil
	}
	return quickPomoStart(m)
}
