package core

import (
	"testing"
)

func mkTask(id, title, due, priority, status string) *Task {
	return &Task{ID: id, Title: title, Due: due, Priority: priority, Status: status, Project: "inbox", Kind: KindTask, Created: "2026-11-07T10:00", Updated: "2026-11-07T10:00"}
}

func TestSortTasksDefaultOrder(t *testing.T) {
	tasks := []*Task{
		mkTask("d1", "same", "2026-11-10", "low", "todo"),
		mkTask("d2", "aaa done", "2026-11-01", "urgent", "done"), // done sorts after not-done
		mkTask("a1", "b title", "2026-11-10", "urgent", "todo"),
		mkTask("a2", "a title", "2026-11-10", "urgent", "todo"),
		mkTask("n1", "no due urgent", "", "urgent", "todo"),
		mkTask("x1", "due first", "2026-11-09", "low", "todo"),
	}
	SortTasks(tasks, nil, nil)
	got := make([]string, len(tasks))
	for i, tt := range tasks {
		got[i] = tt.ID
	}
	// Not-done before done; due ascending with no-due last; priority; title.
	want := []string{"x1", "a2", "a1", "d1", "n1", "d2"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestSortTasksDueTiebreakPriority(t *testing.T) {
	tasks := []*Task{
		mkTask("p1", "medium", "2026-11-10", "medium", "todo"),
		mkTask("p2", "urgent", "2026-11-10", "urgent", "todo"),
		mkTask("p3", "low", "2026-11-10", "low", "todo"),
		mkTask("p4", "high", "2026-11-10", "high", "todo"),
	}
	SortTasks(tasks, []string{"due", "priority"}, nil)
	got := []string{tasks[0].ID, tasks[1].ID, tasks[2].ID, tasks[3].ID}
	want := []string{"p2", "p4", "p1", "p3"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestSortTasksTitleOnly(t *testing.T) {
	tasks := []*Task{
		mkTask("t1", "Zebra", "", "low", "todo"),
		mkTask("t2", "apple", "", "low", "todo"),
		mkTask("t3", "Mango", "", "low", "todo"),
	}
	SortTasks(tasks, []string{"title"}, nil)
	got := []string{tasks[0].ID, tasks[1].ID, tasks[2].ID}
	want := []string{"t2", "t3", "t1"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestSortTasksNoDueLast(t *testing.T) {
	tasks := []*Task{
		mkTask("n1", "nodue", "", "low", "todo"),
		mkTask("d1", "dated", "2030-01-01", "low", "todo"),
	}
	SortTasks(tasks, []string{"due"}, nil)
	if tasks[0].ID != "d1" || tasks[1].ID != "n1" {
		t.Fatalf("no-due must sort last, got %v %v", tasks[0].ID, tasks[1].ID)
	}
}

func TestSortTasksProjectAwareDoneCheck(t *testing.T) {
	// Project "custom" uses "archived-status" as its done status.
	tasks := []*Task{
		mkTask("a", "open", "", "low", "archived-status"),
		mkTask("b", "really open", "", "low", "todo"),
	}
	isDone := func(project, status string) bool {
		return project == "inbox" && status == "archived-status"
	}
	SortTasks(tasks, []string{"done"}, isDone)
	if tasks[0].ID != "b" {
		t.Fatalf("custom done status not honored: %v", tasks[0].ID)
	}
}

func TestParseSortKeys(t *testing.T) {
	tests := []struct {
		name    string
		in      []string
		want    []string
		wantErr bool
	}{
		{"nil defaults", nil, DefaultSortKeys, false},
		{"valid subset", []string{"due", "title"}, []string{"due", "title"}, false},
		{"case and space", []string{" Due ", "PRIORITY"}, []string{"due", "priority"}, false},
		{"unknown key", []string{"urgency"}, nil, true},
		{"empty string entry", []string{""}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSortKeys(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}
