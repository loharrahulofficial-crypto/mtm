package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m *Model) startConflictMode(files []string, note string) {
	m.conflictReturn = m.mode
	m.mode = modeConflict
	m.conflicts = nil
	seen := map[string]bool{}
	for _, f := range files {
		if !seen[f] {
			seen[f] = true
			m.conflicts = append(m.conflicts, f)
		}
	}
	m.conflictSel = 0
	m.conflictDone = map[string]bool{}
	// Mark already-clean files as done (e.g. after external resolution).
	if unmerged, err := m.repo.ConflictedFiles(); err == nil {
		un := map[string]bool{}
		for _, f := range unmerged {
			un[f] = true
		}
		for _, f := range m.conflicts {
			if !un[f] {
				m.conflictDone[f] = true
			}
		}
	}
	m.statusMsg = note
}

func (m Model) allConflictsResolved() bool {
	for _, f := range m.conflicts {
		if !m.conflictDone[f] {
			return false
		}
	}
	return len(m.conflicts) > 0
}

func (m *Model) markConflictResolved(file string) {
	if m.conflictDone == nil {
		m.conflictDone = map[string]bool{}
	}
	m.conflictDone[file] = true
	// Advance selection to the next unresolved file, if any.
	for i, f := range m.conflicts {
		_ = i
		if !m.conflictDone[f] {
			m.conflictSel = i
			return
		}
	}
}

func (m Model) updateConflictKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	switch s {
	case "j", "down":
		if m.conflictSel < len(m.conflicts)-1 {
			m.conflictSel++
		}
		return m, nil
	case "k", "up":
		if m.conflictSel > 0 {
			m.conflictSel--
		}
		return m, nil
	case "esc", "q":
		m.mode = m.conflictReturn
		m.statusMsg = "conflicts unresolved — sync still paused"
		return m, nil
	case "m": // keep mine (local commit; --theirs during rebase)
		return m.resolveConflictFile(true)
	case "r": // keep theirs (remote; --ours during rebase)
		return m.resolveConflictFile(false)
	case "e": // edit merged result in $EDITOR
		return m.editConflictFile()
	case "R": // re-scan from git
		files, err := m.repo.ConflictedFiles()
		if err == nil {
			m.startConflictMode(files, "conflicts re-scanned")
		}
		return m, nil
	case "c":
		if !m.allConflictsResolved() {
			m.statusMsg = "resolve every file first (m/r/e per file)"
			return m, nil
		}
		if err := m.repo.RebaseContinue(); err != nil {
			m.statusMsg = "rebase --continue failed: " + err.Error()
			return m, nil
		}
		if err := m.repo.ClearState(); err != nil {
			m.statusMsg = "state clear failed: " + err.Error()
			return m, nil
		}
		m.mode = modeNormal
		m.conflicts = nil
		m.conflictDone = map[string]bool{}
		m.syncSt = syncSyncing
		m.statusMsg = "conflicts resolved — sync resumed"
		return m, m.kickSyncCmd()
	}
	return m, nil
}

func (m Model) updateConflictMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Rows start a few lines below the header.
	relY := msg.Y - conflictListTop
	idx := relY
	if idx >= 0 && idx < len(m.conflicts) {
		m.conflictSel = idx
	}
	return m, nil
}

func (m Model) resolveConflictFile(keepMine bool) (tea.Model, tea.Cmd) {
	if len(m.conflicts) == 0 {
		return m, nil
	}
	file := m.conflicts[m.conflictSel]
	if err := m.repo.ResolveConflict(file, keepMine); err != nil {
		m.statusMsg = "resolve failed: " + err.Error()
		return m, nil
	}
	side := "remote"
	if keepMine {
		side = "local"
	}
	m.markConflictResolved(file)
	m.statusMsg = fmt.Sprintf("%s → kept %s (%d/%d resolved)", file, side, m.resolvedCount(), len(m.conflicts))
	return m, nil
}

func (m Model) editConflictFile() (tea.Model, tea.Cmd) {
	if len(m.conflicts) == 0 {
		return m, nil
	}
	file := m.conflicts[m.conflictSel]
	abs := filepath.Join(m.root, filepath.FromSlash(file))
	cmd := editorCommand(m.cfg.EditorCmd(), abs)
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return editorDoneMsg{kind: "conflict", path: abs, err: err}
		}
		// Stage whatever the user saved (markers may remain; git decides).
		_ = m.repo.AddConflictPath(file)
		return editorDoneMsg{kind: "conflict", path: abs, err: nil}
	})
}

