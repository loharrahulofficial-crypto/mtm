package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/clock"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

// edge_test.go — item 2 contract: the splash, the Focus view and the
// :ambient preview frame cover every one of width × height cells —
// including the last row and the last column — with no unpainted cell.
// "Unpainted" means a cell carrying the terminal default background
// (the scenes base-fill every cell; overlay cells take the sampled
// scene background).

func checkFullCoverage(t *testing.T, name, frame string, w, h int) {
	t.Helper()
	lines := strings.Split(frame, "\n")
	if len(lines) != h {
		t.Fatalf("%s: %d lines, want %d (full frame, last row included)", name, len(lines), h)
	}
	grid := parseFrame(frame)
	for r := 0; r < h; r++ {
		if len(grid.bg[r]) != w {
			t.Fatalf("%s: row %d has %d cells, want %d (last column included)", name, r, len(grid.bg[r]), w)
		}
		for c := 0; c < w; c++ {
			if grid.bg[r][c] == "" {
				t.Fatalf("%s: cell (%d,%d) carries the terminal default background (unpainted)", name, c, r)
			}
		}
	}
}

func TestSplashFrameCoversEveryCell(t *testing.T) {
	for _, size := range [][2]int{{60, 15}, {80, 24}, {100, 30}, {120, 40}, {200, 50}} {
		m, s := newTestModel(t, nil)
		s.Clock = clock.Fixed(DumpFixedTime)
		m.cfg.UI.Ambient = "both"
		m.animations = true
		if !m.startSplashIfEnabled(true) {
			t.Fatal("splash did not start")
		}
		up, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m = up.(Model)
		m.ambientT = 1500 * time.Millisecond
		frame := m.View()
		checkFullCoverage(t, "splash", frame, size[0], size[1])
		_ = s.Close()
	}
}

func TestFocusFrameCoversEveryCell(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {100, 30}, {120, 40}} {
		s := twoTasksFixture(t)
		cfg := dumpConfig()
		m := BuildDumpModel(cfg, s, sync.New(s.Root), "focus", size[0], size[1], DumpFixedTime)
		frame := m.View()
		checkFullCoverage(t, "focus", frame, size[0], size[1])
		_ = s.Close()
	}
}

func TestAmbientPreviewFrameCoversEveryCell(t *testing.T) {
	for _, size := range [][2]int{{100, 30}, {120, 40}, {200, 50}} {
		m, s := newTestModel(t, nil)
		s.Clock = clock.Fixed(DumpFixedTime)
		up, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m = up.(Model)
		m.mode = modeAmbient
		m.ambientT = 1 * time.Second
		frame := m.View()
		checkFullCoverage(t, "ambient preview", frame, size[0], size[1])
		_ = s.Close()
	}
}
