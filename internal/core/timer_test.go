package core

import (
	"testing"
	"time"
)

var pomCfg = PhaseLengths{Work: 25 * time.Minute, ShortBreak: 5 * time.Minute, LongBreak: 15 * time.Minute, LongEvery: 4}

var t0 = time.Date(2026, 11, 8, 10, 0, 0, 0, time.Local)

func TestNewTimerModes(t *testing.T) {
	plain := NewTimer("abc123", TimerModeTimer, t0, pomCfg)
	plainView := plain.View(t0)
	if plainView.Phase != TimerPhaseWork || plainView.Total != 0 || plainView.Progress != 0 {
		t.Errorf("plain timer: %+v", plainView)
	}
	pom := NewTimer("abc123", TimerModePomodoro, t0, pomCfg)
	if pom.PhaseEndsAt != t0.Add(25*time.Minute) {
		t.Errorf("pomo end = %v", pom.PhaseEndsAt)
	}
	if v := pom.View(t0.Add(5 * time.Minute)); v.Remaining != 20*time.Minute {
		t.Errorf("remaining = %v", v.Remaining)
	}
}

func TestAccrualAndCap(t *testing.T) {
	plain := NewTimer("abc123", TimerModeTimer, t0, pomCfg)
	// 2h of work accrues fully.
	if got := plain.AccruedNow(t0.Add(2 * time.Hour)); got != 2*time.Hour {
		t.Errorf("accrued = %v", got)
	}
	// Stop banks the accrual.
	if got := plain.BankedOnStop(t0.Add(90 * time.Minute)); got != 90*time.Minute {
		t.Errorf("banked = %v", got)
	}
}

func TestWorkPhaseBudgetCap(t *testing.T) {
	pom := NewTimer("abc123", TimerModePomodoro, t0, pomCfg)
	// Work phase budget is 25 minutes; extra sitting is never credited.
	pom.Anchor = t0 // keep
	banked := pom.BankedOnStop(t0.Add(3 * time.Hour))
	if banked != 25*time.Minute {
		t.Errorf("banked = %v, want 25m (phase budget)", banked)
	}
}

func TestPauseShiftsPhaseEnd(t *testing.T) {
	pom := NewTimer("abc123", TimerModePomodoro, t0, pomCfg)
	pom.Pause(t0.Add(10 * time.Minute))
	time.Sleep(0)
	pom.Resume(t0.Add(70 * time.Minute)) // 60-minute pause
	if got := pom.PhaseEndsAt; !got.Equal(t0.Add(25*time.Minute + 60*time.Minute)) {
		t.Errorf("PhaseEndsAt = %v, want pause gap added", got)
	}
	if got := pom.Remaining(t0.Add(70 * time.Minute)); got != 15*time.Minute {
		t.Errorf("remaining = %v, want 15m", got)
	}
	// Work time excludes the pause.
	if got := pom.AccruedNow(t0.Add(70 * time.Minute)); got != 10*time.Minute {
		t.Errorf("accrued = %v, want 10m", got)
	}
}

func TestAdvanceWorkToBreakAndBack(t *testing.T) {
	pom := NewTimer("abc123", TimerModePomodoro, t0, pomCfg)
	// 25 minutes later: work → break, one pomodoro counted, 25m banked.
	events, stale := pom.Advance(t0.Add(25*time.Minute), pomCfg)
	if stale {
		t.Fatal("not stale")
	}
	if len(events) != 1 || events[0].From != TimerPhaseWork || events[0].To != TimerPhaseBreak {
		t.Fatalf("events = %+v", events)
	}
	if pom.Pomodoros != 1 || pom.Phase != TimerPhaseBreak {
		t.Fatalf("state = %+v", pom)
	}
	if got := pom.AccruedNow(t0.Add(25 * time.Minute)); got != 25*time.Minute {
		t.Errorf("accrued after transition = %v, want 25m", got)
	}
	// Break does not accrue work; the break end is 5 minutes after the work end.
	if got := pom.PhaseEndsAt; !got.Equal(t0.Add(30 * time.Minute)) {
		t.Errorf("break ends = %v", got)
	}
	// Accrual is frozen during the break.
	if got := pom.AccruedNow(t0.Add(28 * time.Minute)); got != 25*time.Minute {
		t.Errorf("accrued in break = %v, want 25m", got)
	}
	// Break expires → next work round, anchored at now.
	events, stale = pom.Advance(t0.Add(30*time.Minute), pomCfg)
	if stale {
		t.Fatal("not stale")
	}
	if len(events) != 1 || events[0].To != TimerPhaseWork || pom.PhaseIndex != 2 {
		t.Fatalf("events = %+v state %+v", events, pom)
	}
}

