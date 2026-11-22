package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// guide.go — the built-in MTM guide tab. One continuous, scrollable page of
// nine sections: 1-9 / h/l jump to a section, j/k scroll, esc closes. Open it
// with `7`, `:guide`, or `mtm guide`. Content lives in guideSections() only.

type guideSection struct {
	title  string
	chip   string
	icon   string
	lines  []string        // text sections: `spans` render as key badges
	groups []guideKeyGroup // key-reference sections
	tip    string          // one-line closer for short sections
}

type guideKeyGroup struct {
	name string
	rows []guideKeyRow
}

type guideKeyRow struct {
	keys string
	desc string
}

func guideSections() []guideSection {
	return []guideSection{
		{
			title: "Welcome to MTM",
			chip:  "Welcome",
			icon:  "home",
			lines: []string{
				"MTM is your terminal task manager: keyboard-first, Pomodoro-native, with a calendar, a kanban board, courses/exams, and a heatmap so you can see your rhythm.",
				"Open this guide anytime with `7`, `:guide` in the palette, or `mtm guide` at launch.",
				"Close it with `esc` or `q`. Switch sections with `1`-`9` or `h`/`l`; scroll with `j`/`k`.",
			},
			tip: "One task done beats ten planned.",
		},
		{
			title: "Getting started",
			chip:  "Setup",
			icon:  "plus",
			lines: []string{
				"One-time setup: `mtm init` (creates the repo, git, config), then `mtm` or `mtm tui`.",
				"Run the throwaway demo: `make demo` — 80 tasks in a temp sandbox, never touching your real data.",
				"Check your install: `mtm doctor`.",
				"Launch options: `mtm --no-splash` (skip the startup words), `mtm --no-ambient` (off ambient scenes).",
			},
			tip: "`make demo` is your sandbox — explore freely.",
		},
		{
			title: "The quick-add syntax",
			chip:  "Quick-add",
			icon:  "tag",
			lines: []string{
				"Everything you need in one line: `mtm add \"title !priority #tag @project ~90m\"`",
				"`!high` `!medium` `!low` — priority",
				"`#tag` `#javascript` `#docs` — tags",
				"`@project` — project",
				"`~90m` — estimate (\"~\" for vague guesses)",
				"You can edit anything with the `e` / `s` / `p` / `t` / `m` keys.",
			},
			tip: "`~45m` beats vague estimates.",
		},
		{
			title: "Views",
			chip:  "Views",
			icon:  "list",
			lines: []string{
				"`0` Home — the everyday dashboard: TODAY band (3 tasks), streak chips, heatmap, EXAMS, UP NEXT.",
				"`1` List — the main list with cursor, `j`/`k` navigate, `/` search, groups by week.",
				"`2` Board — kanban columns (To Do / Progress / Done). Drag cards with the mouse, or use `h`/`l` and `x`/`X`. `D` toggles the Done column view.",
				"`3` Calendar — the week agenda with tasks; `d` toggles Done tasks inside a cell.",
				"`4` Courses — master–detail: list on the left, course dashboard on the right (grade gauge, hours, upcoming tasks).",
				"`5` Stats — streaks and monthly heatmaps summary.",
				"`6` Ideas — your captured ideas; `a` captures, `r` rates, `p` promotes to a project.",
				"`7` Guide — this tab.",
			},
			tip: "`tab` reaches every view without leaving home.",
		},
		{
			title: "Working with tasks",
			chip:  "Tasks",
			icon:  "check",
			lines: []string{
				"Add: `mtm add \"...\"` or `a` in the TUI. Edit: `e` title, `E` file. Delete: `dd`.",
				"Complete: `x` (toggles done). Archive: `X` (asks). Archive list: `:archive`.",
				"Focus: `F` on a task for a full-screen Pomodoro. The statusline shows when a timer is running.",
				"Notes: `m` (built-in editor) or `M` (open in $EDITOR).",
			},
			tip: "Archive weekly, not emotionally.",
		},
		{
			title: "Pomodoro & Focus",
			chip:  "Focus",
			icon:  "focus",
			lines: []string{
				"Focus mode (`F`): a full-screen, distraction-free Pomodoro for the selected task.",
				"Phases: work (focus) → break (long break every 4th). The timer is shared: `mtm pomo` in a script works too.",
				"Turn it into a habit: Pomodoro on the same task every morning, review \"Done this morning\" at noon.",
			},
			tip: "Break hard tasks into subtasks before the next Pomodoro.",
		},
		{
			title: "Improving your daily life with MTM",
			chip:  "Daily life",
			icon:  "flame",
			lines: []string{
				"Heatmap: read it like a fitness tracker — green on days you finish tasks. Spike a quiet week? Do 3 small wins to break the streak.",
				"Plan: the Home TODAY band is your 3-a-day rule. Park it: the UP NEXT band is your commitment queue.",
				"Review: every Friday, run `mtm review` (or open the review view) to move unfinished things forward honestly.",
				"Difficulty is real: if you keep losing the Pomodoro to one hard task, break it into subtasks (`mtm sub`) or schedule Pomodoros for the easy fixes first.",
				"Sync: MTM is a git repo — push at the end of the week to your own mirror if you want backup. It never touches anyone else's repos.",
			},
			tip: "The TODAY band is your 3-a-day floor.",
		},
		{
			title: "Pro-grade MTM: automation",
			chip:  "Automation",
			icon:  "sync",
			lines: []string{
				"Systemd: `mtm systemd` creates user timers that sync and remind automatically.",
				"Export/backup: `mtm export` (all tasks to markdown) — nice for archiving or sharing.",
				"Shell: `mtm summary` (status line), `mtm waybar` (one JSON line for a Waybar module).",
				"Filters: `/` search in the list (case-insensitive), `n`/`N` next/prev match.",
				"Keyboard: every keymap is configurable via `keys:` in `config.yaml`. Full table: `:help`.",
			},
			tip: "Wire `mtm systemd` once, then forget it.",
		},
		{
			title: "Keyboard reference",
			chip:  "Keys",
			icon:  "help",
			groups: []guideKeyGroup{
				{name: "Views", rows: []guideKeyRow{
					{"0-7", "switch view"},
					{"tab", "cycle views"},
					{"ctrl+b", "toggle sidebar"},
					{"v", "toggle detail pane"},
				}},
				{name: "Move", rows: []guideKeyRow{
					{"j/k", "down / up one row"},
					{"h/l", "left / right (column)"},
					{"gg", "first row"},
					{"G", "last row"},
					{"ctrl+d/u", "page down / up"},
				}},
				{name: "Tasks", rows: []guideKeyRow{
					{"a", "add task"},
					{"A", "add subtask"},
					{"x", "toggle done"},
					{"dd", "delete (asks)"},
					{"X", "archive (asks)"},
					{"e/s/p/t", "edit title/status/priority/tags"},
					{"m/M", "notes / notes in $EDITOR"},
					{"F", "focus mode"},
				}},
				{name: "System", rows: []guideKeyRow{
					{"/", "search / filter"},
					{"n/N", "next / previous match"},
					{"ctrl+p", "command palette"},
					{"?", "full keybinding table"},
					{"S", "sync now"},
					{"esc", "close / back"},
					{"q", "quit"},
				}},
			},
		},
	}
}

