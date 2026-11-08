package actions

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
)

func TestRecordGrade(t *testing.T) {
	s := openActionStore(t, "college/dbms")
	assign, err := s.Resolve(mustCreateAssignment(t, s))
	if err != nil {
		t.Fatal(err)
	}
	// Record a score with a custom max.
	got, err := RecordGrade(s, assign.ID, 27, 30, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Score == nil || *got.Score != 27 {
		t.Fatalf("score = %v", got.Score)
	}
	if got.ScoreMax == nil || *got.ScoreMax != 30 {
		t.Fatalf("score_max = %v", got.ScoreMax)
	}
	// Recorded grade survives a reload.
	s2 := reopenActionStore(t, s)
	back, err := s2.Resolve(assign.ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.Score == nil || *back.Score != 27 || back.ScoreMax == nil || *back.ScoreMax != 30 {
		t.Fatalf("reloaded score = %v/%v", back.Score, back.ScoreMax)
	}
	// Overriding the weight at grading time sticks.
	w := 25.0
	got, err = RecordGrade(s, assign.ID, 27, 30, &w)
	if err != nil {
		t.Fatal(err)
	}
	if got.Weight == nil || *got.Weight != 25 {
		t.Fatalf("weight = %v", got.Weight)
	}
}

func TestRecordGradeRejects(t *testing.T) {
	s := openActionStore(t, "college/dbms")
	assignID := mustCreateAssignment(t, s)
	// Not an assignment/exam.
	task, err := s.Resolve(assignID)
	if err != nil {
		t.Fatal(err)
	}
	task.Kind = core.KindTask
	if err := s.Update(task); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordGrade(s, assignID, 5, 10, nil); err == nil {
		t.Fatal("grade recorded on a plain task")
	}
	task.Kind = core.KindAssignment
	if err := s.Update(task); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		score, max float64
	}{
		{"zero max", 5, 0},
		{"negative score", -1, 10},
		{"score above max", 11, 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := RecordGrade(s, assignID, tc.score, tc.max, nil); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}

func TestCollectCourseGrades(t *testing.T) {
	s := openActionStore(t, "college/dbms")
	mk := func(title string, weight *float64, kind string) *core.Task {
		task := &core.Task{Title: title, Project: "college/dbms", Status: "todo", Kind: kind, Weight: weight}
		if err := s.Create(task); err != nil {
			t.Fatal(err)
		}
		return task
	}
	w30, w40 := 30.0, 40.0
	mid := mk("midterm", &w30, core.KindExam)
	fin := mk("final", &w40, core.KindExam)
	hw := mk("homework 1", &w30, core.KindAssignment)
	mk("quiz without weight", nil, core.KindAssignment)
	// Grade midterm 27/30 and homework 80%.
	if _, err := RecordGrade(s, mid.ID, 27, 30, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordGrade(s, hw.ID, 8, 10, nil); err != nil {
		t.Fatal(err)
	}
	g, p, err := CollectCourseGrades(s, "dbms", "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Course == nil {
		t.Fatal("project has no course block")
	}
	// total assigned = 100 (30+40+30); graded = 60
	if g.TotalWeight != 100 {
		t.Errorf("TotalWeight = %v, want 100", g.TotalWeight)
	}
	if g.EarnedWeight != 60 {
		t.Errorf("EarnedWeight = %v, want 60", g.EarnedWeight)
	}
	if g.RemainingWeight != 40 {
		t.Errorf("RemainingWeight = %v, want 40", g.RemainingWeight)
	}
	if got := g.CurrentPct; got < 84.9 || got > 85.1 {
		t.Errorf("CurrentPct = %v, want ~85", got)
	}
	if len(g.MissingWeights) != 1 {
		t.Errorf("MissingWeights = %v, want 1", g.MissingWeights)
	}
	// The quiz's missing weight is excluded; final still pending.
	if g.Items == nil || len(g.Items) != 4 {
		t.Errorf("Items = %d, want 4", len(g.Items))
	}
	_ = fin
}

func TestCollectCourseGradesTargetOverride(t *testing.T) {
	s := openActionStore(t, "college/dbms")
	if _, _, err := CollectCourseGrades(s, "dbms", "not-a-grade"); err == nil {
		t.Fatal("bad target override accepted")
	} else if !strings.Contains(err.Error(), "target") {
		t.Fatalf("error should mention target: %v", err)
	}
}

func mustCreateAssignment(t *testing.T, s *store.Store) string {
	t.Helper()
	task := &core.Task{Title: "homework", Project: "college/dbms", Status: "todo", Kind: core.KindAssignment}
	if err := s.Create(task); err != nil {
		t.Fatal(err)
	}
	return task.ID
}

func reopenActionStore(t *testing.T, s *store.Store) *store.Store {
	t.Helper()
	cache := filepath.Join(t.TempDir(), "index.json")
	s2, err := store.Open(s.Root, cache)
	if err != nil {
		t.Fatal(err)
	}
	return s2
}
