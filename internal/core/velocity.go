package core

import (
	"time"
)

// VelocityWindow is the number of trailing weeks the velocity sparkline
// covers (oldest week first, size == weeks).
const VelocityWindow = 12

// Velocity describes how quickly tasks get finished over the trailing
// window. Enough is false when there is not enough history to estimate an
// finish date (no completions, or fewer than 2 distinct active weeks).
type Velocity struct {
	// Weeks holds the completion count per week, oldest first; len == window.
	Weeks      []float64
	AvgPerWeek float64
	// ActiveWeeks counts weeks with at least one completion.
	ActiveWeeks int
	Enough      bool
}

// completionDay is the local day a task counts as finished: its completed
// stamp, else the updated stamp for done-but-undated tasks.
func completionDay(t *Task, isDone func(project, status string) bool, loc *time.Location) (time.Time, bool) {
	stamp := t.Completed
	if stamp == "" && isDone != nil && isDone(t.Project, t.Status) {
		stamp = t.Updated
	}
	if stamp == "" {
		return time.Time{}, false
	}
	ts, ok := parseStamp(stamp, loc)
	if !ok {
		return time.Time{}, false
	}
	return time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, loc), true
}

// parseStamp reads the layouts the store writes (duplicates internal/dates
// because core must not import it — dates imports core).
func parseStamp(s string, loc *time.Location) (time.Time, bool) {
	for _, layout := range []string{TimeFormat, "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// VelocityReport computes per-week completion counts over the trailing
// weeks ending at the week containing now. weekStart locates that week
// (Monday-aligned by the caller or raw calendar weeks — either is fine,
// the math only needs consistent 7-day buckets ending at now).
func VelocityReport(all []*Task, isDone func(project, status string) bool, now time.Time, weeks int) Velocity {
	loc := now.Location()
	if weeks < 1 {
		weeks = 1
	}
	v := Velocity{Weeks: make([]float64, weeks)}
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	// Bucket 0 = the week containing today; bucket weeks-1 = oldest.
	for _, t := range all {
		day, ok := completionDay(t, isDone, loc)
		if !ok {
			continue
		}
		back := int(todayStart.Sub(day).Hours() / (24 * 7))
		if back < 0 {
			back = 0 // stamps in the future still count as this week
		}
		if back >= weeks {
			continue
		}
		v.Weeks[weeks-1-back]++
	}
	total := 0.0
	for _, n := range v.Weeks {
		total += n
		if n > 0 {
			v.ActiveWeeks++
		}
	}
	v.AvgPerWeek = total / float64(weeks)
	v.Enough = v.ActiveWeeks >= 2
	return v
}

// EstimateFinish projects the date the remaining open work completes, at
// the trailing velocity, starting from now. ok=false when the velocity has
// not earned an estimate (fewer than 2 active weeks). remaining == 0 means
// the scope is already complete: (now, true).
func EstimateFinish(remaining int, v Velocity, now time.Time) (time.Time, bool) {
	if remaining <= 0 {
		return now, true
	}
	if !v.Enough || v.AvgPerWeek <= 0 {
		return time.Time{}, false
	}
	weeksLeft := float64(remaining) / v.AvgPerWeek
	days := int(weeksLeft * 7)
	if days < 1 {
		days = 1
	}
	return now.AddDate(0, 0, days), true
}

// ProjectProgress combines task completion and milestone completion into
// one percentage. Hours are reporting-only (there is no per-project target
// in mtm), so they never feed the percentage.
type ProjectProgress struct {
	Done, Total        int
	TaskFrac           float64
	MilestoneDone      int
	MilestoneTotal     int
	MilestoneFrac      float64
	HoursLoggedMinutes int
	Combined           float64
	Enough             bool
}

// CombineProgress blends the task and milestone completion fractions into a
// single [0,1] value, excluding components that have no data. Enough=false
// when neither component has data.
func CombineProgress(done, total, mDone, mTotal int) (frac float64, enough bool) {
	haveTask := total > 0
	haveMilestone := mTotal > 0
	switch {
	case haveTask && haveMilestone:
		return (float64(done)/float64(total) + float64(mDone)/float64(mTotal)) / 2, true
	case haveTask:
		return float64(done) / float64(total), true
	case haveMilestone:
		return float64(mDone) / float64(mTotal), true
	default:
		return 0, false
	}
}
