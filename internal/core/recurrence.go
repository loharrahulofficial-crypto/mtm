package core

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Canonical recurrence strings (stored in frontmatter):
//
//	daily               every day, same time (if any)
//	weekly              every 7 days
//	weekly:mon          every week on a fixed weekday
//	every N days        every N days
//	every N weeks       every N weeks
const (
	RecurDaily   = "daily"
	RecurWeekly  = "weekly"
	RecurWeeklyP = "weekly:"
	RecurEveryP  = "every "
)

var weekdayAbbrev = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday,
	"wed": time.Wednesday, "thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
}

// NextOccurrence returns the first recurrence occurrence strictly after now,
// advancing from `from` (usually the previous due date). A zero `from`
// starts from now.
func NextOccurrence(recur string, from, now time.Time) (time.Time, error) {
	spec, err := parseRecur(recur)
	if err != nil {
		return time.Time{}, err
	}
	t := from
	if t.IsZero() {
		t = now
	}
	for i := 0; i < 10000; i++ {
		t = spec.advance(t)
		if t.After(now) {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("recurrence %q produced no occurrence within 10000 steps", recur)
}

type recurrenceSpec struct {
	kind    string // "daily" | "weekly" | "every" | "on"
	n       int
	weekday time.Weekday
}

func parseRecur(recur string) (recurrenceSpec, error) {
	r := strings.ToLower(strings.TrimSpace(recur))
	switch {
	case r == "daily":
		return recurrenceSpec{kind: "daily"}, nil
	case r == "weekly":
		return recurrenceSpec{kind: "weekly"}, nil
	case strings.HasPrefix(r, RecurWeeklyP):
		wd, ok := weekdayAbbrev[strings.TrimSuffix(strings.TrimPrefix(r, RecurWeeklyP), "s")]
		if !ok {
			// tolerate full names and plural abbrevs
			for name, day := range map[string]time.Weekday{
				"sunday": time.Sunday, "monday": time.Monday, "tuesday": time.Tuesday,
				"wednesday": time.Wednesday, "thursday": time.Thursday, "friday": time.Friday, "saturday": time.Saturday,
			} {
				if strings.TrimPrefix(r, RecurWeeklyP) == name {
					return recurrenceSpec{kind: "on", weekday: day}, nil
				}
			}
			return recurrenceSpec{}, fmt.Errorf("unknown weekday in recurrence %q", recur)
		}
		return recurrenceSpec{kind: "on", weekday: wd}, nil
	case strings.HasPrefix(r, RecurEveryP):
		fields := strings.Fields(strings.TrimPrefix(r, RecurEveryP))
		if len(fields) != 2 {
			return recurrenceSpec{}, fmt.Errorf("recurrence %q: want e.g. \"every 2 weeks\"", recur)
		}
		n, err := strconv.Atoi(fields[0])
		if err != nil || n <= 0 {
			return recurrenceSpec{}, fmt.Errorf("recurrence %q: interval must be a positive number", recur)
		}
		switch fields[1] {
		case "day", "days":
			return recurrenceSpec{kind: "every", n: n}, nil
		case "week", "weeks":
			return recurrenceSpec{kind: "every", n: 7 * n}, nil
		default:
			return recurrenceSpec{}, fmt.Errorf("recurrence %q: unit must be days or weeks", recur)
		}
	default:
		return recurrenceSpec{}, fmt.Errorf("unknown recurrence %q", recur)
	}
}

func (s recurrenceSpec) advance(t time.Time) time.Time {
	switch s.kind {
	case "daily":
		return t.AddDate(0, 0, 1)
	case "weekly":
		return t.AddDate(0, 0, 7)
	case "every":
		return t.AddDate(0, 0, s.n)
	case "on":
		// jump to the next date (strictly after t) on the wanted weekday
		d := 1
		for i := 0; i < 7; i++ {
			c := t.AddDate(0, 0, d)
			if c.Weekday() == s.weekday {
				return c
			}
			d++
		}
		return t.AddDate(0, 0, 7)
	}
	return t.AddDate(0, 0, 1)
}

// SpawnRecurringOccurrence builds the follow-up task to a recurring task
// that was just completed. The caller fills in id/created via store.Create.
// ok is false when the task has no recurrence.
func SpawnRecurringOccurrence(t *Task, firstStatus string, now time.Time) (*Task, bool, error) {
	if strings.TrimSpace(t.Recurrence) == "" {
		return nil, false, nil
	}
	from, err := ParseDue(t.Due, time.Local)
	if t.Due == "" || err != nil {
		from = time.Time{} // zero: anchor to now
	}
	next, err := NextOccurrence(t.Recurrence, from, now)
	if err != nil {
		return nil, false, err
	}
	due := next.Format(TimeFormat)
	if len(t.Due) == len("2006-01-02") {
		due = next.Format("2006-01-02") // keep date-only style
	}
	spawned := &Task{
		Title:           t.Title,
		Project:         t.Project,
		Status:          firstStatus,
		Priority:        t.Priority,
		Tags:            append([]string(nil), t.Tags...),
		Due:             due,
		Recurrence:      t.Recurrence,
		EstimateMinutes: t.EstimateMinutes,
		Kind:            t.Kind,
		Weight:          t.Weight,
	}
	return spawned, true, nil
}
