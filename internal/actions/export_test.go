package actions

import (
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

// exportFixture builds a store with two courses and graded work.
func TestCollectExportShape(t *testing.T) {
	s := openActionStore(t, "college/dbms")
	if err := s.SaveProject(core.DefaultProject("inbox")); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveProject(courseProject(t, "college/os", "Operating Systems")); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 8, 14, 0, 0, 0, time.Local)
	stamp := now.Format(core.TimestampFormat)
	w := 30.0
	score := 27.0
	max := 30.0

	mk := func(task *core.Task) {
		t.Helper()
		if err := s.Create(task); err != nil {
			t.Fatal(err)
		}
	}
	mk(&core.Task{Title: "HW1 — normalization exercises", Project: "college/dbms", Status: "done", Kind: core.KindAssignment,
		Due: now.Format("2006-01-02"), Completed: stamp, Created: stamp, Updated: stamp,
		Weight: &w, Score: &score, ScoreMax: &max, EstimateMinutes: 60, TimeLoggedMinutes: 90})
	mk(&core.Task{Title: "HW2 — relational algebra", Project: "college/dbms", Status: "todo", Kind: core.KindAssignment,
		Created: stamp, Updated: stamp, EstimateMinutes: 90})
	mk(&core.Task{Title: "DBMS midterm — relational algebra + SQL", Project: "college/dbms", Status: "todo", Kind: core.KindExam,
		Due: now.AddDate(0, 0, 6).Format("2006-01-02") + "T09:00", Created: stamp, Updated: stamp,
		Weight: &w})
	examID := findTaskIDByTitle(s, "DBMS midterm — relational algebra + SQL")
	mk(&core.Task{Title: "Revise ER diagrams", Project: "college/dbms", Status: "done", Kind: core.KindStudy,
		Created: stamp, Updated: stamp})
	mk(&core.Task{Title: "Revise SQL joins", Project: "college/dbms", Status: "done", Kind: core.KindStudy,
		Parent: examID, Completed: stamp, Created: stamp, Updated: stamp})
	mk(&core.Task{Title: "Revise transactions", Project: "college/dbms", Status: "todo", Kind: core.KindStudy,
		Parent: examID, Created: stamp, Updated: stamp})
	mk(&core.Task{Title: "OS lab — page replacement", Project: "college/os", Status: "todo", Kind: core.KindAssignment,
		Created: stamp, Updated: stamp})
	mk(&core.Task{Title: "Buy groceries", Project: "inbox", Status: "todo", Kind: core.KindTask,
		Created: stamp, Updated: stamp, TimeLoggedMinutes: 30})

	rep, err := CollectExport(s, now, ExportOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// Top-level shape.
	if rep.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", rep.SchemaVersion)
	}
	if len(rep.Ideas) != 0 || len(rep.Milestones) != 0 {
		t.Errorf("ideas/milestones must be present and empty in schema_version 1, got %d/%d", len(rep.Ideas), len(rep.Milestones))
	}
	if rep.GeneratedAt == "" || rep.Filters.Since != "" {
		t.Errorf("GeneratedAt/Filters.Since unexpected: %+v", rep.Filters)
	}

	// Courses.
	byCourse := map[string]ExportCourse{}
	for _, c := range rep.Courses {
		byCourse[c.Name] = c
	}
	d := byCourse["college/dbms"]
	if d.Done == 0 || d.Total == 0 {
		t.Errorf("dbms course missing from export: %+v", d)
	}
	if d.HoursMinutes != 90 {
		t.Errorf("dbms hours = %d, want 90", d.HoursMinutes)
	}
	if d.GradeStandingPct == nil {
		t.Error("dbms grade rollup missing (has a scored weighted item)")
	} else if got, want := *d.GradeStandingPct, 45.0; got < want-0.01 || got > want+0.01 {
		t.Errorf("dbms standing = %v, want %v", got, want)
	}
	if byCourse["college/os"].Total != 1 {
		t.Errorf("os course total = %d, want 1", byCourse["college/os"].Total)
	}

	// Exams with topics.
	if len(rep.Exams) != 1 {
		t.Fatalf("exams = %d, want 1", len(rep.Exams))
	}
	e := rep.Exams[0]
	if e.Course != "college/dbms" {
		t.Errorf("exam course = %q, want college/dbms", e.Course)
	}
	if e.TopicsTotal != 2 || e.TopicsDone != 1 {
		t.Errorf("exam topics = %d/%d, want 1/2", e.TopicsDone, e.TopicsTotal)
	}

	// Grades: four assignment/exam items, one scored.
	if len(rep.Grades) != 4 {
		t.Fatalf("grades = %d, want 4", len(rep.Grades))
	}
	scored := 0
	for _, g := range rep.Grades {
		if g.Scored {
			scored++
			if g.Kind == core.KindAssignment && (g.Pct < 89.9 || g.Pct > 90.1) {
				t.Errorf("scored pct = %v, want 90", g.Pct)
			}
			if g.Points != 27 {
				t.Errorf("points = %v, want 27", g.Points)
			}
		}
	}
	if scored != 1 {
		t.Errorf("scored = %d, want 1", scored)
	}

	// Hours rollup.
	var dbmsHours, otherHours *ExportHours
	for i := range rep.Hours {
		switch rep.Hours[i].Course {
		case "college/dbms":
			dbmsHours = &rep.Hours[i]
		case "other":
			otherHours = &rep.Hours[i]
		}
	}
	if dbmsHours == nil || dbmsHours.LoggedMinutes != 90 {
		t.Errorf("dbms hours entry = %+v, want 90", dbmsHours)
	}
	if otherHours == nil || otherHours.LoggedMinutes != 30 {
		t.Errorf("other hours entry = %+v, want 30", otherHours)
	}

	// Totals.
	if rep.Totals.Tasks != len(rep.Tasks) {
		t.Errorf("totals.tasks = %d, want %d", rep.Totals.Tasks, len(rep.Tasks))
	}
	if rep.Totals.HoursMinutes != 120 {
		t.Errorf("totals.hours = %d, want 120", rep.Totals.HoursMinutes)
	}
}

