package core

import (
	"math"
	"testing"
)

func TestComputeCourseGrades(t *testing.T) {
	item := func(title string, weight float64, hasWeight bool, score, max float64, scored bool) GradeItem {
		return GradeItem{Title: title, Kind: KindAssignment, Weight: weight, HasWeight: hasWeight, Score: score, ScoreMax: max, Scored: scored}
	}
	tests := []struct {
		name           string
		items          []GradeItem
		target         float64
		hasTarget      bool
		wantTotal      float64
		wantEarned     float64
		wantRemaining  float64
		wantCurrent    float64
		wantStanding   float64
		wantNeeded     float64
		wantFeasible   bool
		wantSecured    bool
		wantUnweighted bool
		wantOff        bool
		wantMissing    int
	}{
		{
			name: "nothing graded",
			items: []GradeItem{
				item("homework", 30, true, 0, 100, false),
				item("exam", 70, true, 0, 100, false),
			},
			target: 90, hasTarget: true,
			wantTotal: 100, wantEarned: 0, wantRemaining: 100,
			wantNeeded: 90, wantFeasible: true, wantSecured: false,
		},
		{
			name: "partial grades",
			items: []GradeItem{
				item("midterm", 30, true, 27, 30, true),  // 90%
				item("homework", 30, true, 20, 25, true), // 80%
				item("final", 40, true, 0, 100, false),
			},
			target: 90, hasTarget: true,
			// earned = .9*30 + .8*30 = 51; current = 51/60 = 85
			wantTotal: 100, wantEarned: 60, wantRemaining: 40,
			wantCurrent: 85, wantStanding: 51,
			// needed = (90*100 - 51) / 40 = 97.5
			wantNeeded: 97.5, wantFeasible: true, wantSecured: false,
		},
		{
			name: "target secured",
			items: []GradeItem{
				item("midterm", 80, true, 100, 100, true),
				item("final", 20, true, 0, 100, false),
			},
			target: 60, hasTarget: true,
			// sum = 8000; needed = (60*100 - 8000)/20 = -100 → secured
			wantTotal: 100, wantEarned: 80, wantRemaining: 20,
			wantCurrent: 100, wantStanding: 80,
			wantNeeded: -100, wantFeasible: true, wantSecured: true,
		},
		{
			name: "weights do not total 100",
			items: []GradeItem{
				item("midterm", 30, true, 27, 30, true),
				item("final", 40, true, 0, 100, false),
			},
			target: 90, hasTarget: true,
			// total 70 ≠ 100: off flag; unknown 30 counts as ungraded.
			// earned = 2700; ungraded = 40 remaining + 30 unknown = 70;
			// needed = (90*100 - 2700)/70 = 51.43
			wantTotal: 70, wantEarned: 30, wantRemaining: 40,
			wantCurrent: 90, wantStanding: 2700.0 / 70,
			wantNeeded: (9000.0 - 2700.0) / 70.0, wantFeasible: true, wantOff: true,
		},
		{
			name: "missing weights excluded",
			items: []GradeItem{
				item("unweighted", 0, false, 10, 10, true),
				item("weighted", 50, true, 40, 50, true),
			},
			target: 0, hasTarget: false,
			wantTotal: 50, wantEarned: 50, wantRemaining: 0,
			wantCurrent: 80, wantStanding: 80, wantMissing: 1, wantOff: true,
		},
		{
			name:           "no weights at all → simple average",
			items:          []GradeItem{item("a", 0, false, 8, 10, true), item("b", 0, false, 6, 10, true)},
			wantUnweighted: true, wantMissing: 2, // CurrentPct stays 0; UnweightedPct is what the CLI prints
		},
		{
			name: "needed unreachable",
			items: []GradeItem{
				item("midterm", 30, true, 3, 30, true), // 10%
				item("final", 70, true, 0, 100, false),
			},
			target: 80, hasTarget: true,
			// sum = 30*10 = 300; needed = (80*100 - 300)/70 = 110 > 100
			wantTotal: 100, wantEarned: 30, wantRemaining: 70,
			wantCurrent: 10, wantNeeded: 110, wantFeasible: false, wantSecured: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := ComputeCourseGrades(tt.items, tt.target, tt.hasTarget)
			if g.TotalWeight != tt.wantTotal {
				t.Errorf("TotalWeight = %v, want %v", g.TotalWeight, tt.wantTotal)
			}
			if g.EarnedWeight != tt.wantEarned {
				t.Errorf("EarnedWeight = %v, want %v", g.EarnedWeight, tt.wantEarned)
			}
			if g.RemainingWeight != tt.wantRemaining {
				t.Errorf("RemainingWeight = %v, want %v", g.RemainingWeight, tt.wantRemaining)
			}
			if tt.wantCurrent != 0 && math.Abs(g.CurrentPct-tt.wantCurrent) > 1e-9 {
				t.Errorf("CurrentPct = %v, want %v", g.CurrentPct, tt.wantCurrent)
			}
			if tt.wantStanding != 0 && math.Abs(g.StandingPct-tt.wantStanding) > 1e-9 {
				t.Errorf("StandingPct = %v, want %v", g.StandingPct, tt.wantStanding)
			}
			if math.Abs(g.NeededOnUngraded-tt.wantNeeded) > 1e-9 {
				t.Errorf("NeededOnUngraded = %v, want %v", g.NeededOnUngraded, tt.wantNeeded)
			}
			if g.NeededFeasible != tt.wantFeasible {
				t.Errorf("NeededFeasible = %v, want %v", g.NeededFeasible, tt.wantFeasible)
			}
			if g.NeededSecured != tt.wantSecured {
				t.Errorf("NeededSecured = %v, want %v", g.NeededSecured, tt.wantSecured)
			}
			if g.Unweighted != tt.wantUnweighted {
				t.Errorf("Unweighted = %v, want %v", g.Unweighted, tt.wantUnweighted)
			}
			if g.TotalWeightOff != tt.wantOff {
				t.Errorf("TotalWeightOff = %v, want %v", g.TotalWeightOff, tt.wantOff)
			}
			if len(g.MissingWeights) != tt.wantMissing {
				t.Errorf("MissingWeights = %v, want %d", g.MissingWeights, tt.wantMissing)
			}
		})
	}
}

