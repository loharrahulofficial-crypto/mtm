package actions

// Export engine (Phase 5, item 2): one read model, three serializers.
// JSON carries the top-level schema_version (1) and is the round-trip
// format; Markdown is a readable report grouped by course; CSV is a flat
// task table with stable columns. Extensions for ideas and milestones
// land as reserved top-level keys in a later phase — adding them then is
// a one-line change here plus a schema_version bump.

import (
	"sort"
	"strings"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/dates"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
)

// ExportReport is the top-level export read model (schema_version 1).
type ExportReport struct {
	SchemaVersion int           `json:"schema_version"`
	GeneratedAt   string        `json:"generated_at"`
	Filters       ExportFilters `json:"filters"`

	Tasks   []ExportTask   `json:"tasks"`
	Courses []ExportCourse `json:"courses"`
	Exams   []ExportExam   `json:"exams"`
	Grades  []ExportGrade  `json:"grades"`
	Hours   []ExportHours  `json:"hours"`

	Totals ExportTotals `json:"totals"`

	// Reserved for Phase 6 ideas and milestones; empty in schema_version 1.
	Ideas      []any `json:"ideas"`
	Milestones []any `json:"milestones"`
}

type ExportFilters struct {
	Project string `json:"project,omitempty"`
	Course  string `json:"course,omitempty"`
	Since   string `json:"since,omitempty"`
}

type ExportTotals struct {
	Tasks        int `json:"tasks"`
	Completed    int `json:"completed"`
	HoursMinutes int `json:"hours_minutes"`
}

type ExportTask struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Project    string   `json:"project"`
	Status     string   `json:"status"`
	Priority   string   `json:"priority"`
	Kind       string   `json:"kind"`
	Tags       []string `json:"tags,omitempty"`
	Due        string   `json:"due,omitempty"`
	Reminder   string   `json:"reminder,omitempty"`
	Recurrence string   `json:"recurrence,omitempty"`
	Parent     string   `json:"parent,omitempty"`
	Created    string   `json:"created"`
	Updated    string   `json:"updated"`
	Completed  string   `json:"completed,omitempty"`

	EstimateMinutes   int      `json:"estimate_minutes,omitempty"`
	TimeLoggedMinutes int      `json:"time_logged_minutes,omitempty"`
	Weight            *float64 `json:"weight,omitempty"`
	Score             *float64 `json:"score,omitempty"`
	ScoreMax          *float64 `json:"score_max,omitempty"`
	Body              string   `json:"body,omitempty"`
}

type ExportCourse struct {
	Name             string   `json:"name"` // project path, e.g. college/dbms
	Code             string   `json:"code,omitempty"`
	Title            string   `json:"title,omitempty"`
	Semester         string   `json:"semester,omitempty"`
	Credits          int      `json:"credits,omitempty"`
	TargetGrade      string   `json:"target_grade,omitempty"`
	Color            string   `json:"color,omitempty"`
	Done             int      `json:"done"`
	Total            int      `json:"total"`
	HoursMinutes     int      `json:"hours_minutes"`
	GradeStandingPct *float64 `json:"grade_standing_pct,omitempty"`
	GradeCurrentPct  *float64 `json:"grade_current_pct,omitempty"`
}

type ExportExam struct {
	TaskID      string   `json:"task_id"`
	Title       string   `json:"title"`
	Course      string   `json:"course,omitempty"`
	Due         string   `json:"due,omitempty"`
	Weight      *float64 `json:"weight,omitempty"`
	TopicsDone  int      `json:"topics_done"`
	TopicsTotal int      `json:"topics_total"`
}

type ExportGrade struct {
	TaskID   string   `json:"task_id"`
	Title    string   `json:"title"`
	Kind     string   `json:"kind"`
	Course   string   `json:"course,omitempty"`
	Weight   *float64 `json:"weight,omitempty"`
	Score    *float64 `json:"score,omitempty"`
	ScoreMax *float64 `json:"score_max,omitempty"`
	Pct      float64  `json:"pct"`
	Scored   bool     `json:"scored"`
	Points   float64  `json:"points"` // weight × pct/100 (submitted work only)
}

type ExportHours struct {
	Course            string `json:"course"` // project path, or "other" for the rest
	LoggedMinutes     int    `json:"logged_minutes"`
	WeekLoggedMinutes int    `json:"week_logged_minutes"`
}

// ExportOptions filters the report.
type ExportOptions struct {
	// Project restricts the export to one project (tasks, and the matching
	// entry in Courses/Hours when that project is a course).
	Project string
	// Course restricts the export to one course.
	Course string
	// Since keeps only tasks touched on/after this time (any of
	// created/updated/completed/due). Zero means everything.
	Since time.Time
}

