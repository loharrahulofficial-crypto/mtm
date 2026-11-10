package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"golang.org/x/term"

	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/perf"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

// Run opens the data repo and launches the interactive TUI. profilePath,
// when non-empty (--profile FILE), requests a CPU profile exactly like
// MTM_PPROF=cpu:FILE. Profiling refuses to start outside the sandbox.
func Run(profilePath string) error {
	stopProfile, err := perf.Setup(profilePath)
	if err != nil {
		return err
	}
	defer stopProfile()
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
	isTTY := term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
	m.startSplashIfEnabled(isTTY)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithReportFocus())
	final, err := p.Run()
	if err != nil {
		_ = s.Close()
		return fmt.Errorf("tui: %w", err)
	}
	_ = final
	return s.Close()
}
