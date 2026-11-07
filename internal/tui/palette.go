package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Command palette (ctrl+p or ":"): floating rounded modal on surface_raised
// with fuzzy search over commands, tasks, projects and views. Recent items
// rank first for empty queries.
type paletteItem struct {
	id      string // stable key (used for recents), e.g. "view:list", "task:abc123"
	label   string
	hint    string // right-aligned key hint or context
	hintDim bool   // render hint dimmed (context) rather than accent (key)
	group   string
}

// buildPaletteItems assembles the full searchable set from the store.
func (m Model) buildPaletteItems() []paletteItem {
	items := []paletteItem{
		{id: "view:home", label: "Go to Home", hint: "0", group: "Commands"},
		{id: "view:list", label: "Go to List", hint: "1", group: "Commands"},
		{id: "view:board", label: "Go to Board", hint: "2", group: "Commands"},
		{id: "view:calendar", label: "Go to Calendar", hint: "3", group: "Commands"},
		{id: "focus", label: "Focus current task", hint: "F", group: "Commands"},
		{id: "toggle:sidebar", label: "Toggle sidebar", hint: m.keymap.Seq(aSidebar), group: "Commands"},
		{id: "toggle:detail", label: "Toggle detail pane", hint: m.keymap.Seq(aDetail), group: "Commands"},
		{id: "sync", label: "Sync now", hint: m.keymap.Seq(aSync), group: "Commands"},
		{id: "archive", label: "Archive completed tasks", hint: "", group: "Commands"},
		{id: "reload", label: "Reload tasks", hint: "", group: "Commands"},
		{id: "help", label: "Help / keybindings", hint: "?", group: "Commands"},
	}
	for _, t := range m.store.Tasks() {
		if m.store.IsDoneStatus(t.Project, t.Status) {
			continue
		}
		items = append(items, paletteItem{
			id:      "task:" + t.ID,
			label:   t.Title,
			hint:    t.ID,
			hintDim: true,
			group:   "Tasks",
		})
	}
	projects, _ := m.store.ListProjects()
	for _, p := range projects {
		items = append(items, paletteItem{
			id:      "project:" + p.Name,
			label:   "Filter project: " + p.Name,
			hint:    "course",
			hintDim: true,
			group:   "Courses",
		})
	}
	for _, v := range m.cfg.SavedViews {
		items = append(items, paletteItem{
			id:      "savedview:" + v.Name,
			label:   "Saved view: " + v.Name,
			hint:    "view",
			hintDim: true,
			group:   "Views",
		})
	}
	return items
}

// paletteMatch filters and ranks items for query. Empty query: recents first
// (stable), then the rest in definition order. Non-empty: subsequence fuzzy
// match with a small contiguity/start bonus; no match → excluded.
func paletteMatch(items []paletteItem, query string, recents []string) []paletteItem {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		order := map[string]int{}
		for i, id := range recents {
			order[id] = i
		}
		var recent, rest []paletteItem
		for _, it := range items {
			if _, ok := order[it.id]; ok {
				recent = append(recent, it)
				continue
			}
			rest = append(rest, it)
		}
		sort.SliceStable(recent, func(a, b int) bool { return order[recent[a].id] < order[recent[b].id] })
		return append(recent, rest...)
	}
	type scored struct {
		item  paletteItem
		score int
	}
	matched := make([]scored, 0, len(items))
	for _, it := range items {
		score, ok := fuzzyScore(q, strings.ToLower(it.label+" "+it.hint))
		if !ok {
			continue
		}
		// Recency bonus (small; never dominates match quality).
		for _, id := range recents {
			if id == it.id {
				score += 8
				break
			}
		}
		matched = append(matched, scored{it, score})
	}
	for i := 1; i < len(matched); i++ {
		for j := i; j > 0 && matched[j].score > matched[j-1].score; j-- {
			matched[j], matched[j-1] = matched[j-1], matched[j]
		}
	}
	out := make([]paletteItem, 0, len(matched))
	for _, s := range matched {
		out = append(out, s.item)
	}
	return out
}

// fuzzyScore scores query against hay as a subsequence match.
func fuzzyScore(query, hay string) (int, bool) {
	if query == "" {
		return 0, true
	}
	qi, score := 0, 0
	streak := 0
	for hi := 0; hi < len(hay) && qi < len(query); hi++ {
		if hay[hi] == query[qi] {
			if hi == 0 {
				score += 3
			}
			streak++
			score += streak
			qi++
		} else {
			streak = 0
		}
	}
	if qi != len(query) {
		return 0, false
	}
	return score, true
}