func TestExportRoundTripJSON(t *testing.T) {
	s := openActionStore(t, "college/dbms")
	now := time.Date(2026, 11, 8, 14, 0, 0, 0, time.Local)
	stamp := now.Format(core.TimestampFormat)
	w := 30.0
	task := &core.Task{Title: "HW1", Project: "college/dbms", Status: "done", Kind: core.KindAssignment,
		Completed: stamp, Created: stamp, Updated: stamp, Weight: &w, TimeLoggedMinutes: 45}
	if err := s.Create(task); err != nil {
		t.Fatal(err)
	}
	rep, err := CollectExport(s, now, ExportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	data, err := ExportJSON(rep)
	if err != nil {
		t.Fatal(err)
	}
	// Structural validation against a small schema (types + required keys).
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	schema := map[string]string{
		"schema_version": "number",
		"generated_at":   "string",
		"filters":        "object",
		"tasks":          "array",
		"courses":        "array",
		"exams":          "array",
		"grades":         "array",
		"hours":          "array",
		"totals":         "object",
		"ideas":          "array",
		"milestones":     "array",
	}
	for key, wantType := range schema {
		value, present := doc[key]
		if !present {
			t.Errorf("missing top-level key %q", key)
			continue
		}
		if jsonTypeOf(value) != wantType {
			t.Errorf("key %q: want %s, got %s", key, wantType, jsonTypeOf(value))
		}
	}
	// Task entries carry the documented required fields with the right types.
	taskDoc, _ := doc["tasks"].([]any)
	if len(taskDoc) == 0 {
		t.Fatal("no tasks in export")
	}
	for _, raw := range taskDoc {
		m, _ := raw.(map[string]any)
		for _, key := range []string{"id", "title", "project", "status", "priority", "kind", "created", "updated"} {
			if _, ok := m[key].(string); !ok {
				t.Errorf("task missing string key %q: %v", key, m)
			}
		}
	}
	// And the model round-trips through ExportReport.
	var back ExportReport
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back.SchemaVersion != 1 || len(back.Tasks) != len(rep.Tasks) {
		t.Errorf("round trip mismatch")
	}
}

func TestExportCSVStableColumns(t *testing.T) {
	s := openActionStore(t, "college/dbms")
	now := time.Date(2026, 11, 8, 14, 0, 0, 0, time.Local)
	stamp := now.Format(core.TimestampFormat)
	for i := 0; i < 3; i++ {
		task := &core.Task{Title: "task", Project: "college/dbms", Status: "todo", Kind: core.KindTask,
			Created: stamp, Updated: stamp}
		if err := s.Create(task); err != nil {
			t.Fatal(err)
		}
	}
	rep, err := CollectExport(s, now, ExportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	data, err := ExportCSV(rep)
	if err != nil {
		t.Fatal(err)
	}
	r := csv.NewReader(strings.NewReader(string(data)))
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatalf("rows = %d, want 4 (header + 3)", len(rows))
	}
	want := strings.Join(CSVColumns, ",")
	if strings.Join(rows[0], ",") != want {
		t.Errorf("header = %q, want %q", strings.Join(rows[0], ","), want)
	}
	for i, row := range rows[1:] {
		if len(row) != len(CSVColumns) {
			t.Errorf("row %d has %d fields, want %d", i, len(row), len(CSVColumns))
		}
	}
}

func TestExportMarkdownGroupsByCourse(t *testing.T) {
	s := openActionStore(t, "college/dbms")
	if err := s.SaveProject(core.DefaultProject("inbox")); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 8, 14, 0, 0, 0, time.Local)
	stamp := now.Format(core.TimestampFormat)
	mk := func(task *core.Task) {
		t.Helper()
		if err := s.Create(task); err != nil {
			t.Fatal(err)
		}
	}
	_ = mk
	w := 30.0
	score := 27.0
	max := 30.0
	mk(&core.Task{Title: "HW1 — normalization", Project: "college/dbms", Status: "done", Kind: core.KindAssignment,
		Completed: stamp, Created: stamp, Updated: stamp, Weight: &w, Score: &score, ScoreMax: &max})
	mk(&core.Task{Title: "DBMS midterm", Project: "college/dbms", Status: "todo", Kind: core.KindExam,
		Weight: &w, Due: now.AddDate(0, 0, 6).Format("2006-01-02") + "T09:00", Created: stamp, Updated: stamp})
	mk(&core.Task{Title: "Buy groceries", Project: "inbox", Status: "todo", Kind: core.KindTask,
		Created: stamp, Updated: stamp})
	rep, err := CollectExport(s, now, ExportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	md := ExportMarkdown(rep, now)
	for _, want := range []string{
		"# mtm export",
		"## college/dbms",
		"### exams",
		"## inbox",
		"HW1 — normalization",
		"score 90%",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
	}
}

func TestExportFilters(t *testing.T) {
	s := openActionStore(t, "college/dbms")
	if err := s.SaveProject(core.DefaultProject("inbox")); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveProject(courseProject(t, "college/os", "Operating Systems")); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 8, 14, 0, 0, 0, time.Local)
	stamp := now.Format(core.TimestampFormat)
	mk := func(title, project string) {
		t.Helper()
		task := &core.Task{Title: title, Project: project, Status: "todo", Kind: core.KindTask,
			Created: stamp, Updated: stamp}
		if err := s.Create(task); err != nil {
			t.Fatal(err)
		}
	}
	mk("dbms task", "college/dbms")
	mk("os task", "college/os")
	mk("inbox task", "inbox")

	rep, err := CollectExport(s, now, ExportOptions{Project: "college/os"})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Totals.Tasks != 1 || rep.Tasks[0].Project != "college/os" {
		t.Errorf("--project filter: %d tasks, want 1 in college/os", rep.Totals.Tasks)
	}

	rep, err = CollectExport(s, now, ExportOptions{Since: now.AddDate(0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Totals.Tasks != 0 {
		t.Errorf("--since future: %d tasks, want 0", rep.Totals.Tasks)
	}

	rep, err = CollectExport(s, now, ExportOptions{Since: now})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Totals.Tasks != 3 {
		t.Errorf("--since today: %d tasks, want 3", rep.Totals.Tasks)
	}
}

func courseProject(t *testing.T, name, full string) *core.Project {
	t.Helper()
	return &core.Project{Name: name, Statuses: []core.ProjectStatus{
		{Name: "todo"}, {Name: "done", Done: true},
	}, Course: &core.Course{Code: strings.ToUpper(name), Name: full, Semester: "2026-odd", Credits: 4, TargetGrade: "A"}}
}

func findTaskIDByTitle(s interface {
	Tasks() []*core.Task
}, title string) string {
	for _, t := range s.Tasks() {
		if t.Title == title {
			return t.ID
		}
	}
	return ""
}

func jsonTypeOf(v any) string {
	switch v.(type) {
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "bool"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case nil:
		return "null"
	default:
		return "unknown"
	}
}