// CollectExport assembles the export read model.
func CollectExport(s *store.Store, now time.Time, opts ExportOptions) (*ExportReport, error) {
	rep := &ExportReport{
		SchemaVersion: 1,
		GeneratedAt:   now.Format(core.TimestampFormat),
		Filters:       ExportFilters{Project: opts.Project, Course: opts.Course},
		Tasks:         []ExportTask{},
		Courses:       []ExportCourse{},
		Exams:         []ExportExam{},
		Grades:        []ExportGrade{},
		Hours:         []ExportHours{},
		Ideas:         []any{},
		Milestones:    []any{},
	}
	if !opts.Since.IsZero() {
		rep.Filters.Since = opts.Since.Format("2006-01-02")
	}

	loc := now.Location()
	isDone := func(p, st string) bool { return s.IsDoneStatus(p, st) }
	courseNames, err := courseNameSet(s)
	if err != nil {
		return nil, err
	}

	// --- collect tasks (with project/course/since filters) ----------------
	resolvedCourse := ""
	if opts.Course != "" {
		name, err := ResolveCourse(s, opts.Course)
		if err != nil {
			return nil, err
		}
		resolvedCourse = name
	}
	courseFilter := map[string]bool{}
	if resolvedCourse != "" {
		courseFilter[resolvedCourse] = true
	} else if opts.Project != "" {
		courseFilter[opts.Project] = true
	}
	inScope := func(t *core.Task) bool {
		if len(courseFilter) > 0 && !courseFilter[t.Project] {
			return false
		}
		if !opts.Since.IsZero() {
			stamp := maxStamp(t.Created, t.Updated, t.Completed, t.Due)
			if ts, ok := dates.Parse(stamp, loc); ok {
				return !ts.Before(opts.Since)
			}
			// No parseable stamp at all: keep (conservative).
			return true
		}
		return true
	}

	var tasks []*core.Task
	for _, t := range s.Tasks() {
		if inScope(t) {
			tasks = append(tasks, t)
		}
	}
	byProject := map[string][]*core.Task{}
	for _, t := range tasks {
		byProject[t.Project] = append(byProject[t.Project], t)
	}
	for _, t := range tasks {
		rep.Tasks = append(rep.Tasks, taskToExport(t))
	}

	// --- courses -----------------------------------------------------------
	courses, err := ListCourses(s)
	if err != nil {
		return nil, err
	}
	courseHours := map[string]int{}
	courseWeek := map[string]int{}
	today := dayStamp(now)
	weekStart := today.AddDate(0, 0, -6)
	for _, c := range courses {
		name := c.Name
		if len(courseFilter) > 0 && !courseFilter[name] {
			continue
		}
		cExport := ExportCourse{Name: name, Color: c.Color}
		if c.Course != nil {
			cExport.Code = c.Course.Code
			cExport.Title = c.Course.Name
			cExport.Semester = c.Course.Semester
			cExport.Credits = c.Course.Credits
			cExport.TargetGrade = c.Course.TargetGrade
		}
		cTasks, ok := byProject[name]
		if !ok {
			// No tasks in scope — still list the course with zeroes.
			cTasks = nil
		}
		for _, t := range cTasks {
			cExport.Total++
			if isDone(t.Project, t.Status) {
				cExport.Done++
			}
			cExport.HoursMinutes += t.TimeLoggedMinutes
			courseHours[name] += t.TimeLoggedMinutes
			if weekMinutes := weekLoggedMinutes(t, isDone, loc, weekStart); weekMinutes > 0 {
				courseWeek[name] += weekMinutes
			}
		}
		// Grade rollup for the course (needs the full task set for sane
		// weights — use the scoped set so filters stay honest).
		if items := gradeItemsFromTasks(cTasks, s); len(items) > 0 {
			grades := computeCourseGrades(items, cExport.TargetGrade)
			cExport.GradeStandingPct = &grades.StandingPct
			cExport.GradeCurrentPct = &grades.CurrentPct
		}
		rep.Courses = append(rep.Courses, cExport)
	}

	// --- exams -------------------------------------------------------------
	for _, t := range tasks {
		if !strings.EqualFold(t.Kind, core.KindExam) {
			continue
		}
		e := ExportExam{
			TaskID: t.ID,
			Title:  t.Title,
			Due:    t.Due,
			Weight: t.Weight,
		}
		if courseNames[t.Project] {
			e.Course = t.Project
		}
		// Topics = direct subtasks; done = subtasks that are done.
		for _, child := range tasks {
			if child.Parent != t.ID {
				continue
			}
			e.TopicsTotal++
			if isDone(child.Project, child.Status) {
				e.TopicsDone++
			}
		}
		rep.Exams = append(rep.Exams, e)
	}

	// --- grades --------------------------------------------------------------
	for _, t := range tasks {
		if !strings.EqualFold(t.Kind, core.KindAssignment) && !strings.EqualFold(t.Kind, core.KindExam) {
			continue
		}
		g := ExportGrade{
			TaskID:   t.ID,
			Title:    t.Title,
			Kind:     t.Kind,
			Weight:   t.Weight,
			Score:    t.Score,
			ScoreMax: t.ScoreMax,
		}
		if courseNames[t.Project] {
			g.Course = t.Project
		}
		max := 100.0
		if t.ScoreMax != nil && *t.ScoreMax > 0 {
			max = *t.ScoreMax
		}
		if t.Score != nil {
			pct := *t.Score / max * 100
			if pct < 0 {
				pct = 0
			}
			if pct > 100 {
				pct = 100
			}
			g.Pct = pct
			g.Scored = true
			if t.Weight != nil {
				g.Points = *t.Weight * pct / 100
			}
		}
		rep.Grades = append(rep.Grades, g)
	}

	// --- hours ---------------------------------------------------------------
	otherHours, otherWeek := 0, 0
	for project, pTasks := range byProject {
		if courseNames[project] {
			continue
		}
		for _, t := range pTasks {
			otherHours += t.TimeLoggedMinutes
			if wm := weekLoggedMinutes(t, isDone, loc, weekStart); wm > 0 {
				otherWeek += wm
			}
		}
	}
	for _, c := range rep.Courses {
		rep.Hours = append(rep.Hours, ExportHours{
			Course:            c.Name,
			LoggedMinutes:     courseHours[c.Name],
			WeekLoggedMinutes: courseWeek[c.Name],
		})
	}
	rep.Hours = append(rep.Hours, ExportHours{Course: "other", LoggedMinutes: otherHours, WeekLoggedMinutes: otherWeek})

	// totals
	for _, t := range tasks {
		rep.Totals.Tasks++
		if isDone(t.Project, t.Status) {
			rep.Totals.Completed++
		}
		rep.Totals.HoursMinutes += t.TimeLoggedMinutes
	}

	sort.SliceStable(rep.Courses, func(i, j int) bool { return rep.Courses[i].Name < rep.Courses[j].Name })
	sort.SliceStable(rep.Exams, func(i, j int) bool {
		if rep.Exams[i].Due == rep.Exams[j].Due {
			return rep.Exams[i].Title < rep.Exams[j].Title
		}
		return rep.Exams[i].Due < rep.Exams[j].Due
	})
	sort.SliceStable(rep.Grades, func(i, j int) bool { return rep.Grades[i].Title < rep.Grades[j].Title })
	return rep, nil
}

