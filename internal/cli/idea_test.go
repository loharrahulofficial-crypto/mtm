package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runIdeaCmd(t *testing.T, args ...string) error {
	t.Helper()
	cmd := newIdeaCmd()
	cmd.SetArgs(args)
	return cmd.Execute()
}

func runIdeasCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return captureStdout(t, func() error {
		cmd := newIdeasCmd()
		cmd.SetArgs(args)
		return cmd.Execute()
	})
}

func TestIdeaLifecycleAndPromote(t *testing.T) {
	gitEnv(t)
	sandbox(t)
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}
	// Seed a project to promote into target... promote creates its own.
	if err := runIdeaCmd(t, "offline music player", "--tag", "audio", "--rating", "4"); err != nil {
		t.Fatalf("idea create: %v", err)
	}
	// Template for starter tasks.
	dataDir := filepath.Join(os.Getenv("XDG_DATA_HOME"), "mtm")
	if err := os.MkdirAll(filepath.Join(dataDir, "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	template := "# starter work\n- [ ] sketch the data model !high\n- pick an audio backend\n"
	if err := os.WriteFile(filepath.Join(dataDir, "templates", "web.md"), []byte(template), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runIdeasCmd(t)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "offline music player") || !strings.Contains(out, "seed") || !strings.Contains(out, "4/5") {
		t.Fatalf("ideas list output missing the new idea:\n%s", out)
	}
	// Resolve the id from the list output (first field of the first row).
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		t.Fatalf("ideas list has no rows:\n%s", out)
	}
	id := strings.Fields(lines[1])[0]
	// Note + rate + status transitions.
	if err := runIdeaCmd(t, "note", id, "talked to Mei, she wants offline maps too"); err != nil {
		t.Fatal(err)
	}
	if err := runIdeaCmd(t, "rate", id, "5"); err != nil {
		t.Fatal(err)
	}
	if err := runIdeaCmd(t, "status", id, "exploring"); err != nil {
		t.Fatal(err)
	}
	// Promote with the template.
	if err := runIdeaCmd(t, "promote", id, "--template", "web"); err != nil {
		t.Fatalf("promote: %v", err)
	}
	// The idea is marked promoted and keeps its file.
	promoted := filepath.Join(dataDir, "ideas")
	entries, err := os.ReadDir(promoted)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("idea files = %d, want 1", len(entries))
	}
	raw, err := os.ReadFile(filepath.Join(promoted, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"status: promoted", "promoted_to: offline-music-player", "talked to Mei"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("idea file missing %q:\n%s", want, raw)
		}
	}
	// Project + milestone + starter tasks were created.
	if _, err := os.Stat(filepath.Join(dataDir, "projects", "offline-music-player", "project.yaml")); err != nil {
		t.Fatalf("promoted project missing: %v", err)
	}
	ms, err := filepath.Glob(filepath.Join(dataDir, "projects", "offline-music-player", "milestones", "*.md"))
	if err != nil || len(ms) != 1 {
		t.Fatalf("milestones = %v, %v; want exactly 1", ms, err)
	}
	tasks, err := filepath.Glob(filepath.Join(dataDir, "projects", "offline-music-player", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("starter tasks = %v, want 2", tasks)
	}
	// The tasks are linked to the milestone.
	for _, raw := range mustReadAll(t, tasks...) {
		if !strings.Contains(raw, "milestone: ") {
			t.Errorf("starter task missing milestone link:\n%s", raw)
		}
	}
	// Invalid rating refused.
	if err := runIdeaCmd(t, "rate", id, "9"); err == nil {
		t.Fatal("rating 9 accepted")
	}
}

func mustReadAll(t *testing.T, paths ...string) []string {
	t.Helper()
	var out []string
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, string(data))
	}
	return out
}

func TestIdeasListFilters(t *testing.T) {
	gitEnv(t)
	sandbox(t)
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}
	mk := func(title string, tags ...string) {
		t.Helper()
		args := append([]string{title}, "--tag", strings.Join(tags, ","))
		if err := runIdeaCmd(t, args...); err != nil {
			t.Fatalf("idea create %q: %v", title, err)
		}
	}
	mk("sync idea", "sync")
	mk("editor idea", "editor")
	out, err := runIdeasCmd(t, "--tag", "sync")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "sync idea") || strings.Contains(out, "editor idea") {
		t.Fatalf("--tag filter wrong:\n%s", out)
	}
	out, err = runIdeasCmd(t, "--status", "exploring")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "no ideas") {
		t.Fatalf("--status exploring should be empty:\n%s", out)
	}
	if _, err := runIdeasCmd(t, "--status", "bogus"); err == nil {
		t.Fatal("bogus --status accepted")
	}
}