// openPalette initializes the palette state on first ctrl+p.
func (m *Model) openPalette() tea.Cmd {
	if m.paletteInput.Prompt == "" {
		m.paletteInput.Prompt = ""
		m.paletteInput.CharLimit = 128
	}
	m.mode = modePalette
	m.paletteQuery = ""
	m.paletteSel = 0
	m.paletteItems = paletteMatch(m.buildPaletteItems(), "", m.paletteRecent)
	m.paletteInput.SetValue("")
	return m.paletteInput.Focus()
}

// updatePaletteKey handles keys while the palette is open.
func (m Model) updatePaletteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.paletteInput.Blur()
		return m, nil
	case "enter":
		if paletteCommandArgv(m) {
			cmd := strings.TrimSpace(m.paletteInput.Value())
			m.mode = modeNormal
			m.paletteInput.Blur()
			return m.execCommand(cmd)
		}
		return m.executePaletteSelection()
	case "up":
		if m.paletteSel > 0 {
			m.paletteSel--
		}
		return m, nil
	case "down":
		if m.paletteSel < len(m.paletteItems)-1 {
			m.paletteSel++
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.paletteInput, cmd = m.paletteInput.Update(msg)
	q := m.paletteInput.Value()
	if q != m.paletteQuery {
		m.paletteQuery = q
		m.paletteItems = paletteMatch(m.buildPaletteItems(), q, m.paletteRecent)
		if m.paletteSel >= len(m.paletteItems) {
			m.paletteSel = len(m.paletteItems) - 1
		}
		if m.paletteSel < 0 {
			m.paletteSel = 0
		}
	}
	return m, cmd
}

func (m Model) executePaletteSelection() (tea.Model, tea.Cmd) {
	if len(m.paletteItems) == 0 {
		m.mode = modeNormal
		m.paletteInput.Blur()
		return m, nil
	}
	sel := m.paletteItems[m.paletteSel]
	m.paletteRecent = append([]string{sel.id}, filterOut(m.paletteRecent, sel.id)...)
	if len(m.paletteRecent) > 8 {
		m.paletteRecent = m.paletteRecent[:8]
	}
	m.mode = modeNormal
	m.paletteInput.Blur()
	m, cmd := m.executePaletteAction(sel.id)
	return m, cmd
}

// paletteCommandArgv reports whether the query begins with a known command
// word (the old ":" command line, folded into the palette).
func paletteCommandArgv(m Model) bool {
	q := strings.TrimSpace(m.paletteInput.Value())
	if q == "" {
		return false
	}
	return knownCommands[strings.ToLower(strings.Fields(q)[0])]
}

var knownCommands = map[string]bool{
	"add": true, "subtask": true, "done": true, "toggle": true,
	"status": true, "priority": true, "prio": true, "due": true,
	"tags": true, "title": true, "project": true, "archive": true,
	"delete": true, "rm": true, "sync": true, "help": true,
	"list": true, "view1": true, "board": true, "kanban": true, "view2": true,
	"calendar": true, "cal": true, "view3": true, "quit": true, "q": true,
	"home": true, "view0": true, "focus": true,
}

func filterOut(ids []string, drop string) []string {
	out := ids[:0]
	for _, id := range ids {
		if id != drop {
			out = append(out, id)
		}
	}
	return out
}

// executePaletteAction performs the selected palette action.
func (m Model) executePaletteAction(id string) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch id {
	case "view:home":
		m.view = viewHome
	case "view:list":
		m.view = viewList
	case "view:board":
		m.view = viewBoard
		m.resetBoardCursor()
	case "view:calendar":
		m.view = viewCalendar
	case "focus":
		if t := m.selectedTask(); t != nil {
			return m.enterFocusTask(t)
		}
		m.statusMsg = "no task selected to focus"
		return m, nil
	case "toggle:sidebar":
		m.sidebarHidden = !m.sidebarHidden
	case "toggle:detail":
		m.detailOn = !m.detailOn
		m.clampScrolls()
	case "sync":
		m, cmd = m.startSync()
	case "archive":
		m = m.askArchiveAllConfirm()
	case "reload":
		m.refreshTasks()
		m.sidebarItems = m.buildSidebar()
		m.statusMsg = "tasks reloaded"
	case "help":
		m.mode = modeHelp
		m.helpSel = 0
	default:
		switch {
		case strings.HasPrefix(id, "task:"):
			taskID := strings.TrimPrefix(id, "task:")
			m = sidebarSelectSmart(m, "all")
			for i := range m.rows {
				if m.rows[i].Task.ID == taskID {
					m.cursor = i
					break
				}
			}
			m.ensureCursorVisible()
			m.view = viewList
			m.focus = focusMain
		case strings.HasPrefix(id, "project:"):
			name := strings.TrimPrefix(id, "project:")
			m = sidebarSelectKey(m, "project", name)
			m.view = viewList
			m.focus = focusMain
		case strings.HasPrefix(id, "savedview:"):
			name := strings.TrimPrefix(id, "savedview:")
			m = sidebarSelectKey(m, "view", name)
			m.view = viewList
			m.focus = focusMain
		}
	}
	return m, cmd
}

