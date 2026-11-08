package tui

import (
	"testing"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

func testTask(title string, kind string) *core.Task {
	return &core.Task{Title: title, Status: "todo", Priority: "medium", Kind: kind}
}

func TestExecCommandGrade(t *testing.T) {
	m, s := newTestModel(t, []*core.Task{testTask("homework 1", core.KindAssignment)})
	tasks := s.Tasks()
	if len(tasks) != 1 {
		t.Fatalf("tasks = %d", len(tasks))
	}
	out, _ := m.execCommand("grade " + tasks[0].ID + " 27 --max 30")
	m2 := out.(Model)
	got, err := m2.store.Resolve(tasks[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Score == nil || *got.Score != 27 || got.ScoreMax == nil || *got.ScoreMax != 30 {
		t.Fatalf("score = %v/%v", got.Score, got.ScoreMax)
	}
	// Rejections do not panic or write.
	out, _ = m.execCommand("grade " + tasks[0].ID + " 40 --max 30")
	m3 := out.(Model)
	got, _ = m3.store.Resolve(tasks[0].ID)
	if *got.Score != 27 {
		t.Fatal("score above max was recorded")
	}
}

func TestExecCommandTimerToggle(t *testing.T) {
	m, s := newTestModel(t, []*core.Task{testTask("study normalization", core.KindStudy)})
	id := s.Tasks()[0].ID
	// No timer: starts one on the selected task.
	out, _ := m.execCommand("timer")
	m2 := out.(Model)
	if m2.timer.Phase == core.TimerPhaseIdle {
		t.Fatal("timer did not start")
	}
	if m2.timer.TaskID != id {
		t.Fatalf("timer on %s, want %s", m2.timer.TaskID, id)
	}
	// Running: toggles off with a stop and banks minutes.
	out, _ = m2.execCommand("timer")
	m3 := out.(Model)
	if m3.timer.Phase != core.TimerPhaseIdle {
		t.Fatal("timer did not stop")
	}
}

func TestExecCommandPomoSingleSession(t *testing.T) {
	m, _ := newTestModel(t, []*core.Task{testTask("revise ch.5", core.KindStudy)})
	out, _ := m.execCommand("pomo")
	m2 := out.(Model)
	if m2.timer.Phase == core.TimerPhaseIdle || m2.timer.Mode != core.TimerModePomodoro {
		t.Fatalf("pomo did not start: %+v", m2.timer)
	}
	// A second pomo does not start a second session.
	out, _ = m2.execCommand("pomo")
	m3 := out.(Model)
	if m3.timer.Phase != m2.timer.Phase || m3.timer.TaskID != m2.timer.TaskID || m3.timer.PhaseIndex != m2.timer.PhaseIndex {
		t.Fatalf("pomo restarted a session: %+v → %+v", m2.timer, m3.timer)
	}
	// Cleanup: stop so no state leaks into other tests in this package run.
	out, _ = m3.execCommand("timer stop")
	m4 := out.(Model)
	if m4.timer.Phase != core.TimerPhaseIdle {
		t.Fatal("cleanup stop failed")
	}
}

func TestExecCommandCoursesView(t *testing.T) {
	m, _ := newTestModel(t, nil)
	out, _ := m.execCommand("courses")
	if out.(Model).view != viewCourses {
		t.Fatal("courses did not switch the view")
	}
}

func TestPaletteCoursesEntry(t *testing.T) {
	m, _ := newTestModel(t, nil)
	found := false
	for _, it := range m.buildPaletteItems() {
		if it.id == "view:courses" {
			found = true
		}
	}
	if !found {
		t.Fatal("palette is missing the Courses entry")
	}
	found = false
	for _, it := range m.buildPaletteItems() {
		for _, want := range []string{"timer:toggle", "pomo:start", "exam:add", "grade:record"} {
			if it.id == want {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("palette is missing a quick-action entry")
	}
}

func TestFocusStartsTimerOnSpaceWhenIdle(t *testing.T) {
	m, s := newTestModel(t, []*core.Task{testTask("revise ch.6", core.KindStudy)})
	m.view = viewList
	m2, _ := m.enterFocusTask(s.Tasks()[0])
	m = m2
	if m.timer.Phase == core.TimerPhaseIdle {
		t.Fatal("enterFocusTask did not attach/start a timer")
	}
	// Space in pomodoro work phase pauses; timer stays visible.
	updated, _ := m.updateFocusKey(keyMsg(" "))
	m3 := updated.(Model)
	if m3.timer.Running {
		t.Fatal("space did not pause the focus timer")
	}
	// Esc leaves focus mode; the timer keeps running.
	updated, _ = m3.updateFocusKey(keyMsg("esc"))
	m4 := updated.(Model)
	if m4.mode == modeFocus {
		t.Fatal("esc did not leave focus mode")
	}
	if m4.timer.Phase == core.TimerPhaseIdle {
		t.Fatal("esc stopped the timer")
	}
	updated, _ = m4.execCommand("timer stop")
	_ = updated
}
