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

// openIdeaStore opens a temp repo with one project (inbox) for idea tests.
func openIdeaStore(t *testing.T) *store.Store {
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
	s.Clock = clock.Fixed(time.Date(2026, 11, 9, 10, 0, 0, 0, time.Local))
	if err := s.SaveProject(core.DefaultProject("college/dbms")); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
	return s
}

func TestAddIdeaAndStatus(t *testing.T) {
	s := openIdeaStore(t)
	defer func() { _ = s.Close() }()

	i, err := AddIdea(s, "  local-first ", []string{"Go", "#tui", "go"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if i.Status != core.IdeaSeed || len(i.Tags) != 2 {
		t.Fatalf("idea = %+v", i)
	}
	if s.Ideas()[0].Title != "local-first" {
		t.Fatalf("title not trimmed: %q", s.Ideas()[0].Title)
	}
	st, err := SetIdeaStatus(s, i, "EXPLORING")
	if err != nil || st != core.IdeaExploring {
		t.Fatalf("status = %q, %v", st, err)
	}
	if _, err := SetIdeaStatus(s, i, "bogus"); err == nil {
		t.Fatal("bogus status accepted")
	}
	if err := NoteIdea(s, i, "first thought", time.Date(2026, 11, 9, 10, 0, 0, 0, time.Local)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s.Ideas()[0].Body, "- 2026-11-09 first thought") {
		t.Fatalf("note not appended as dated bullet:\n%s", s.Ideas()[0].Body)
	}
}

func TestPromoteIdeaCreatesProjectMilestoneAndTasks(t *testing.T) {
	s := openIdeaStore(t)
	defer func() { _ = s.Close() }()

	// A template with comments, bullets and quick-add grammar.
	if err := os.MkdirAll(filepath.Join(s.Root, "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "# comment to skip\n\n- [ ] set up repo fri !high #setup\n- write first milestone spec\n  \n- [x] initial commit\n"
	if err := os.WriteFile(filepath.Join(s.Root, "templates", "web.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	i, err := AddIdea(s, "Offline music player", []string{"audio"}, 4)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 9, 10, 0, 0, 0, time.Local)
	res, err := PromoteIdea(s, i.ID, "", "web", now)
	if err != nil {
		t.Fatal(err)
	}
	// Project exists with the standard workflow.
	p, err := s.GetProject(res.Project)
	if err != nil {
		t.Fatalf("promoted project %q not readable: %v", res.Project, err)
	}
	if len(p.Statuses) != 3 || p.Statuses[0].Name != "todo" {
		t.Fatalf("project statuses = %+v", p.Statuses)
	}
	// Initial milestone exists, planned, ~30 days out.
	ms := s.Milestones(res.Project)
	if len(ms) != 1 {
		t.Fatalf("milestones = %d, want 1", len(ms))
	}
	if ms[0].Status != core.MilestonePlanned {
		t.Fatalf("milestone status = %q", ms[0].Status)
	}
	day, err := time.ParseInLocation("2006-01-02", ms[0].Target, time.Local)
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 12, 9, 0, 0, 0, 0, time.Local); !day.Equal(want) {
		t.Fatalf("milestone target = %s, want %s", ms[0].Target, want.Format("2006-01-02"))
	}
	// 3 starter tasks, linked to the milestone, quick-add parsed.
	tasks := 0
	byID := map[string]*core.Task{}
	for _, task := range s.Tasks() {
		if task.Project == res.Project {
			tasks++
			byID[task.ID] = task
			if task.Milestone != res.MilestoneID {
				t.Errorf("task %q milestone = %q, want %q", task.ID, task.Milestone, res.MilestoneID)
			}
		}
	}
	if tasks != 3 {
		t.Fatalf("starter tasks = %d, want 3", tasks)
	}
	found := map[string]*core.Task{}
	for _, task := range byID {
		found[strings.ToLower(task.Title)] = task
	}
	setup, ok := found["set up repo"]
	if !ok {
		t.Fatalf("quick-add title missing; got %v", found)
	}
	if setup.Priority != core.PriorityHigh {
		t.Fatalf("quick-add priority = %q, want high", setup.Priority)
	}
	if setup.Due != "2026-11-13" { // friday after 2026-11-09 (a Monday)
		t.Fatalf("quick-add due = %q, want 2026-11-13", setup.Due)
	}
	// Idea records the promotion; file is kept.
	got := s.Ideas()[0]
	if got.Status != core.IdeaPromoted || got.PromotedTo != res.Project {
		t.Fatalf("idea after promote = status %q promoted_to %q", got.Status, got.PromotedTo)
	}
}

func TestPromoteIdeaErrors(t *testing.T) {
	s := openIdeaStore(t)
	defer func() { _ = s.Close() }()
	i, err := AddIdea(s, "Promote me", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 9, 10, 0, 0, 0, time.Local)
	// Project already exists.
	if err := s.SaveProject(core.DefaultProject("promote-me")); err != nil {
		t.Fatal(err)
	}
	if _, err := PromoteIdea(s, i.ID, "", "", now); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err = %v, want already-exists", err)
	}
	// Template missing reports the partial project.
	res, err := PromoteIdea(s, i.ID, "other-proj", "nosuchtemplate", now)
	if err == nil {
		t.Fatal("missing template accepted")
	}
	if res == nil || res.Project != "other-proj" || res.MilestoneID == "" {
		t.Fatalf("partial result = %+v, want project+milestone reported", res)
	}
	// Idea not marked promoted on failure.
	if got := s.Ideas()[0]; got.Status == core.IdeaPromoted {
		t.Fatal("idea marked promoted despite the template failure")
	}
	// Promoting twice is refused.
	if _, err := PromoteIdea(s, i.ID, "", "", now); err == nil {
		t.Fatal("re-promote accepted")
	}
}

func TestNextMilestone(t *testing.T) {
	s := openIdeaStore(t)
	defer func() { _ = s.Close() }()
	now := time.Date(2026, 11, 9, 10, 0, 0, 0, time.Local)
	if _, _, ok := NextMilestone(s, now); ok {
		t.Fatal("NextMilestone found one in an empty store")
	}
	mk := func(title, target, status string) {
		m := &core.Milestone{Title: title, Target: target, Status: status, Project: "college/dbms"}
		if err := s.CreateMilestone(m); err != nil {
			t.Fatal(err)
		}
	}
	mk("done soon", "2026-11-12", core.MilestoneDone) // ignored (done)
	mk("far", "2026-12-25", core.MilestonePlanned)
	mk("near", "2026-11-14", core.MilestoneActive)
	mk("no date", "", core.MilestonePlanned) // ignored (no target)
	m, days, ok := NextMilestone(s, now)
	if !ok {
		t.Fatal("NextMilestone not ok")
	}
	if m.Title != "near" || days != 5 {
		t.Fatalf("next = %q in %dd, want near in 5d", m.Title, days)
	}
}

func TestTemplateTaskLines(t *testing.T) {
	data := []byte("---\nname: x\n---\n# heading\n- [ ] one fri !high\n* two\n- plain three\n\n#ignored\n")
	got := templateTaskLines(data)
	want := []string{"one fri !high", "two", "plain three"}
	if len(got) != len(want) {
		t.Fatalf("lines = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}
