package core

import "fmt"

// GradeItem is one assignment or exam contributing to a course grade.
type GradeItem struct {
	ID        string
	Title     string
	Kind      string
	Weight    float64 // percent weight; meaningful only when HasWeight
	HasWeight bool
	Score     float64 // raw recorded score; meaningful only when Scored
	ScoreMax  float64
	Scored    bool
}

// ScorePct normalizes a scored item to 0..100. Unscored items return 0.
func (g GradeItem) ScorePct() float64 {
	if !g.Scored || g.ScoreMax <= 0 {
		return 0
	}
	p := g.Score / g.ScoreMax * 100
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}

// CourseGrades is the weighted rollup over one course's assignment/exam
// items. All percentages are 0..100.
type CourseGrades struct {
	Items []GradeItem
	// TotalWeight is Σ weights over weight-bearing items (may be ≠ 100).
	TotalWeight float64
	// EarnedWeight is Σ weights of items that have been scored.
	EarnedWeight float64
	// RemainingWeight is TotalWeight − EarnedWeight.
	RemainingWeight float64
	// CurrentPct is the average over graded work: Σ(w·pct)/EarnedWeight.
	CurrentPct float64
	// StandingPct is earned over assigned weight: Σ(w·pct)/TotalWeight.
	// Counts ungraded weight as unearned; equals the grade if no more
	// work is ever scored.
	StandingPct float64
	// NeededOnRemaining is the average percentage required on the
	// remaining weight to reach TargetPct of the assigned weight.
	NeededOnRemaining float64
	// NeededFeasible reports whether NeededOnRemaining ≤ 100.
	NeededFeasible bool
	// NeededSecured reports whether even scoring 0 on the remaining
	// weight keeps the target (NeededOnRemaining ≤ 0).
	NeededSecured bool
	// TargetPct is the target used for the needed-on-remaining math.
	// 0 means no target was set; the needed-* fields are then meaningless.
	TargetPct float64
	// Unweighted reports whether no item carries a weight; the CLI then
	// falls back to a simple average for display.
	Unweighted bool
	// UnweightedPct is the simple average over scored items (0 when none).
	UnweightedPct float64
	// MissingWeights lists items without a weight (excluded from weighted math).
	MissingWeights []string
	// TotalWeightOff notes that assigned weights do not add up to 100.
	TotalWeightOff bool
	// HasTarget reports whether TargetPct was provided.
	HasTarget bool
}

// ComputeCourseGrades rolls items up. Items are expected to be the course's
// assignment and exam tasks. targetPct is 0..100 and may be 0 when unknown
// (hasTarget=false then).
func ComputeCourseGrades(items []GradeItem, targetPct float64, hasTarget bool) CourseGrades {
	g := CourseGrades{Items: items, TargetPct: targetPct, HasTarget: hasTarget}
	var sum, earned float64
	scoredCount := 0
	simpleSum := 0.0
	for _, it := range items {
		if it.HasWeight {
			g.TotalWeight += it.Weight
			if it.Scored {
				earned += it.Weight
				sum += it.Weight * it.ScorePct()
			}
		} else {
			g.MissingWeights = append(g.MissingWeights, it.Title)
		}
		if it.Scored {
			scoredCount++
			simpleSum += it.ScorePct()
		}
	}
	g.EarnedWeight = earned
	g.RemainingWeight = g.TotalWeight - g.EarnedWeight
	g.Unweighted = g.TotalWeight == 0
	if scoredCount > 0 {
		g.UnweightedPct = simpleSum / float64(scoredCount)
	}
	if earned > 0 {
		g.CurrentPct = sum / earned
	}
	if g.TotalWeight > 0 {
		g.StandingPct = sum / g.TotalWeight
	}
	g.TotalWeightOff = g.TotalWeight > 0 && g.TotalWeight != 100
	if hasTarget && g.RemainingWeight > 0 && g.TotalWeight > 0 {
		needed := (targetPct*g.TotalWeight - sum) / g.RemainingWeight
		g.NeededOnRemaining = needed
		g.NeededFeasible = needed <= 100
		g.NeededSecured = needed <= 0
	} else if hasTarget && g.RemainingWeight <= 0 && g.TotalWeight > 0 {
		g.NeededFeasible = true
		g.NeededSecured = sum/g.TotalWeight >= targetPct
	}
	return g
}

// FormatSummary renders the one-line rollup used in `mtm grades` output and
// in the TUI course dashboard.
func FormatSummary(g CourseGrades) string {
	if g.Unweighted {
		if len(g.Items) == 0 {
			return "no graded work yet"
		}
		return fmt.Sprintf("avg %.1f%% (no weights set)", g.UnweightedPct)
	}
	return fmt.Sprintf("current %.1f%% · standing %.1f%% of %.0f assigned", g.CurrentPct, g.StandingPct, g.TotalWeight)
}