func (m Model) resolvedCount() int {
	n := 0
	for _, f := range m.conflicts {
		if m.conflictDone[f] {
			n++
		}
	}
	return n
}

const conflictListTop = 5

// conflictTitle tries to extract the frontmatter title from a conflicted file.
func conflictTitle(abs string) string {
	data, err := os.ReadFile(abs)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "title:") {
			v := strings.TrimSpace(strings.TrimPrefix(line, "title:"))
			v = strings.Trim(v, `"'`)
			return v
		}
	}
	return ""
}

func (m Model) renderConflictScreen() string {
	s := m.styles
	w, h := m.width, m.height
	baseColor := lipgloss.Color(m.palette.Base)
	plain := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(baseColor)
	overdueBase := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.HeatOverdue)).Background(baseColor).Bold(true)
	statusDoneBase := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Success)).Background(baseColor)

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(s.syncBad.Width(w).Render("  ⚠  sync paused — unresolved conflicts") + "\n")
	b.WriteString(s.dim.Width(w).Render("  mtm never applies either side automatically. Pick per file.") + "\n\n")

	if len(m.conflicts) == 0 {
		b.WriteString(s.dim.Render("  no unmerged files — nothing to do") + "\n\n")
	} else {
		// Window the list so the selection never leaves the screen.
		listH := h - 8 // header (3) + counts + footer rows
		if listH < 1 {
			listH = 1
		}
		start := 0
		if m.conflictSel >= listH {
			start = m.conflictSel - listH + 1
		}
		if max := len(m.conflicts) - listH; start > max {
			start = max
		}
		if start < 0 {
			start = 0
		}
		end := start + listH
		if end > len(m.conflicts) {
			end = len(m.conflicts)
		}
		if start > 0 {
			b.WriteString(s.dim.Width(w).Render(fmt.Sprintf("  ↑%d more", start)) + "\n")
		}
		for i := start; i < end; i++ {
			f := m.conflicts[i]
			mark := "✗"
			markStyle := overdueBase
			if m.conflictDone[f] {
				mark = "✓"
				markStyle = statusDoneBase
			}
			cursor := "  "
			if i == m.conflictSel {
				cursor = "▸ "
			}
			pathLine := plain.Render(cursor) + markStyle.Render(mark) + plain.Render(" "+truncatePlain(f, w-4))
			if i == m.conflictSel {
				b.WriteString(s.selected.Width(w).Render(pathLine) + "\n")
			} else {
				b.WriteString(pathLine + "\n")
			}
			if i == m.conflictSel {
				abs := filepath.Join(m.root, filepath.FromSlash(f))
				if t := conflictTitle(abs); t != "" {
					b.WriteString(s.dim.Width(w).Render("      task: "+t) + "\n")
				}
			}
		}
		if end < len(m.conflicts) {
			b.WriteString(s.dim.Width(w).Render(fmt.Sprintf("  ↓%d more", len(m.conflicts)-end)) + "\n")
		}
		b.WriteString("\n")
		resolved, total := m.resolvedCount(), len(m.conflicts)
		b.WriteString(s.dim.Width(w).Render(fmt.Sprintf("  %d/%d resolved", resolved, total)) + "\n")
	}

	// Footer hints.
	b.WriteString(s.dim.Width(w).Render("  ─────────────────────────────────────────") + "\n")
	b.WriteString(s.dim.Render("  [m] keep mine (local)   [r] keep theirs (remote)   [e] edit in $EDITOR") + "\n")
	b.WriteString(s.dim.Render("  [j/k] move   [R] re-scan   [esc] later") + "\n")
	if m.allConflictsResolved() {
		b.WriteString(s.accent.Render("  [c] continue the rebase & resume sync") + "\n")
	}
	if m.statusMsg != "" {
		b.WriteString("\n" + s.accent.Render("  "+m.statusMsg) + "\n")
	}
	for strings.Count(b.String(), "\n") < h {
		b.WriteString("\n")
	}
	return paint(b.String(), w, h, baseColor)
}
