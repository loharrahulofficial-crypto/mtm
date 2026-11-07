package core

import "strings"

// Filter narrows a task list. Empty fields match everything.
type Filter struct {
	Project          string
	Tag              string
	Status           string
	Priority         string
	IncludeCompleted bool
	// DoneCheck reports whether a task's status counts as done. May be nil,
	// in which case IsDoneStatusFallback is used.
	DoneCheck func(project, status string) bool
}

// Apply returns the tasks matching the filter.
func (f Filter) Apply(tasks []*Task) []*Task {
	out := make([]*Task, 0, len(tasks))
	for _, t := range tasks {
		if !f.match(t) {
			continue
		}
		out = append(out, t)
	}
	return out
}

func (f Filter) match(t *Task) bool {
	if f.Project != "" && !strings.EqualFold(t.Project, f.Project) {
		return false
	}
	if f.Tag != "" {
		found := false
		for _, tag := range t.Tags {
			if strings.EqualFold(tag, f.Tag) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if f.Status != "" && !strings.EqualFold(t.Status, f.Status) {
		return false
	}
	if f.Priority != "" && !strings.EqualFold(t.Priority, f.Priority) {
		return false
	}
	done := IsDoneStatusFallback(t.Status)
	if f.DoneCheck != nil {
		done = f.DoneCheck(t.Project, t.Status)
	}
	if !f.IncludeCompleted && done {
		return false
	}
	return true
}
