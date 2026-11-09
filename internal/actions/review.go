package actions

// Weekly review (Phase 5, item 1): the three sections and the per-item
// decisions. The prompt loop lives in the CLI; everything that touches the
// store goes through here.

import (
	"fmt"
	"sort"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/dates"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
)

// ReviewReport is the weekly review's data: what finished, what slipped,
// what is coming up (exams included).
type ReviewReport struct {
	Finished      []*core.Task
	Overdue       []*core.Task
	Upcoming      []*core.Task
	Since         time.Time // start of the trailing 7 days
	Until         time.Time // today
	ParseFailures int       // tasks with unparseable due dates (skipped everywhere)
}

// CollectReview builds the report for the trailing 7 days (today included).
// Overdue is anything open whose due date has passed (timed items overdue
// earlier today count too).
func CollectReview(s *store.Store, now time.Time) (*ReviewReport, error) {
	loc := now.Location()
	isDone := func(p, st string) bool { return s.IsDoneStatus(p, st) }
	today := dayStamp(now)
	since := today.AddDate(0, 0, -6)
	until := today.AddDate(0, 0, 6)
	rep := &ReviewReport{Since: since, Until: today}
	for _, t := range s.Tasks() {
		ts, ok := completionStamp(t, isDone, loc)
		if ok {
			day := dayStamp(ts)
			if !day.Before(since) && !day.After(today) {
				rep.Finished = append(rep.Finished, t)
			}
		}
		if isDone(t.Project, t.Status) {
			continue
		}
		if t.Due == "" {
			continue
		}
		parsed, err := time.ParseInLocation(core.TimestampFormat, t.Due, loc)
		timed := len(t.Due) > len("2006-01-02")
		if err != nil {
			if d, err2 := time.ParseInLocation("2006-01-02", t.Due, loc); err2 == nil {
				parsed = d
			} else {
				rep.ParseFailures++ // unparseable due: leave it out of the report entirely
				continue
			}
		}
		overdue := false
		if timed {
			overdue = !parsed.After(now)
		} else {
			overdue = dayStamp(parsed).Before(today)
		}
		if overdue {
			// slipped
			rep.Overdue = append(rep.Overdue, t)
			continue
		}
		if !dayStamp(parsed).After(until) {
			rep.Upcoming = append(rep.Upcoming, t)
		}
	}
	byDue := func(ts []*core.Task) {
		sort.SliceStable(ts, func(i, j int) bool { return ts[i].Due < ts[j].Due })
	}
	byDue(rep.Overdue)
	byDue(rep.Upcoming)
	sort.SliceStable(rep.Finished, func(i, j int) bool {
		si := rep.Finished[i].Completed
		if si == "" {
			si = rep.Finished[i].Updated
		}
		sj := rep.Finished[j].Completed
		if sj == "" {
			sj = rep.Finished[j].Updated
		}
		return si < sj
	})
	return rep, nil
}

// ReviewReschedule sets a task's due date from user input (normalized via
// core.DueInput; date-only or local-naive datetime).
func ReviewReschedule(s *store.Store, t *core.Task, input string) error {
	due, err := core.DueInput(input)
	if err != nil {
		return fmt.Errorf("invalid due: %w", err)
	}
	t.Due = due
	return s.Update(t)
}

// ReviewArchive archives one overdue task. Callers confirm first — nothing
// here touches a task without the user choosing to.
func ReviewArchive(s *store.Store, t *core.Task) error {
	return s.ArchiveTask(t)
}

// ReviewFinishedLabel renders one "Mon 2 Nov  Read ch.4" line.
func ReviewRow(t *core.Task, now time.Time) string {
	stamp := t.Completed
	if stamp == "" {
		stamp = t.Updated
	}
	if ts, ok := dates.Parse(stamp, now.Location()); ok {
		return fmt.Sprintf("%s  %s", dates.HumanDay(ts), t.Title)
	}
	return t.Title
}
