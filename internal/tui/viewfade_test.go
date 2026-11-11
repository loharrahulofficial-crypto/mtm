package tui

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/ambient"
	"github.com/loharrahulofficial-crypto/mtm/internal/clock"
)

func TestViewFadeAmountRunsToZeroOverTicks(t *testing.T) {
	m, s := newTestModel(t, nil)
	defer func() { _ = s.Close() }()
	start := time.Date(2026, 11, 11, 12, 0, 0, 0, time.Local)
	m.viewFadeStart = start
	prev := 2.0
	for i := 0; i <= 5; i++ {
		at := start.Add(time.Duration(i) * viewFadeDur / 5)
		m.clock = clock.Fixed(at)
		got := m.viewFadeAmount()
		if i == 0 && got != 1 {
			t.Fatalf("amount at switch = %v, want 1", got)
		}
		if i > 0 && got >= prev {
			t.Fatalf("amount not decreasing at step %d: %v >= %v", i, got, prev)
		}
		prev = got
	}
	if prev != 0 {
		t.Fatalf("amount at end of transition = %v, want 0", prev)
	}
	m.clock = clock.Fixed(start.Add(viewFadeDur + time.Millisecond))
	if got := m.viewFadeAmount(); got != 0 {
		t.Fatalf("amount after duration = %v, want 0", got)
	}
	m.clock = clock.Fixed(start)
	if p := viewFadeProgress(m); p != 0 {
		t.Fatalf("progress at switch = %v, want 0", p)
	}
	m.clock = clock.Fixed(start.Add(viewFadeDur))
	if p := viewFadeProgress(m); p != 1 {
		t.Fatalf("progress after duration = %v, want 1", p)
	}
	m.viewFadeStart = time.Time{}
	if p := viewFadeProgress(m); p != 1 {
		t.Fatalf("progress with zero viewFadeStart = %v, want 1", p)
	}
}

func solidFrame(r, g, b int) string {
	line := "\x1b[48;2;" + strconv.Itoa(r) + ";" + strconv.Itoa(g) + ";" + strconv.Itoa(b) + "m     \x1b[0m"
	return strings.Join([]string{line, line, line}, "\n")
}

func TestCrossfadeFramesMidpointBlends(t *testing.T) {
	frameA := solidFrame(200, 60, 40) // red-ish
	frameB := solidFrame(40, 90, 220) // blue-ish
	mid := crossfadeFrames(frameA, frameB, 0.5)
	if mid == frameA || mid == frameB {
		t.Fatal("midpoint identical to an endpoint")
	}
	re := regexp.MustCompile(`48;2;(\d+);(\d+);(\d+)`)
	mm := re.FindStringSubmatch(mid)
	if mm == nil {
		t.Fatalf("no truecolor bg SGR in midpoint frame: %q", mid)
	}
	want := [3]int{(200 + 40) / 2, (60 + 90) / 2, (40 + 220) / 2}
	var got [3]int
	for i := 0; i < 3; i++ {
		v := 0
		for _, c := range mm[i+1] {
			v = v*10 + int(c-'0')
		}
		got[i] = v
		if got[i] < want[i]-2 || got[i] > want[i]+2 {
			t.Fatalf("channel %d = %d, want %d ± 2 (full match %q)", i, got[i], want[i], mm[0])
		}
	}
	// Endpoints: non-positive t returns curr exactly; Crossfade(t=1) returns to exactly.
	if got := crossfadeFrames(frameA, frameB, 0); got != frameB {
		t.Fatal("t=0 did not return curr exactly")
	}
	if got := crossfadeFrames(frameA, frameB, -0.25); got != frameB {
		t.Fatal("negative t did not return curr exactly")
	}
	aLines := strings.Split(frameA, "\n")
	bLines := strings.Split(frameB, "\n")
	if got := ambient.Crossfade(aLines, bLines, 1); strings.Join(got, "\n") != frameB {
		t.Fatal("ambient.Crossfade(t=1) did not return curr exactly")
	}
}

func TestCrossfadeFramesEmptyPrevReturnsCurr(t *testing.T) {
	curr := solidFrame(40, 90, 220)
	if got := crossfadeFrames("", curr, 0.5); got != curr {
		t.Fatal("empty prev did not return curr unchanged")
	}
}

func TestCrossfadeFramesLineCountMismatchReturnsCurr(t *testing.T) {
	prev := solidFrame(200, 60, 40)                 // 3 lines
	curr := solidFrame(40, 90, 220) + "\n" + "tail" // 4 lines
	if got := crossfadeFrames(prev, curr, 0.5); got != curr {
		t.Fatal("line-count mismatch did not return curr unchanged")
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
