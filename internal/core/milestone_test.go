package core

import (
	"testing"
	"time"
)

func mtask(id, parent, milestone, status string) *Task {
	return &Task{ID: id, Project: "p", Parent: parent, Milestone: milestone, Status: status, Title: id}
}

func TestMilestoneScopeIncludesNestedSubtasks(t *testing.T) {
	all := []*Task{
		mtask("a", "", "m1", "todo"),
		mtask("a1", "a", "", "done"),
		mtask("a1i", "a1", "", "done"),
		mtask("b", "", "m1", "done"),
		mtask("c", "", "m2", "done"),
		&Task{ID: "self", Project: "p", Parent: "self", Milestone: "m1", Status: "todo", Title: "self"},
	}
	scope := MilestoneScope(all, "m1")
	got := map[string]bool{}
	for _, x := range scope {
		got[x.ID] = true
	}
	for _, want := range []string{"a", "a1", "a1i", "b", "self"} {
		if !got[want] {
			t.Errorf("scope missing %q (got %v)", want, got)
		}
	}
	if got["c"] {
		t.Errorf("scope includes c, which belongs to m2")
	}
	// The self-parenting task must not hang the walker.
}

func TestMilestoneProgress(t *testing.T) {
	isDone := func(_, st string) bool { return st == "done" }
	all := []*Task{
		mtask("a", "", "m1", "done"),
		mtask("a1", "a", "", "done"),
		mtask("a2", "a", "", "todo"),
		mtask("b", "", "m1", "todo"),
		mtask("z", "", "m2", "done"),
	}
	done, total, frac := MilestoneProgress(all, "m1", isDone)
	if done != 2 || total != 4 {
		t.Fatalf("progress = %d/%d, want 2/4", done, total)
	}
	if frac != 0.5 {
		t.Fatalf("frac = %v, want 0.5", frac)
	}
	// Empty scope reports zeroes, never a divide-by-zero.
	done, total, frac = MilestoneProgress(all, "nope", isDone)
	if done != 0 || total != 0 || frac != 0 {
		t.Fatalf("empty scope = %d/%d/%v, want 0/0/0", done, total, frac)
	}
}

func TestMilestoneRisk(t *testing.T) {
	now := time.Date(2026, 11, 9, 14, 0, 0, 0, time.Local)
	mk := func(target, status string) *Milestone { return &Milestone{Title: "m", Target: target, Status: status} }
	cases := []struct {
		m    *Milestone
		want int
	}{
		{mk("2026-11-20", "planned"), MilestoneRiskOK},
		{mk("2026-11-14", "active"), MilestoneRiskAt}, // 5 days out
		{mk("2026-11-09", "active"), MilestoneRiskAt}, // today
		{mk("2026-11-01", "active"), MilestoneRiskOverdue},
		{mk("2026-10-20", "done"), MilestoneRiskOK}, // done is never overdue
		{mk("", "active"), MilestoneRiskOK},         // no target
		{mk("garbage", "active"), MilestoneRiskOK},  // unparseable target
	}
	for _, tc := range cases {
		if got := MilestoneRisk(tc.m, now); got != tc.want {
			t.Errorf("MilestoneRisk(%s/%s) = %d, want %d", tc.m.Target, tc.m.Status, got, tc.want)
		}
	}
}

func TestMilestoneValidate(t *testing.T) {
	good := &Milestone{Title: "m", Status: "planned", Target: "2026-12-01"}
	if err := good.Validate(); err != nil {
		t.Fatalf("good milestone rejected: %v", err)
	}
	bad := &Milestone{Title: "m", Status: "planned", Target: "01-12-2026"}
	if err := bad.Validate(); err == nil {
		t.Fatal("bad target accepted")
	}
	empty := &Milestone{Title: " ", Status: "planned"}
	if err := empty.Validate(); err == nil {
		t.Fatal("empty title accepted")
	}
}
