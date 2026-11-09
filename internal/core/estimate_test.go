package core

import "testing"

func TestEstimateAccuracyOf(t *testing.T) {
	tasks := []*Task{
		{Title: "a", EstimateMinutes: 60, TimeLoggedMinutes: 30},
		{Title: "b", EstimateMinutes: 30, TimeLoggedMinutes: 60},
		{Title: "c"}, // no estimate
	}
	a := EstimateAccuracyOf(tasks)
	if a.Estimated != 2 || a.Total != 3 {
		t.Errorf("Estimated/Total = %d/%d, want 2/3", a.Estimated, a.Total)
	}
	if a.LoggedMin != 90 {
		t.Errorf("LoggedMin = %d, want 90 (only estimated tasks count)", a.LoggedMin)
	}
	if a.EstimateMin != 90 {
		t.Errorf("EstimateMin = %d, want 90", a.EstimateMin)
	}
	if a.Enough {
		t.Error("Enough = true with only 2 estimated tasks, want false (<3 hides the %%)")
	}
	if got := a.Pct(); got != 100 {
		t.Errorf("Pct = %v, want 100", got)
	}
	if got := a.Label(); got != "2 of 3 tasks estimated" {
		t.Errorf("Label = %q, want %q", got, "2 of 3 tasks estimated")
	}
}

func TestEstimateAccuracyEnoughAtThree(t *testing.T) {
	tasks := []*Task{
		{Title: "a", EstimateMinutes: 10, TimeLoggedMinutes: 10},
		{Title: "b", EstimateMinutes: 10, TimeLoggedMinutes: 20},
		{Title: "c", EstimateMinutes: 10, TimeLoggedMinutes: 30},
		{Title: "d", TimeLoggedMinutes: 999}, // ignored entirely
	}
	a := EstimateAccuracyOf(tasks)
	if !a.Enough {
		t.Fatal("Enough = false with 3 estimated tasks, want true")
	}
	if a.LoggedMin != 60 || a.EstimateMin != 30 {
		t.Errorf("Logged/Estimate = %d/%d, want 60/30", a.LoggedMin, a.EstimateMin)
	}
	if got := a.Pct(); got != 200 {
		t.Errorf("Pct = %v, want 200", got)
	}
}

func TestEstimateAccuracyEmpty(t *testing.T) {
	a := EstimateAccuracyOf(nil)
	if a.Total != 0 || a.Estimated != 0 || a.Enough {
		t.Errorf("empty = %+v, want all zero and Enough=false", a)
	}
	if got := a.Pct(); got != 0 {
		t.Errorf("Pct = %v, want 0 with no estimates", got)
	}
	if got := a.Label(); got != "0 of 0 tasks estimated" {
		t.Errorf("Label = %q", got)
	}
}