// guideState is the guide tab's scroll + section cursor.
type guideState struct {
	Scroll  int
	Section int
}

func (m *Model) openGuide() {
	m.view = viewGuide
	m.guide.Scroll = 0
	m.guide.Section = 0
	m.focus = focusMain
	m.viewStamp++
}

func (m Model) updateGuideKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.view != viewGuide {
		return m, nil
	}
	s := msg.String()
	switch s {
	case "esc", "q":
		m.view = viewHome
		m.viewStamp++
		return m, nil
	case "j", "down":
		m.guide.Scroll++
		m.guideSync()
	case "k", "up":
		m.guide.Scroll--
		m.guideSync()
	case "h", "left":
		m.guideJump(m.guide.Section - 1)
	case "l", "right":
		m.guideJump(m.guide.Section + 1)
	case "g":
		m.guideJump(0)
	case "G":
		m.guide.Scroll = 1 << 30 // clamped at render
		m.guideSync()
	case "ctrl+d", "pgdown":
		m.guide.Scroll += m.guideRoom()
		m.guideSync()
	case "ctrl+u", "pgup":
		m.guide.Scroll -= m.guideRoom()
		m.guideSync()
	case "?":
		m.mode = modeHelp
		m.helpSel = 0
		return m, nil
	case "tab":
		m.view = (m.view + 1) % (viewGuide + 1)
		m.guide.Scroll = 0
		return m, nil
	default:
		if len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
			if idx := int(s[0] - '1'); idx < len(guideSections()) {
				m.guideJump(idx)
			}
		}
	}
	return m, nil
}

