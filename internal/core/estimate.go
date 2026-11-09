package core

import "fmt"

// EstimateAccuracy rolls logged time against estimates over a set of tasks.
// Spec (Phase 5, item 0.2): only tasks that carry an estimate participate in
// the percentage; the rollup is shown as "N of M tasks estimated"; the
// percentage itself is meaningful only with at least 3 estimated tasks.
type EstimateAccuracy struct {
	// Estimated is the number of tasks with a positive estimate.
	Estimated int
	// Total is the number of tasks in scope.
	Total int
	// LoggedMin is time logged on estimated tasks.
	LoggedMin int
	// EstimateMin is the summed estimate of estimated tasks.
	EstimateMin int
	// Enough reports whether at least 3 tasks have estimates; the
	// percentage is hidden below that.
	Enough bool
}

// EstimateAccuracyOf computes the rollup over tasks.
func EstimateAccuracyOf(tasks []*Task) EstimateAccuracy {
	var a EstimateAccuracy
	for _, t := range tasks {
		a.Total++
		if t.EstimateMinutes > 0 {
			a.Estimated++
			a.LoggedMin += t.TimeLoggedMinutes
			a.EstimateMin += t.EstimateMinutes
		}
	}
	a.Enough = a.Estimated >= 3
	return a
}

// Pct is logged-vs-estimate over estimated tasks, or 0 when there is
// nothing to compare. Only meaningful when Enough is true.
func (a EstimateAccuracy) Pct() float64 {
	if a.EstimateMin <= 0 {
		return 0
	}
	return float64(a.LoggedMin) / float64(a.EstimateMin) * 100
}

// Label is the "N of M tasks estimated" fragment.
func (a EstimateAccuracy) Label() string {
	return fmt.Sprintf("%d of %d tasks estimated", a.Estimated, a.Total)
}
