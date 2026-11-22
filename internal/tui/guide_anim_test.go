package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/clock"
	"github.com/loharrahulofficial-crypto/mtm/internal/mascot"
)

func guideModelOf(u tea.Model) Model {
	switch v := u.(type) {
	case Model:
		return v
	case *Model:
		return *v
	}
	panic("bad model")
}

func guideModel(t *testing.T) Model {
	t.Helper()
	m, _ := newTestModel(t, nil)
	m.clock = clock.Fixed(time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC))
	return m
}

func TestGuideStepsBasics(t *testing.T) {
	if guideStepCount() == 0 {
		t.Fatal("no guide steps")
	}
	for i, s := range guideSteps {
		if s.Title == "" || s.Body == "" {
			t.Errorf("step %d: title or body empty", i)
		}
		if s.Pose < 0 || s.Pose >= mascot.NumPoses() {
			t.Errorf("step %d: pose %d out of range", i, s.Pose)
		}
	}
}

func TestGuideTypewriter(t *testing.T) {
	m := guideModel(t)
	m.openGuide()
	if m.guide.Step != 0 {
		t.Fatal("step not 0 after openGuide")
	}
	if got := m.guideText(); got != "" {
		t.Fatalf("expected empty text at t=0, got %q", got)
	}
	m.clock = clock.Fixed(time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC).Add(time.Hour))
	if got := m.guideText(); got != guideSteps[0].Body {
		t.Fatalf("after 1h expected full body, got %q", got)
	}
}

func TestGuideEnterAdvances(t *testing.T) {
	m := guideModel(t)
	m.openGuide()
	m.clock = clock.Fixed(m.clock.Now().Add(time.Hour))
	u, _ := m.Update(keyMsg("enter"))
	m = guideModelOf(u)
	if m.guide.Step != 0 {
		t.Fatalf("first enter reveals; step = %d, want 0", m.guide.Step)
	}
	u, _ = m.Update(keyMsg("enter"))
	m = guideModelOf(u)
	if m.guide.Step != 1 {
		t.Fatalf("second enter should advance to step 1, got %d", m.guide.Step)
	}
}

func TestGuideTypingSkippedByN(t *testing.T) {
	m := guideModel(t)
	m.openGuide()
	u, _ := m.Update(keyMsg("n"))
	m = guideModelOf(u)
	if m.guide.Step != 1 {
		t.Fatalf("n should advance to step 1, got %d", m.guide.Step)
	}
	if m.guideText() != "" {
		t.Fatal("guideText should start empty on a new step")
	}
}

func TestGuideTypingSkippedByArrowRight(t *testing.T) {
	m := guideModel(t)
	m.openGuide()
	u, _ := m.Update(keyMsg("right"))
	m = guideModelOf(u)
	if m.guide.Step != 1 {
		t.Fatalf("right should advance to step 1, got %d", m.guide.Step)
	}
}

func TestGuideClosesAtLastStep(t *testing.T) {
	m := guideModel(t)
	m.openGuide()
	m.guide.Step = guideStepCount() - 1
	m.guide.Revealed = true
	m.clock = clock.Fixed(m.clock.Now().Add(time.Hour))
	u, _ := m.Update(keyMsg("enter"))
	m = guideModelOf(u)
	if m.mode != modeNormal {
		t.Fatal("guide should close when advancing past the last step")
	}
}

func TestGuideArmHygiene(t *testing.T) {
	m := guideModel(t)
	m.openGuide()
	if cmd := m.armGuide(); cmd == nil {
		t.Fatal("armGuide nil for focused guide")
	}
	m.guideArmed = true
	m.ambientFocused = false
	if cmd := m.armGuide(); cmd != nil {
		t.Fatal("armGuide armed while blurred")
	}
	m.ambientFocused = true
	m.cfg.UI.ReducedMotion = true
	if cmd := m.armGuide(); cmd != nil {
		t.Fatal("armGuide armed under reduced motion")
	}
	m.mode = modeNormal
	if cmd := m.armGuide(); cmd != nil {
		t.Fatal("armGuide armed when not in guide mode")
	}
}

func TestGuideBlurStopsTicking(t *testing.T) {
	m := guideModel(t)
	m.openGuide()
	m.ambientFocused = true
	_ = m.armGuide()
	u, _ := m.Update(tea.BlurMsg{})
	m = guideModelOf(u)
	if m.guideArmed {
		t.Fatal("guide kept ticking on BlurMsg")
	}
}

func TestGuideTickWhenClosed(t *testing.T) {
	m := guideModel(t)
	m.mode = modeNormal
	m.guideArmed = true
	u, _ := m.Update(guideTickMsg{})
	m = guideModelOf(u)
	if m.guideArmed {
		t.Fatal("guide armed while closed")
	}
}

func TestGuideCardBounds(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}, {200, 50}} {
		w, h := size[0], size[1]
		for step := 0; step < guideStepCount(); step++ {
			m := guideModel(t)
			m.mode = modeGuide
			m.guide.Step = step
			m.guide.Revealed = true
			m.clock = clock.Fixed(m.clock.Now().Add(time.Hour))
			view := m.renderGuide(w, h)
			lines := strings.Split(view, "\n")
			if len(lines) != h {
				t.Fatalf("size %v step %d: frame has %d rows, want %d", size, step, len(lines), h)
			}
			for _, ln := range lines {
				if fastWidth(ln) > w {
					t.Fatalf("size %v step %d: line wider than %d", size, step, w)
				}
			}
		}
	}
}

func TestGuideCardPositionDocksToSidebar(t *testing.T) {
	m := guideModel(t)
	m.mode = modeGuide
	m.guide.Step = 1 // sidebar spotlight
	m.guide.Revealed = true
	x, _ := m.guideCardRect(m.width, m.height)
	if x <= 0 {
		t.Fatalf("expected card right of sidebar, got x=%d", x)
	}
}

func TestGuideTypewriterDeterministic(t *testing.T) {
	m := guideModel(t)
	m.openGuide()
	m.clock = clock.Fixed(m.clock.Now().Add(12345 * time.Millisecond))
	a := m.guideText()
	b := m.guideText()
	if a != b {
		t.Fatal("guideText not deterministic at same clock")
	}
	m.clock = clock.Fixed(m.clock.Now().Add(10 * time.Second))
	c := m.guideText()
	if len(c) < len(a) {
		t.Fatal("guideText should reveal more over time")
	}
	if !strings.HasPrefix(c, a) && c != guideSteps[m.guide.Step].Body {
		t.Fatal("guideText should reveal in order")
	}
}