func TestLongBreakEveryFour(t *testing.T) {
	pom := NewTimer("abc123", TimerModePomodoro, t0, pomCfg)
	// Complete 4 work rounds; the 4th break is the long one.
	for round := 1; round <= 4; round++ {
		events, stale := pom.Advance(pom.PhaseEndsAt, pomCfg)
		if stale {
			t.Fatalf("round %d: stale", round)
		}
		if len(events) != 1 || events[0].To != TimerPhaseBreak {
			t.Fatalf("round %d: events = %+v", round, events)
		}
		if round < 4 {
			if got := pom.PhaseEndsAt.Sub(pom.Anchor); got != 5*time.Minute {
				t.Errorf("round %d break = %v, want 5m short break", round, got)
			}
			// Skip the break.
			events, stale = pom.Advance(pom.PhaseEndsAt, pomCfg)
			if stale || len(events) != 1 || events[0].To != TimerPhaseWork {
				t.Fatalf("round %d back-to-work: %+v stale=%v", round, events, stale)
			}
		}
	}
	if got := pom.PhaseEndsAt.Sub(pom.Anchor); got != 15*time.Minute {
		t.Errorf("4th break = %v, want 15m long break", got)
	}
}

func TestAdvanceStaleStopsInsteadOfCycling(t *testing.T) {
	pom := NewTimer("abc123", TimerModePomodoro, t0, pomCfg)
	// Machine "slept" 6 hours past the work-phase end.
	_, stale := pom.Advance(t0.Add(31*time.Hour), pomCfg)
	if !stale {
		t.Fatal("expected stale")
	}
	// Exactly one work phase was banked — never phantom hours.
	if got := pom.AccruedNow(t0.Add(31 * time.Hour)); got != 25*time.Minute {
		t.Errorf("accrued = %v, want 25m cap", got)
	}
	if pom.Pomodoros != 1 {
		t.Errorf("pomodoros = %d, want 1", pom.Pomodoros)
	}
}

func TestStalePlainTimerDetected(t *testing.T) {
	plain := NewTimer("abc123", TimerModeTimer, t0, pomCfg)
	if plain.Stale(t0.Add(30 * time.Minute)) {
		t.Error("30m timer marked stale")
	}
	if !plain.Stale(t0.Add(TimerCap + time.Minute)) {
		t.Error("over-cap timer not stale")
	}
}

func TestTogglePause(t *testing.T) {
	plain := NewTimer("abc123", TimerModeTimer, t0, pomCfg)
	plain.TogglePause(t0.Add(30 * time.Minute))
	if plain.Running {
		t.Fatal("still running after pause")
	}
	paused := plain.AccruedNow(t0.Add(90 * time.Minute))
	plain.TogglePause(t0.Add(60 * time.Minute))
	if !plain.Running {
		t.Fatal("not running after resume")
	}
	// 30m accrued pre-pause; the elapsed pause window accrues nothing.
	if paused != 30*time.Minute {
		t.Errorf("paused accrual = %v, want 30m", paused)
	}
	if got := plain.AccruedNow(t0.Add(90 * time.Minute)); got != 60*time.Minute {
		t.Errorf("accrual after resume = %v, want 60m", got)
	}
}
