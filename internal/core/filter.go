package core

import (
	"fmt"
	"strings"
	"time"
)

// Filter narrows a task list. Empty fields match everything.
type Filter struct {
	Project          string
	Tag              string
	Status           string
	Priority         string
	Due              string // "" | today | week | overdue
	IncludeCompleted bool
	// Contains is a free-text needle matched case-insensitively against
	// title, body and tags. "" matches everything.
	Contains string
	// Now is injected for testability; zero means time.Now() at Apply time.
	Now time.Time
	// DoneCheck reports whether a task's status counts as done. May be nil,
	// in which case IsDoneStatusFallback is used.
	DoneCheck func(project, status string) bool
}

// ValidDue reports whether d is a supported --due value.
func ValidDue(d string) bool {
	switch d {
	case "", "today", "week", "overdue":
		return true
	}
	return false
}

// Apply returns the tasks matching the filter.
func (f Filter) Apply(tasks []*Task) []*Task {
	now := f.Now
	if now.IsZero() {
		now = time.Now()
	}
	out := make([]*Task, 0, len(tasks))
	for _, t := range tasks {
		if !f.match(t, now) {
			continue
		}
		out = append(out, t)
	}
	return out
}

func (f Filter) match(t *Task, now time.Time) bool {
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
	if f.Contains != "" {
		haystack := strings.ToLower(strings.Join([]string{
			t.Title,
			t.Body,
			strings.Join(t.Tags, " "),
		}, "\n"))
		if !strings.Contains(haystack, strings.ToLower(f.Contains)) {
			return false
		}
	}
	if f.Due != "" && !dueMatches(t.Due, f.Due, now) {
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

// ParseDue parses a due value in either accepted layout; date-only values
// are returned as midnight in loc.
func ParseDue(due string, loc *time.Location) (time.Time, error) {
	switch len(due) {
	case len("2006-01-02"):
		return time.ParseInLocation("2006-01-02", due, loc)
	case len(TimeFormat):
		return time.ParseInLocation(TimeFormat, due, loc)
	default:
		t, err := time.ParseInLocation(TimeFormat, due, loc)
		if err == nil {
			return t, nil
		}
		return time.ParseInLocation("2006-01-02", due, loc)
	}
}

func dayStart(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// dueMatches answers whether due (a task's due string) falls in the window.
// Tasks without a due never match. Date-only semantics: day granularity.
func dueMatches(due, window string, now time.Time) bool {
	if due == "" {
		return false
	}
	loc := now.Location()
	d, err := ParseDue(due, loc)
	if err != nil {
		return false
	}
	today := dayStart(now)
	day := dayStart(d)
	switch window {
	case "today":
		return day.Equal(today)
	case "week":
		end := today.AddDate(0, 0, 7)
		return !day.Before(today) && day.Before(end)
	case "overdue":
		// Datetime due: compare against now; date-only due: compare days.
		if len(due) > len("2006-01-02") {
			return d.Before(now)
		}
		return day.Before(today)
	}
	return false
}

// DueInput normalizes and validates a user-supplied due value: date-only or
// datetime in TimeFormat.
func DueInput(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	for _, layout := range []string{"2006-01-02", TimeFormat, "2006-01-02 15:04"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			if layout == "2006-01-02" {
				return t.Format("2006-01-02"), nil
			}
			return t.Format(TimeFormat), nil
		}
	}
	return "", fmt.Errorf("invalid due %q (want YYYY-MM-DD or YYYY-MM-DDTHH:MM)", s)
}
