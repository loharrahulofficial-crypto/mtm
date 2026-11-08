package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/parse"
)

func (m Model) updateInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if m.inputKind == inputSearch {
			m.searchQuery = ""
			m.input.SetValue("")
			m.refreshTasks()
			m.sidebarItems = m.buildSidebar()
		}
		m.mode = modeNormal
		m.input.Blur()
		return m, nil
	case "enter":
		return m.submitInput()
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.inputKind == inputSearch {
		m.searchQuery = m.input.Value()
		m.refreshTasks()
		m.sidebarItems = m.buildSidebar()
	}
	return m, cmd
}

func (m Model) submitInput() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.input.Value())
	switch m.inputKind {
	case inputSearch:
		m.searchQuery = m.input.Value()
		m.mode = modeNormal
		m.input.Blur()
		return m, nil
	case inputAdd:
		return m.submitQuickAdd(value, "")
	case inputSubtask:
		parent := ""
		if t := m.selectedTask(); t != nil {
			parent = t.ID
		}
		return m.submitQuickAdd(value, parent)
	case inputDue:
		t := m.selectedTask()
		if t == nil {
			m.mode = modeNormal
			return m, nil
		}
		if value == "" || strings.EqualFold(value, "clear") {
			t.Due = ""
			if err := m.store.Update(t); err != nil {
				m.statusMsg = err.Error()
				m.mode = modeNormal
				return m, nil
			}
			m.statusMsg = "due cleared " + t.ID
			m.mode = modeNormal
			m.input.Blur()
			return m, m.afterChange("edit: " + t.Title)
		}
		due, err := core.DueInput(value)
		if err != nil {
			m.statusMsg = err.Error()
			return m, nil // keep the input open
		}
		t.Due = due
		if err := m.store.Update(t); err != nil {
			m.statusMsg = err.Error()
			m.mode = modeNormal
			return m, nil
		}
		m.statusMsg = "due " + due + " → " + t.ID
		m.mode = modeNormal
		m.input.Blur()
		return m, m.afterChange("edit: " + t.Title)
	case inputTags:
		t := m.selectedTask()
		if t == nil {
			m.mode = modeNormal
			return m, nil
		}
		m.mode = modeNormal
		m.input.Blur()
		tags := parseTagList(value)
		t.Tags = tags
		if err := m.store.Update(t); err != nil {
			m.statusMsg = err.Error()
			return m, nil
		}
		m.statusMsg = "tags updated " + t.ID
		return m, m.afterChange("edit: " + t.Title)
	case inputTitle:
		t := m.selectedTask()
		if t == nil {
			m.mode = modeNormal
			return m, nil
		}
		if value == "" {
			m.statusMsg = "title cannot be empty"
			return m, nil
		}
		m.mode = modeNormal
		m.input.Blur()
		t.Title = value
		if err := m.store.Update(t); err != nil {
			m.statusMsg = err.Error()
			return m, nil
		}
		m.statusMsg = "title updated " + t.ID
		return m, m.afterChange("edit: " + t.Title)
	case inputCommand:
		m.mode = modeNormal
		m.input.Blur()
		return m.execCommand(value)
	}
	m.mode = modeNormal
	return m, nil
}

func (m Model) submitQuickAdd(value, parent string) (tea.Model, tea.Cmd) {
	return m.submitQuickAddKind(value, parent, "", "")
}

func (m Model) submitQuickAddKind(value, parent, kind, project string) (tea.Model, tea.Cmd) {
	m.mode = modeNormal
	m.input.Blur()
	if value == "" {
		return m, nil
	}
	t, err := actions.QuickAddTask(m.cfg, m.store, value, actions.QuickAddOverrides{
		Parent:  parent,
		Kind:    kind,
		Project: project,
		Now:     m.clock.Now(),
	})
	if err != nil {
		m.statusMsg = err.Error()
		return m, nil
	}
	if parent == "" {
		m.statusMsg = fmt.Sprintf("added %s  %s", t.ID, t.Title)
	} else {
		m.statusMsg = fmt.Sprintf("added subtask %s under %s", t.ID, parent)
	}
	return m, m.afterChange("add: " + t.Title)
}

