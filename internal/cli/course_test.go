package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
)

// captureStdout runs fn with os.Stdout redirected to a pipe and returns
// everything it printed.
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	fnErr := fn()
	_ = w.Close()
	os.Stdout = orig
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String(), fnErr
}

func runCourseCmd(t *testing.T, args ...string) error {
	t.Helper()
	cmd := newCourseCmd()
	cmd.SetOut(os.Stdout)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func addCourse(t *testing.T, args ...string) error {
	t.Helper()
	full := append([]string{"add"}, args...)
	return runCourseCmd(t, full...)
}

func TestCourseAddListShow(t *testing.T) {
	gitEnv(t)
	sandbox(t)
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}

	if err := addCourse(t, "college/dbms", "--code", "dbms", "--name", "Database Systems", "--semester", "2026-Fall", "--credits", "4", "--target-grade", "A", "--exam-date", "2026-12-01"); err != nil {
		t.Fatalf("course add: %v", err)
	}
	// Missing flags refuse.
	if err := addCourse(t, "college/os", "--code", "OS"); err == nil {
		t.Fatal("add without --name accepted")
	}
	// Duplicate project refuses.
	if err := addCourse(t, "college/dbms", "--code", "DBMS", "--name", "Dup"); err == nil {
		t.Fatal("duplicate course add accepted")
	}

	// project.yaml carries the course block with the normalized code.
	data, err := os.ReadFile(filepath.Join(os.Getenv("XDG_DATA_HOME"), "mtm", "projects", "college", "dbms", "project.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	p := &core.Project{}
	if err := yaml.Unmarshal(data, p); err != nil {
		t.Fatal(err)
	}
	if p.Course == nil {
		t.Fatal("project.yaml missing course block")
	}
	if p.Course.Code != "DBMS" || p.Course.Name != "Database Systems" || p.Course.Credits != 4 {
		t.Fatalf("course block wrong: %+v", p.Course)
	}
	if got := p.Course.ExamDates; len(got) != 1 || got[0] != "2026-12-01" {
		t.Fatalf("exam dates wrong: %v", got)
	}

	listOut, err := captureStdout(t, func() error { return runCourseCmd(t, "list") })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"DBMS", "Database Systems", "2026-Fall", "target"} {
		if !strings.Contains(listOut, want) {
			t.Errorf("list output missing %q:\n%s", want, listOut)
		}
	}

	showOut, err := captureStdout(t, func() error { return runCourseCmd(t, "show", "dbms") }) // leaf resolution
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Database Systems", "2026-Fall", "credits     4", "target      A (90%)", "exam dates  2026-12-01", "overall"} {
		if !strings.Contains(showOut, want) {
			t.Errorf("show output missing %q:\n%s", want, showOut)
		}
	}

	// JSON listing round-trips.
	jsonOut, err := captureStdout(t, func() error { return runCourseCmd(t, "list", "--json") })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(jsonOut, `"code": "DBMS"`) {
		t.Fatalf("json list missing course:\n%s", jsonOut)
	}
}

func TestCourseShowIncludesProgressBars(t *testing.T) {
	gitEnv(t)
	sandbox(t)
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}
	if err := addCourse(t, "college/os", "--code", "OS", "--name", "Operating Systems"); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(os.Getenv("XDG_DATA_HOME"), "mtm")
	s, err := store.Open(root, filepath.Join(t.TempDir(), "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	first := "todo"
	p, err := s.GetProject("college/os")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Statuses) > 0 {
		first = p.Statuses[0].Name
	}
	w := 30.0
	tasks := []*core.Task{
		{Title: "read ch.1", Project: "college/os", Status: "done", Kind: core.KindAssignment, Weight: &w, Completed: "2026-11-06T10:00", Updated: "2026-11-06T10:00"},
		{Title: "read ch.2", Project: "college/os", Status: first, Kind: core.KindAssignment, Weight: &w, Created: "2026-11-07T10:00"},
	}
	for _, tk := range tasks {
		if err := s.Create(tk); err != nil {
			t.Fatal(err)
		}
	}
	_ = s.Close()

	out, err := captureStdout(t, func() error { return runCourseCmd(t, "show", "college/os") })
	if err != nil {
		t.Fatal(err)
	}
	// 1 of 2 done → the bar must report 1/2; weighted equally (both 30).
	if !strings.Contains(out, "(1/2)") {
		t.Errorf("show output missing fraction (1/2):\n%s", out)
	}
	if !strings.Contains(out, "weighted") {
		t.Errorf("show output missing weighted bar:\n%s", out)
	}
	if !strings.Contains(out, "assignments") {
		t.Errorf("show output missing assignments bar:\n%s", out)
	}
}

func TestCourseResolveErrors(t *testing.T) {
	gitEnv(t)
	sandbox(t)
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}
	// No courses at all → clear message.
	if _, err := captureStdout(t, func() error { return runCourseCmd(t, "show", "dbms") }); err == nil {
		t.Fatal("show on empty repo succeeded")
	} else if !strings.Contains(err.Error(), "no courses") {
		t.Fatalf("wrong error: %v", err)
	}
	if err := addCourse(t, "college/dbms", "--code", "DBMS", "--name", "Database Systems"); err != nil {
		t.Fatal(err)
	}
	_, err := captureStdout(t, func() error { return runCourseCmd(t, "show", "nope") })
	if err == nil || !strings.Contains(err.Error(), "unknown course") {
		t.Fatalf("unknown course error wrong: %v", err)
	}
}
