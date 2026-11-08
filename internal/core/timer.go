package core

import "time"

// Timer modes and phases. Mode distinguishes a plain open-ended stopwatch
// from a pomodoro cycle; phase distinguishes accruing work time from
// non-accruing breaks. PhaseIdle is the absence of a timer.
const (
	TimerModeTimer    = "timer"
	TimerModePomodoro = "pomodoro"
	TimerPhaseWork    = "work"
	TimerPhaseBreak   = "break"
	TimerPhaseIdle    = "idle"
)

const (
	// TimerCap bounds a single plain-timer session; stopping past it banks
	// the cap, never the wall-clock gap (a machine asleep for a day is not
	// 24 hours of study).
	TimerCap = 4 * time.Hour
	// TimerStaleAfter is how far past a pomodoro phase end the timer may
	// drift before it counts as abandoned (reboot, forgotten session).
	// Accruing is capped at the phase length and the timer stops itself.
	TimerStaleAfter = 2 * time.Hour
)

// PhaseLengths carries the configured pomodoro durations.
type PhaseLengths struct {
	Work       time.Duration
	ShortBreak time.Duration
	LongBreak  time.Duration
	LongEvery  int
}

func (p PhaseLengths) breakFor(pomodoros int) time.Duration {
	if p.LongEvery > 0 && pomodoros%p.LongEvery == 0 {
		return p.LongBreak
	}
	return p.ShortBreak
}

