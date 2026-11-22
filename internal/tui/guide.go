package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// guide.go — the built-in comprehensive MTM guide. A clean, scrollable
// reference: the essentials every user should know to get the most out of
// MTM, plus how it supports a better daily rhythm. There is no mascot here;
// this is a text reference tab.

type guideSection struct {
	title string
	lines []string
}

func guideSections() []guideSection {
	return []guideSection{
		{
			title: "Welcome to MTM",
			lines: []string{
				"MTM is your terminal task manager: keyboard-first, Pomodoro-native, with a calendar, a kanban board, courses/exams, and a heatmap so you can see your rhythm.",
				"Open the guide anytime: `:guide` in the palette, `alt+g` anywhere, or `mtm guide` at launch.",
				"Close it with esc or q.",
			},
		},
		{
			title: "Getting started",
			lines: []string{
				"One-time setup: `mtm init` (creates the repo, git, config), then `mtm` or `mtm tui`.",
				"Run the throwaway demo: `make demo` — 80 tasks in a temp sandbox, never touching your real data.",
				"Check your install: `mtm doctor`.",
				"Launch options: `mtm --no-splash` (skip the startup words), `mtm --no-ambient` (off ambient scenes).",
			},
		},
		{
			title: "The quick-add syntax",
			lines: []string{
				"Everything you need in one line: `mtm add \"title !priority #tag @project ~90m\"`",
				"  !high   !medium   !low    — priority",
				"  #tag    #javascript #docs — tags",
				"  @project              — project",
				"  ~90m                  — estimate (\"~\" for vague guesses)",
				"You can edit anything with the `e` / `s` / `p` / `t` / `m` keys.",
			},
		},
		{
			title: "Views",
			lines: []string{
				"Home  (0) — the everyday dashboard: TODAY band (3 tasks), streak chips, heatmap, EXAMS, UP NEXT.",
				"List  (1) — the main list with cursor, j/k navigate, / search, groups by week.",
				"Board (2) — kanban columns (To Do / Progress / Done, plus a Done column). Drag cards with the mouse, or use h/l and x/X. `D` toggles the Done column view.",
				"Calendar (3) — the week agenda with tasks; `d` toggles Done tasks inside a cell.",
				"Courses (4) — master–detail: list on the left, course dashboard on the right (grade gauge, hours, upcoming tasks).",
				"Stats (5)  — streaks and monthly heatmaps summary.",
				"Ideas (6)  — your captured ideas, capture with the `I` key (journal or note idea views).",
			},
		},
		{
			title: "Working with tasks",
			lines: []string{
				"Add: `mtm add \"...\"` or `a` in the TUI. Edit: `e` title, `E` file. Delete: `dd`.",
				"Complete: `x` (toggles done). Archive: `X` (asks). Archive list: `:archive`.",
				"Focus: `F` on a task for a full-screen Pomodoro. Timer statusline shows when it's running.",
				"Notes: `m` (built-in editor) or `M` (open in $EDITOR). Ideas: capture anywhere you like it — put it in the Ideas view.",
			},
		},
		{
			title: "Pomodoro & Focus — the heartbeat of MTM",
			lines: []string{
				"Focus mode (F): a full-screen, distraction-free Pomodoro for the selected task.",
				"Phases: work (focus) → break (long break every 4th). The timer is shared: `mtm pomo` in a script works too.",
				"Turn it into a habit: Pomodoro on the same task every morning, review \"Done this morning\" at noon.",
			},
		},
		{
			title: "Improving your daily life with MTM",
			lines: []string{
				"Heatmap: read it like a fitness tracker — green on days you finish tasks. Spike a quiet week? Do 3 small wins to break the streak.",
				"Plan: the Home TODAY band is your 3-a-day rule. Park it: the UP NEXT band is your commitment queue.",
				"Review: every Friday, run `mtm review` (or open the review view) to move unfinished things forward honestly.",
				"Difficulty is real: if you keep losing the Pomodoro to one hard task, break it into subtasks (`mtm sub`) or schedule Pomodoros for the easy fixes first.",
				"Sync: MTM is a git repo — push at the end of the week to your own mirror if you want backup. It never touches anyone else's repos.",
				"Integration: `mtm --waybar`, `mtm --statusline` (script-friendly output), `mtm waybar` for a periodic view.",
			},
		},
		{
			title: "Pro-grade MTM: automation",
			lines: []string{
				"Systemd: `mtm systemd` creates a user timer that logs MTM time automatically when you log out.",
				"Export/backup: `mtm export` (all tasks to markdown) — nice for archiving or sharing.",
				"Filters: `/` search in the list (case-insensitive), `n`/`N` next/prev match.",
				"Keyboard: every keymap is configurable via `keys:` in `config.yaml`.",
				"Style: `:help` for the full keybinding table.",
			},
		},
		{
			title: "Keyboard reference (the essentials)",
			lines: []string{
				"NAVIGATION",
				"  j k          down/up one row          gg        first row          G   last row",
				"  ctrl+b       toggle sidebar                           v   toggle detail pane",
				"TASKS",
				"  a            add task                                x   toggle done           dd  delete",
				"  X            archive (asks)                          m/M notes              e/s/p/t  edit",
				"  F            focus mode                              0-6 views              tab  cycle view",
				"POMODORO/SYSTEM",
				"  ctrl+p       palette (commands)                     :help                  q/esc  quit/close",
				"  ctrl+d/u     page down/up                            1 2 3 4 5 6            S    sync now",
			},
		},
	}
}

