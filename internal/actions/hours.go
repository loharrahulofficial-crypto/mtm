package actions

import (
	"sort"
	"strconv"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
)

// HoursRollup is the study-time total for one course. Time is attributed by
// course project; subtasks roll into their parent's course automatically.
type HoursRollup struct {
	Course          string
	LoggedMinutes   int
	EstimateMinutes int
	// WeekMinutes is the subset attributed to the trailing 7 days (same rule
	// as the Home weekly chart: completed day, or updated day for done tasks
	// without a completion stamp).
	WeekMinutes int
	// Accuracy is the logged-vs-estimate rollup over the course's tasks
	// (spec 0.2): only estimated tasks enter the percentage; enough =
	// at least 3 estimated tasks.
	Accuracy core.EstimateAccuracy
	// Tasks carries the tasks that have time logged, most minutes first.
	Tasks []*core.Task
}

// HoursReport is the per-course rollup plus a bucket for logged time on
// non-course projects (shown so it never disappears silently).
type HoursReport struct {
	Courses              []HoursRollup
	OtherLoggedMinutes   int
	OtherWeekMinutes     int
	OtherEstimateMinutes int
}

// CollectHours rolls time_logged_minutes and estimates up per course.
// now anchors the trailing-7-day week window.
func CollectHours(s *store.Store, now time.Time) (*HoursReport, error) {
	courses, err := ListCourses(s)
	if err != nil {
		return nil, err
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekStart := today.AddDate(0, 0, -6)
	isDone := func(t *core.Task) bool { return s.IsDoneStatus(t.Project, t.Status) }
	attribute := func(t *core.Task) bool {
		stamp := t.Completed
		if stamp == "" && isDone(t) {
			stamp = t.Updated
		}
		if stamp == "" {
			return false
		}
		ts, err := time.ParseInLocation(core.TimestampFormat, stamp, now.Location())
		if err != nil {
			return false
		}
		return !ts.Before(weekStart)
	}
	courseNames := map[string]bool{}
	rep := &HoursReport{}
	for _, c := range courses {
		courseNames[c.Name] = true
		r := HoursRollup{Course: c.Name}
		courseTasks := CourseTasks(s, c.Name)
		for _, t := range courseTasks {
			r.LoggedMinutes += t.TimeLoggedMinutes
			r.EstimateMinutes += t.EstimateMinutes
			if attribute(t) {
				r.WeekMinutes += t.TimeLoggedMinutes
			}
			if t.TimeLoggedMinutes > 0 {
				r.Tasks = append(r.Tasks, t)
			}
		}
		r.Accuracy = core.EstimateAccuracyOf(courseTasks)
		sort.Slice(r.Tasks, func(i, j int) bool { return r.Tasks[i].TimeLoggedMinutes > r.Tasks[j].TimeLoggedMinutes })
		rep.Courses = append(rep.Courses, r)
	}
	sort.Slice(rep.Courses, func(i, j int) bool { return rep.Courses[i].LoggedMinutes > rep.Courses[j].LoggedMinutes })
	for _, t := range s.Tasks() {
		if courseNames[t.Project] {
			continue
		}
		rep.OtherLoggedMinutes += t.TimeLoggedMinutes
		rep.OtherEstimateMinutes += t.EstimateMinutes
		if attribute(t) {
			rep.OtherWeekMinutes += t.TimeLoggedMinutes
		}
	}
	return rep, nil
}

// FormatMinutes renders minutes as "4h 30m", "45m" or "0m".
func FormatMinutes(mins int) string {
	if mins <= 0 {
		return "0m"
	}
	h, m := mins/60, mins%60
	switch {
	case h == 0:
		return strconv.Itoa(m) + "m"
	case m == 0:
		return strconv.Itoa(h) + "h"
	default:
		return strconv.Itoa(h) + "h " + strconv.Itoa(m) + "m"
	}
}