func taskToExport(t *core.Task) ExportTask {
	return ExportTask{
		ID:                t.ID,
		Title:             t.Title,
		Project:           t.Project,
		Status:            t.Status,
		Priority:          t.Priority,
		Kind:              t.Kind,
		Tags:              t.Tags,
		Due:               t.Due,
		Reminder:          t.Reminder,
		Recurrence:        t.Recurrence,
		Parent:            t.Parent,
		Created:           t.Created,
		Updated:           t.Updated,
		Completed:         t.Completed,
		EstimateMinutes:   t.EstimateMinutes,
		TimeLoggedMinutes: t.TimeLoggedMinutes,
		Weight:            t.Weight,
		Score:             t.Score,
		ScoreMax:          t.ScoreMax,
		Body:              t.Body,
	}
}

// courseNameSet returns the set of projects that are courses (project path → true).
func courseNameSet(s *store.Store) (map[string]bool, error) {
	courses, err := ListCourses(s)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, c := range courses {
		out[c.Name] = true
	}
	return out, nil
}

// weekLoggedMinutes attributes a task's logged time to the trailing window
// using the same rule as hours/stats: completion day (updated fallback),
// or the updated day for work in progress.
func weekLoggedMinutes(t *core.Task, isDone func(p, st string) bool, loc *time.Location, weekStart time.Time) int {
	if t.TimeLoggedMinutes <= 0 {
		return 0
	}
	day, ok := loggedDay(t, isDone, loc)
	if !ok {
		return 0
	}
	if day.Before(weekStart) {
		return 0
	}
	return t.TimeLoggedMinutes
}

func maxStamp(a, b, c, d string) string {
	max := ""
	for _, s := range []string{a, b, c, d} {
		if s > max {
			max = s
		}
	}
	return max
}

// gradeItemsFromTasks builds grade rollup items from a scoped task set.
func gradeItemsFromTasks(tasks []*core.Task, s *store.Store) []core.GradeItem {
	var items []core.GradeItem
	for _, t := range tasks {
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

// computeCourseGrades is the scoped variant of ComputeCourseGrades: the
// target comes from the course's target_grade (or none when unset).
func computeCourseGrades(items []core.GradeItem, targetRaw string) core.CourseGrades {
	target := 0.0
	hasTarget := false
	if strings.TrimSpace(targetRaw) != "" {
		if pct, err := core.GradeToPercent(targetRaw); err == nil {
			target, hasTarget = pct, true
		}
	}
	return core.ComputeCourseGrades(items, target, hasTarget)
}
