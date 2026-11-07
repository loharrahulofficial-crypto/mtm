package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// editorCommand builds an *exec.Cmd for the configured editor + file.
func editorCommand(editorCmd, path string) *exec.Cmd {
	fields := strings.Fields(editorCmd)
	if len(fields) == 0 {
		fields = []string{"vi"}
	}
	args := append(fields[1:], path)
	return exec.Command(fields[0], args...)
}

func (m Model) openNotesInEditor() (tea.Model, tea.Cmd) {
	t := m.selectedTask()
	if t == nil {
		return m, nil
	}
	temp, err := os.CreateTemp("", "mtm-notes-*.md")
	if err != nil {
		m.statusMsg = err.Error()
		return m, nil
	}
	if _, err := temp.WriteString(t.Body); err != nil {
		temp.Close()
		os.Remove(temp.Name())
		m.statusMsg = err.Error()
		return m, nil
	}
	temp.Close()
	path := temp.Name()
	cmd := editorCommand(m.cfg.EditorCmd(), path)
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorDoneMsg{kind: "notes", path: path, err: err}
	})
}

func (m Model) openTaskFileInEditor() (tea.Model, tea.Cmd) {
	t := m.selectedTask()
	if t == nil {
		return m, nil
	}
	path, err := m.store.Path(t.ID)
	if err != nil {
		m.statusMsg = err.Error()
		return m, nil
	}
	cmd := editorCommand(m.cfg.EditorCmd(), path)
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorDoneMsg{kind: "taskfile", path: path, err: err}
	})
}

// handleEditorDone processes a finished $EDITOR session.
func (m Model) handleEditorDone(msg editorDoneMsg) (tea.Model, tea.Cmd) {
	switch msg.kind {
	case "notes":
		defer os.Remove(msg.path)
		if msg.err != nil {
			m.statusMsg = "editor: " + msg.err.Error()
			return m, nil
		}
		data, err := os.ReadFile(msg.path)
		if err != nil {
			m.statusMsg = err.Error()
			return m, nil
		}
		t := m.selectedTask()
		if t == nil {
			return m, nil
		}
		body := string(data)
		if t.Body != body {
			t.Body = body
			if err := m.store.Update(t); err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}
			m.statusMsg = "notes saved " + t.ID
			return m, m.afterChange("note: " + t.Title)
		}
		m.statusMsg = "no changes"
		return m, nil

	case "taskfile":
		if msg.err != nil {
			m.statusMsg = "editor: " + msg.err.Error()
			return m, nil
		}
		t := m.selectedTask()
		if t == nil {
			return m, nil
		}
		if err := m.store.Reparse(t); err != nil {
			m.statusMsg = fmt.Sprintf("could not parse %s after edit: %v", msg.path, err)
			return m, nil
		}
		if err := m.store.Update(t); err != nil {
			m.statusMsg = err.Error()
			return m, nil
		}
		m.statusMsg = "updated " + t.ID
		return m, m.afterChange("edit: " + t.Title)

	case "conflict":
		if msg.err != nil {
			m.statusMsg = "editor: " + msg.err.Error()
			return m, nil
		}
		// Stage the edited file and refresh the conflict state.
		rel := relToRoot(m.root, msg.path)
		if err := m.repo.AddConflictPath(rel); err != nil {
			m.statusMsg = "git add: " + err.Error()
			return m, nil
		}
		m.markConflictResolved(rel)
		remaining, _ := m.repo.ConflictedFiles()
		for i := range m.conflicts {
			found := false
			for _, u := range remaining {
				if u == m.conflicts[i] {
					found = true
				}
			}
			m.conflictDone[m.conflicts[i]] = !found
		}
		m.statusMsg = fmt.Sprintf("staged %s (%d/%d resolved)", rel, m.resolvedCount(), len(m.conflicts))
		return m, nil
	}
	return m, nil
}

func relToRoot(root, path string) string {
	rel := strings.TrimPrefix(path, root)
	rel = strings.TrimPrefix(rel, string(os.PathSeparator))
	return strings.ReplaceAll(rel, string(os.PathSeparator), "/")
}

// renderNotesEditor renders the built-in Markdown notes editor (textarea).
func (m Model) renderNotesEditor(w, h int) string {
	s := m.styles
	header := " notes "
	if t := m.findTaskByID(m.notesID); t != nil {
		header = " notes — " + t.ID + "  " + truncatePlain(t.Title, w-16)
	}
	if h < 3 {
		return s.base.Width(w).Height(h).Render("...")
	}
	m.notes.SetWidth(w - 1)
	m.notes.SetHeight(h - 2)
	body := m.notes.View()
	return lipgloss.JoinVertical(lipgloss.Left,
		s.paneTitle.Width(w).Render(truncatePlain(" "+header, w)),
		body,
		s.dim.Render(" esc/ctrl+s save · ctrl+c discard "))
}
