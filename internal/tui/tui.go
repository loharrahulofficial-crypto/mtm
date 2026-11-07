package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

// Run opens the data repo and launches the interactive TUI.
func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	s, err := store.Open(cfg.DataRepoPath(), config.CachePath())
	if err != nil {
		return err
	}
	printDiagnostics := func() {}
	_ = printDiagnostics
	m := New(cfg, s, sync.New(s.Root))
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	final, err := p.Run()
	if err != nil {
		_ = s.Close()
		return fmt.Errorf("tui: %w", err)
	}
	_ = final
	return s.Close()
}
