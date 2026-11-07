package core

import "testing"

func task(id, project, status, priority string, tags ...string) *Task {
	return &Task{ID: id, Title: "t" + id, Project: project, Status: status, Priority: priority, Tags: tags}
}

func TestFilter(t *testing.T) {
	tasks := []*Task{
		task("1", "inbox", "todo", "high", "home"),
		task("2", "inbox", "doing", "low", "home", "urgent"),
		task("3", "college/dbms", "done", "high", "exam"),
		task("4", "college/dbms", "todo", "high", "exam", "urgent"),
	}
	doneCheck := func(p, s string) bool { return IsDoneStatusFallback(s) }
	cases := []struct {
		name string
		f    Filter
		want []string
	}{
		{"no filter hides done", Filter{DoneCheck: doneCheck}, []string{"1", "2", "4"}},
		{"include completed", Filter{IncludeCompleted: true, DoneCheck: doneCheck}, []string{"1", "2", "3", "4"}},
		{"by project", Filter{Project: "college/dbms", DoneCheck: doneCheck}, []string{"4"}},
		{"project case-insensitive", Filter{Project: "College/DBMS", IncludeCompleted: true}, []string{"3", "4"}},
		{"by tag", Filter{Tag: "urgent", DoneCheck: doneCheck}, []string{"2", "4"}},
		{"by status", Filter{Status: "todo", DoneCheck: doneCheck}, []string{"1", "4"}},
		{"by priority", Filter{Priority: "high", DoneCheck: doneCheck}, []string{"1", "4"}},
		{"combined", Filter{Project: "college/dbms", Tag: "exam", Priority: "high", DoneCheck: doneCheck}, []string{"4"}},
		{"nothing matches", Filter{Project: "nope", DoneCheck: doneCheck}, nil},
	}
	for _, c := range cases {
		got := c.f.Apply(tasks)
		ids := make([]string, 0, len(got))
		for _, tk := range got {
			ids = append(ids, tk.ID)
		}
		if len(ids) != len(c.want) {
			t.Errorf("%s: got ids %v, want %v", c.name, ids, c.want)
			continue
		}
		for i := range ids {
			if ids[i] != c.want[i] {
				t.Errorf("%s: got ids %v, want %v", c.name, ids, c.want)
				break
			}
		}
	}
}
