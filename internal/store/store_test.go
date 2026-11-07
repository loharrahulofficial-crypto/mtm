package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

func TestParseSerializeRoundtrip(t *testing.T) {
	in := []byte("---\nid: a1b2c3\ntitle: Test task\nproject: inbox\nstatus: todo\npriority: high\ntags:\n    - exam\nkind: exam\ncreated: 2026-11-07T10:00\nupdated: 2026-11-07T10:00\nweight: 15\n---\nSome notes here.\n\nMore detail.\n")
	task, err := ParseTask(in)
	if err != nil {
		t.Fatalf("ParseTask: %v", err)
	}
	if task.ID != "a1b2c3" || task.Title != "Test task" || task.Weight == nil || *task.Weight != 15 {
		t.Fatalf("unexpected parse: %+v", task)
	}
	if !strings.Contains(task.Body, "More detail.") {
		t.Fatalf("body lost: %q", task.Body)
	}
	out, err := SerializeTask(task)
	if err != nil {
		t.Fatalf("SerializeTask: %v", err)
	}
	again, err := ParseTask(out)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if again.ID != task.ID || again.Body != task.Body {
		t.Fatalf("roundtrip mismatch: %+v vs %+v", task, again)
	}
}

func TestParseMissingFence(t *testing.T) {
	if _, err := ParseTask([]byte("id: x\n")); err == nil {
		t.Fatal("expected error for missing fence")
	}
	if _, err := ParseTask([]byte("---\nid: x\n")); err == nil {
		t.Fatal("expected error for unterminated frontmatter")
	}
}

func openTestStore(t *testing.T) (*Store, string) {
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
	return s, root
}

func TestOpenUninitialized(t *testing.T) {
	_, err := Open(filepath.Join(t.TempDir(), "nope"), filepath.Join(t.TempDir(), "c.json"))
	if err == nil {
		t.Fatal("expected error for missing repo")
	}
}

func TestCreateResolveRoundtrip(t *testing.T) {
	s, root := openTestStore(t)
	defer func() { _ = s.Close() }()
	if err := s.SaveProject(core.DefaultProject("inbox")); err != nil {
		t.Fatal(err)
	}
	task := &core.Task{Title: "Submit DBMS assignment 2", Project: "inbox", Status: "todo", Priority: "high", Kind: core.KindTask}
	if err := s.Create(task); err != nil {
		t.Fatal(err)
	}
	if len(task.ID) != 6 {
		t.Fatalf("id = %q, want 6 chars", task.ID)
	}
	// File should exist on disk.
	path := filepath.Join(root, "projects", "inbox", task.ID+"-submit-dbms-assignment-2.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("task file missing: %v", err)
	}
	// Resolve by full id and by prefix.
	got, err := s.Resolve(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != task.Title {
		t.Fatalf("resolve title = %q", got.Title)
	}
	got, err = s.Resolve(task.ID[:3])
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != task.ID {
		t.Fatal("prefix resolve mismatch")
	}
	// Unknown id.
	if _, err := s.Resolve("ffffff"); err == nil {
		t.Fatal("expected error for unknown id")
	}
}

func TestAmbiguousPrefix(t *testing.T) {
	s, _ := openTestStore(t)
	defer func() { _ = s.Close() }()
	if err := s.SaveProject(core.DefaultProject("inbox")); err != nil {
		t.Fatal(err)
	}
	a := &core.Task{ID: "aa1111", Title: "a", Project: "inbox", Status: "todo", Priority: "low", Kind: core.KindTask}
	b := &core.Task{ID: "aa2222", Title: "b", Project: "inbox", Status: "todo", Priority: "low", Kind: core.KindTask}
	if err := s.Create(a); err != nil {
		t.Fatal(err)
	}
	if err := s.Create(b); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Resolve("aa"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguous error, got %v", err)
	}
}

func TestCreateInUnknownProjectFails(t *testing.T) {
	s, _ := openTestStore(t)
	defer func() { _ = s.Close() }()
	task := &core.Task{Title: "x", Project: "nope", Status: "todo", Priority: "low", Kind: core.KindTask}
	if err := s.Create(task); err == nil {
		t.Fatal("expected error for unknown project")
	}
}

func TestCreateRejectsTraversalProject(t *testing.T) {
	s, _ := openTestStore(t)
	defer func() { _ = s.Close() }()
	task := &core.Task{Title: "x", Project: "../escape", Status: "todo", Priority: "low", Kind: core.KindTask}
	if err := s.Create(task); err == nil {
		t.Fatal("expected error for traversal project name")
	}
}

func TestUpdateAndRemove(t *testing.T) {
	s, _ := openTestStore(t)
	defer func() { _ = s.Close() }()
	if err := s.SaveProject(core.DefaultProject("inbox")); err != nil {
		t.Fatal(err)
	}
	task := &core.Task{Title: "to change", Project: "inbox", Status: "todo", Priority: "medium", Kind: core.KindTask}
	if err := s.Create(task); err != nil {
		t.Fatal(err)
	}
	task.Status = "doing"
	task.Body = "note"
	if err := s.Update(task); err != nil {
		t.Fatal(err)
	}
	got, err := s.Resolve(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "doing" || got.Body != "note" {
		t.Fatalf("update not visible in index: %+v", got)
	}
	if err := s.Remove(task); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Resolve(task.ID); err == nil {
		t.Fatal("removed task still resolvable")
	}
}

func TestCachePersistsAndInvalidates(t *testing.T) {
	s, root := openTestStore(t)
	cache := s.cachePath
	if err := s.SaveProject(core.DefaultProject("inbox")); err != nil {
		t.Fatal(err)
	}
	task := &core.Task{Title: "cached", Project: "inbox", Status: "todo", Priority: "low", Kind: core.KindTask}
	if err := s.Create(task); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cache); err != nil {
		t.Fatalf("cache file missing: %v", err)
	}
	// Reopen: cache should satisfy the index build.
	s2, err := Open(root, cache)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s2.Close() }()
	got, err := s2.Resolve(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "cached" {
		t.Fatalf("title = %q", got.Title)
	}
}

func TestCorruptFileBecomesDiagnosticNotFatal(t *testing.T) {
	s, root := openTestStore(t)
	if err := s.SaveProject(core.DefaultProject("inbox")); err != nil {
		t.Fatal(err)
	}
	task := &core.Task{Title: "good", Project: "inbox", Status: "todo", Priority: "low", Kind: core.KindTask}
	if err := s.Create(task); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(root, "projects", "inbox", "badbad-bad.md")
	if err := os.WriteFile(bad, []byte("not frontmatter"), 0o644); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(root, filepath.Join(t.TempDir(), "c.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s2.Close() }()
	if _, err := s2.Resolve(task.ID); err != nil {
		t.Fatalf("good task lost due to corrupt sibling: %v", err)
	}
	if len(s2.Diagnostics()) == 0 {
		t.Fatal("expected a diagnostic for the corrupt file")
	}
}

func TestReloadProject(t *testing.T) {
	s, root := openTestStore(t)
	defer func() { _ = s.Close() }()
	if err := s.SaveProject(core.DefaultProject("inbox")); err != nil {
		t.Fatal(err)
	}
	// Simulate external edit making project.yaml invalid.
	path := filepath.Join(root, "projects", "inbox", "project.yaml")
	if err := os.WriteFile(path, []byte("statuses: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.ReloadProject("inbox"); err == nil {
		t.Fatal("expected validation error for empty statuses")
	}
}
