package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/clock"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

func fixedEnv(tb testing.TB) {
	tb.Helper()
	tmp := tb.TempDir()
	tb.Setenv("HOME", tmp)
	tb.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "config"))
	tb.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	tb.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))
	tb.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))
}

func openDocStore(t *testing.T) *Store {
	t.Helper()
	fixedEnv(t)
	s, err := Open(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	s.Clock = clock.Fixed(fixedTime)
	if err := s.SaveProject(core.DefaultProject("college/dbms")); err != nil {
		t.Fatal(err)
	}
	return s
}

var fixedTime = func() time.Time { return time.Date(2026, 11, 8, 14, 32, 0, 0, time.Local) }()

func TestIdeaRoundTripAndFiles(t *testing.T) {
	s := openDocStore(t)
	defer func() { _ = s.Close() }()
	i := &core.Idea{Title: "A sync-safe git-backed idea", Status: core.IdeaSeed, Rating: 4, Tags: []string{"go", "tui"}}
	if err := s.CreateIdea(i); err != nil {
		t.Fatal(err)
	}
	if i.ID == "" {
		t.Fatal("id not assigned")
	}
	path, err := s.IdeaPath(i.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filepath.ToSlash(path), "/ideas/") {
		t.Fatalf("idea stored at %q, want an ideas/ path", path)
	}
	// Frontmatter declares the documented keys.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"id: ", "title: ", "status: seed", "rating: 4", "created: ", "updated: ", "tags:"} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("idea file missing %q:\n%s", key, raw)
		}
	}
	// Re-open: the idea survives a fresh index.
	root := s.Root
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	cfgCache := filepath.Join(t.TempDir(), "idx.json")
	s2, err := Open(root, cfgCache)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s2.Close() }()
	got, err := s2.ResolveIdea(i.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != i.Title || got.Rating != 4 || len(got.Tags) != 2 {
		t.Fatalf("round trip mismatch: %+v", got)
	}
}

func TestMilestoneAndDevLogRoundTrip(t *testing.T) {
	s := openDocStore(t)
	defer func() { _ = s.Close() }()
	m := &core.Milestone{Title: "v0.1 ship", Target: "2026-12-15", Status: core.MilestonePlanned, Project: "college/dbms", Description: "relational chapter done"}
	if err := s.CreateMilestone(m); err != nil {
		t.Fatal(err)
	}
	mPath, err := s.MilestonePath(m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filepath.ToSlash(mPath), "projects/college/dbms/milestones/") {
		t.Fatalf("milestone path %q not under the project's milestones/ dir", mPath)
	}
	d := &core.DevLogEntry{Title: "normal forms", Project: "college/dbms", TimeSpentMinutes: 45, Tags: []string{"study"}, Body: "worked through 3NF"}
	if err := s.CreateDevLog(d); err != nil {
		t.Fatal(err)
	}
	dPath, err := s.DevLogPath(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantBase := "projects/college/dbms/devlog/"
	if !strings.Contains(filepath.ToSlash(dPath), wantBase) {
		t.Fatalf("devlog path %q not under the project's devlog/ dir", dPath)
	}
	base := filepath.Base(dPath)
	if len(base) < len("2006-01-02-") || base[10] != '-' {
		t.Fatalf("devlog file name %q does not start with YYYY-MM-DD-", base)
	}
	// Docs are not indexed as tasks.
	if n := s.TaskCount("college/dbms"); n != 0 {
		t.Fatalf("TaskCount = %d, want 0 (milestones/devlogs are not tasks)", n)
	}
	for _, task := range s.Tasks() {
		if strings.Contains(task.Title, "ship") || strings.Contains(task.Title, "normal forms") {
			t.Fatalf("document leaked into the task index: %+v", task)
		}
	}
	// Listing by project.
	if got := s.Milestones("college/dbms"); len(got) != 1 {
		t.Fatalf("Milestones = %d, want 1", len(got))
	}
	if got := s.DevLogs("college/dbms"); len(got) != 1 {
		t.Fatalf("DevLogs = %d, want 1", len(got))
	}
	if got := s.Milestones("inbox"); len(got) != 0 {
		t.Fatalf("Milestones(inbox) = %d, want 0", len(got))
	}
}

func TestUniqueIDsDoNotCollideAcrossDocTypes(t *testing.T) {
	s := openDocStore(t)
	defer func() { _ = s.Close() }()
	i := &core.Idea{Title: "one", Status: core.IdeaSeed}
	if err := s.CreateIdea(i); err != nil {
		t.Fatal(err)
	}
	m := &core.Milestone{Title: "two", Status: core.MilestonePlanned, Project: "college/dbms"}
	if err := s.CreateMilestone(m); err != nil {
		t.Fatal(err)
	}
	d := &core.DevLogEntry{Title: "three", Project: "college/dbms"}
	if err := s.CreateDevLog(d); err != nil {
		t.Fatal(err)
	}
	ids := []string{i.ID, m.ID, d.ID}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("duplicate id %q across document types", id)
		}
		seen[id] = true
	}
}

func TestTemplateFileGuard(t *testing.T) {
	s := openDocStore(t)
	defer func() { _ = s.Close() }()
	for _, bad := range []string{"../etc/passwd", "a/b", "a\\b", "..", ""} {
		if _, err := s.TemplateFile(bad); err == nil {
			t.Errorf("TemplateFile(%q) accepted; want error", bad)
		}
	}
	if err := os.MkdirAll(filepath.Join(s.Root, "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Root, "templates", "web.md"), []byte("- first\n- second\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := s.TemplateFile("web")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "first") {
		t.Fatalf("template content = %q", data)
	}
	// explicit .md suffix also works
	if _, err := s.TemplateFile("web.md"); err != nil {
		t.Fatal(err)
	}
}

func TestMilestoneResolutionAmbiguity(t *testing.T) {
	s := openDocStore(t)
	defer func() { _ = s.Close() }()
	if err := s.SaveProject(core.DefaultProject("inbox")); err != nil {
		t.Fatal(err)
	}
	m1 := &core.Milestone{ID: "aa1000", Title: "one", Status: core.MilestonePlanned, Project: "college/dbms"}
	m2 := &core.Milestone{ID: "aa1001", Title: "two", Status: core.MilestonePlanned, Project: "inbox"}
	if err := s.CreateMilestone(m1); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateMilestone(m2); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ResolveMilestone("aa1"); err == nil {
		t.Fatal("ambiguous prefix accepted")
	}
	got, err := s.ResolveMilestone("aa1000")
	if err != nil || got.Title != "one" {
		t.Fatalf("ResolveMilestone(aa1000) = %v, %v", got, err)
	}
}

func TestIdeaRemove(t *testing.T) {
	s := openDocStore(t)
	defer func() { _ = s.Close() }()
	i := &core.Idea{Title: "temporary", Status: core.IdeaSeed}
	if err := s.CreateIdea(i); err != nil {
		t.Fatal(err)
	}
	path, err := s.IdeaPath(i.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveIdea(i); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("idea file still exists at %s", path)
	}
	if _, err := s.ResolveIdea(i.ID); err == nil {
		t.Fatal("removed idea still resolves")
	}
}
