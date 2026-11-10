package tui

import (
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/clock"
	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

// The idle contract: with ambient off and no input, ten seconds of ticks
// repaint nothing; only the minute rollover repaints (the footer clock);
// a real keypress always repaints.
func TestIdleAmbientOffEmitsZeroRepaints(t *testing.T) {
	fixedEnv(t)
	root := t.TempDir()
	s, err := store.Open(root, filepath.Join(t.TempDir(), "cache.json"))
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 11, 10, 15, 30, 5, 0, time.Local) // 5s into the minute
	s.Clock = clock.Fixed(t0)
	cfg := config.Default()
	cfg.DataRepo = root
	cfg.Sync.Auto = false
	cfg.UI.Ambient = "off"

	m := New(cfg, s, sync.New(root))
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	_ = m.View() // initial frame
	base := atomic.LoadInt64(&frameBuilds)

	// Simulate ten seconds: one timer tick per second, one theme poll
	// every other second, the spinner every 100ms.
	for i := 1; i <= 10; i++ {
		m.clock = clock.Fixed(t0.Add(time.Duration(i) * time.Second))
		var u tea.Model
		u, _ = m.Update(timerTickMsg{})
		m = u.(Model)
		if i%2 == 0 {
			u, _ = m.Update(themeTickMsg(time.Now()))
			m = u.(Model)
		}
		for j := 0; j < 10; j++ {
			u, _ = m.Update(spinnerTickMsgForTest())
			m = u.(Model)
		}
		_ = m.View()
	}
	if got := atomic.LoadInt64(&frameBuilds) - base; got != 0 {
		t.Fatalf("idle ticks with ambient off rebuilt the frame %d times, want 0", got)
	}

	// Minute rollover: exactly one repaint (the footer clock).
	m.clock = clock.Fixed(t0.Add(time.Minute))
	u, _ := m.Update(timerTickMsg{})
	m = u.(Model)
	_ = m.View()
	if got := atomic.LoadInt64(&frameBuilds) - base; got != 1 {
		t.Fatalf("minute rollover rebuilt the frame %d times, want 1", got)
	}

	// A real keypress must repaint.
	base2 := atomic.LoadInt64(&frameBuilds)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = u.(Model)
	_ = m.View()
	if got := atomic.LoadInt64(&frameBuilds) - base2; got != 1 {
		t.Fatalf("a keypress rebuilt the frame %d times, want 1", got)
	}
}

// spinnerTickMsgForTest stands in for a spinner tick at rest (not syncing).
func spinnerTickMsgForTest() tea.Msg {
	return spinner.TickMsg{Time: time.Now()}
}

// The clock redraw alignment: an idle timer tick arms the next wake at the
// minute rollover, not one second later.
func TestIdleTimerTickIntervalAlignsToMinute(t *testing.T) {
	fixedEnv(t)
	root := t.TempDir()
	s, err := store.Open(root, filepath.Join(t.TempDir(), "cache.json"))
	if err != nil {
		t.Fatal(err)
	}
	s.Clock = clock.Fixed(time.Now())
	cfg := config.Default()
	cfg.DataRepo = root
	cfg.Sync.Auto = false
	cfg.UI.Ambient = "off"
	m := New(cfg, s, sync.New(root))
	if d := timerTickInterval(m); d <= 0 || d > time.Minute+time.Second {
		t.Fatalf("idle interval = %v, want (0, ~1m]", d)
	}
}
