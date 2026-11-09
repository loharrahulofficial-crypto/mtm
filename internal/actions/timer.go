package actions

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/paths"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
)

const (
	timerStateName = "timer.json"
	timerLockName  = "timer.lock"
)

// TimerPhaseLengths maps config to the engine's PhaseLengths.
func TimerPhaseLengths(cfg config.Config) core.PhaseLengths {
	p := cfg.Pomodoro
	work := time.Duration(p.WorkMinutes) * time.Minute
	shortBreak := time.Duration(p.ShortBreakMinutes) * time.Minute
	longBreak := time.Duration(p.LongBreakMinutes) * time.Minute
	if work <= 0 {
		work = 25 * time.Minute
	}
	if shortBreak <= 0 {
		shortBreak = 5 * time.Minute
	}
	if longBreak <= 0 {
		longBreak = 15 * time.Minute
	}
	return core.PhaseLengths{Work: work, ShortBreak: shortBreak, LongBreak: longBreak, LongEvery: p.LongEvery}
}

// withTimerLock serializes read-modify-write cycles of the timer state file.
// paths.StateDir() guards the directory; the file itself is created 0600.
func withTimerLock(fn func(dir string) error) error {
	dir := paths.StateDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("state dir: %w", err)
	}
	lock, err := os.OpenFile(filepath.Join(dir, timerLockName), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("timer lock: %w", err)
	}
	defer func() { _ = lock.Close() }()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("timer lock: %w", err)
	}
	defer func() { _ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) }()
	return fn(dir)
}

// loadTimerState reads the state file. A missing or corrupt file yields nil
// (no active timer); a corrupt file is renamed aside, never silently kept.
func loadTimerState(dir string) *core.TimerState {
	data, err := os.ReadFile(filepath.Join(dir, timerStateName))
	if err != nil {
		return nil
	}
	var st core.TimerState
	if err := json.Unmarshal(data, &st); err != nil {
		_ = os.Rename(filepath.Join(dir, timerStateName), filepath.Join(dir, timerStateName+".corrupt"))
		return nil
	}
	return &st
}

func saveTimerState(dir string, st *core.TimerState) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return store.WriteFileAtomic(filepath.Join(dir, timerStateName), data, 0o600)
}

// TimerStopResult summarizes a finalized timer session.
type TimerStopResult struct {
	TaskID    string
	Title     string
	Mode      string
	Minutes   int
	Pomodoros int
	// Capped reports that the banked time was truncated at TimerCap
	// (a stale/idle plain timer). The dropped wall-clock stretch is gone.
	Capped bool
	// Auto reports the session was finalized by the stale guard rather
	// than an explicit stop.
	Auto bool
}

// bankAndRecordLocked credits the session's accrued work time into the
// task's time_logged_minutes and clears the state file. capMinutes truncates
// absurd gaps (0 = no cap). It runs under withTimerLock.
func bankAndRecordLocked(s *store.Store, dir string, st *core.TimerState, capMinutes int, auto bool) (*TimerStopResult, error) {
	now := s.Now()
	banked := st.BankedOnStop(now)
	minutes := int(banked.Round(time.Minute) / time.Minute)
	capped := false
	if capMinutes > 0 && minutes > capMinutes {
		minutes = capMinutes
		capped = true
	}
	t, err := s.Resolve(st.TaskID)
	if err != nil {
		// Keep the state file so the accrual is not lost silently.
		return nil, fmt.Errorf("timer stopped but task %s no longer exists — state kept in %s: %w", st.TaskID, filepath.Join(dir, timerStateName), err)
	}
	res := &TimerStopResult{TaskID: t.ID, Title: t.Title, Mode: st.Mode, Minutes: minutes, Pomodoros: st.Pomodoros, Capped: capped, Auto: auto}
	if minutes > 0 {
		t.TimeLoggedMinutes += minutes
		if err := s.Update(t); err != nil {
			return nil, err
		}
	}
	if err := os.Remove(filepath.Join(dir, timerStateName)); err != nil && !os.IsNotExist(err) {
		return res, fmt.Errorf("task updated but state file not removed: %w", err)
	}
	return res, nil
}

// LoadTimerView peeks at the timer without advancing it. Idle when absent.
func LoadTimerView(s *store.Store) core.TimerView {
	view := core.TimerView{Phase: core.TimerPhaseIdle}
	_ = withTimerLock(func(dir string) error {
		if st := loadTimerState(dir); st != nil {
			view = st.View(s.Now())
		}
		return nil
	})
	return view
}

