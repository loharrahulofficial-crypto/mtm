package core

import "testing"

func TestDefaultProject(t *testing.T) {
	p := DefaultProject("inbox")
	if err := p.Validate(); err != nil {
		t.Fatalf("default project invalid: %v", err)
	}
	if got := p.DoneStatus(); got != "done" {
		t.Errorf("DoneStatus() = %q, want done", got)
	}
}

func TestProjectStatusLookup(t *testing.T) {
	p := &Project{Name: "x", Statuses: []ProjectStatus{
		{Name: "Backlog"},
		{Name: "In Progress", Color: "#fff"},
		{Name: "Shipped", Done: true},
	}}
	cases := []struct {
		q         string
		exists    bool
		canonical string
		done      bool
	}{
		{"backlog", true, "Backlog", false},
		{"IN PROGRESS", true, "In Progress", false},
		{"Shipped", true, "Shipped", true},
		{"done", false, "", false},
	}
	for _, c := range cases {
		if got := p.StatusExists(c.q); got != c.exists {
			t.Errorf("StatusExists(%q) = %v, want %v", c.q, got, c.exists)
		}
		if got := p.CanonicalStatus(c.q); got != c.canonical {
			t.Errorf("CanonicalStatus(%q) = %q, want %q", c.q, got, c.canonical)
		}
		if got := p.IsDoneStatus(c.q); got != c.done {
			t.Errorf("IsDoneStatus(%q) = %v, want %v", c.q, got, c.done)
		}
	}
	if got := p.DoneStatus(); got != "Shipped" {
		t.Errorf("DoneStatus() = %q, want Shipped", got)
	}
}

func TestProjectValidate(t *testing.T) {
	cases := []struct {
		name string
		p    *Project
		want string
	}{
		{"no statuses", &Project{Name: "x"}, "at least one"},
		{"empty status name", &Project{Name: "x", Statuses: []ProjectStatus{{Name: " "}}}, "empty"},
		{"duplicate status", &Project{Name: "x", Statuses: []ProjectStatus{{Name: "ToDo"}, {Name: "todo"}}}, "duplicate"},
		{"valid", &Project{Name: "x", Statuses: []ProjectStatus{{Name: "a"}}}, ""},
	}
	for _, c := range cases {
		err := c.p.Validate()
		if c.want == "" {
			if err != nil {
				t.Errorf("%s: unexpected error %v", c.name, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("%s: expected error containing %q", c.name, c.want)
		}
	}
}