// sidebarSelectSmart moves the sidebar selection to the smart item with key.
func sidebarSelectSmart(m Model, key string) Model {
	return sidebarSelectKey(m, "smart", key)
}

func sidebarSelectKey(m Model, kind, key string) Model {
	for i, it := range m.sidebarItems {
		if it.Kind == kind && it.Key == key {
			m.sidebarSel = i
			break
		}
	}
	m.refreshTasks()
	m.sidebarItems = m.buildSidebar()
	return m
}

// renderPaletteBox renders the floating palette modal.
func (m Model) renderPaletteBox() string {
	w := m.width - 16
	if w > 72 {
		w = 72
	}
	if w < 24 {
		w = 24
	}
	inner := w - 2 - 2 // border(2) + padX(2)
	if inner < 8 {
		inner = 8
	}

	raised := lipgloss.Color(m.palette.SurfaceRaised)
	borderC := lipgloss.Color(m.palette.Accent)
	edge := lipgloss.NewStyle().Foreground(borderC).Background(raised)
	fill := lipgloss.NewStyle().Background(raised)
	txt := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(raised)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(raised)
	accentTxt := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(raised).Bold(true)
	selBg := lipgloss.NewStyle().Background(lipgloss.Color(m.palette.SurfaceStrong))
	selTxt := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.BrightText)).Background(lipgloss.Color(m.palette.SurfaceStrong)).Bold(true)

	maxItems := m.height - 8
	if maxItems > 10 {
		maxItems = 10
	}
	if maxItems < 3 {
		maxItems = 3
	}

	content := []string{}
	// Input line.
	prompt := muted.Render("› ") + txt.Render(m.paletteInput.Value()) + selTxt.Render("▏")
	content = append(content, padBoxLine(prompt, inner, fill))

	items := m.paletteItems
	if len(items) == 0 {
		content = append(content, muted.Render("  no matches — keep typing"))
	}
	lastGroup := ""
	shown := 0
	for i, it := range items {
		if shown >= maxItems {
			break
		}
		if it.group != lastGroup && strings.TrimSpace(it.group) != "" {
			content = append(content,
				muted.Render(" "+letterspace(strings.ToUpper(it.group))))
			lastGroup = it.group
			shown++
			if shown >= maxItems {
				break
			}
		}
		line := ""
		hint := it.hint
		hintStyle := accentTxt
		if it.hintDim {
			hintStyle = muted
		}
		body := " " + truncatePlain(it.label, inner-2-lipgloss.Width(hint)-1)
		pad := inner - lipgloss.Width(body) - lipgloss.Width(hint) - 1
		if pad < 1 {
			pad = 1
		}
		if i == m.paletteSel {
			line = selBg.Render(selTxt.Render(body + strings.Repeat(" ", pad) + hint))
		} else {
			line = txt.Render(body) + strings.Repeat(" ", pad) + hintStyle.Render(hint)
		}
		content = append(content, line)
		shown++
	}
	if extra := len(items) - shown; extra > 0 && shown < maxItems+1 {
		content = append(content, muted.Render(fmt.Sprintf("  +%d more", extra)))
	}
	if paletteCommandArgv(m) {
		content = append(content, muted.Render(" ⏎ run command · esc close"))
	} else {
		content = append(content, muted.Render(" ↑↓ move · ⏎ run · esc close"))
	}

	// Assemble with rounded border.
	box := []string{edge.Render("╭" + strings.Repeat("─", w-2) + "╮")}
	for _, l := range content {
		box = append(box, edge.Render("│")+" "+padBoxLine(l, inner, fill)+" "+edge.Render("│"))
	}
	box = append(box, edge.Render("╰"+strings.Repeat("─", w-2)+"╯"))
	return strings.Join(box, "\n")
}

func padBoxLine(line string, w int, fill lipgloss.Style) string {
	if lipgloss.Width(line) > w {
		return truncatePlain(line, w)
	}
	return lipgloss.NewStyle().MaxWidth(w).Render(line + strings.Repeat(" ", w-lipgloss.Width(line)))
}
