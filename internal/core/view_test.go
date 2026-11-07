package core

import (
	"strings"
	"testing"
	"time"
)

func TestParseViewExpr(t *testing.T) {
	cases := []struct {
		expr    string
		want    Filter
		wantErr string
	}{
		{"tag:college", Filter{Tag: "college"}, ""},
		{"project:college/dbms due:today", Filter{Project: "college/dbms", Due: "today"}, ""},
		{"priority:high due:overdue all", Filter{Priority: "high", Due: "overdue", IncludeCompleted: true}, ""},
		{"status:doing", Filter{Status: "doing"}, ""},
		{"tag:home tag:work", Filter{Tag: "work"}, ""}, // last wins
		{"priority:HIGH", Filter{Priority: "high"}, ""},
		{"due:bogus", Filter{}, "due"},
		{"wat:x", Filter{}, "unknown"},
		{"no-colon-here", Filter{}, "key:value"},
		{"priority:critical", Filter{}, "priority"},
	}
	for _, c := range cases {
		t.Run(c.expr, func(t *testing.T) {
			got, err := ParseViewExpr(c.expr)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Tag != c.want.Tag || got.Project != c.want.Project || got.Status != c.want.Status ||
				got.Priority != c.want.Priority || got.Due != c.want.Due ||
				got.IncludeCompleted != c.want.IncludeCompleted {
				t.Fatalf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestFilterDueWindows(t *testing.T) {
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.Local) // Saturday
	mk := func(id, due string) *Task {
		return &Task{ID: id, Title: id, Project: "p", Status: "todo", Priority: "low", Due: due}
	}
	tasks := []*Task{
		mk("today1", "2026-11-07"),       // today, date-only
		mk("today2", "2026-11-07T18:00"), // today, datetime
		mk("tomorrow", "2026-11-08T09:00"),
		mk("in6", "2026-11-13"),               // edge of week window (today+6)
		mk("in7", "2026-11-14"),               // outside week window (today+7)
		mk("yesterday", "2026-11-06"),         // overdue (date-only)
		mk("thismorning", "2026-11-07T06:00"), // overdue (datetime in the past)
		mk("nodue", ""),
		mk("bad", "not-a-date"),
	}
	cases := []struct {
		name string
		due  string
		want []string
	}{
		{"today", "today", []string{"today1", "today2", "thismorning"}},
		{"week", "week", []string{"today1", "today2", "tomorrow", "in6", "thismorning"}},
		{"overdue", "overdue", []string{"yesterday", "thismorning"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Filter{Due: c.due, Now: now}.Apply(tasks)
			ids := make([]string, 0, len(got))
			for _, tk := range got {
				ids = append(ids, tk.ID)
			}
			if len(ids) != len(c.want) {
				t.Fatalf("got %v, want %v", ids, c.want)
			}
			for i := range ids {
				if ids[i] != c.want[i] {
					t.Fatalf("got %v, want %v", ids, c.want)
				}
			}
		})
	}
}