// TimerState is the persisted runtime timer. It lives in the state file
// (outside the data repo); only the accrued result is ever written into a
// task file (time_logged_minutes).
type TimerState struct {
	TaskID string `json:"task_id"`
	Mode   string `json:"mode"`
	Phase  string `json:"phase"`
	// Anchor is the start of the current running segment; accrual grows
	// from Anchor while Running and Phase == work.
	Anchor       time.Time `json:"anchor"`
	Running      bool      `json:"running"`
	Accrued      int64     `json:"accrued"`       // work seconds banked across all segments
	PhaseElapsed int64     `json:"phase_elapsed"` // work seconds banked within the current phase
	PhaseEndsAt  time.Time `json:"phase_ends_at"` // zero for an open-ended (mode=timer) work phase
	PhaseTotal   int64     `json:"phase_total"`   // seconds; 0 = open-ended
	PhaseIndex   int       `json:"phase_index"`   // pomodoro: current work round (1-based)
	Pomodoros    int       `json:"pomodoros"`     // completed work rounds
	PausedAt     time.Time `json:"paused_at,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TimerEvent is one phase transition produced by advancing the state.
type TimerEvent struct {
	From      string // phase we came from
	To        string // phase we moved into
	Banked    time.Duration
	Pomodoros int
}

// NewTimer starts a work phase for taskID. mode=pomodoro bounds work phases
// by cfg.Work and cycles through breaks; mode=timer leaves the phase
// open-ended (no phase end, no breaks).
func NewTimer(taskID, mode string, now time.Time, cfg PhaseLengths) *TimerState {
	st := &TimerState{
		TaskID:     taskID,
		Mode:       mode,
		Phase:      TimerPhaseWork,
		Anchor:     now,
		Running:    true,
		PhaseIndex: 1,
		UpdatedAt:  now,
	}
	if mode == TimerModePomodoro {
		st.beginPhase(now, cfg.Work)
	}
	return st
}

func (st *TimerState) beginPhase(now time.Time, d time.Duration) {
	st.PhaseElapsed = 0
	st.Anchor = now
	st.Running = true
	st.PhaseTotal = int64(d / time.Second)
	st.PhaseEndsAt = now.Add(d)
}

func (st *TimerState) elapsedSegment(now time.Time) time.Duration {
	if !st.Running || st.Phase != TimerPhaseWork {
		return 0
	}
	d := now.Sub(st.Anchor)
	if d < 0 {
		d = 0
	}
	return d
}

// bankSegment credits the running segment, capped by what remains of the
// current phase budget. It is safe to call at any point; anything past the
// phase end is idle time and is not credited.
func (st *TimerState) bankSegment(end time.Time) {
	if !st.Running || st.Phase != TimerPhaseWork {
		st.Anchor = end
		return
	}
	seg := end.Sub(st.Anchor)
	if seg < 0 {
		seg = 0
	}
	if st.PhaseTotal > 0 {
		budget := time.Duration(st.PhaseTotal)*time.Second - time.Duration(st.PhaseElapsed)*time.Second
		if budget < 0 {
			budget = 0
		}
		if seg > budget {
			seg = budget
		}
	}
	st.Accrued += int64(seg / time.Second)
	st.PhaseElapsed += int64(seg / time.Second)
	st.Anchor = end
}

// AccruedNow reports the total work time banked so far this session.
func (st *TimerState) AccruedNow(now time.Time) time.Duration {
	seg := st.elapsedSegment(now)
	if st.PhaseTotal > 0 {
		budget := time.Duration(st.PhaseTotal)*time.Second - time.Duration(st.PhaseElapsed)*time.Second
		if budget < 0 {
			budget = 0
		}
		if seg > budget {
			seg = budget
		}
	}
	return time.Duration(st.Accrued)*time.Second + seg
}

// Pause halts accrual. For pomodoro phases the end shifts by the paused
// length, so a paused work phase stays a full work phase.
func (st *TimerState) Pause(now time.Time) {
	if !st.Running {
		return
	}
	st.bankSegment(now)
	st.Running = false
	st.PausedAt = now
	st.UpdatedAt = now
}

// Resume continues after a Pause; the phase end shifts by the pause gap.
func (st *TimerState) Resume(now time.Time) {
	if st.Running {
		return
	}
	gap := time.Duration(0)
	if !st.PausedAt.IsZero() {
		gap = now.Sub(st.PausedAt)
	}
	if !st.PhaseEndsAt.IsZero() {
		st.PhaseEndsAt = st.PhaseEndsAt.Add(gap)
	}
	st.PausedAt = time.Time{}
	st.Anchor = now
	st.Running = true
	st.UpdatedAt = now
}

// TogglePause flips between Pause and Resume.
func (st *TimerState) TogglePause(now time.Time) {
	if st.Running {
		st.Pause(now)
		return
	}
	st.Resume(now)
}

// Advance transitions phases that ended since the last tick, banking capped
// work segments. New phases always anchor at now (never in the past), so a
// missed break never manufactures extra work time; a pomodoro so stale the
// break would have cycled is not advanced at all — Advance reports stale and
// the caller finalizes the session (capped accrual, state cleared).
func (st *TimerState) Advance(now time.Time, cfg PhaseLengths) (events []TimerEvent, stale bool) {
	for !st.PhaseEndsAt.IsZero() && !now.Before(st.PhaseEndsAt) {
		if st.PhaseEndsAt.Before(now.Add(-TimerStaleAfter)) {
			if st.Phase == TimerPhaseWork {
				st.bankSegment(st.PhaseEndsAt)
				st.Pomodoros++
			}
			st.UpdatedAt = now
			return events, true
		}
		end := st.PhaseEndsAt
		if st.Phase == TimerPhaseWork {
			before := st.Accrued
			st.bankSegment(end)
			banked := time.Duration(st.Accrued-before) * time.Second
			st.Pomodoros++
			breakLen := cfg.breakFor(st.Pomodoros)
			st.Phase = TimerPhaseBreak
			st.beginPhase(now, breakLen)
			events = append(events, TimerEvent{From: TimerPhaseWork, To: TimerPhaseBreak, Banked: banked, Pomodoros: st.Pomodoros})
			break // anchored at now; nothing else due
		} else {
			st.Phase = TimerPhaseWork
			st.PhaseIndex++
			st.beginPhase(now, cfg.Work)
			events = append(events, TimerEvent{From: TimerPhaseBreak, To: TimerPhaseWork, Pomodoros: st.Pomodoros})
			break
		}
	}
	st.UpdatedAt = now
	return events, false
}

// Remaining returns how long is left in the current phase (zero for open-ended).
func (st *TimerState) Remaining(now time.Time) time.Duration {
	if st.PhaseEndsAt.IsZero() {
		return 0
	}
	ref := now
	if !st.Running && !st.PausedAt.IsZero() {
		ref = st.PausedAt
	}
	d := st.PhaseEndsAt.Sub(ref)
	if d < 0 {
		return 0
	}
	return d
}

// Progress reports the phase completion fraction in [0,1]; 0 for open-ended.
// It counts down uniformly for work and break phases alike.
func (st *TimerState) Progress(now time.Time) float64 {
	if st.PhaseTotal <= 0 {
		return 0
	}
	total := time.Duration(st.PhaseTotal) * time.Second
	el := total - st.Remaining(now)
	frac := float64(el) / float64(total)
	if frac < 0 {
		return 0
	}
	if frac > 1 {
		return 1
	}
	return frac
}

// Stale reports whether the session looks abandoned: a pomodoro phase that
// ended more than TimerStaleAfter ago, or a plain timer that has accrued
// past TimerCap.
func (st *TimerState) Stale(now time.Time) bool {
	if !st.PhaseEndsAt.IsZero() && now.Sub(st.PhaseEndsAt) > TimerStaleAfter {
		return true
	}
	return st.AccruedNow(now) > TimerCap
}

// TimerView is the clean snapshot every surface renders from (statusline,
// Focus mode, mtm summary, ambient scenes). Phase is work|break|idle.
type TimerView struct {
	Phase      string
	Mode       string
	TaskID     string
	Running    bool
	Progress   float64 // 0 when the phase has no fixed end
	Remaining  time.Duration
	Total      time.Duration
	Accrued    time.Duration
	Pomodoros  int
	PhaseIndex int
	Stale      bool
}

// View renders the state as a TimerView for display.
func (st *TimerState) View(now time.Time) TimerView {
	if st == nil {
		return TimerView{Phase: TimerPhaseIdle}
	}
	v := TimerView{
		Phase:      st.Phase,
		Mode:       st.Mode,
		TaskID:     st.TaskID,
		Running:    st.Running,
		Progress:   st.Progress(now),
		Accrued:    st.AccruedNow(now),
		Pomodoros:  st.Pomodoros,
		PhaseIndex: st.PhaseIndex,
		Stale:      st.Stale(now),
	}
	if !st.PhaseEndsAt.IsZero() {
		v.Total = time.Duration(st.PhaseTotal) * time.Second
		v.Remaining = st.PhaseEndsAt.Sub(now)
		if v.Remaining < 0 {
			v.Remaining = 0
		}
	}
	return v
}

// BankedOnStop credits the final segment and returns the session's accrued
// work time. Callers persist time_logged_minutes and delete the state file.
func (st *TimerState) BankedOnStop(now time.Time) time.Duration {
	st.bankSegment(now)
	return time.Duration(st.Accrued) * time.Second
}
