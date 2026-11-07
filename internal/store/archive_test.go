package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

func setupArchiveStore(t *testing.T) (*Store, string) {
	t.Helper()
	root := t.TempDir()
	cache := filepath.Join(t.TempDir(), "index.json")
	if err := os.MkdirAll(filepath.Join(root, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "archive"), 0o755); err != nil {
		t.Fatal(err)
	}
	s, err := Open(root, cache)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.SaveProject(core.DefaultProject("inbox")); err != nil {
		t.Fatal(err)
	}
	return s, root
}

func createDoneTask(t *testing.T, s *Store, title, completed string) *core.Task {
	t.Helper()
	tk := &core.Task{
		Title:     title,
		Project:   "inbox",
		Status:    "done",
		Priority:  "medium",
		Kind:      core.KindTask,
		Completed: completed,
		Body:      "notes for " + title,
	}
	if err := s.Create(tk); err != nil {
		t.Fatal(err)
	}
	return tk
}

func TestArchiveDoneMovesFilesIntact(t *testing.T) {
	s, root := setupArchiveStore(t)
	now := time.Now()
	old := now.Add(-40 * 24 * time.Hour).Format(core.TimeFormat)
	doneOld := createDoneTask(t, s, "old done task", old)
	doneNew := createDoneTask(t, s, "fresh done task", now.Format(core.TimeFormat))
	open := &core.Task{Title: "still open", Project: "inbox", Status: "todo", Priority: "high", Kind: core.KindTask}
	if err := s.Create(open); err != nil {
		t.Fatal(err)
	}
	path, err := s.Path(doneOld.ID)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	moved, err := s.ArchiveDone(now, 30*24*time.Hour, nil)
	if err != nil {
		t.Fatal(err)
	}
	if moved != 1 {
		t.Fatalf("moved = %d, want 1 (only the older-than-30d task)", moved)
	}

	// File kept intact at the archive path.
	archivedPath := filepath.Join(root, "archive", "inbox", filepath.Base(path))
	after, err := os.ReadFile(archivedPath)
	if err != nil {
		t.Fatalf("archived file missing: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("archived file content changed")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("original file still present")
	}

	// Index: old task gone, new done task still indexed.
	for _, tt := range s.Tasks() {
		if tt.ID == doneOld.ID {
			t.Fatal("archived task still in default index")
		}
	}
	found := false
	for _, tt := range s.Tasks() {
		if tt.ID == doneNew.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("fresh done task wrongly archived")
	}

	// Reopening with archive: archived task visible and marked.
	cache := filepath.Join(t.TempDir(), "index.json")
	s2, err := Open(root, cache, true)
	if err != nil {
		t.Fatal(err)
	}
	var got *core.Task
	for _, tt := range s2.Tasks() {
		if tt.ID == doneOld.ID {
			got = tt
		}
	}
	if got == nil {
		t.Fatal("archived task not visible with includeArchived")
	}
	if !got.Archived {
		t.Fatal("archived task not marked Archived")
	}
	archivedPathResolved, err := s2.Path(doneOld.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rel, err := filepath.Rel(root, archivedPathResolved); err != nil || rel != filepath.Join("archive", "inbox", filepath.Base(path)) {
		t.Fatalf("archived path = %q (err %v)", rel, err)
	}
}

func TestArchiveDoneAllWhenNoOlderThan(t *testing.T) {
	s, _ := setupArchiveStore(t)
	now := time.Now()
	createDoneTask(t, s, "done one", now.Format(core.TimeFormat))
	createDoneTask(t, s, "done two", now.Format(core.TimeFormat))
	moved, err := s.ArchiveDone(now, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if moved != 2 {
		t.Fatalf("moved = %d, want 2", moved)
	}
	if len(s.Tasks()) != 0 {
		t.Fatalf("index not empty after archiving all done tasks: %d", len(s.Tasks()))
	}
}

func TestArchiveDoneSkipsSubtaskTreeSafely(t *testing.T) {
	// Done child with not-done parent: the child moves, the parent stays.
	// (Documented behavior: printTasks renders the orphan child as a root.)
	s, _ := setupArchiveStore(t)
	now := time.Now()
	parent := &core.Task{Title: "parent open", Project: "inbox", Status: "todo", Priority: "low", Kind: core.KindTask}
	if err := s.Create(parent); err != nil {
		t.Fatal(err)
	}
	child := &core.Task{Title: "child done", Project: "inbox", Status: "done", Priority: "low", Kind: core.KindTask, Parent: parent.ID, Completed: now.Format(core.TimeFormat)}
	if err := s.Create(child); err != nil {
		t.Fatal(err)
	}
	moved, err := s.ArchiveDone(now, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if moved != 1 {
		t.Fatalf("moved = %d, want 1", moved)
	}
	remaining := map[string]bool{}
	for _, tt := range s.Tasks() {
		remaining[tt.ID] = true
	}
	if !remaining[parent.ID] {
		t.Fatal("not-done parent was archived")
	}
}
