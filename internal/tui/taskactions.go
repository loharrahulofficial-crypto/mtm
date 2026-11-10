package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

// refreshTasks rebuilds the filtered/sorted/tree-flattened row list from the
// store, preserving the selected task id when possible.
func (m *Model) refreshTasks() {
	prevID := ""
	if row := m.selectedRow(); row != nil {
		prevID = row.Task.ID
	}

	isDone := func(p, st string) bool { return m.store.IsDoneStatus(p, st) }

	// Sidebar-selected filter combined with the live search query.
	m.filter = m.sidebarFilter()
	m.filter.DoneCheck = isDone
	m.filter.Now = m.clock.Now()

	// Live search: key:value tokens + free text (core.ParseSearchQuery).
	base := m.filter
	needle := ""
	if q := strings.TrimSpace(m.searchQuery); q != "" {
		if qFilter, qNeedle, err := core.ParseSearchQuery(q); err == nil {
			if qFilter.Tag != "" {
				base.Tag = qFilter.Tag
			}
			if qFilter.Project != "" {
				base.Project = qFilter.Project
			}
			if qFilter.Status != "" {
				base.Status = qFilter.Status
			}
			if qFilter.Priority != "" {
				base.Priority = qFilter.Priority
			}
			if qFilter.Due != "" {
				base.Due = qFilter.Due
			}
			base.Contains = qFilter.Contains
			needle = qNeedle
		} else {
			base.Contains = q
			needle = q
		}
	}
	m.needle = needle

	filtered := base.Apply(m.store.Tasks())
	core.SortTasks(filtered, m.cfg.Sort, isDone)
	m.rows = flattenRows(filtered)
	m.computeGroups()

	// Restore selection by id if possible.
	m.cursor = 0
	if prevID != "" {
		for i := range m.rows {
			if m.rows[i].Task.ID == prevID {
				m.cursor = i
				break
			}
		}
	}
	m.clampCursor()
	if max := m.totalListLines() - m.visibleListRows(); max >= 0 && m.scroll > max {
		m.scroll = max
	}
	m.ensureCursorVisible()
	homeData := buildHomeData(*m)
	m.homeCache = &homeData
	if n := len(homeData.todayRows); m.homeSel >= n {
		m.homeSel = n - 1
	}
	if m.homeSel < 0 {
		m.homeSel = 0
	}
}

