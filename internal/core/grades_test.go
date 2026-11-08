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
			// total 70 ≠ 100: off flag; needed = (90*70 - 2700)/40 = 90
			wantTotal: 70, wantEarned: 30, wantRemaining: 40,
			wantCurrent: 90, wantStanding: 2700.0 / 70,
			wantNeeded: 90, wantFeasible: true, wantOff: true,
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
			if math.Abs(g.NeededOnRemaining-tt.wantNeeded) > 1e-9 {
				t.Errorf("NeededOnRemaining = %v, want %v", g.NeededOnRemaining, tt.wantNeeded)
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