// submitExam creates an exam targeted at a course:
//
//	:exam <course> <name> <date>
//
// The course must be an existing project (exact or case-insensitive match);
// exams are course events, so an unknown course is an error listing the
// courses, never the default project. Everything after the course is parsed
// by the quick-add grammar, so the date may be ISO (2026-11-19) or a phrase
// ("friday", "in 3 days"); the rest is the exam name.
func (m Model) submitExam(arg string) (tea.Model, tea.Cmd) {
	fields := strings.Fields(arg)
	if len(fields) < 3 {
		m.statusMsg = "usage: :exam <course> <name> <date>  e.g. :exam college/dbms DBMS midterm 2026-11-13"
		return m, nil
	}
	course, err := m.lookupCourse(fields[0])
	if err != nil {
		m.statusMsg = err.Error()
		return m, nil
	}
	rest := strings.Join(fields[1:], " ")
	parsed := parse.QuickAdd(rest, m.clock.Now())
	if parsed.Due == "" {
		m.statusMsg = "exam needs a date — :exam <course> <name> <date>  e.g. friday or 2026-11-19"
		return m, nil
	}
	if strings.TrimSpace(parsed.Title) == "" {
		m.statusMsg = "exam needs a name — :exam <course> <name> <date>"
		return m, nil
	}
	// The :exam palette command and `mtm exam add` share the same core
	// function (actions.AddExam); only the input source differs (the palette
	// accepts relative dates via the quick-add grammar, the CLI takes ISO).
	if _, err := actions.AddExam(m.store, course, parsed.Title, parsed.Due, nil, nil); err != nil {
		m.statusMsg = err.Error()
		return m, nil
	}
	m.statusMsg = "exam added: " + parsed.Title
	return m, m.afterChange("exam add: " + parsed.Title)
}

// lookupCourse resolves a course name to an existing project. Exact match
// wins; a case-insensitive match is accepted. Otherwise the error lists the
// known courses.
func (m Model) lookupCourse(name string) (string, error) {
	if m.store.ProjectExists(name) {
		return name, nil
	}
	projects, _ := m.store.ListProjects()
	names := make([]string, 0, len(projects))
	for _, p := range projects {
		if strings.EqualFold(p.Name, name) {
			return p.Name, nil
		}
		names = append(names, p.Name)
	}
	if len(names) == 0 {
		return "", fmt.Errorf("no course %q — no courses yet (mtm project add <name>)", name)
	}
	return "", fmt.Errorf("no course %q — courses: %s", name, strings.Join(names, ", "))
}

func parseTagList(value string) []string {
	seen := map[string]bool{}
	var out []string
	for _, part := range strings.Split(value, ",") {
		tag := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(part), "#")))
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		out = append(out, tag)
	}
	return out
}

// --- command line (:) ---

func (m Model) execCommand(value string) (tea.Model, tea.Cmd) {
	value = strings.TrimPrefix(strings.TrimSpace(value), ":")
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return m, nil
	}
	cmd := strings.ToLower(fields[0])
	arg := strings.TrimSpace(strings.Join(fields[1:], " "))

	switch cmd {
	case "add":
		if arg == "" {
			m.statusMsg = "usage: :add <text>"
			return m, nil
		}
		return m.submitQuickAdd(arg, "")
	case "exam":
		return m.submitExam(arg)
	case "subtask":
		if arg == "" {
			m.statusMsg = "usage: :subtask <text>"
			return m, nil
		}
		parent := ""
		if t := m.selectedTask(); t != nil {
			parent = t.ID
		}
		return m.submitQuickAdd(arg, parent)
	case "done", "toggle":
		return m, m.toggleDoneAction()
	case "status":
		return m.commandSetStatus(arg)
	case "priority", "prio":
		return m.commandSetPriority(arg)
	case "due":
		return m.commandSetDue(arg)
	case "tags":
		if t := m.selectedTask(); t != nil {
			t.Tags = parseTagList(arg)
			if err := m.store.Update(t); err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}
			m.statusMsg = "tags updated " + t.ID
			return m, m.afterChange("edit: " + t.Title)
		}
	case "title":
		if arg == "" {
			m.statusMsg = "usage: :title <new title>"
			return m, nil
		}
		if t := m.selectedTask(); t != nil {
			t.Title = arg
			if err := m.store.Update(t); err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}
			m.statusMsg = "title updated " + t.ID
			return m, m.afterChange("edit: " + t.Title)
		}
	case "project":
		if arg == "" {
			m.statusMsg = "usage: :project <name>"
			return m, nil
		}
		if _, err := m.store.GetProject(arg); err != nil {
			m.statusMsg = fmt.Sprintf("unknown project %q — create it with `mtm project add %s`", arg, arg)
			return m, nil
		}
		if t := m.selectedTask(); t != nil {
			t.Project = arg
			if err := m.store.Update(t); err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}
			m.statusMsg = "moved " + t.ID + " → " + arg
			return m, m.afterChange("edit: " + t.Title)
		}
	case "archive":
		return m.askArchiveAllConfirm(), nil
	case "delete", "rm":
		return m.askDeleteConfirm(), nil
	case "sync":
		msgs := m.pendingCommits
		m.pendingCommits = nil
		m.syncSt = syncSyncing
		return m, m.flushWith(msgs)
	case "list", "view1":
		m.view = viewList
		return m, nil
	case "home", "view0":
		m.view = viewHome
		return m, nil
	case "focus":
		if t := m.selectedTask(); t != nil {
			return m.enterFocusTask(t)
		}
		m.statusMsg = "no task selected to focus"
		return m, nil
	case "board", "kanban", "view2":
		m.view = viewBoard
		m.resetBoardCursor()
		return m, nil
	case "calendar", "cal", "view3":
		m.view = viewCalendar
		return m, nil
	case "quit", "q":
		return m.quitWithFlush()
	case "help":
		m.mode = modeHelp
		m.helpSel = 0
		return m, nil
	default:
		m.statusMsg = fmt.Sprintf("unknown command %q — try :help", cmd)
		return m, nil
	}
	return m, nil
}

