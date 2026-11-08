package cli

import (
	"strings"
	"testing"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

func f64ptr(f float64) *float64 { return &f }

func TestListProgressLines(t *testing.T) {
	done := func(p, st string) bool { return st == "done" }
	tasks := []*core.Task{
		{ID: "1", Project: "dbms", Status: "done", Kind: core.KindTask},
		{ID: "2", Project: "dbms", Status: "todo", Kind: core.KindAssignment, Weight: f64ptr(30)},
		{ID: "3", Project: "dbms", Status: "todo", Kind: core.KindExam, Weight: f64ptr(40)},
		{ID: "4", Project: "os", Status: "todo", Kind: core.KindTask},
	}
	// Plain bars: subtasks included, everything counts.
	lines := listProgressLines(tasks, tasks, done, false)
	if len(lines) != 2 {
		t.Fatalf("lines = %v", lines)
	}
	if !strings.HasPrefix(lines[0], "  dbms") || !strings.Contains(lines[0], "33%") {
		t.Errorf("dbms line = %q, want 33%%", lines[0])
	}
	if !strings.HasPrefix(lines[1], "  os") || !strings.Contains(lines[1], "0%") {
		t.Errorf("os line = %q, want 0%%", lines[1])
	}
	// Weighted bars: only weighted assignment/exam tasks count.
	lines = listProgressLines(tasks, tasks, done, true)
	if !strings.Contains(lines[0], "weighted") || !strings.Contains(lines[0], "0%") {
		t.Errorf("dbms weighted line = %q, want 0%% weighted", lines[0])
	}
	if !strings.Contains(lines[1], "no weighted") {
		t.Errorf("os weighted line = %q, want no-weighted note", lines[1])
	}
	// Done weighted work shows up.
	tasks[1].Status = "done"
	lines = listProgressLines(tasks, tasks, done, true)
	if !strings.Contains(lines[0], "43%") { // 30 of 70
		t.Errorf("dbms weighted line after grading = %q, want 43%%", lines[0])
	}
	// Only projects present in the filtered output appear.
	lines = listProgressLines(tasks, tasks[:3], done, false)
	if len(lines) != 1 {
		t.Fatalf("filtered lines = %v", lines)
	}
}
