package actions

import (
	"fmt"
	"strings"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
)

// AddExam creates a kind:exam task under course, with a due date, an optional
// weight and optional topics (stored as tags). The project is resolved like
// any project argument: exact, case-insensitive, then a unique leaf-name
// match. It must already exist.
// This is the single core function behind both `mtm exam add` and the `:exam`
// palette command.
func AddExam(s *store.Store, course, title, due string, weight *float64, topics []string) (*core.Task, error) {
	name, err := ResolveProject(s, course)
	if err != nil {
		return nil, err
	}
	p, err := s.GetProject(name)
	if err != nil {
		return nil, fmt.Errorf("course %q: %w", name, err)
	}
	if _, err := time.Parse("2006-01-02", due); err != nil {
		return nil, fmt.Errorf("exam date %q must be 2006-01-02", due)
	}
	if strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf("exam needs a name")
	}
	t := &core.Task{
		Title:   title,
		Project: name,
		Status:  firstStatus(p),
		Due:     due,
		Weight:  weight,
		Kind:    core.KindExam,
		Tags:    uniqueTags(topics),
	}
	if err := s.Create(t); err != nil {
		return nil, err
	}
	return t, nil
}

// firstStatus returns the project's first (initial) status.
func firstStatus(p *core.Project) string {
	if len(p.Statuses) > 0 {
		return p.Statuses[0].Name
	}
	return "todo"
}

// uniqueTags deduplicates and trims topic tags, preserving order.
func uniqueTags(topics []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, t := range topics {
		tag := strings.ToLower(strings.TrimSpace(t))
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		out = append(out, tag)
	}
	return out
}

// ListExams returns all non-done kind:exam tasks, nearest due first.
func ListExams(s *store.Store) []*core.Task {
	var out []*core.Task
	for _, t := range s.Tasks() {
		if strings.EqualFold(t.Kind, core.KindExam) && !s.IsDoneStatus(t.Project, t.Status) {
			out = append(out, t)
		}
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0; j-- {
			if dueBefore(out[j], out[j-1]) {
				out[j], out[j-1] = out[j-1], out[j]
			} else {
				break
			}
		}
	}
	return out
}

func dueBefore(a, b *core.Task) bool {
	da := parseDayStart(a.Due)
	db := parseDayStart(b.Due)
	if da.IsZero() || db.IsZero() {
		return da.IsZero() && !db.IsZero()
	}
	return da.Before(db)
}

// PlanExam generates kind:study tasks, one per exam topic, spread across the
// days before the exam. Each task gets an estimate. It is idempotent: if study
// tasks already exist as children of this exam (Parent == exam id), it does
// nothing and returns nil.
func PlanExam(s *store.Store, examID string, estMinutes int) ([]*core.Task, error) {
	exam, err := s.Resolve(examID)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(exam.Kind, core.KindExam) {
		return nil, fmt.Errorf("task %s is not an exam (kind %q)", exam.ID, exam.Kind)
	}
	// Idempotent: a previous plan already created child study tasks.
	for _, t := range s.Tasks() {
		if t.Parent == exam.ID {
			return nil, nil
		}
	}
	topics := exam.Tags
	if len(topics) == 0 {
		return nil, fmt.Errorf("exam %s has no topics — add topics with `mtm exam add --topics`", exam.ID)
	}
	dueDay, err := time.Parse("2006-01-02", exam.Due)
	if err != nil {
		return nil, fmt.Errorf("exam %s has an unparseable due date %q", exam.ID, exam.Due)
	}
	now := s.Clock.Now()
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	daysBefore := core.CalDays(today, dueDay)
	if daysBefore < 0 {
		daysBefore = 0
	}
	spacing := daysBefore / len(topics)
	if spacing < 1 {
		spacing = 1
	}
	var created []*core.Task
	for i, topic := range topics {
		offset := (i + 1) * spacing
		studyDay := dueDay.AddDate(0, 0, -offset)
		if !studyDay.After(today) && !studyDay.Equal(today) {
			studyDay = today
		}
		t := &core.Task{
			Title:           fmt.Sprintf("study %s for %s", topic, exam.Title),
			Project:         exam.Project,
			Status:          firstStatus(mustProject(s, exam.Project)),
			Due:             studyDay.Format("2006-01-02"),
			EstimateMinutes: estMinutes,
			Kind:            core.KindStudy,
			Tags:            []string{topic},
			Parent:          exam.ID,
		}
		if err := s.Create(t); err != nil {
			return created, err
		}
		created = append(created, t)
	}
	return created, nil
}

func mustProject(s *store.Store, name string) *core.Project {
	p, _ := s.GetProject(name)
	return p
}

// parseDayStart parses a due string (2006-01-02 or 2006-01-02T15:04) to midnight.
func parseDayStart(due string) time.Time {
	if due == "" {
		return time.Time{}
	}
	loc := time.Local
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, due, loc); err == nil {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
		}
	}
	return time.Time{}
}
