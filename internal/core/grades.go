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
	// NeededOnUngraded is the average percentage required on ALL ungraded
	// weight — assigned-but-ungraded plus unassigned weight (treated as up
	// to 100, i.e. unknown weight is ungraded) — to reach TargetPct.
	NeededOnUngraded float64
	// NeededFeasible reports whether NeededOnUngraded ≤ 100.
	NeededFeasible bool
	// NeededSecured reports whether even scoring 0 on the remaining
	// ungraded weight (incl. unassigned) keeps the target.
	NeededSecured bool
	// UngradedWeight is RemainingWeight + UnknownWeight: every point of
	// weight on which the target is not yet mathematically known.
	UngradedWeight float64
	// UnknownWeight is max(0, 100 − TotalWeight) — weight no item was
	// assigned. It counts as ungraded for target math (spec: treat
	// unknown weight up to 100 as ungraded).
	UnknownWeight float64
	// WorstPct is the final grade if every ungraded weight scores 0:
	// Σ(w·pct)/100. The target is "met" only at or above this point or
	// later finals.
	WorstPct float64
	// MarginPct is 100 − NeededOnUngraded: the headroom on the rest.
	// Negative when the target is out of reach.
	MarginPct float64
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
	g.UnknownWeight = 100 - g.TotalWeight
	if g.UnknownWeight < 0 {
		g.UnknownWeight = 0
	}
	g.UngradedWeight = g.RemainingWeight + g.UnknownWeight
	if g.TotalWeight > 0 {
		g.WorstPct = sum / 100
	}
	switch {
	case hasTarget && g.UngradedWeight > 0 && g.TotalWeight > 0:
		needed := (targetPct*100 - sum) / g.UngradedWeight
		g.NeededOnUngraded = needed
		g.MarginPct = 100 - needed
		g.NeededFeasible = needed <= 100
		g.NeededSecured = needed <= 0
	case hasTarget && g.UngradedWeight <= 0 && g.TotalWeight > 0:
		// Everything is graded: the final grade is fixed.
		g.NeededFeasible = true
		g.NeededSecured = sum/g.TotalWeight >= targetPct
	}
	return g
}

// TargetOutcome is the target-grade verdict shared by the CLI and the TUI,
// so both render the same "met / on track / out of reach" rule.
type TargetOutcome int

const (
	TargetOutcomeNoTarget   TargetOutcome = iota // no target set
	TargetOutcomeFinalMet                        // all weight graded, final meets target
	TargetOutcomeFinalMiss                       // all weight graded, final misses target
	TargetOutcomeSecured                         // not final, but even 0 on the ungraded weight keeps target
	TargetOutcomeOnTrack                         // feasible: target reachable with the right work
	TargetOutcomeOutOfReach                      // not feasible: even 100 on the rest misses
	TargetOutcomeWeightless                      // no weighted items at all (unweighted average path)
)

// TargetOutcome maps the rollup to the shared verdict. "Met" is only ever
// returned when the outcome is FinalMet or Secured — i.e. the target is
// mathematically secured given the remaining (ungraded + unassigned) weight.
func (g CourseGrades) TargetOutcome() TargetOutcome {
	if g.Unweighted || g.TotalWeight == 0 {
		return TargetOutcomeWeightless
	}
	if !g.HasTarget {
		return TargetOutcomeNoTarget
	}
	if g.UngradedWeight <= 0 {
		if g.StandingPct >= g.TargetPct {
			return TargetOutcomeFinalMet
		}
		return TargetOutcomeFinalMiss
	}
	switch {
	case g.NeededSecured:
		return TargetOutcomeSecured
	case g.NeededFeasible:
		return TargetOutcomeOnTrack
	default:
		return TargetOutcomeOutOfReach
	}
}

// BestCasePct is the final grade if everything ungraded scores 100:
// Σ(w·pct)/100 + UngradedWeight. Used by the out-of-reach wording.
func (g CourseGrades) BestCasePct() float64 { return g.WorstPct + g.UngradedWeight }

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