// guideState is the guide overlay state.
type guideState struct {
	Scroll int
}

func (m *Model) openGuide() {
	m.mode = modeGuide
	m.guide.Scroll = 0
	m.viewStamp++
}

// guideLines returns the full flattened guide text (scrollable body).
func guideLines() []string {
	var lines []string
	for _, sec := range guideSections() {
		lines = append(lines, "── "+sec.title+" "+strings.Repeat("─", 1))
		lines = append(lines, sec.lines...)
		lines = append(lines, "")
	}
	return lines
}

func guideLineCount() int { return len(guideLines()) }

func (m *Model) updateGuideKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.mode = modeNormal
		m.viewStamp++
		return m, nil
	case "j", "down":
		if m.guide.Scroll < guideLineCount()-1 {
			m.guide.Scroll++
		}
	case "k", "up":
		if m.guide.Scroll > 0 {
			m.guide.Scroll--
		}
	case "0":
		m.guide.Scroll = 0
	case "G":
		m.guide.Scroll = 1 << 30 // clamped at render
	}
	return m, nil
}

// renderGuidePanel draws the scrollable guide panel (borders + a scroll hint).
func (m Model) renderGuidePanel() string {
	s := m.styles
	lines := guideLines()
	maxContent := m.height - 6
	if maxContent < 3 {
		maxContent = 3
	}
	start := m.guide.Scroll
	if maxStart := len(lines) - maxContent; start > maxStart {
		start = maxStart
	}
	if start < 0 {
		start = 0
	}
	visible := lines
	if len(lines) > maxContent {
		visible = lines[start : start+maxContent]
	}

	var b strings.Builder
	b.WriteString(s.mAccent.Render("  MTM Guide — j/k scroll · esc closes"))
	b.WriteString("\n\n")
	for _, l := range visible {
		b.WriteString(s.base.Render(l))
		b.WriteString("\n")
	}
	hint := helpScrollHint(len(lines), maxContent, start)
	if hint != "" {
		b.WriteString("\n" + s.mMuted.Render(hint))
	} else {
		b.WriteString("\n" + s.mMuted.Render("  esc closes · press :help for the full keybinding table"))
	}

	raised := lipgloss.Color(s.p.SurfaceRaised)
	block := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(s.p.Accent)).
		Background(raised).
		Padding(1, 2).
		Render(b.String())
	return paint(block, fastWidth(block), lipgloss.Height(block), raised)
}