func (m Model) commandSetStatus(arg string) (tea.Model, tea.Cmd) {
	if arg == "" {
		m.statusMsg = "usage: :status <name>"
		return m, nil
	}
	t := m.selectedTask()
	if t == nil {
		return m, nil
	}
	p, err := m.store.GetProject(t.Project)
	if err != nil {
		m.statusMsg = err.Error()
		return m, nil
	}
	canonical := p.CanonicalStatus(arg)
	if canonical == "" {
		m.statusMsg = fmt.Sprintf("unknown status %q (valid: %s)", arg, strings.Join(statusNames(p.Statuses), ", "))
		return m, nil
	}
	msg, err := actions.SetStatus(m.store, t, canonical)
	if err != nil {
		m.statusMsg = err.Error()
		return m, nil
	}
	m.statusMsg = "→ " + canonical + spawnSuffix(msg)
	return m, m.afterChange("status: " + t.Title)
}

func (m Model) commandSetPriority(arg string) (tea.Model, tea.Cmd) {
	arg = strings.ToLower(strings.TrimSpace(arg))
	if !core.ValidPriority(arg) {
		m.statusMsg = "usage: :priority low|medium|high|urgent"
		return m, nil
	}
	t := m.selectedTask()
	if t == nil {
		return m, nil
	}
	t.Priority = arg
	if err := m.store.Update(t); err != nil {
		m.statusMsg = err.Error()
		return m, nil
	}
	m.statusMsg = "priority " + arg + " → " + t.ID
	return m, m.afterChange("edit: " + t.Title)
}

func (m Model) commandSetDue(arg string) (tea.Model, tea.Cmd) {
	t := m.selectedTask()
	if t == nil {
		return m, nil
	}
	if arg == "" || strings.EqualFold(arg, "clear") {
		t.Due = ""
	} else {
		due, err := core.DueInput(arg)
		if err != nil {
			m.statusMsg = err.Error()
			return m, nil
		}
		t.Due = due
	}
	if err := m.store.Update(t); err != nil {
		m.statusMsg = err.Error()
		return m, nil
	}
	m.statusMsg = "due updated → " + t.ID
	return m, m.afterChange("edit: " + t.Title)
}

func statusNames(ss []core.ProjectStatus) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		out = append(out, s.Name)
	}
	return out
}

// --- confirm overlay (delete / archive-all) ---

func (m Model) updateConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch strings.ToLower(msg.String()) {
	case "y", "yes":
		m.mode = modeNormal
		fn := m.confirmYes
		m.confirmYes = nil
		if fn == nil {
			return m, nil
		}
		return m, fn(&m)
	default:
		m.mode = modeNormal
		m.confirmYes = nil
		m.statusMsg = "cancelled"
		return m, nil
	}
}

// askArchiveAllConfirm opens the archive-all confirmation.
func (m Model) askArchiveAllConfirm() Model {
	m.mode = modeConfirm
	m.confirmTitle = "archive all done tasks?"
	m.confirmBody = "every done task moves to archive/<project>/ (one commit, nothing deleted)"
	m.confirmYes = func(mm *Model) tea.Cmd {
		cmd, note := mm.archiveAllDoneAction()
		mm.statusMsg = note
		return cmd
	}
	return m
}

// --- picker overlay ---

func (m Model) updatePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.mode = modeNormal
		return m, nil
	case "j", "down":
		if m.pickerSel < len(m.pickerItems)-1 {
			m.pickerSel++
		}
		return m, nil
	case "k", "up":
		if m.pickerSel > 0 {
			m.pickerSel--
		}
		return m, nil
	case "enter":
		m2, cmd := m.pickerAccept()
		return m2, cmd
	}
	return m, nil
}

func (m Model) pickerAccept() (Model, tea.Cmd) {
	m.mode = modeNormal
	if m.pickerSel < 0 || m.pickerSel >= len(m.pickerItems) {
		return m, nil
	}
	choice := m.pickerItems[m.pickerSel]
	t := m.selectedTask()
	if t == nil {
		return m, nil
	}
	switch m.picker {
	case pickerStatus:
		msg, err := actions.SetStatus(m.store, t, choice)
		if err != nil {
			m.statusMsg = err.Error()
			return m, nil
		}
		m.statusMsg = "→ " + choice + spawnSuffix(msg)
		return m, m.afterChange("status: " + t.Title)
	case pickerPriority:
		t.Priority = choice
		if err := m.store.Update(t); err != nil {
			m.statusMsg = err.Error()
			return m, nil
		}
		m.statusMsg = "priority " + choice + " → " + t.ID
		return m, m.afterChange("edit: " + t.Title)
	}
	return m, nil
}