// guideChips renders the section chip row, wrapped to width w. The active
// section carries the accent pill.
func (m Model) guideChips(w int) []string {
	s := m.styles
	if w < 20 {
		return nil
	}
	var rows []string
	cur := ""
	for i, sec := range guideSections() {
		label := sec.chip
		if label == "" {
			label = sec.title
		}
		chip := " " + fmt.Sprintf("%d %s", i+1, label) + " "
		if i == m.guide.Section {
			chip = s.pillActive.Render(chip)
		} else {
			chip = s.guideChip.Render(chip)
		}
		if cur == "" {
			cur = chip
		} else if fastWidth(cur)+fastWidth(chip) <= w {
			cur += chip
		} else {
			rows = append(rows, cur)
			cur = chip
		}
	}
	if cur != "" {
		rows = append(rows, cur)
	}
	return rows
}

// guideSectionLines renders one section: accent header + rule + body.
func (m Model) guideSectionLines(sec guideSection, w int) []string {
	s := m.styles
	label := strings.ToUpper(sec.title)
	if icon := m.icon(sec.icon); icon != "" {
		label = icon + "  " + label
	}
	head := "  " + label
	out := []string{s.accent.Render(head)}
	rule := w - fastWidth(head) - 2
	if rule < 1 {
		rule = 1
	}
	out = append(out, s.muted.Render("  "+strings.Repeat("─", rule)), "")
	if sec.groups != nil {
		out = append(out, m.guideKeyGrid(sec.groups, w)...)
	} else {
		for _, l := range sec.lines {
			out = append(out, m.guideText("  "+l, w)...)
		}
		if sec.tip != "" {
			out = append(out, m.guideText("  "+sec.tip, w)...)
		}
	}
	return out
}

// guideToken is one word (or space) of guide copy; key tokens render as
// accent badges.
type guideToken struct {
	text string
	key  bool
}

