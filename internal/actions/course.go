package actions

import (
	"fmt"
	"strings"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
)

// ListCourses returns all projects carrying a course block, ordered by name.
func ListCourses(s *store.Store) ([]*core.Project, error) {
	projects, warnings := s.ListProjects()
	var out []*core.Project
	for _, p := range projects {
		if p.IsCourse() {
			out = append(out, p)
		}
	}
	_ = warnings // callers surface warnings themselves when listing tasks
	return out, nil
}

// ResolveCourse maps a user-supplied course argument to a project name:
// exact match first, then case-insensitive, then a unique match on the leaf
// directory name. Unknown or ambiguous names are errors listing the known
// courses.
func ResolveCourse(s *store.Store, arg string) (string, error) {
	courses, err := ListCourses(s)
	if err != nil {
		return "", err
	}
	names := make([]string, 0, len(courses))
	for _, c := range courses {
		names = append(names, c.Name)
	}
	if len(courses) == 0 {
		return "", fmt.Errorf("no courses yet — mark one with `mtm course add`")
	}
	arg = strings.TrimSpace(arg)
	for _, c := range courses { // exact
		if c.Name == arg {
			return c.Name, nil
		}
	}
	for _, c := range courses { // case-insensitive full path
		if strings.EqualFold(c.Name, arg) {
			return c.Name, nil
		}
	}
	var leaf []string // unique leaf (dbms → college/dbms)
	for _, c := range courses {
		base := c.Name
		if i := strings.LastIndex(base, "/"); i >= 0 {
			base = base[i+1:]
		}
		if strings.EqualFold(base, arg) {
			leaf = append(leaf, c.Name)
		}
	}
	switch len(leaf) {
	case 1:
		return leaf[0], nil
	case 0:
		return "", fmt.Errorf("unknown course %q — known: %s", arg, strings.Join(names, ", "))
	default:
		return "", fmt.Errorf("course %q is ambiguous — use the full path: %s", arg, strings.Join(leaf, ", "))
	}
}

// CourseTasks returns all non-archived tasks belonging to a course project.
func CourseTasks(s *store.Store, name string) []*core.Task {
	var out []*core.Task
	for _, t := range s.Tasks() {
		if t.Project == name {
			out = append(out, t)
		}
	}
	return out
}

// CourseStats bundles the rollups every course surface needs (CLI show,
// grades, hours, TUI dashboard). Pure over the store snapshot.
type CourseStats struct {
	Project *core.Project
	Tasks   []*core.Task
	Overall core.Progress
	// OverallWeighted carries the assignment/exam-weighted fraction; only
	// meaningful when some task has a weight.
	OverallWeighted core.Progress
	HasWeights      bool
	ByKind          map[string]core.Progress
}

// CollectCourseStats computes the rollups for one course.
func CollectCourseStats(s *store.Store, name string) (*CourseStats, error) {
	p, err := s.GetProject(name)
	if err != nil {
		return nil, err
	}
	tasks := CourseTasks(s, name)
	isDone := func(t *core.Task) bool { return s.IsDoneStatus(t.Project, t.Status) }
	stats := &CourseStats{Project: p, Tasks: tasks, ByKind: map[string]core.Progress{}}
	stats.Overall = core.ProgressFraction(tasks, isDone, false)
	weighted := false
	for _, t := range tasks {
		if t.Weight != nil && *t.Weight > 0 {
			weighted = true
			break
		}
	}
	stats.HasWeights = weighted
	stats.OverallWeighted = core.ProgressFraction(tasks, isDone, true)
	for _, kind := range []string{core.KindAssignment, core.KindExam, core.KindStudy} {
		var subset []*core.Task
		for _, t := range tasks {
			if strings.EqualFold(t.Kind, kind) {
				subset = append(subset, t)
			}
		}
		if len(subset) > 0 {
			stats.ByKind[kind] = core.ProgressFraction(subset, isDone, false)
		}
	}
	return stats, nil
}
