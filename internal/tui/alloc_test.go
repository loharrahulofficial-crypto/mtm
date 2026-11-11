package tui

import (
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/clock"
	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

// Item 3/6: allocation budget for one ambient tick end-to-end
// (handleAmbientTick + View()). The scene itself is excluded from the
// tightest bound; the budget leaves ~40% headroom over the measured value
// so unrelated refactors don't trip it, but a regression that re-renders
// everything on a tick does.
func TestAmbientTickAllocationBudget(t *testing.T) {
	fixedEnv(t)
	root := t.TempDir()
	s, err := store.Open(root, filepath.Join(t.TempDir(), "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err := s.SaveProject(core.DefaultProject("inbox")); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.DataRepo = root
	cfg.Sync.Auto = false
	cfg.UI.Ambient = "both"
	cfg.UI.AmbientSidebar = true
	cfg.UI.Scene = "night"

	m := New(cfg, s, sync.New(root))
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	_ = m.View() // warm caches

	// Accumulate the model like the real event loop does — otherwise
	// every run hits the shared frame cache from the previous run.
	acc := m
	allocs := testing.AllocsPerRun(20, func() {
		v := acc
		v.clock = clock.Fixed(acc.clock.Now().Add(83 * time.Millisecond))
		u, _ := v.Update(ambientTickMsg{})
		acc = u.(Model)
		_ = acc.View()
	})
	t.Logf("allocations per ambient tick: %.0f", allocs)
	const budget = 2500
	if allocs > budget {
		t.Fatalf("ambient tick allocates %.0f times, budget %d (see BenchmarkAmbientTickViewHome)", allocs, budget)
	}
}

// Item 6: a relative wall-clock budget for one ambient tick (end-to-end,
// Update + View rebuild). Measured value on this machine is ~2 ms; the
// budget is ~12x generous so a failing result means a real regression, not
// a loaded CI box.
func TestAmbientTickWallBudget(t *testing.T) {
	fixedEnv(t)
	root := t.TempDir()
	s, err := store.Open(root, filepath.Join(t.TempDir(), "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err := s.SaveProject(core.DefaultProject("inbox")); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.DataRepo = root
	cfg.Sync.Auto = false
	cfg.UI.Ambient = "both"
	cfg.UI.AmbientSidebar = true
	cfg.UI.Scene = "night"
	m := New(cfg, s, sync.New(root))
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	_ = m.View() // warm caches

	acc := m
	start := time.Now()
	const N = 20
	for i := 0; i < N; i++ {
		v := acc
		v.clock = clock.Fixed(acc.clock.Now().Add(time.Duration(i+1) * 83 * time.Millisecond))
		u, _ := v.Update(ambientTickMsg{})
		acc = u.(Model)
		_ = acc.View()
	}
	per := time.Since(start) / N
	t.Logf("end-to-end ambient tick: %v", per)
	if per > 25*time.Millisecond {
		t.Fatalf("end-to-end ambient tick costs %v, budget 25ms — likely a full-rebuild regression", per)
	}
}

// A fully cached redraw allocates nothing.
func TestNonAmbientRedrawAllocatesNothing(t *testing.T) {
	fixedEnv(t)
	root := t.TempDir()
	s, err := store.Open(root, filepath.Join(t.TempDir(), "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	cfg := config.Default()
	cfg.DataRepo = root
	cfg.Sync.Auto = false
	m := New(cfg, s, sync.New(root))
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	_ = m.View()
	allocs := testing.AllocsPerRun(20, func() {
		_ = m.View()
	})
	if allocs > 4 {
		t.Fatalf("cached redraw allocates %.0f times, want ~0", allocs)
	}
}
