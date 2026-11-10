package core

import (
	"testing"
	"time"
)

func doneOn(id string, day time.Time) *Task {
	stamp := day.Format(TimestampFormat)
	return &Task{ID: id, Project: "p", Title: id, Status: "done", Completed: stamp, Created: stamp, Updated: stamp}
}

func TestVelocityReportBucketsWeeks(t *testing.T) {
	now := time.Date(2026, 11, 9, 14, 0, 0, 0, time.Local) // Monday
	isDone := func(_, st string) bool { return st == "done" }
	tasks := []*Task{
		doneOn("t0a", time.Date(2026, 11, 9, 9, 0, 0, 0, time.Local)),  // this week
		doneOn("t0b", time.Date(2026, 11, 9, 10, 0, 0, 0, time.Local)), // this week
		doneOn("t1", time.Date(2026, 11, 2, 9, 0, 0, 0, time.Local)),   // 1 week back
		doneOn("t3", time.Date(2026, 10, 20, 9, 0, 0, 0, time.Local)),  // 3 weeks back
		doneOn("t9", time.Date(2026, 9, 14, 9, 0, 0, 0, time.Local)),   // 8 weeks back
		doneOn("old", time.Date(2026, 5, 4, 9, 0, 0, 0, time.Local)),   // beyond window
	}
	v := VelocityReport(tasks, isDone, now, 12)
	if len(v.Weeks) != 12 {
		t.Fatalf("weeks = %d, want 12", len(v.Weeks))
	}
	want := make([]float64, 12)
	want[11] = 2 // this week
	want[10] = 1 // 7 days back
	want[9] = 1  // 20 days back (floor(20/7)=2 weeks)
	want[3] = 1  // 8 weeks back
	for i, n := range want {
		if v.Weeks[i] != n {
			t.Errorf("week bucket %d = %v, want %v (all: %v)", i, v.Weeks[i], n, v.Weeks)
		}
	}
	if v.ActiveWeeks != 4 {
		t.Errorf("active weeks = %d, want 4", v.ActiveWeeks)
	}
	if !v.Enough {
		t.Error("Enough = false, want true")
	}
	// Avg = total completions in window / 12 weeks.
	if got := v.AvgPerWeek; got != 5.0/12.0 {
		t.Errorf("avg = %v, want 5/12", got)
	}
}

func TestVelocityNotEnoughData(t *testing.T) {
	now := time.Date(2026, 11, 9, 14, 0, 0, 0, time.Local)
	isDone := func(_, st string) bool { return st == "done" }
	// Only this week's completions — a single active week.
	tasks := []*Task{doneOn("a", now), doneOn("b", now)}
	v := VelocityReport(tasks, isDone, now, 12)
	if v.Enough {
		t.Error("Enough = true with a single active week, want false")
	}
	if _, ok := EstimateFinish(5, v, now); ok {
		t.Error("EstimateFinish reported ok with insufficient velocity")
	}
	// No completions at all.
	v = VelocityReport(nil, isDone, now, 12)
	if v.Enough {
		t.Error("Enough = true with zero completions, want false")
	}
	if _, ok := EstimateFinish(3, v, now); ok {
		t.Error("EstimateFinish reported ok with zero velocity")
	}
}

func TestEstimateFinishMath(t *testing.T) {
	now := time.Date(2026, 11, 9, 0, 0, 0, 0, time.Local)
	v := Velocity{Weeks: []float64{4, 4}, AvgPerWeek: 4, ActiveWeeks: 2, Enough: true}
	finish, ok := EstimateFinish(8, v, now)
	if !ok {
		t.Fatal("EstimateFinish not ok")
	}
	want := now.AddDate(0, 0, 14) // 8 tasks / 4 per week = 2 weeks = 14 days
	if !finish.Equal(want) {
		t.Errorf("finish = %v, want %v", finish.Format("2006-01-02"), want.Format("2006-01-02"))
	}
	// Zero remaining: done today, regardless of velocity.
	finish, ok = EstimateFinish(0, Velocity{}, now)
	if !ok || !finish.Equal(now) {
		t.Errorf("zero remaining = %v, %v; want now, true", finish, ok)
	}
	// Less than a week of work rounds to at least one day.
	v = Velocity{Weeks: []float64{40}, AvgPerWeek: 40, ActiveWeeks: 12, Enough: true}
	finish, ok = EstimateFinish(1, v, now)
	if !ok || finish.Sub(now).Hours() < 24 {
		t.Errorf("one task at 40/wk: finish %v ok=%v, want ≥1 day out", finish, ok)
	}
}

func TestCombineProgress(t *testing.T) {
	cases := []struct {
		done, total, mDone, mTotal int
		frac                       float64
		enough                     bool
	}{
		{0, 0, 0, 0, 0, false},
		{1, 2, 0, 0, 0.5, true},
		{0, 0, 1, 4, 0.25, true},
		{1, 2, 1, 4, 0.375, true}, // (0.5 + 0.25) / 2
	}
	for _, tc := range cases {
		frac, enough := CombineProgress(tc.done, tc.total, tc.mDone, tc.mTotal)
		if frac != tc.frac || enough != tc.enough {
			t.Errorf("CombineProgress(%d,%d,%d,%d) = %v,%v; want %v,%v",
				tc.done, tc.total, tc.mDone, tc.mTotal, frac, enough, tc.frac, tc.enough)
		}
	}
}
