package tui

// Project page: a full-width tabbed overlay (Overview · Tasks · Roadmap ·
// Dev-log) opened with `enter` on a sidebar project, and via :roadmap.
// The page owns the frame while open (sidebar and detail are hidden) and
// uses the scrollable-pane pattern shared with the course dashboard.

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

const (
	projectTabOverview = iota
	projectTabTasks
	projectTabRoadmap
	projectTabDevlog
)

var projectTabNames = []string{"Overview", "Tasks", "Roadmap", "Dev-log"}

// openProjectPage opens the page and kicks off the background GitHub load
// when the project is linked. tab is one of projectTab*; an out-of-range
// tab clamps to Overview.
func (m *Model) openProjectPage(project string, tab int) tea.Cmd {
	m.projectReturn = m.view
	m.projectPage = project
	if tab < 0 || tab >= len(projectTabNames) {
		tab = projectTabOverview
	}
	m.projectTab = tab
	m.projectSel = 0
	m.projectScroll = 0
	m.roadmapPan = 0
	return m.maybeKickGitHub(project)
}

// closeProjectPage returns to the view the page was opened from.
func (m *Model) closeProjectPage() {
	m.projectPage = ""
	m.projectScroll = 0
	m.projectSel = 0
}

// projectPageOpen reports whether the page is currently shown.
func (m Model) projectPageOpen() bool { return m.projectPage != "" }

// renderProjectPage renders the whole frame (footer is appended by the
// caller): 3 header rows, the tab body, a hint line.
func (m Model) renderProjectPage(w, h int) string {
	s := m.styles
	base := lipgloss.Color(m.palette.Base)
	baseBg := lipgloss.NewStyle().Background(base)

	leaf := leafName(m.projectPage)
	headLeft := " " + s.header.Render("mtm") + s.muted.Render(" · study desk")
	headRight := s.mText.Render(leaf) + s.muted.Render("  ·  "+m.projectPage)
	head := headLeft + baseBg.Render(strings.Repeat(" ", maxInt(0, w-fastWidth(headLeft)-fastWidth(headRight)))) + headRight
	tabs := m.renderProjectTabs()
	body := m.renderProjectTabBody(w, h-5) // 2 header rows + tab row + hint
	hint := m.projectPageHint()

	slab := make([]string, 0, h)
	slab = append(slab, head)
	slab = append(slab, baseBg.Render(strings.Repeat(" ", w)))
	slab = append(slab, tabs)
	slab = append(slab, body...)
	for len(slab) < h-1 {
		slab = append(slab, baseBg.Render(strings.Repeat(" ", w)))
	}
	if len(slab) > h-1 {
		slab = slab[:h-1]
	}
	slab = append(slab, clipStyled(hint, w))
	return paint(strings.Join(slab, "\n"), w, h, base)
}

// renderProjectTabs renders the tab strip: the active tab is bold accent,
// the rest muted. h/l and [/] move between tabs.
func (m Model) renderProjectTabs() string {
	s := m.styles
	parts := []string{}
	for i, name := range projectTabNames {
		if i == m.projectTab {
			parts = append(parts, s.mAccent.Render(" "+letterspace(strings.ToUpper(name))+" "))
			continue
		}
		parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(lipgloss.Color(m.palette.Base)).Render("  "+name+"  "))
	}
	line := strings.Join(parts, "")
	fill := lipgloss.Color(m.palette.Base)
	lineW := fastWidth(line)
	pad := m.width - lineW - 1
	if pad < 0 {
		pad = 0
	}
	return line + lipgloss.NewStyle().Background(fill).Render(strings.Repeat(" ", pad))
}

// projectPageHint is the muted bottom line for the active tab.
func (m Model) projectPageHint() string {
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(lipgloss.Color(m.palette.Base))
	key := m.styles.footerKey
	// Tab keys shown as two keys individually (not one ambiguous "[ ]" token).
	tabs := key.Render("[") + muted.Render(" ") + key.Render("]") + muted.Render(" tabs")
	switch m.projectTab {
	case projectTabRoadmap:
		return muted.Render(" ") + tabs + muted.Render(" · j/k milestone · h/l pan timeline · esc close")
	case projectTabDevlog:
		return muted.Render(" ") + tabs + muted.Render(" · j/k · ctrl+d/ctrl+u scroll · esc close")
	case projectTabTasks:
		return muted.Render(" ") + tabs + muted.Render(" · j/k move · enter open task · esc close")
	default:
		return muted.Render(" ") + tabs + muted.Render(" · j/k scroll · esc close")
	}
}

func (m Model) renderProjectTabBody(w, h int) []string {
	switch m.projectTab {
	case projectTabTasks:
		return m.renderProjectTasks(w, h)
	case projectTabRoadmap:
		return m.renderProjectRoadmap(w, h)
	case projectTabDevlog:
		return m.renderProjectDevlog(w, h)
	default:
		return m.renderProjectOverview(w, h)
	}
}

// ---- key handling ---------------------------------------------------------

