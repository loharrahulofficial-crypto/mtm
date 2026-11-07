package core

import "testing"

func TestSubtaskProgress(t *testing.T) {
	child := func(id, parent, status string) *Task {
		return &Task{ID: id, Parent: parent, Status: status, Project: "p"}
	}
	all := []*Task{
		{ID: "p1", Title: "parent", Project: "p", Status: "todo"},
		child("c1", "p1", "done"),
		child("c2", "p1", "doing"),
		child("c3", "p1", "DONE"),
		child("c4", "other", "done"),
	}
	done, total, pct := SubtaskProgress(all, "p1", nil)
	if done != 2 || total != 3 {
		t.Fatalf("done=%d total=%d, want 2/3", done, total)
	}
	if pct < 66.6 || pct > 66.7 {
		t.Fatalf("pct=%f, want ~66.67", pct)
	}
	done, total, pct = SubtaskProgress(all, "nosubtasks", nil)
	if done != 0 || total != 0 || pct != 0 {
		t.Fatalf("no subtasks: done=%d total=%d pct=%f", done, total, pct)
	}
}

func TestSubtaskProgressDoneCheck(t *testing.T) {
	child := func(id, parent, status string) *Task {
		return &Task{ID: id, Parent: parent, Status: status, Project: "p"}
	}
	fallback := func(p, s string) bool { return s == "shipped" }
	all := []*Task{child("c1", "p1", "shipped"), child("c2", "p1", "shipped")}
	done, total, pct := SubtaskProgress(all, "p1", fallback)
	if done != 2 || total != 2 || pct != 100 {
		t.Fatalf("got %d/%d pct=%f", done, total, pct)
	}
}