func (m *Model) clampCursor() {
	if len(m.rows) == 0 {
		m.cursor, m.scroll = 0, 0
		return
	}
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// flattenRows converts a sorted filtered slice into tree-ordered rows.
// Children (parent in the filtered set) are nested under their parent.
func flattenRows(filtered []*core.Task) []flatRow {
	inSet := make(map[string]bool, len(filtered))
	for _, t := range filtered {
		inSet[t.ID] = true
	}
	children := map[string][]*core.Task{}
	var roots []*core.Task
	for _, t := range filtered {
		if t.Parent != "" && inSet[t.Parent] {
			children[t.Parent] = append(children[t.Parent], t)
			continue
		}
		roots = append(roots, t)
	}
	var out []flatRow
	var walk func(t *core.Task, depth int)
	walk = func(t *core.Task, depth int) {
		out = append(out, flatRow{Task: t, Depth: depth})
		for _, c := range children[t.ID] {
			walk(c, depth+1)
		}
	}
	for _, r := range roots {
		walk(r, 0)
	}
	return out
}

func (m *Model) selectedRow() *flatRow {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	return &m.rows[m.cursor]
}

func (m *Model) selectedTask() *core.Task {
	if r := m.selectedRow(); r != nil {
		return r.Task
	}
	return nil
}

// sidebarFilter builds the Filter implied by the sidebar selection.
func (m *Model) sidebarFilter() core.Filter {
	sel := m.sidebarSelection()
	if sel == nil {
		return core.Filter{}
	}
	switch sel.Kind {
	case "home":
		return core.Filter{}
	case "smart":
		if sel.Key == "all" {
			// All shows everything, done included (done sorts last), so
			// marking done never makes the row vanish mid-view.
			return core.Filter{IncludeCompleted: true}
		}
		return core.Filter{Due: sel.Key}
	case "view":
		for _, v := range m.cfg.SavedViews {
			if v.Name == sel.Key {
				if f, err := core.ParseViewExpr(v.Filter); err == nil {
					return f
				}
			}
		}
		return core.Filter{}
	case "project":
		return core.Filter{Project: sel.Key}
	}
	return core.Filter{}
}

type sidebarRow struct {
	header string
	sel    int // index into selectable items, or -1 for headers
}

// buildSidebar computes items + header rows and re-indexes sidebarSel.
func (m *Model) buildSidebar() []sidebarItem {
	tasks := m.store.Tasks()
	isDone := func(p, st string) bool { return m.store.IsDoneStatus(p, st) }
	now := m.clock.Now()
	base := core.Filter{Now: now, DoneCheck: isDone}
	count := func(f core.Filter) int {
		f.DoneCheck = isDone
		f.Now = now
		return len(f.Apply(tasks))
	}

	items := []sidebarItem{
		{Kind: "home", Name: "Home", Key: "home"},
		{Kind: "smart", Name: "All", Key: "all", Count: count(core.Filter{IncludeCompleted: true})},
		{Kind: "smart", Name: "Today", Key: "today", Count: count(core.Filter{Due: "today"})},
		{Kind: "smart", Name: "Week", Key: "week", Count: count(core.Filter{Due: "week"})},
		{Kind: "smart", Name: "Overdue", Key: "overdue", Count: count(core.Filter{Due: "overdue"})},
	}
	for _, v := range m.cfg.SavedViews {
		items = append(items, sidebarItem{Kind: "view", Name: v.Name, Key: v.Name})
	}

	projects, _ := m.store.ListProjects()
	for _, p := range projects {
		done, total := 0, 0
		for _, t := range tasks {
			if strings.EqualFold(t.Project, p.Name) {
				total++
				if isDone(t.Project, t.Status) {
					done++
				}
			}
		}
		items = append(items, sidebarItem{
			Kind:  "project",
			Name:  p.Name,
			Key:   p.Name,
			Count: total - done,
			Done:  done,
			Total: total,
		})
	}

	if m.sidebarSel >= len(items) {
		m.sidebarSel = len(items) - 1
	}
	if m.sidebarSel < 0 {
		m.sidebarSel = 0
	}
	_ = base
	return items
}

// sidebarSelection returns the currently selected item, or nil.
func (m *Model) sidebarSelection() *sidebarItem {
	if m.sidebarSel < 0 || m.sidebarSel >= len(m.sidebarItems) {
		return nil
	}
	return &m.sidebarItems[m.sidebarSel]
}

// refreshAfterStoreOp rebuilds rows, sidebar and clamped cursor after a mutation.
func (m *Model) refreshAfterStoreOp() {
	m.refreshTasks()
	m.sidebarItems = m.buildSidebar()
	m.clampCursor()
}

// --- mutations (all funneled through here) ---

func (m *Model) setStatusAction(canonical string) tea.Cmd {
	t := m.selectedTask()
	if t == nil {
		return nil
	}
	msg, err := actions.SetStatus(m.store, t, canonical)
	if err != nil {
		m.statusMsg = "status: " + err.Error()
		return nil
	}
	m.statusMsg = "→ " + canonical + spawnSuffix(msg)
	return m.afterChange("status: " + t.Title)
}

func (m *Model) toggleDoneAction() tea.Cmd {
	t := m.selectedTask()
	if t == nil {
		return nil
	}
	p, err := m.store.GetProject(t.Project)
	if err != nil {
		m.statusMsg = err.Error()
		return nil
	}
	if p.IsDoneStatus(t.Status) {
		// Re-open: first non-done status in the workflow.
		next := ""
		for _, st := range p.Statuses {
			if !st.Done {
				next = st.Name
				break
			}
		}
		if next == "" {
			next = t.Status
		}
		if _, err := actions.SetStatus(m.store, t, next); err != nil {
			m.statusMsg = err.Error()
			return nil
		}
		m.statusMsg = "reopened " + t.ID
		return m.afterChange("reopen: " + t.Title)
	}
	spawnMsg, err := actions.CompleteTask(m.store, t)
	if err != nil {
		m.statusMsg = err.Error()
		return nil
	}
	m.statusMsg = "done " + t.ID + spawnSuffix(spawnMsg)
	rewardCmd := m.fireReward(t.Title)
	m.animTaskID = t.ID
	if m.animations {
		m.animFrame = animFrames
		return tea.Batch(m.afterChange("done: "+t.Title), animTickCmd(), rewardCmd)
	}
	return tea.Batch(m.afterChange("done: "+t.Title), rewardCmd)
}

func spawnSuffix(msg string) string {
	if strings.TrimSpace(msg) == "" {
		return ""
	}
	return " (" + strings.TrimSpace(msg) + ")"
}

func (m *Model) deleteSelectedAction() tea.Cmd {
	t := m.selectedTask()
	if t == nil {
		return nil
	}
	return func() tea.Msg { return nil } // placeholder, replaced by confirm flow
}

// archiveSelectedAction archives the selected task if it is done.
func (m *Model) archiveSelectedAction() (tea.Cmd, string) {
	t := m.selectedTask()
	if t == nil {
		return nil, "no task selected"
	}
	p, err := m.store.GetProject(t.Project)
	if err != nil {
		return nil, err.Error()
	}
	if !p.IsDoneStatus(t.Status) {
		return nil, "only done tasks can be archived (press " + m.keymap.Seq(aToggleDone) + " to mark done)"
	}
	if err := m.store.ArchiveTask(t); err != nil {
		return nil, err.Error()
	}
	return m.afterChange("archive: " + t.Title), "archived " + t.ID
}

// archiveAllDoneAction archives every done task (with confirm upstream).
func (m *Model) archiveAllDoneAction() (tea.Cmd, string) {
	n, err := m.store.ArchiveDone(m.clock.Now(), 0, func(p, st string) bool { return m.store.IsDoneStatus(p, st) })
	if err != nil {
		return nil, err.Error()
	}
	if n == 0 {
		return nil, "nothing to archive"
	}
	return m.afterChange(fmt.Sprintf("archive: %d done tasks", n)), fmt.Sprintf("archived %d done task(s)", n)
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