// guideTokens splits copy on spaces, treating `backticked` runs as key badges.
func guideTokens(s string) []guideToken {
	var toks []guideToken
	key := false
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			toks = append(toks, guideToken{text: cur.String(), key: key})
			cur.Reset()
		}
	}
	for _, r := range s {
		switch r {
		case '`':
			flush()
			key = !key
		case ' ':
			if key {
				cur.WriteRune(' ') // spaces stay inside a `key badge`
			} else {
				flush()
				toks = append(toks, guideToken{text: " "})
			}
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return toks
}

// guideText word-wraps copy to w cells and renders `spans` as key badges.
// Continuation lines keep the caller's indent.
func (m Model) guideText(s string, w int) []string {
	indent := ""
	for strings.HasPrefix(s, " ") {
		indent += " "
		s = s[1:]
	}
	avail := w - fastWidth(indent)
	if avail < 4 {
		avail = 4
	}
	var out []string
	var b strings.Builder
	lineW := 0
	flush := func() {
		out = append(out, indent+b.String())
		b.Reset()
		lineW = 0
	}
	for _, t := range guideTokens(s) {
		if t.text == " " {
			if lineW == 0 {
				continue // no leading space
			}
			if lineW+1 > avail {
				flush()
				continue
			}
			b.WriteString(m.styles.surface.Render(" "))
			lineW++
			continue
		}
		tw := fastWidth(t.text)
		if t.key {
			tw += 2 // guideKey pads one cell each side
		}
		if lineW > 0 && lineW+tw > avail {
			flush()
		}
		if t.key {
			b.WriteString(m.styles.guideKey.Render(t.text))
		} else {
			b.WriteString(m.styles.surface.Render(t.text))
		}
		lineW += tw
	}
	if lineW > 0 || len(out) == 0 {
		flush()
	}
	return out
}

// guideClip truncates a styled line to w cells, appending an ellipsis.
func guideClip(s string, w int) string {
	if fastWidth(s) <= w {
		return s
	}
	cut := w - 1
	if cut < 1 {
		cut = 1
	}
	return ansi.Cut(s, 0, cut) + "…"
}

// guideContentWidth is the text width of the guide page: the pane is
// full-width in guide view, minus the panel border and the scrollbar gutter.
func (m Model) guideContentWidth() int {
	w := m.width - 4 // panel border (2) + scrollbar gutter (2)
	if w < 20 {
		w = 20
	}
	return w
}

// guideRoom is the number of document rows visible under the pinned chips.
func (m Model) guideRoom() int {
	innerH := m.height - 4 // footer + bleed row, then the panel border
	if innerH < 3 {
		innerH = 3
	}
	room := innerH - len(m.guideChips(m.guideContentWidth())) - 1
	if room < 3 {
		room = 3
	}
	return room
}

// guideDocLines flattens every section into one scrollable document and
// returns the line index where each section starts.
func (m Model) guideDocLines(w int) ([]string, []int) {
	secs := guideSections()
	starts := make([]int, len(secs))
	var out []string
	for i, sec := range secs {
		starts[i] = len(out)
		if i > 0 {
			out = append(out, "")
		}
		out = append(out, m.guideSectionLines(sec, w)...)
	}
	return out, starts
}

// guideJump scrolls so section idx starts at the top (clamped to the end).
func (m *Model) guideJump(idx int) {
	doc, starts := m.guideDocLines(m.guideContentWidth())
	if idx < 0 || idx >= len(starts) {
		return
	}
	maxStart := len(doc) - m.guideRoom()
	if maxStart < 0 {
		maxStart = 0
	}
	m.guide.Scroll = clampInt(starts[idx], 0, maxStart)
	m.guide.Section = idx
}

// guideSync clamps the scroll offset and derives the active section from the
// row at the top of the viewport.
func (m *Model) guideSync() {
	doc, starts := m.guideDocLines(m.guideContentWidth())
	maxStart := len(doc) - m.guideRoom()
	if maxStart < 0 {
		maxStart = 0
	}
	m.guide.Scroll = clampInt(m.guide.Scroll, 0, maxStart)
	if maxStart > 0 && m.guide.Scroll >= maxStart {
		m.guide.Section = len(starts) - 1 // at the end, show the last section
		return
	}
	for i := len(starts) - 1; i >= 0; i-- {
		if m.guide.Scroll >= starts[i] {
			m.guide.Section = i
			return
		}
	}
	m.guide.Section = 0
}

// guideChipHit maps a content-relative cell to a section chip. Chips wrap, so
// both the row and the column matter.
func (m Model) guideChipHit(x, row, w int) (int, bool) {
	if row < 0 || row >= len(m.guideChips(w)) {
		return 0, false
	}
	curX, curRow := 0, 0
	for i, sec := range guideSections() {
		label := sec.chip
		if label == "" {
			label = sec.title
		}
		cw := fastWidth(" " + fmt.Sprintf("%d %s", i+1, label) + " ")
		if curX > 0 && curX+cw > w {
			curRow++
			curX = 0
		}
		if curRow == row && x >= curX && x < curX+cw {
			return i, true
		}
		curX += cw
	}
	return 0, false
}

// guideKeyGrid lays the key reference out in two columns when the pane is
// wide enough; group names are quiet caps labels.
func (m Model) guideKeyGrid(groups []guideKeyGroup, w int) []string {
	s := m.styles
	twoCol := w >= 96
	colW := (w - 2) / 2
	var out []string
	for _, g := range groups {
		out = append(out, s.muted.Render("  "+strings.ToUpper(g.name)))
		if twoCol {
			half := (len(g.rows) + 1) / 2
			for i := 0; i < half; i++ {
				left := m.guideKeyRowLine(g.rows[i], colW)
				line := padVisible(left, colW)
				if i+half < len(g.rows) {
					line += m.guideKeyRowLine(g.rows[i+half], colW)
				}
				out = append(out, line)
			}
		} else {
			for _, r := range g.rows {
				out = append(out, m.guideKeyRowLine(r, w))
			}
		}
		out = append(out, "")
	}
	return out
}

func (m Model) guideKeyRowLine(r guideKeyRow, w int) string {
	line := "  " + m.styles.guideKey.Render(r.keys) + " " + m.styles.surface.Render(r.desc)
	return guideClip(line, w)
}

// renderGuideBody is the scrollable pane body: a pinned chip nav row, then
// the windowed page with a scrollbar on the right edge.
func (m Model) renderGuideBody(w, innerH int) []string {
	bodyW := w - 2 // scrollbar gutter
	if bodyW < 20 {
		bodyW = 20
	}
	chips := m.guideChips(bodyW)
	doc, _ := m.guideDocLines(bodyW)
	room := innerH - len(chips) - 1 // chips + blank line stay pinned
	if room < 3 {
		room = 3
	}
	start := clampInt(m.guide.Scroll, 0, maxInt(0, len(doc)-room))

	out := make([]string, 0, len(chips)+1+room)
	out = append(out, chips...)
	out = append(out, "")
	out = append(out, m.guideScrollRows(doc, start, room, bodyW)...)
	return out
}

// guideScrollRows windows the document and appends a scrollbar column.
func (m Model) guideScrollRows(doc []string, start, room, bodyW int) []string {
	s := m.styles
	total := len(doc)
	thumbSize, thumbPos := 0, -1
	if total > room && room > 0 {
		thumbSize = room * room / total
		if thumbSize < 1 {
			thumbSize = 1
		}
		if thumbSize > room {
			thumbSize = room
		}
		maxPos := room - thumbSize
		thumbPos = 0
		if total-room > 0 {
			thumbPos = start * maxPos / (total - room)
		}
	}

	rows := make([]string, 0, room)
	for i := 0; i < room; i++ {
		line := ""
		if start+i < total {
			line = doc[start+i]
		}
		line = padVisible(line, bodyW)
		if thumbPos >= 0 {
			if i >= thumbPos && i < thumbPos+thumbSize {
				line += " " + s.accent.Render("▐")
			} else {
				line += " " + s.muted.Render("│")
			}
		}
		rows = append(rows, line)
	}
	return rows
}

// padVisible pads a styled line to w cells without touching its escapes.
func padVisible(s string, w int) string {
	if pad := w - fastWidth(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

// renderGuidePane draws the guide tab as a normal focused/unfocused panel.
func (m Model) renderGuidePane(w, h int, focused bool) string {
	bodyW := w - 4 // panel border + scrollbar gutter
	if bodyW < 20 {
		bodyW = 20
	}
	room := (h - 2) - len(m.guideChips(bodyW)) - 1
	if room < 3 {
		room = 3
	}
	doc, _ := m.guideDocLines(bodyW)
	pct := 100
	if len(doc) > room {
		pct = m.guide.Scroll * 100 / maxInt(1, len(doc)-room)
		pct = clampInt(pct, 0, 100)
	}
	hint := fmt.Sprintf("%d/%d · %d%%", m.guide.Section+1, len(guideSections()), pct)
	return renderPanelBottom(m, "MTM GUIDE", w, h, focused,
		m.renderGuideBody(w-2, h-2), "",
		"1-9/h/l section · j/k scroll · ? keys · esc close", hint)
}
