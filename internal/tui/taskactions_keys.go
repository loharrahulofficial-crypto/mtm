package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

// updateSimpleTaskAction handles the task-centric actions that work the same
// in every view (they act on the selected/cursor task or open an overlay).
// handled is false when act is not a simple task action.
func (m Model) updateSimpleTaskAction(act string) (tea.Model, tea.Cmd, bool) {
	switch act {
	case aAdd:
		m.openInput(inputAdd, "add ▸ ", "")
		return m, nil, true
	case aAddSubtask:
		t := m.selectedTask()
		if t == nil {
			return m, nil, true
		}
		m.openInput(inputSubtask, "subtask of "+t.ID+" ▸ ", "")
		return m, nil, true
	case aToggleDone:
		return m, m.toggleDoneAction(), true
	case aDelete:
		return m.askDeleteConfirm(), nil, true
	case aStatus:
		return m.openStatusPicker(), nil, true
	case aPriority:
		m.openPriorityPicker()
		return m, nil, true
	case aDue:
		t := m.selectedTask()
		if t == nil {
			return m, nil, true
		}
		m.openInput(inputDue, "due (YYYY-MM-DD[THH:MM], empty clears) ▸ ", t.Due)
		return m, nil, true
	case aTags:
		t := m.selectedTask()
		if t == nil {
			return m, nil, true
		}
		m.openInput(inputTags, "tags (comma separated, empty clears) ▸ ", strings.Join(t.Tags, ", "))
		return m, nil, true
	case aTitle:
		t := m.selectedTask()
		if t == nil {
			return m, nil, true
		}
		m.openInput(inputTitle, "title ▸ ", t.Title)
		return m, nil, true
	case aNotes:
		return m.openNotesEditor(), nil, true
	case aNotesEditor:
		m2, cmd := m.openNotesInEditor()
		return m2, cmd, true
	case aEditFile:
		m2, cmd := m.openTaskFileInEditor()
		return m2, cmd, true
	case aArchive:
		cmd, note := m.archiveSelectedAction()
		m.statusMsg = note
		return m, cmd, true
	case aSearch:
		m.openInput(inputSearch, "filter ▸ ", m.searchQuery)
		return m, nil, true
	case aSearchNext:
		return m.jumpMatch(+1), nil, true
	case aSearchPrev:
		return m.jumpMatch(-1), nil, true
	}
	return m, nil, false
}

// openInput switches to the input bar with the given purpose and prefill.
func (m *Model) openInput(kind inputKind, label, value string) {
	m.mode = modeInput
	m.inputKind = kind
	m.inputLabel = label
	m.input.SetValue(value)
	m.input.CursorEnd()
	m.input.Focus()
}

func (m Model) openStatusPicker() Model {
	t := m.selectedTask()
	if t == nil {
		return m
	}
	p, err := m.store.GetProject(t.Project)
	if err != nil {
		m.statusMsg = err.Error()
		return m
	}
	items := make([]string, 0, len(p.Statuses))
	cur := 0
	for i, st := range p.Statuses {
		if strings.EqualFold(st.Name, t.Status) {
			cur = i
		}
		items = append(items, st.Name)
	}
	m.mode = modePicker
	m.picker = pickerStatus
	m.pickerItems = items
	m.pickerSel = cur
	m.pickerTitle = fmt.Sprintf("status of %s", t.ID)
	return m
}

func (m Model) openPriorityPicker() {
	t := m.selectedTask()
	if t == nil {
		return
	}
	items := core.Priorities
	cur := 1
	for i, p := range items {
		if strings.EqualFold(p, t.Priority) {
			cur = i
		}
	}
	m.mode = modePicker
	m.picker = pickerPriority
	m.pickerItems = append([]string(nil), items...)
	m.pickerSel = cur
	m.pickerTitle = "priority of " + t.ID
}

// askDeleteConfirm opens the delete confirmation overlay.
func (m Model) askDeleteConfirm() Model {
	t := m.selectedTask()
	if t == nil {
		return m
	}
	m.mode = modeConfirm
	m.confirmTitle = "delete task?"
	m.confirmBody = fmt.Sprintf("%s  %s  (%s)", t.ID, t.Title, t.Project)
	m.confirmYes = func(m *Model) tea.Cmd {
		if err := m.store.Remove(t); err != nil {
			m.statusMsg = err.Error()
			return nil
		}
		m.statusMsg = "deleted " + t.ID
		return m.afterChange("rm: " + t.Title)
	}
	return m
}

// jumpMatch moves the cursor to the next/previous row whose task matches
// the current search needle (or key:value filter).
func (m Model) jumpMatch(dir int) Model {
	if len(m.rows) == 0 {
		return m
	}
	if strings.TrimSpace(m.searchQuery) == "" {
		return m
	}
	start := m.cursor
	for i := 1; i <= len(m.rows); i++ {
		idx := (start + dir*i + len(m.rows)*len(m.rows)) % len(m.rows)
		if m.rows[idx].Task != nil && rowMatchesNeedle(m.rows[idx].Task, m.needle) {
			m.cursor = idx
			m.ensureCursorVisible()
			return m
		}
	}
	return m
}

func rowMatchesNeedle(t *core.Task, needle string) bool {
	if needle == "" {
		return true
	}
	hay := strings.ToLower(strings.Join([]string{t.Title, t.Body, strings.Join(t.Tags, " ")}, "\n"))
	return strings.Contains(hay, strings.ToLower(needle))
}

// --- notes (built-in editor) ---

func (m Model) openNotesEditor() Model {
	t := m.selectedTask()
	if t == nil {
		return m
	}
	m.mode = modeNotes
	m.notesID = t.ID
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.Placeholder = "Markdown notes…"
	ta.SetValue(t.Body)
	ta.CursorEnd()
	m.notes = ta
	m.notes.SetWidth(m.width - 4)
	m.notes.SetHeight(m.height - 4)
	m.notes.Focus()
	return m
}

func (m Model) updateNotesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+s":
		return m.saveNotesAndClose()
	case "ctrl+c":
		m.mode = modeNormal
		return m, nil
	}
	var cmd tea.Cmd
	m.notes, cmd = m.notes.Update(msg)
	return m, cmd
}

func (m Model) saveNotesAndClose() (tea.Model, tea.Cmd) {
	t := m.findTaskByID(m.notesID)
	if t == nil {
		m.mode = modeNormal
		return m, nil
	}
	body := m.notes.Value()
	if strings.TrimRight(t.Body, "\n") == strings.TrimRight(body, "\n") {
		m.mode = modeNormal
		return m, nil
	}
	t.Body = body
	if err := m.store.Update(t); err != nil {
		m.statusMsg = "notes: " + err.Error()
		m.mode = modeNormal
		return m, nil
	}
	m.statusMsg = "notes saved " + t.ID
	m.mode = modeNormal
	return m, m.afterChange("note: " + t.Title)
}

func (m *Model) findTaskByID(id string) *core.Task {
	for _, t := range m.store.Tasks() {
		if t.ID == id {
			return t
		}
	}
	return nil
}
