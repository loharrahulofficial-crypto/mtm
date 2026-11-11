package actions

// Stats and streak engine (Phase 5, item 1). Hours are attributed the same
// way the Home chart does: a task's logged minutes land on its completion
// day (updated stamp as fallback); unfinished tasks with updated > created
// count toward their updated day so the streak keeps counting while the
// work is open.

import (
	"sort"
	"strings"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/dates"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
)

// dayStamp returns local midnight of t.
func dayStamp(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// dayKey is the calendar-day identity of t (YYYY-MM-DD). Unix-second
// arithmetic breaks across DST transitions (midnights can be 82800/90000s
// apart), so streak math and day-bucket indexing work on day keys.
func dayKey(t time.Time) string {
	return t.Format("2006-01-02")
}

// completionStamp is the moment a task is counted as finished: the
// completion stamp when present, falling back to the updated stamp for
// tasks that are done but were never stamped.
func completionStamp(t *core.Task, isDone func(project, status string) bool, loc *time.Location) (time.Time, bool) {
	stamp := t.Completed
	if stamp == "" && isDone(t.Project, t.Status) {
		stamp = t.Updated
	}
	if stamp == "" {
		return time.Time{}, false
	}
	return dates.Parse(stamp, loc)
}

// loggedDay is the day a task's logged minutes count toward: its
// completion day, or (work still open) the day it was last touched —
// every task with logged time is proof of work on its updated day.
func loggedDay(t *core.Task, isDone func(project, status string) bool, loc *time.Location) (time.Time, bool) {
	if ts, ok := completionStamp(t, isDone, loc); ok {
		return dayStamp(ts), true
	}
	if t.TimeLoggedMinutes <= 0 {
		return time.Time{}, false
	}
	upd, ok := dates.Parse(t.Updated, loc)
	if !ok {
		return time.Time{}, false
	}
	return dayStamp(upd), true
}

// onTime reports whether the completion landed within the due window: a
// dated due means on-or-before the due day; a timed due means on-or-before
// the due instant.
func onTime(t *core.Task, completed time.Time, loc *time.Location) bool {
	if t.Due == "" {
		return false
	}
	if due, err := time.ParseInLocation(core.TimestampFormat, t.Due, loc); err == nil {
		return !completed.After(due)
	}
	if d, err := time.ParseInLocation("2006-01-02", t.Due, loc); err == nil {
		return !dayStamp(completed).After(dayStamp(d))
	}
	return false
}

// StreakReport carries the current and longest run of consecutive active
// days.
type StreakReport struct {
	Current int
	Longest int
}

// Streaks counts consecutive active days over all history. A day is active
// when at least one task completed that day, or when the day collected at
// least minMinutes of logged time (0 disables minute-based activity).
// "Current" keeps the streak alive from yesterday when today has not (yet)
// been active; a day with no activity breaks the run.
func Streaks(s *store.Store, minMinutes int, now time.Time) StreakReport {
	loc := now.Location()
	isDone := func(project, status string) bool { return s.IsDoneStatus(project, status) }
	active := map[string]bool{}
	minutes := map[string]int{}
	for _, t := range s.Tasks() {
		if ts, ok := completionStamp(t, isDone, loc); ok {
			active[dayKey(ts)] = true
		}
		if t.TimeLoggedMinutes > 0 {
			if day, ok := loggedDay(t, isDone, loc); ok {
				minutes[dayKey(day)] += t.TimeLoggedMinutes
			}
		}
	}
	if minMinutes > 0 {
		for d, m := range minutes {
			if m >= minMinutes {
				active[d] = true
			}
		}
	}
	days := make([]string, 0, len(active))
	for d := range active {
		days = append(days, d)
	}
	sort.Strings(days)
	longest, run := 0, 0
	var prev time.Time
	for i, d := range days {
		cur, err := time.ParseInLocation("2006-01-02", d, loc)
		if err != nil {
			continue
		}
		if i > 0 && cur.Equal(prev.AddDate(0, 0, 1)) {
			run++
		} else {
			run = 1
		}
		if run > longest {
			longest = run
		}
		prev = cur
	}
	current := 0
	for d := dayStamp(now); ; d = d.AddDate(0, 0, -1) {
		if active[dayKey(d)] {
			current++
			continue
		}
		if d.Equal(dayStamp(now)) {
			continue // today may not be done yet; the streak lives off yesterday
		}
		break
	}
	return StreakReport{Current: current, Longest: longest}
}

// StatsCourse is the per-course slice of a report.
type StatsCourse struct {
	Course    string
	Completed int
	Hours     float64
	OnTime    int
	WithDue   int
	Estimate  core.EstimateAccuracy
}

// StatsReport is one period's rollup.
type StatsReport struct {
	Period string
	Since  time.Time
	Until  time.Time
	Days   int

	Completed      int
	DayCompletions []int
	DayHours       []float64
	OnTime         int
	WithDue        int
	BusiestDay     time.Time // zero when nothing completed
	BusiestCount   int
	BusiestHours   float64

	Courses        []StatsCourse // hours desc, then completed desc, then name
	OtherCompleted int
	OtherHours     float64
	OtherOnTime    int
	OtherWithDue   int

	// Accuracy over the period's completed tasks (spec item 0.2).
	Estimate core.EstimateAccuracy
	// Streaks over all history, not the period.
	Streaks StreakReport
}

// CollectStats builds the report for [since, today]. until is always
// today; the label is produced by callers (they own the wording).
func CollectStats(s *store.Store, now time.Time, since time.Time, period string, minStreakMinutes int) (*StatsReport, error) {
	loc := now.Location()
	isDone := func(project, status string) bool { return s.IsDoneStatus(project, status) }
	since = dayStamp(since)
	until := dayStamp(now)
	days := 1
	for d := since; d.Before(until); d = d.AddDate(0, 0, 1) {
		days++
	}
	if days > 800 {
		// Documented limit: ancient history beyond 800 days is dropped,
		// never mis-bucketed.
		since = until.AddDate(0, 0, -799)
		days = 800
	}
	if days < 1 {
		days = 1
	}
	rep := &StatsReport{
		Period:         period,
		Since:          since,
		Until:          until,
		Days:           days,
		DayCompletions: make([]int, days),
		DayHours:       make([]float64, days),
	}
	// Calendar-day bucket index; Hour/24 arithmetic mis-buckets across DST.
	dayIndex := map[string]int{}
	for x, i := since, 0; !x.After(until); x = x.AddDate(0, 0, 1) {
		dayIndex[dayKey(x)] = i
		i++
	}
	courses, err := ListCourses(s)
	if err != nil {
		return nil, err
	}
	courseIdx := map[string]int{}
	for _, c := range courses {
		courseIdx[strings.ToLower(c.Name)] = len(rep.Courses)
		rep.Courses = append(rep.Courses, StatsCourse{Course: c.Name})
	}
	completedByCourse := make([][]*core.Task, len(rep.Courses))
	var completedOther []*core.Task
	for _, t := range s.Tasks() {
		ts, ok := completionStamp(t, isDone, loc)
		if !ok {
			continue
		}
		day := dayStamp(ts)
		if day.Before(since) || day.After(until) {
			continue
		}
		idx, ok := dayIndex[dayKey(day)]
		if !ok {
			continue
		}
		hours := float64(t.TimeLoggedMinutes) / 60
		rep.Completed++
		rep.DayCompletions[idx]++
		rep.DayHours[idx] += hours
		dueSet := strings.TrimSpace(t.Due) != ""
		if dueSet {
			rep.WithDue++
			if onTime(t, ts, loc) {
				rep.OnTime++
			}
		}
		key := strings.ToLower(t.Project)
		if i, ok := courseIdx[key]; ok {
			c := &rep.Courses[i]
			c.Completed++
			c.Hours += hours
			completedByCourse[i] = append(completedByCourse[i], t)
			if dueSet {
				c.WithDue++
				if onTime(t, ts, loc) {
					c.OnTime++
				}
			}
			continue
		}
		rep.OtherCompleted++
		rep.OtherHours += hours
		completedOther = append(completedOther, t)
		if dueSet {
			rep.OtherWithDue++
			if onTime(t, ts, loc) {
				rep.OtherOnTime++
			}
		}
	}
	// Estimate accuracy over the period's completed tasks (spec 0.2).
	for i := range rep.Courses {
		rep.Courses[i].Estimate = core.EstimateAccuracyOf(completedByCourse[i])
	}
	// The global accuracy counts every completed task in the period.
	global := append([]*core.Task{}, completedOther...)
	for _, tasks := range completedByCourse {
		global = append(global, tasks...)
	}
	rep.Estimate = core.EstimateAccuracyOf(global)
	// Busiest day.
	best := -1
	for i, n := range rep.DayCompletions {
		if n > best {
			best = n
			rep.BusiestDay = since.AddDate(0, 0, i)
			rep.BusiestCount = n
			rep.BusiestHours = rep.DayHours[i]
		}
	}
	sort.SliceStable(rep.Courses, func(i, j int) bool {
		a, b := rep.Courses[i], rep.Courses[j]
		if a.Hours != b.Hours {
			return a.Hours > b.Hours
		}
		if a.Completed != b.Completed {
			return a.Completed > b.Completed
		}
		return strings.ToLower(a.Course) < strings.ToLower(b.Course)
	})
	rep.Streaks = Streaks(s, minStreakMinutes, now)
	return rep, nil
}

// TotalHours is the logged hours in the period.
func (r *StatsReport) TotalHours() float64 {
	total := 0.0
	for _, h := range r.DayHours {
		total += h
	}
	return total
}

// OnTimePct is the on-time rate over completed tasks with a due date.
func (r *StatsReport) OnTimePct() float64 {
	if r.WithDue == 0 {
		return 0
	}
	return float64(r.OnTime) / float64(r.WithDue) * 100
}

// CourseOnTimePct is the on-time rate for one course row.
func CourseOnTimePct(c StatsCourse) float64 {
	if c.WithDue == 0 {
		return 0
	}
	return float64(c.OnTime) / float64(c.WithDue) * 100
}