func TestMilestoneAddListShowDone(t *testing.T) {
	gitEnv(t)
	sandbox(t)
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}
	// A task linked to the milestone (created before the link exists, via flag).
	if err := runMilestoneCmd(t, "add", "inbox", "v0.1 cut", "--target", "2030-05-01", "--description", "first release"); err != nil {
		t.Fatalf("milestone add: %v", err)
	}
	out, err := captureStdout(t, func() error {
		return runMilestoneCmd(t, "list")
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "v0.1 cut") || !strings.Contains(out, "planned") || !strings.Contains(out, "no linked tasks") {
		t.Fatalf("milestone list output:\n%s", out)
	}
	// Resolve the milestone id from the list (first field of line 2).
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		t.Fatalf("list output has no rows:\n%s", out)
	}
	id := strings.Fields(lines[1])[0]
	// Link a task to it.
	if err := runAddWithMilestone(t, "draft release notes", id); err != nil {
		t.Fatalf("mtm add --milestone: %v", err)
	}
	out, err = captureStdout(t, func() error {
		return runMilestoneCmd(t, "show", id)
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"v0.1 cut", "2030-05-01", "draft release notes", "(0/1)"} {
		if !strings.Contains(out, want) {
			t.Errorf("show output missing %q:\n%s", want, out)
		}
	}
	if err := runMilestoneCmd(t, "done", id); err != nil {
		t.Fatal(err)
	}
	out, err = captureStdout(t, func() error {
		return runMilestoneCmd(t, "list", "--status", "done")
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "v0.1 cut") {
		t.Fatalf("--status done list output:\n%s", out)
	}
	// Unknown status refused.
	if _, err := captureStdout(t, func() error { return runMilestoneCmd(t, "list", "--status", "bogus") }); err == nil {
		t.Fatal("bogus --status accepted")
	}
}

func runMilestoneCmd(t *testing.T, args ...string) error {
	t.Helper()
	cmd := newMilestoneCmd()
	cmd.SetArgs(args)
	return cmd.Execute()
}

func runAddWithMilestone(t *testing.T, title, milestone string) error {
	t.Helper()
	cmd := newAddCmd()
	cmd.SetArgs([]string{title, "--milestone", milestone})
	return cmd.Execute()
}

func TestAddWithUnknownMilestoneFails(t *testing.T) {
	gitEnv(t)
	sandbox(t)
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}
	if err := runAddWithMilestone(t, "dangling task", "ffffff"); err == nil {
		t.Fatal("add with unknown milestone accepted")
	}
}

// writeEditorScript writes an $EDITOR stand-in that overwrites the file it
// is given with content (like a user pasting a note and saving).
func writeEditorScript(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "editor.sh")
	// %q-quoted payload keeps the backtick contents intact.
	body := "#!/bin/sh\ncat > \"$1\" <<'MTEOF'\n" + content + "\nMTEOF\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return script
}

func TestIdeaCaptureFromEditor(t *testing.T) {
	gitEnv(t)
	sandbox(t)
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", writeEditorScript(t, "# Local-first sync\n\nCRDT or oplog? Leaning oplog."))
	out, err := captureCombined(t, func() error {
		cmd := newIdeaCmd()
		cmd.SetArgs([]string{"--tag", "sync", "--rating", "5"})
		return cmd.Execute()
	})
	if err != nil {
		t.Fatalf("idea capture from editor: %v", err)
	}
	if !strings.Contains(out, "Local-first sync") {
		t.Fatalf("output missing the idea title:\n%s", out)
	}
	// The idea file landed with title, body and flags applied.
	ides, err := filepath.Glob(filepath.Join(os.Getenv("XDG_DATA_HOME"), "mtm", "ideas", "*.md"))
	if err != nil || len(ides) != 1 {
		t.Fatalf("idea files = %v, %v; want 1", ides, err)
	}
	raw, err := os.ReadFile(ides[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"title: Local-first sync", "status: seed", "rating: 5", "- sync", "CRDT or oplog?"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("idea file missing %q:\n%s", want, raw)
		}
	}
}

func TestIdeaCaptureFromEditorDiscardsEmpty(t *testing.T) {
	gitEnv(t)
	sandbox(t)
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", writeEditorScript(t, "# \n")) // heading with no text
	if err := newIdeaCmd().Execute(); err == nil {
		t.Fatal("empty editor content accepted as an idea")
	}
	ides, err := filepath.Glob(filepath.Join(os.Getenv("XDG_DATA_HOME"), "mtm", "ideas", "*.md"))
	if err != nil || len(ides) != 0 {
		t.Fatalf("empty idea was saved: %v", ides)
	}
}

func TestLogFromEditor(t *testing.T) {
	gitEnv(t)
	sandbox(t)
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", writeEditorScript(t, "# Week 2 wrap\n\nER diagrams, then 3NF."))
	out, err := captureCombined(t, func() error {
		cmd := newLogCmd()
		cmd.SetArgs([]string{"inbox", "--tag", "study", "--mood", "flow"})
		return cmd.Execute()
	})
	if err != nil {
		t.Fatalf("log from editor: %v", err)
	}
	if !strings.Contains(out, "Week 2 wrap") {
		t.Fatalf("output missing the entry title:\n%s", out)
	}
	entries, err := filepath.Glob(filepath.Join(os.Getenv("XDG_DATA_HOME"), "mtm", "projects", "inbox", "devlog", "*.md"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("devlog files = %v, %v; want 1", entries, err)
	}
	raw, err := os.ReadFile(entries[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"title: Week 2 wrap", "mood: flow", "- study", "ER diagrams, then 3NF."} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("devlog file missing %q:\n%s", want, raw)
		}
	}
}