// Spec (Phase 5, item 0.1): "target met" may only be said when the target
// is mathematically secured given the remaining ungraded weight, where
// unassigned weight up to 100 counts as ungraded. Otherwise: "on track"
// with the margin. Weights totaling 30, 60 and 100.
func TestTargetOutcomeUnknownWeightCountsAsUngraded(t *testing.T) {
	item := func(title string, weight, score, max float64, scored bool) GradeItem {
		return GradeItem{Title: title, Kind: KindAssignment, Weight: weight, HasWeight: true, Score: score, ScoreMax: max, Scored: scored}
	}

	// Weights totaling 30, every item scored perfectly.
	g := ComputeCourseGrades([]GradeItem{item("a", 30, 30, 30, true)}, 50, true)
	if g.UngradedWeight != 70 {
		t.Errorf("total 30: UngradedWeight = %v, want 70 (0 remaining + 70 unassigned)", g.UngradedWeight)
	}
	if g.UnknownWeight != 70 {
		t.Errorf("total 30: UnknownWeight = %v, want 70", g.UnknownWeight)
	}
	if got := g.TargetOutcome(); got != TargetOutcomeOnTrack {
		t.Errorf("total 30 perfect 100, target 50: outcome = %v, want OnTrack (secured would require 50%% of the unknown 70 to count)", got)
	}
	if math.Abs(g.WorstPct-30) > 1e-9 {
		t.Errorf("total 30: WorstPct = %v, want 30", g.WorstPct)
	}
	// needed = (50*100 - 3000)/70
	if math.Abs(g.NeededOnUngraded-2000.0/70.0) > 1e-9 {
		t.Errorf("total 30: NeededOnUngraded = %v, want %v", g.NeededOnUngraded, 2000.0/70.0)
	}

	// Same, but the target is low enough that even zero on the unknown
	// weight keeps it: met, not just "on track".
	g = ComputeCourseGrades([]GradeItem{item("a", 30, 30, 30, true)}, 25, true)
	if got := g.TargetOutcome(); got != TargetOutcomeSecured {
		t.Errorf("total 30 perfect 100, target 25: outcome = %v, want Secured (worst case 30 ≥ 25)", got)
	}

	// Weights totaling 60, 30 scored of the 60, 100 on the rest needed to
	// move: target 50. sum = 3000. ungraded = 30 remaining + 40 unknown = 70.
	g = ComputeCourseGrades([]GradeItem{item("a", 30, 30, 30, true), item("b", 30, 0, 30, false)}, 50, true)
	if g.UngradedWeight != 70 {
		t.Errorf("total 60: UngradedWeight = %v, want 70", g.UngradedWeight)
	}
	if got := g.TargetOutcome(); got != TargetOutcomeOnTrack {
		t.Errorf("total 60: outcome = %v, want OnTrack", got)
	}
	// needed = (5000 - 3000)/70 ≈ 28.57, margin ≈ 71.43
	if math.Abs(g.NeededOnUngraded-2000.0/70.0) > 1e-9 {
		t.Errorf("total 60: NeededOnUngraded = %v, want %v", g.NeededOnUngraded, 2000.0/70.0)
	}
	if math.Abs(g.MarginPct-(100-2000.0/70.0)) > 1e-9 {
		t.Errorf("total 60: MarginPct = %v, want %v", g.MarginPct, 100-2000.0/70.0)
	}
	// All 60 scored at 100: worst case 60 ≥ target 55 → secured.
	g = ComputeCourseGrades([]GradeItem{item("a", 30, 30, 30, true), item("b", 30, 30, 30, true)}, 55, true)
	if got := g.TargetOutcome(); got != TargetOutcomeSecured {
		t.Errorf("total 60 all perfect, target 55: outcome = %v, want Secured", got)
	}
	// All 60 scored at 100: worst case 60 < target 65 → on track, not met.
	g = ComputeCourseGrades([]GradeItem{item("a", 30, 30, 30, true), item("b", 30, 30, 30, true)}, 65, true)
	if got := g.TargetOutcome(); got != TargetOutcomeOnTrack {
		t.Errorf("total 60 all perfect, target 65: outcome = %v, want OnTrack — never 'met' with unknown weight", got)
	}

	// Weights totaling 100: no unknown weight, final grade is meaningful.
	g = ComputeCourseGrades([]GradeItem{item("a", 50, 45, 50, true), item("b", 50, 0, 50, false)}, 80, true)
	if g.UngradedWeight != 50 || g.UnknownWeight != 0 {
		t.Errorf("total 100: UngradedWeight = %v, UnknownWeight = %v; want 50/0", g.UngradedWeight, g.UnknownWeight)
	}
	if got := g.TargetOutcome(); got != TargetOutcomeOnTrack {
		t.Errorf("total 100: outcome = %v, want OnTrack (need 110%% → impossible? no: (80*100-4500)/50 = 70 ≤ 100)", got)
	}
	// Secured: 45/50 + perfect 50 on the rest = 95 ≥ 80... check: sum = 4500,
	// needed = (8000-4500)/50 = 70 → on track. Force secured instead:
	g = ComputeCourseGrades([]GradeItem{item("a", 80, 80, 80, true), item("b", 20, 0, 20, false)}, 75, true)
	if got := g.TargetOutcome(); got != TargetOutcomeSecured {
		t.Errorf("total 100: outcome = %v, want Secured (worst case 80 ≥ 75)", got)
	}
	// Final: everything graded, 80 < 85 → FinalMiss; 80 ≥ 75 → FinalMet.
	g = ComputeCourseGrades([]GradeItem{item("a", 100, 80, 100, true)}, 85, true)
	if got := g.TargetOutcome(); got != TargetOutcomeFinalMiss {
		t.Errorf("total 100 final: outcome = %v, want FinalMiss", got)
	}
	g = ComputeCourseGrades([]GradeItem{item("a", 100, 80, 100, true)}, 75, true)
	if got := g.TargetOutcome(); got != TargetOutcomeFinalMet {
		t.Errorf("total 100 final: outcome = %v, want FinalMet", got)
	}
	// Out of reach: 30/30 scored at 10%, target 80.
	g = ComputeCourseGrades([]GradeItem{item("a", 30, 3, 30, true), item("b", 70, 0, 70, false)}, 80, true)
	if got := g.TargetOutcome(); got != TargetOutcomeOutOfReach {
		t.Errorf("total 100, needed 110: outcome = %v, want OutOfReach", got)
	}
}

func TestTargetOutcomeNoTargetAndWeightless(t *testing.T) {
	item := func(title string, weight float64, scored bool) GradeItem {
		return GradeItem{Title: title, Kind: KindAssignment, Weight: weight, HasWeight: weight > 0, Score: 10, ScoreMax: 10, Scored: scored}
	}
	g := ComputeCourseGrades([]GradeItem{item("a", 50, true), item("b", 50, false)}, 0, false)
	if got := g.TargetOutcome(); got != TargetOutcomeNoTarget {
		t.Errorf("no target: outcome = %v, want NoTarget", got)
	}
	g = ComputeCourseGrades([]GradeItem{item("a", 0, true)}, 0, false)
	if got := g.TargetOutcome(); got != TargetOutcomeWeightless {
		t.Errorf("unweighted: outcome = %v, want Weightless", got)
	}
}
