package tui

import (
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/clock"
	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

// splash_bench_test.go — §7.3: splash frame cost (the 10 FPS splash
// ticker renders one of these every 100ms).

func benchSplashModel(b *testing.B, ms int) Model {
	fixedEnv(b)
	cfg := config.Default()
	cfg.DataRepo = b.TempDir()
	cfg.Sync.Auto = false
	cfg.UI.Ambient = "both"
	cfg.UI.Splash = "on"
	root := b.TempDir()
	s, err := store.Open(root, filepath.Join(root, "index.json"))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = s.Close() })
	s.Clock = clock.Fixed(DumpFixedTime)
	m := New(cfg, s, sync.New(root))
	m.ambientSceneName = "night"
	if !m.startSplashIfEnabled(true) {
		b.Fatal("splash did not start")
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)
	m.ambientT = time.Duration(ms) * time.Millisecond
	return m
}

func BenchmarkSplashRenderFadeMid(b *testing.B) {
	m := benchSplashModel(b, 300)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.renderSplash()
	}
}

func BenchmarkSplashRenderLettersLanded(b *testing.B) {
	m := benchSplashModel(b, 1200)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.renderSplash()
	}
}
