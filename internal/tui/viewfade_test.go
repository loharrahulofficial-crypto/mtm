package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/clock"
)

func TestViewFadeAmount(t *testing.T) {
	m, s := newTestModel(t, nil)
	defer func() { _ = s.Close() }()
	fixed := time.Date(2026, 11, 11, 12, 0, 0, 0, time.Local)
	m.clock = clock.Fixed(fixed)
	if got := m.viewFadeAmount(); got != 0 {
		t.Fatalf("inactive fade amount = %v, want 0", got)
	}
	m.viewFadeStart = fixed
	if got := m.viewFadeAmount(); got <= 0 || got > 1 {
		t.Fatalf("just-armed fade amount = %v, want in (0, 1]", got)
	}
	m.viewFadeStart = fixed.Add(-viewFadeDur - time.Millisecond)
	if got := m.viewFadeAmount(); got != 0 {
		t.Fatalf("expired fade amount = %v, want 0", got)
	}
}

func TestFadeBaseFullDimBecomesBase(t *testing.T) {
	line := "\x1b[38;2;200;100;50mred text \x1b[48;5;21m on blue"
	got := fadeBase(line, 1.0, "#14282d")
	for _, seq := range scanSGR(got) {
		params := strings.TrimSuffix(strings.TrimPrefix(seq.seq, "\x1b["), "m")
		if strings.Contains(params, "200;100;50") || strings.Contains(params, ";21m") {
			t.Fatalf("unblended color survived full fade: %q", seq.seq)
		}
	}
	if !strings.Contains(got, fmt.Sprintf("%d;%d;%d", 20, 40, 45)) {
		t.Fatalf("base color missing after full fade: %q", got)
	}
}

func TestFadeBaseZeroIsIdentity(t *testing.T) {
	line := "\x1b[38;2;200;100;50mplain \x1b[0m\x1b[48;5;21mblue"
	if got := fadeBase(line, 0, "#14282d"); got != line {
		t.Fatalf("zero fade changed the line:\ngot  %q\nwant %q", got, line)
	}
}

func TestTabSwitchArmsFadeAndTick(t *testing.T) {
	m, s := newTestModel(t, nil)
	defer func() { _ = s.Close() }()
	m.view = viewHome
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m = updated.(Model)
	if m.view != viewList {
		t.Fatalf("view = %v, want viewList", m.view)
	}
	if m.viewFadeStart.IsZero() {
		t.Fatal("view change did not arm the fade")
	}
	if cmd == nil {
		t.Fatal("no fade tick cmd after view change")
	}
	if a := m.viewFadeAmount(); a <= 0.9 {
		t.Fatalf("fade amount right after switch = %v, want ~1", a)
	}
}

func TestTabSwitchNoFadeWithReducedMotion(t *testing.T) {
	m, s := newTestModel(t, nil)
	defer func() { _ = s.Close() }()
	m.cfg.UI.ReducedMotion = true
	m.view = viewHome
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m = updated.(Model)
	if !m.viewFadeStart.IsZero() {
		t.Fatal("fade armed despite reduced motion")
	}
}
