package core

import "testing"

func TestParseSearchQuery(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		wantTag     string
		wantProject string
		wantDue     string
		wantNeedle  string
		wantErr     bool
	}{
		{"empty", "", "", "", "", "", false},
		{"plain text", "dbms assignment", "", "", "", "dbms assignment", false},
		{"keys only", "tag:college due:today", "college", "", "today", "", false},
		{"mixed", "tag:exam due:week calculus notes", "exam", "", "week", "calculus notes", false},
		{"project key", "project:college/dbms report", "", "college/dbms", "", "report", false},
		{"unknown key stays needle", "foo:bar baz", "", "", "", "foo:bar baz", false},
		{"bad priority is an error", "priority:mega urgent thing", "", "", "", "", true},
		{"case insensitive keys", "TAG:college x", "college", "", "", "x", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, needle, err := ParseSearchQuery(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if f.Tag != tt.wantTag || f.Project != tt.wantProject || f.Due != tt.wantDue {
				t.Fatalf("filter = %+v", f)
			}
			if needle != tt.wantNeedle {
				t.Fatalf("needle = %q, want %q", needle, tt.wantNeedle)
			}
		})
	}
}

func TestFilterContains(t *testing.T) {
	tasks := []*Task{
		sortTaskMk2("a", "DBMS assignment 2", "2026-11-10", "high", "todo"),
		sortTaskMk2("b", "Buy milk", "", "low", "todo"),
	}
	tasks[0].Body = "chapter 4 joins"
	tasks[1].Tags = []string{"grocery"}
	f := Filter{Contains: "dbms"}
	got := f.Apply(tasks)
	if len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("title needle failed: %v", got)
	}
	f = Filter{Contains: "CH5 joins"}
	got = f.Apply(tasks)
	// "CH5 joins" is not a substring of "chapter 4 joins".
	if len(got) != 0 {
		t.Fatalf("unexpected match: %v", got)
	}
	f = Filter{Contains: "joins"}
	if got := f.Apply(tasks); len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("body needle failed: %v", got)
	}
	f = Filter{Contains: "grocery"}
	if got := f.Apply(tasks); len(got) != 1 || got[0].ID != "b" {
		t.Fatalf("tag needle failed: %v", got)
	}
}

func sortTaskMk2(id, title, due, priority, status string) *Task {
	return &Task{ID: id, Title: title, Due: due, Priority: priority, Status: status, Project: "inbox", Kind: KindTask, Created: "2026-11-07T10:00", Updated: "2026-11-07T10:00"}
}
