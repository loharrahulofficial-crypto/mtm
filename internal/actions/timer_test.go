package actions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/clock"
	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
)

var timerT0 = time.Date(2026, 11, 8, 10, 0, 0, 0, time.Local)

func openTimerTestStore(t *testing.T, now time.Time) (*store.Store, string, config.Config) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s := openActionStore(t, "college/dbms")
	s.Clock = clock.Fixed(now)
	task := &core.Task{Title: "study normalization", Project: "college/dbms", Status: "todo", Kind: core.KindStudy}
	if err := s.Create(task); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Pomodoro: config.PomodoroConfig{WorkMinutes: 1, ShortBreakMinutes: 1, LongBreakMinutes: 2, LongEvery: 4}}
	return s, task.ID, cfg
}

func stateFile(t *testing.T, name string) string {
	t.Helper()
	for _, root := range []string{os.Getenv("XDG_STATE_HOME")} {
		p := filepath.Join(root, "mtm", name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func TestStartTimerCreatesStateFileOutsideRepo(t *testing.T) {
	s, taskID, _ := openTimerTestStore(t, timerT0)
	st, err := StartTimer(s, config.Config{}, taskID, core.TimerModeTimer)
	if err != nil {
		t.Fatal(err)
	}
	if st.TaskID != taskID || st.Mode != core.TimerModeTimer {
		t.Fatalf("state = %+v", st)
	}
	if stateFile(t, "timer.json") == "" {
		t.Fatal("timer.json not in the state dir")
	}
	// The data repo must not contain the runtime state.
	entries, err := os.ReadDir(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() == "timer.json" {
			t.Fatal("runtime state leaked into the data repo")
		}
	}
	view := LoadTimerView(s)
	if view.Phase != core.TimerPhaseWork || !view.Running || view.TaskID != taskID {
		t.Fatalf("view = %+v", view)
	}
	// A second start errors and names the active task.
	if _, err := StartTimer(s, config.Config{}, taskID, core.TimerModeTimer); err == nil || !strings.Contains(err.Error(), "study normalization") {
		t.Fatalf("second start = %v", err)
	}
}

func TestStopTimerBanksMinutesAndClearsState(t *testing.T) {
	s, taskID, cfg := openTimerTestStore(t, timerT0)
	_ = cfg
	if _, err := StartTimer(s, config.Config{}, taskID, core.TimerModeTimer); err != nil {
		t.Fatal(err)
	}
	s.Clock = clock.Fixed(timerT0.Add(47 * time.Minute))
	res, err := StopTimer(s, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Minutes != 47 || res.Capped {
		t.Fatalf("res = %+v", res)
	}
	if stateFile(t, "timer.json") != "" {
		t.Fatal("state file not cleared")
	}
	got, err := s.Resolve(taskID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TimeLoggedMinutes != 47 {
		t.Fatalf("time_logged_minutes = %d, want 47", got.TimeLoggedMinutes)
	}
	if _, err := StopTimer(s, false); err == nil {
		t.Fatal("second stop accepted")
	}
}

func TestStopTimerCapsStalePlainTimer(t *testing.T) {
	s, taskID, _ := openTimerTestStore(t, timerT0)
	if _, err := StartTimer(s, config.Config{}, taskID, core.TimerModeTimer); err != nil {
		t.Fatal(err)
	}
	s.Clock = clock.Fixed(timerT0.Add(6 * time.Hour))
	res, err := StopTimer(s, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Minutes != int(core.TimerCap/time.Minute) || !res.Capped {
		t.Fatalf("res = %+v, want %d capped", res, int(core.TimerCap/time.Minute))
	}
	got, _ := s.Resolve(taskID)
	if got.TimeLoggedMinutes != int(core.TimerCap/time.Minute) {
		t.Fatalf("time_logged_minutes = %d", got.TimeLoggedMinutes)
	}
	// --full logs everything.
	if _, err := StartTimer(s, config.Config{}, taskID, core.TimerModeTimer); err != nil {
		t.Fatal(err)
	}
	s.Clock = clock.Fixed(s.Clock.Now().Add(6 * time.Hour))
	res, err = StopTimer(s, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Minutes != 360 || res.Capped {
		t.Fatalf("full res = %+v, want 360 uncapped", res)
	}
}

func TestPomoLifecycleTransitions(t *testing.T) {
	s, taskID, cfg := openTimerTestStore(t, timerT0)
	if _, err := StartTimer(s, cfg, taskID, core.TimerModePomodoro); err != nil {
		t.Fatal(err)
	}
	// 70s later the 1-minute work phase has ended: work → break.
	s.Clock = clock.Fixed(timerT0.Add(70 * time.Second))
	view, events, auto, err := TickTimer(s, cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	if auto != nil {
		t.Fatalf("unexpected auto finish: %+v", auto)
	}
	if len(events) != 1 || events[0].To != core.TimerPhaseBreak {
		t.Fatalf("events = %+v", events)
	}
	if view.Phase != core.TimerPhaseBreak || view.Pomodoros != 1 {
		t.Fatalf("view = %+v", view)
	}
	// Break expires after 1 minute: break → work round 2.
	s.Clock = clock.Fixed(timerT0.Add(130 * time.Second))
	view, events, _, err = TickTimer(s, cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].To != core.TimerPhaseWork || view.Phase != core.TimerPhaseWork {
		t.Fatalf("events = %+v view = %+v", events, view)
	}
	if view.PhaseIndex != 2 {
		t.Fatalf("phase index = %d", view.PhaseIndex)
	}
	// 10s into round 2, stop: 60s (round 1) + 10s (round 2) = 70s → 1 minute.
	s.Clock = clock.Fixed(timerT0.Add(140 * time.Second))
	res, err := StopTimer(s, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Minutes != 1 || res.Pomodoros != 1 {
		t.Fatalf("res = %+v", res)
	}
	got, _ := s.Resolve(taskID)
	if got.TimeLoggedMinutes != 1 {
		t.Fatalf("time_logged_minutes = %d", got.TimeLoggedMinutes)
	}
}

func TestStalePomoAutoFinalizes(t *testing.T) {
	s, taskID, cfg := openTimerTestStore(t, timerT0)
	if _, err := StartTimer(s, cfg, taskID, core.TimerModePomodoro); err != nil {
		t.Fatal(err)
	}
	// Machine "slept" three hours past the work-phase end.
	s.Clock = clock.Fixed(timerT0.Add(3 * time.Hour))
	view, events, auto, err := TickTimer(s, cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	if auto == nil {
		t.Fatalf("expected auto finish, events=%+v view=%+v", events, view)
	}
	if auto.Minutes != 1 || !auto.Auto { // work budget is 1 minute here
		t.Fatalf("auto = %+v", auto)
	}
	if view.Phase != core.TimerPhaseIdle {
		t.Fatalf("view = %+v, want idle", view)
	}
	if stateFile(t, "timer.json") != "" {
		t.Fatal("state file kept after auto finalize")
	}
	got, _ := s.Resolve(taskID)
	if got.TimeLoggedMinutes != 1 {
		t.Fatalf("time_logged_minutes = %d, want 1", got.TimeLoggedMinutes)
	}
}

func TestStartTimerUnknownTask(t *testing.T) {
	s, _, cfg := openTimerTestStore(t, timerT0)
	if _, err := StartTimer(s, cfg, "ffffff", core.TimerModeTimer); err == nil {
		t.Fatal("unknown task accepted")
	}
}

func TestTogglePauseTimer(t *testing.T) {
	s, taskID, cfg := openTimerTestStore(t, timerT0)
	if _, err := StartTimer(s, cfg, taskID, core.TimerModeTimer); err != nil {
		t.Fatal(err)
	}
	view, err := TogglePauseTimer(s)
	if err != nil {
		t.Fatal(err)
	}
	if view.Running {
		t.Fatal("still running")
	}
	s.Clock = clock.Fixed(timerT0.Add(5 * time.Minute))
	if got := LoadTimerView(s).Accrued; got != 0 {
		t.Errorf("paused accrual = %v, want 0", got)
	}
	view, err = TogglePauseTimer(s)
	if err != nil {
		t.Fatal(err)
	}
	if !view.Running {
		t.Fatal("not running after resume")
	}
}

func TestTickTimerNoopWhenIdle(t *testing.T) {
	s, _, cfg := openTimerTestStore(t, timerT0)
	view, events, auto, err := TickTimer(s, cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	if view.Phase != core.TimerPhaseIdle || len(events) != 0 || auto != nil {
		t.Fatalf("idle tick: view=%+v events=%+v auto=%+v", view, events, auto)
	}
}
