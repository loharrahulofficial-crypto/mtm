package ambient

import (
	"testing"
	"time"
)

// reduced_motion_test.go — §4.6: reduced motion slows every scene to a
// crawl and freezes reward frames; no crossfades (see tui tests).

func TestReducedMotionCrawlsTime(t *testing.T) {
	sReduced := New("ocean", ModeHalfblock)
	sFull := New("ocean", ModeHalfblock)
	st := stateFor(0.4, PhaseWork)
	stR := st
	stR.ReducedMotion = true
	// 8s of wall time at 12% speed equals 0.96s of full animation
	// (ocean has no reducedAmp-damped oscillator, so time dominates)
	longT := sReduced.Frame(8*time.Second, 60, 14, stR)
	shortT := sFull.Frame(960*time.Millisecond, 60, 14, st)
	same := true
	for i := range longT {
		if longT[i] != shortT[i] {
			same = false
			break
		}
	}
	if !same {
		t.Fatal("reduced motion did not crawl time to 0.12x")
	}
}

func TestReducedMotionStillAdvancesLegitimately(t *testing.T) {
	s := New("night", ModeHalfblock)
	st := stateFor(0.4, PhaseWork)
	st.ReducedMotion = true
	a := s.Frame(0, 60, 14, st)
	b := s.Frame(4*time.Second, 60, 14, st) // 0.48s inside
	same := true
	for i := range a {
		if a[i] != b[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("reduced motion froze the scene completely — idle breathing expected")
	}
}

func TestReducedMotionRewardFrameIsFrozen(t *testing.T) {
	s := New("bonsai", ModeHalfblock)
	st := stateFor(1, PhaseWork)
	st.ReducedMotion = true
	a := s.Reward(2*time.Second, 60, 14, st)
	b := s.Reward(7*time.Second, 60, 14, st)
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("reduced motion reward frame advanced — expected a frozen still")
		}
	}
}
