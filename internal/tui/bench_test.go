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

// ambientBenchModel builds a Home model with the sidebar window on.
func ambientBenchModel(b *testing.B) (Model, *store.Store) {
	b.Helper()
	fixedEnv(b)
	root := b.TempDir()
	cache := filepath.Join(b.TempDir(), "index.json")
	s, err := store.Open(root, cache)
	if err != nil {
		b.Fatal(err)
	}
	if err := s.SaveProject(core.DefaultProject("inbox")); err != nil {
		b.Fatal(err)
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
	_ = m.View() // warm the caches
	return m, s
}

// BenchmarkAmbientTickViewHome measures one full ambient tick end-to-end:
// handleAmbientTick + the partial View() rebuild (only the sidebar window
// re-renders; everything else hits caches).
func BenchmarkAmbientTickViewHome(b *testing.B) {
	m, s := ambientBenchModel(b)
	defer func() { _ = s.Close() }()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := m
		v.clock = clock.Fixed(m.clock.Now().Add(time.Duration(i+1) * 83 * time.Millisecond))
		updated, _ := v.Update(ambientTickMsg{})
		m = updated.(Model)
		_ = m.View()
	}
}

// BenchmarkNonAmbientRedrawHome measures a full View() rebuild with no
// visible change — the frame cache serves it; this is the floor.
func BenchmarkNonAmbientRedrawHome(b *testing.B) {
	m, s := ambientBenchModel(b)
	defer func() { _ = s.Close() }()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}
