package actions

import (
	"fmt"
	"strings"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
)

// RecordGrade stores a score on an assignment or exam task. If weight is
// non-nil it also sets (or overrides) the task's grade weight. The score is
// stored raw together with its maximum; percentages are derived at display
// time.
func RecordGrade(s *store.Store, id string, score, max float64, weight *float64) (*core.Task, error) {
	t, err := s.Resolve(id)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(t.Kind, core.KindAssignment) && !strings.EqualFold(t.Kind, core.KindExam) {
		return nil, fmt.Errorf("task %s (%q) is kind %q — only assignments and exams carry grades", t.ID, t.Title, t.Kind)
	}
	if max <= 0 {
		return nil, fmt.Errorf("--max must be > 0, got %v", max)
	}
	if score < 0 {
		return nil, fmt.Errorf("score must be >= 0, got %v", score)
	}
	if score > max {
		return nil, fmt.Errorf("score %v is above --max %v", score, max)
	}
	if weight != nil && (*weight < 0 || *weight > 100) {
		return nil, fmt.Errorf("--weight must be between 0 and 100, got %v", *weight)
	}
	t.Score = &score
	t.ScoreMax = &max
	if weight != nil {
		t.Weight = weight
	}
	if err := s.Update(t); err != nil {
		return nil, err
	}
	return t, nil
}

// CollectCourseGrades builds the grade rollup over a course's assignment and
// exam tasks. targetOverride, when non-empty, wins over the course's
// target_grade; either accepts a letter grade or a percentage.
func CollectCourseGrades(s *store.Store, courseArg, targetOverride string) (*core.CourseGrades, *core.Project, error) {
	name, err := ResolveCourse(s, courseArg)
	if err != nil {
		return nil, nil, err
	}
	p, err := s.GetProject(name)
	if err != nil {
		return nil, nil, err
	}
	items := gradeItems(s, name)
	target := 0.0
	hasTarget := false
	raw := targetOverride
	if raw == "" && p.Course != nil {
		raw = p.Course.TargetGrade
	}
	if strings.TrimSpace(raw) != "" {
		pct, err := core.GradeToPercent(raw)
		if err != nil {
			return nil, nil, fmt.Errorf("target grade: %w", err)
		}
		target, hasTarget = pct, true
	}
	g := core.ComputeCourseGrades(items, target, hasTarget)
	return &g, p, nil
}

// gradeItems converts a course's assignment/exam tasks into grade items.
func gradeItems(s *store.Store, name string) []core.GradeItem {
	var items []core.GradeItem
	for _, t := range CourseTasks(s, name) {
		if !strings.EqualFold(t.Kind, core.KindAssignment) && !strings.EqualFold(t.Kind, core.KindExam) {
			continue
		}
		item := core.GradeItem{ID: t.ID, Title: t.Title, Kind: t.Kind}
		if t.Weight != nil && *t.Weight > 0 {
			item.Weight = *t.Weight
			item.HasWeight = true
		}
		max := 100.0
		if t.ScoreMax != nil && *t.ScoreMax > 0 {
			max = *t.ScoreMax
		}
		if t.Score != nil {
			item.Score = *t.Score
			item.ScoreMax = max
			item.Scored = true
		}
		items = append(items, item)
	}
	return items
}
