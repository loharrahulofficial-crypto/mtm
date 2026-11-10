package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
)

// GitHub link glue: the only async piece of the link. LoadGitHub itself is
// safe (read-only git + a cached gh call), but it can take seconds, so it
// runs as a background tea.Cmd and the Overview tab renders "loading…"
// until the message lands. Never blocks, never panics the UI.
type githubInfoMsg struct {
	project string
	info    *actions.GitHubInfo
	err     error
}

func (m Model) githubLoadCmd(project string) tea.Cmd {
	s := m.store
	now := m.clock.Now()
	// Pre-warm the project cache on the UI goroutine so LoadGitHub's
	// GetProject hits the cache and never writes it from a worker.
	_, _ = s.GetProject(project)
	return func() tea.Msg {
		info, err := actions.LoadGitHub(s, project, now, true)
		return githubInfoMsg{project: project, info: info, err: err}
	}
}

// handleGitHubInfo stores the background load result.
func (m Model) handleGitHubInfo(msg githubInfoMsg) (Model, tea.Cmd) {
	if msg.err == nil && msg.info != nil {
		m.github[msg.project] = msg.info
	}
	return m, nil
}

// maybeKickGitHub starts a background load when the project is linked;
// returns nil otherwise. Loads are idempotent refreshes.
func (m *Model) maybeKickGitHub(project string) tea.Cmd {
	return m.githubLoadCmdFor(project)
}

// githubFor returns the cached info for project, or nil.
func (m Model) githubFor(project string) *actions.GitHubInfo {
	gi, ok := m.github[project]
	if !ok {
		return nil
	}
	return gi
}

// githubStreak returns the local commit streak for a linked project, or
// 0/false when unknown. Used by the sidebar and Home chips (best effort —
// populated once the background load lands).
func (m Model) githubStreak(project string) (int, bool) {
	gi := m.githubFor(project)
	if gi == nil || !gi.LocalOK {
		return 0, false
	}
	return gi.Streak, true
}