// TickTimer advances phases, fires transition events (and their
// notifications when notify is set), and auto-finalizes stale sessions.
// The TUI tick and the pomo watcher both come through here, so phases
// advance exactly once even if both are open.
func TickTimer(s *store.Store, cfg config.Config, notify bool) (core.TimerView, []core.TimerEvent, *TimerStopResult, error) {
	var view = core.TimerView{Phase: core.TimerPhaseIdle}
	var events []core.TimerEvent
	var auto *TimerStopResult
	err := withTimerLock(func(dir string) error {
		st := loadTimerState(dir)
		if st == nil {
			return nil
		}
		now := s.Now()
		if st.Mode == core.TimerModeTimer && st.AccruedNow(now) > core.TimerCap {
			res, err := bankAndRecordLocked(s, dir, st, int(core.TimerCap/time.Minute), true)
			if err == nil {
				auto = res
				view = st.View(now)
				view.Phase = core.TimerPhaseIdle
			}
			return err
		}
		evs, stale := st.Advance(now, TimerPhaseLengths(cfg))
		events = evs
		if stale {
			res, err := bankAndRecordLocked(s, dir, st, int(core.TimerCap/time.Minute), true)
			if err == nil {
				auto = res
				v := st.View(now)
				v.Phase = core.TimerPhaseIdle
				view = v
			}
			return err
		}
		if err := saveTimerState(dir, st); err != nil {
			return err
		}
		view = st.View(now)
		return nil
	})
	if err != nil {
		return view, events, auto, err
	}
	if notify {
		for _, ev := range events {
			switch {
			case ev.From == core.TimerPhaseWork && ev.To == core.TimerPhaseBreak:
				Notify("mtm", fmt.Sprintf("work round done — %d done, banked %s; break time", ev.Pomodoros, FormatMinutes(int(ev.Banked/time.Minute))))
			case ev.From == core.TimerPhaseBreak && ev.To == core.TimerPhaseWork:
				Notify("mtm", "break over — back to work")
			}
		}
		if auto != nil {
			Notify("mtm", fmt.Sprintf("timer ended while away — logged %s%s", FormatMinutes(auto.Minutes), capNote(auto)))
		}
	}
	return view, events, auto, nil
}

func capNote(r *TimerStopResult) string {
	if r.Capped {
		return " (capped)"
	}
	return ""
}

// StartTimer begins a plain timer or pomodoro on a task. Only one timer may
// be active at a time; starting a second is an error naming the active one.
func StartTimer(s *store.Store, cfg config.Config, taskIDArg, mode string) (*core.TimerState, error) {
	t, err := s.Resolve(taskIDArg)
	if err != nil {
		return nil, err
	}
	if mode != core.TimerModeTimer && mode != core.TimerModePomodoro {
		return nil, fmt.Errorf("mode %q must be timer or pomodoro", mode)
	}
	var out *core.TimerState
	err = withTimerLock(func(dir string) error {
		if st := loadTimerState(dir); st != nil {
			active := shortRef(st.TaskID)
			if activeTask, resolveErr := s.Resolve(st.TaskID); resolveErr == nil {
				active = fmt.Sprintf("%s (%s)", shortRef(st.TaskID), activeTask.Title)
			}
			return fmt.Errorf("timer already running on %s — stop it first with `mtm timer stop`", active)
		}
		now := s.Now()
		out = core.NewTimer(t.ID, mode, now, TimerPhaseLengths(cfg))
		return saveTimerState(dir, out)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// StopTimer stops the active timer, banks capped accrual into the task, and
// clears the state. full=true forces logging everything (stale watchdogs
// aside, that is what the user asked for).
func StopTimer(s *store.Store, full bool) (*TimerStopResult, error) {
	var res *TimerStopResult
	err := withTimerLock(func(dir string) error {
		st := loadTimerState(dir)
		if st == nil {
			return fmt.Errorf("no timer running")
		}
		capMinutes := int(core.TimerCap / time.Minute)
		if full {
			capMinutes = 0
		}
		var err error
		res, err = bankAndRecordLocked(s, dir, st, capMinutes, false)
		return err
	})
	return res, err
}

// TogglePauseTimer pauses a running timer or resumes a paused one.
func TogglePauseTimer(s *store.Store) (core.TimerView, error) {
	var view = core.TimerView{Phase: core.TimerPhaseIdle}
	err := withTimerLock(func(dir string) error {
		st := loadTimerState(dir)
		if st == nil {
			return fmt.Errorf("no timer running")
		}
		st.TogglePause(s.Now())
		if err := saveTimerState(dir, st); err != nil {
			return err
		}
		view = st.View(s.Now())
		return nil
	})
	return view, err
}

// Notify sends a desktop notification via notify-send; if it is missing we
// print a line instead. Best effort, never fatal.
func Notify(title, body string) {
	_ = NotifyErr(title, body)
}

// NotifyErr is Notify with an error return: it reports delivery failure so
// callers that retry (reminders) can skip marking a send as done.
func NotifyErr(title, body string) error {
	bin, err := exec.LookPath("notify-send")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", title, body)
		return fmt.Errorf("notify-send not found in PATH")
	}
	return exec.Command(bin, title, body).Run()
}

func shortRef(id string) string {
	if len(id) > 6 {
		return id[:6]
	}
	return id
}
