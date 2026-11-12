package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/clock"
	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/paths"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

// splash_startup_test.go — §7.1 & §7.2: the very first rendered frame is
// the plain base-colored frame; ui.splash controls when the splash shows.

// splashModel builds a splash-capable model against the CURRENT process
// env (callers set fixedEnv(t) once per subtest for isolation/stamp paths).
func splashModel(t *testing.T, splash string) Model {
	t.Helper()
	cfg := config.Default()
	cfg.DataRepo = t.TempDir()
	cfg.Sync.Auto = false
	cfg.UI.Ambient = "both"
	cfg.UI.Splash = splash
	root := t.TempDir()
	s, err := store.Open(root, filepath.Join(root, "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	s.Clock = clock.Fixed(DumpFixedTime)
	m := New(cfg, s, sync.New(root))
	return m
}

func TestFirstSplashFrameIsPlainBase(t *testing.T) {
	fixedEnv(t)
	m := splashModel(t, "on")
	m.ambientSceneName = "night"
	if !m.startSplashIfEnabled(true) {
		t.Fatal("splash did not start")
	}
	up, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = up.(Model)
	m.ambientT = 0
	frame := m.View()
	for i, l := range strings.Split(frame, "\n") {
		for _, r := range stripANSI(l) {
			if r != ' ' {
				t.Fatalf("line %d: first frame contains glyph %q, want a plain base-colored frame", i, string(r))
			}
		}
	}
	// the base background must be what the cells sit on
	if !strings.Contains(frame, "48;2;20;40;45") {
		t.Fatalf("first frame does not sit on the base background #14282d")
	}
}

func TestSplashConfigPolicy(t *testing.T) {
	t.Run("off skips the splash", func(t *testing.T) {
		fixedEnv(t)
		m := splashModel(t, "off")
		if m.startSplashIfEnabled(true) {
			t.Fatal("ui.splash: off still started the splash")
		}
	})
	t.Run("on starts the splash", func(t *testing.T) {
		fixedEnv(t)
		m := splashModel(t, "on")
		if !m.startSplashIfEnabled(true) {
			t.Fatal("ui.splash: on did not start the splash")
		}
	})
	t.Run("once-a-day shows once per local day", func(t *testing.T) {
		fixedEnv(t)
		m := splashModel(t, "once-a-day")
		if !m.startSplashIfEnabled(true) {
			t.Fatal("once-a-day did not start the splash on the first launch of the day")
		}
		// the stamp records today's local date
		data, err := os.ReadFile(paths.StateFile("splash.stamp"))
		if err != nil {
			t.Fatalf("once-a-day did not write the splash stamp: %v", err)
		}
		want := m.clock.Now().Format("2006-01-02")
		if strings.TrimSpace(string(data)) != want {
			t.Fatalf("splash stamp = %q, want %q", strings.TrimSpace(string(data)), want)
		}
		// a second launch the same day skips it
		m2 := splashModel(t, "once-a-day")
		if m2.startSplashIfEnabled(true) {
			t.Fatal("once-a-day started the splash twice on the same day")
		}
	})
	t.Run("once-a-day shows again on a new day", func(t *testing.T) {
		fixedEnv(t)
		m := splashModel(t, "once-a-day")
		if !m.startSplashIfEnabled(true) {
			t.Fatal("once-a-day did not start the splash on the first launch of the day")
		}
		// backdate the stamp by one day
		yesterday := m.clock.Now().AddDate(0, 0, -1).Format("2006-01-02")
		if err := os.MkdirAll(filepath.Dir(paths.StateFile("splash.stamp")), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(paths.StateFile("splash.stamp"), []byte(yesterday+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		m2 := splashModel(t, "once-a-day")
		if !m2.startSplashIfEnabled(true) {
			t.Fatal("once-a-day did not start the splash after a new day began")
		}
	})
	t.Run("env override wins over config", func(t *testing.T) {
		fixedEnv(t)
		t.Setenv("MTM_NO_SPLASH", "1")
		m := splashModel(t, "on")
		if m.startSplashIfEnabled(true) {
			t.Fatal("MTM_NO_SPLASH=1 did not suppress the splash")
		}
	})
}

func TestSplashKeySkipBehavior(t *testing.T) {
	// any key during the splash skips immediately with a short fade:
	// the skip target is <350ms away, never a hard cut to full duration.
	fixedEnv(t)
	m := splashModel(t, "on")
	m.ambientSceneName = "night"
	if !m.startSplashIfEnabled(true) {
		t.Fatal("splash did not start")
	}
	m.splashTotal = 3 * time.Second
	m.ambientT = 0
	m.skipSplash()
	want := m.splashTotal - 350*time.Millisecond
	if m.ambientT != want {
		t.Fatalf("skipSplash moved to %v, want %v", m.ambientT, want)
	}
	// a key press late in the splash does not bounce the clock backwards
	m.ambientT = m.splashTotal - 100*time.Millisecond
	m.skipSplash()
	if m.ambientT != m.splashTotal-100*time.Millisecond {
		t.Fatalf("skipSplash moved the clock backwards to %v", m.ambientT)
	}
}

func TestRenderSplashDump(t *testing.T) {
	fixedEnv(t)
	cfg := config.Default()
	cfg.DataRepo = t.TempDir()
	cfg.Sync.Auto = false
	cfg.UI.Ambient = "both"
	root := t.TempDir()
	s, err := store.Open(root, filepath.Join(root, "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	// invalid times are rejected
	if _, err := RenderSplashDump(cfg, s, sync.New(s.Root), 100, 30, -1); err == nil {
		t.Fatal("--splash-t -1 was accepted")
	}

	// t=0 renders the plain base-colored frame (no wordmark yet)
	frame, err := RenderSplashDump(cfg, s, sync.New(s.Root), 100, 30, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i, l := range strings.Split(frame, "\n") {
		for _, r := range stripANSI(l) {
			if r != ' ' {
				t.Fatalf("t=0 dump line %d contains glyph %q, want a plain base-colored frame", i, string(r))
			}
		}
	}

	// t=0 shows no bright letter cells in the wordmark band; t=1.2 and
	// later do (the letters are painted through the raster now, so the
	// check counts bright foreground cells, not a specific glyph).
	brightLetterCells := func(frame string) int {
		grid := parseFrame(frame)
		n := 0
		for y := 3; y < 14; y++ {
			for x := 20; x < 75; x++ {
				if fg := grid.foreground(y, x); fg != "" && fgCellLum(fg) > 0.55 {
					n++
				}
			}
		}
		return n
	}
	if got := brightLetterCells(frame); got != 0 {
		t.Fatalf("t=0 shows %d bright cells in the wordmark band, want 0", got)
	}
	frame, err = RenderSplashDump(cfg, s, sync.New(s.Root), 100, 30, 1.2)
	if err != nil {
		t.Fatal(err)
	}
	if got := brightLetterCells(frame); got < 4 {
		t.Fatalf("t=1.2 shows %d bright cells in the wordmark band, want >= 4 (the letters mid-rise)", got)
	}
	// fully landed letters at t=2.5 are bright across the block
	frame, err = RenderSplashDump(cfg, s, sync.New(s.Root), 100, 30, 2.5)
	if err != nil {
		t.Fatal(err)
	}
	if got := brightLetterCells(frame); got < 24 {
		t.Fatalf("t=2.5 shows %d bright cells in the wordmark band, want >= 24 (fully-lit letters)", got)
	}

	// every reviewed time stays free of the old magenta sentinel
	for _, ts := range []float64{0.1, 0.3, 0.55, 1.2, 2.8} {
		frame, err = RenderSplashDump(cfg, s, sync.New(s.Root), 100, 30, ts)
		if err != nil {
			t.Fatalf("t=%v: %v", ts, err)
		}
		for _, sentinel := range []string{"255;0;255", "ff00ff", "FF00FF"} {
			if strings.Contains(frame, sentinel) {
				t.Fatalf("t=%v dump contains sentinel %q", ts, sentinel)
			}
		}
	}
}
