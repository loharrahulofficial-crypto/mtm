package actions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/clock"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
)

// openActionStore creates a temp data repo with one course project
// (college/dbms) and returns an opened store over it.
func openActionStore(t *testing.T, courseName string) *store.Store {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(t.TempDir(), "index.json")
	s, err := store.Open(root, cache)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// Pin the clock: store.Create stamps Updated from it, and the tests
	// compare against stamps frozen at 2026-11-08T14:00.
	s.Clock = clock.Fixed(time.Date(2026, 11, 8, 14, 0, 0, 0, time.Local))
	p := &core.Project{Name: courseName, Statuses: []core.ProjectStatus{
		{Name: "todo"}, {Name: "doing"}, {Name: "done", Done: true},
	}, Course: &core.Course{Code: "DBMS", Name: "Database Systems"}}
	if err := s.SaveProject(p); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
	return s
}

func TestAddExam(t *testing.T) {
	s := openActionStore(t, "college/dbms")
	w := 30.0
	task, err := AddExam(s, "college/dbms", "DBMS midterm", "2026-12-01", &w, []string{"ER", "SQL", "normalization"})
	if err != nil {
		t.Fatalf("AddExam: %v", err)
	}
	if !strings.EqualFold(task.Kind, core.KindExam) {
		t.Fatalf("kind = %q, want exam", task.Kind)
	}
	if task.Project != "college/dbms" || task.Due != "2026-12-01" {
		t.Fatalf("project/due wrong: %+v", task)
	}
	if task.Weight == nil || *task.Weight != 30 {
		t.Fatalf("weight = %v, want 30", task.Weight)
	}
	if len(task.Tags) != 3 {
		t.Fatalf("topics/tags = %v, want 3", task.Tags)
	}
	// Re-opening the store sees the task.
	s2, err := store.Open(s.Root, filepath.Join(t.TempDir(), "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := s2.Resolve(task.ID)
	if err != nil {
		t.Fatalf("re-resolve: %v", err)
	}
	if got.Due != "2026-12-01" || len(got.Tags) != 3 {
		t.Fatalf("persisted wrong: %+v", got)
	}
}

func TestAddExamRejectsUnknownCourse(t *testing.T) {
	s := openActionStore(t, "college/dbms")
	if _, err := AddExam(s, "nope", "x", "2026-12-01", nil, nil); err == nil {
		t.Fatal("unknown course accepted")
	}
}

func TestAddExamRejectsBadDate(t *testing.T) {
	s := openActionStore(t, "college/dbms")
	if _, err := AddExam(s, "college/dbms", "x", "01/12/2026", nil, nil); err == nil {
		t.Fatal("bad date accepted")
	}
}

func TestAddExamDedupesTopics(t *testing.T) {
	s := openActionStore(t, "college/dbms")
	task, err := AddExam(s, "college/dbms", "x", "2026-12-01", nil, []string{"ER", "er", "SQL", "SQL"})
	if err != nil {
		t.Fatal(err)
	}
	if len(task.Tags) != 2 {
		t.Fatalf("topics = %v, want 2 deduped", task.Tags)
	}
}

func TestPlanExamCreatesOneStudyPerTopic(t *testing.T) {
	s := openActionStore(t, "college/dbms")
	w := 30.0
	exam, err := AddExam(s, "college/dbms", "OS final", "2026-12-10", &w, []string{"threads", "sync", "memory"})
	if err != nil {
		t.Fatal(err)
	}
	created, err := PlanExam(s, exam.ID, 45)
	if err != nil {
		t.Fatalf("PlanExam: %v", err)
	}
	if len(created) != 3 {
		t.Fatalf("created %d study tasks, want 3", len(created))
	}
	for _, task := range created {
		if !strings.EqualFold(task.Kind, core.KindStudy) {
			t.Fatalf("kind = %q, want study", task.Kind)
		}
		if task.Parent != exam.ID {
			t.Fatalf("parent = %q, want exam %q", task.Parent, exam.ID)
		}
		if task.EstimateMinutes != 45 {
			t.Fatalf("estimate = %d, want 45", task.EstimateMinutes)
		}
		if task.Project != "college/dbms" {
			t.Fatalf("project = %q", task.Project)
		}
	}
	// Due dates spread before the exam, none after it.
	examDay := parseDayStart("2026-12-10")
	for _, task := range created {
		d := parseDayStart(task.Due)
		if d.IsZero() || d.After(examDay) {
			t.Fatalf("study due %s not on/before exam", task.Due)
		}
	}
}

func TestPlanExamIsIdempotent(t *testing.T) {
	s := openActionStore(t, "college/dbms")
	exam, err := AddExam(s, "college/dbms", "OS final", "2026-12-10", nil, []string{"threads"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PlanExam(s, exam.ID, 45); err != nil {
		t.Fatal(err)
	}
	if _, err := PlanExam(s, exam.ID, 45); err != nil {
		t.Fatal("second PlanExam errored")
	}
	// Only one study task exists.
	n := 0
	for _, task := range s.Tasks() {
		if task.Parent == exam.ID {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("expected exactly 1 child study task, got %d", n)
	}
}

func TestPlanExamRequiresExamAndTopics(t *testing.T) {
	s := openActionStore(t, "college/dbms")
	// Not an exam.
	plain, err := AddExam(s, "college/dbms", "x", "2026-12-10", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	plain.Kind = core.KindTask
	if err := s.Update(plain); err != nil {
		t.Fatal(err)
	}
	if _, err := PlanExam(s, plain.ID, 45); err == nil {
		t.Fatal("plan on non-exam accepted")
	}
	// Exam with no topics.
	noTopics, err := AddExam(s, "college/dbms", "x", "2026-12-10", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PlanExam(s, noTopics.ID, 45); err == nil {
		t.Fatal("plan on exam without topics accepted")
	}
}

// ListExams orders nearest-due first and skips done exams.
func TestListExams(t *testing.T) {
	s := openActionStore(t, "college/dbms")
	AddExam(s, "college/dbms", "later", "2026-12-20", nil, nil)
	AddExam(s, "college/dbms", "earlier", "2026-12-02", nil, nil)
	AddExam(s, "college/dbms", "done exam", "2026-12-01", nil, nil)
	// Mark the "done exam" done.
	for _, task := range s.Tasks() {
		if task.Title == "done exam" {
			task.Status = "done"
			if err := s.Update(task); err != nil {
				t.Fatal(err)
			}
		}
	}
	exams := ListExams(s)
	if len(exams) != 2 {
		t.Fatalf("got %d exams, want 2 (done excluded)", len(exams))
	}
	if exams[0].Title != "earlier" || exams[1].Title != "later" {
		t.Fatalf("wrong order: %v", exams)
	}
}