// updateProjectPageKey routes keys while the page is open. Anything the
// page does not own falls through to the regular normal-mode handler by
// returning (m, nil) unchanged sections to the caller via the sentinel
// handled return value.
func (m Model) updateProjectPageKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	switch s {
	case "esc", "q":
		m.closeProjectPage()
		return m, nil
	case "[", "h":
		// h/l pan the timeline in the roadmap; everywhere else they switch tabs.
		if m.projectTab == projectTabRoadmap && s == "h" {
			m.roadmapPan = clampRoadmapPan(m.roadmapPan - 28)
			return m, nil
		}
		m.projectTab = (m.projectTab - 1 + len(projectTabNames)) % len(projectTabNames)
		m.projectSel = 0
		m.projectScroll = 0
		return m, nil
	case "]", "l":
		if m.projectTab == projectTabRoadmap && s == "l" {
			m.roadmapPan = clampRoadmapPan(m.roadmapPan + 28)
			return m, nil
		}
		m.projectTab = (m.projectTab + 1) % len(projectTabNames)
		m.projectSel = 0
		m.projectScroll = 0
		return m, nil
	case "j", "down":
		m.projectSel++
		m.clampProjectSel()
		return m, nil
	case "k", "up":
		m.projectSel--
		m.clampProjectSel()
		return m, nil
	case "pgdown", "ctrl+d":
		if m.projectTab == projectTabDevlog || m.projectTab == projectTabTasks {
			m.projectScroll += 8
			m.clampProjectScroll()
			return m, nil
		}
		m.projectSel += 4
		m.clampProjectSel()
		return m, nil
	case "pgup", "ctrl+u":
		if m.projectTab == projectTabDevlog || m.projectTab == projectTabTasks {
			m.projectScroll -= 8
			if m.projectScroll < 0 {
				m.projectScroll = 0
			}
			return m, nil
		}
		m.projectSel -= 4
		m.clampProjectSel()
		return m, nil
	case "g":
		m.projectSel = 0
		m.clampProjectSel()
		return m, nil
	case "G":
		m.projectScroll = 1 << 30 // clamp on render
		m.clampProjectScroll()
		m.projectSel = 1 << 30
		m.clampProjectSel()
		return m, nil
	case "enter":
		if m.projectTab == projectTabTasks {
			return m.openProjectTask()
		}
		return m, nil
	}
	return m, nil
}

// clampRoadmapPan bounds the window shift in days.
func clampRoadmapPan(days int) int {
	if days < -84 {
		return -84
	}
	if days > 112 {
		return 112
	}
	return days
}

// projectTasks returns the project's tasks in render order: top-level first,
// subtasks indented under their parent (both depth and indent preserved).
func projectTasks(m Model) []*core.Task {
	var tops, children []*core.Task
	for _, t := range m.store.Tasks() {
		if t.Project != m.projectPage {
			continue
		}
		if t.Parent == "" {
			tops = append(tops, t)
		} else {
			children = append(children, t)
		}
	}
	sortTasksForPage(tops, m)
	out := []*core.Task{}
	for _, t := range tops {
		out = append(out, t)
		subs := []*core.Task{}
		for _, c := range children {
			if c.Parent == t.ID {
				subs = append(subs, c)
			}
		}
		sortTasksForPage(subs, m)
		out = append(out, subs...)
	}
	// Orphaned children (parent in another project) still show, flat at the end.
	extra := []*core.Task{}
	for _, c := range children {
		found := false
		for _, t := range tops {
			if c.Parent == t.ID {
				found = true
				break
			}
		}
		if !found {
			extra = append(extra, c)
		}
	}
	sortTasksForPage(extra, m)
	return append(out, extra...)
}

// sortTasksForPage orders by open-first then due date then title.
func sortTasksForPage(ts []*core.Task, m Model) {
	isDone := func(p, st string) bool { return m.store.IsDoneStatus(p, st) }
	sort.SliceStable(ts, func(i, j int) bool {
		di, dj := isDone(ts[i].Project, ts[i].Status), isDone(ts[j].Project, ts[j].Status)
		if di != dj {
			return !di // open first
		}
		if ts[i].Due == "" || ts[j].Due == "" {
			return ts[i].Due != "" // dated first
		}
		return ts[i].Due < ts[j].Due
	})
}

// clampProjectSel bounds the project-page cursor. The bound depends on the
// active tab (tasks rows, roadmap milestones; Overview/Dev-log have no rows).
func (m *Model) clampProjectSel() {
	n := 0
	switch m.projectTab {
	case projectTabTasks:
		n = len(projectTasks(*m))
	case projectTabRoadmap:
		n = len(m.store.Milestones(m.projectPage))
	}
	if m.projectSel < 0 {
		m.projectSel = 0
	}
	if n > 0 && m.projectSel >= n {
		m.projectSel = n - 1
	}
}

// clampProjectScroll bounds the Dev-log/Tasks body scroll. The exact bound is
// applied when the body is rendered (it knows the final line count); here we
// just keep the offset sane.
func (m *Model) clampProjectScroll() {
	if m.projectScroll < 0 {
		m.projectScroll = 0
	}
	if m.projectScroll > 1<<20 {
		m.projectScroll = 1 << 20
	}
}

// openProjectTask jumps from the Tasks tab into the list view with the
// project sidebar filter selected and the task under the cursor.
func (m Model) openProjectTask() (tea.Model, tea.Cmd) {
	tasks := projectTasks(m)
	if m.projectSel < 0 || m.projectSel >= len(tasks) {
		return m, nil
	}
	t := tasks[m.projectSel]
	project := m.projectPage
	m.closeProjectPage()
	m = sidebarSelectKey(m, "project", project)
	m.view = viewList
	m.focus = focusMain
	for i := range m.rows {
		if m.rows[i].Task.ID == t.ID {
			m.cursor = i
			break
		}
	}
	m.ensureCursorVisible()
	return m, nil
}
