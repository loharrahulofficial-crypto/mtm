package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func gitEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_AUTHOR_NAME", "test")
	t.Setenv("GIT_AUTHOR_EMAIL", "t@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "test")
	t.Setenv("GIT_COMMITTER_EMAIL", "t@example.com")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	// Deterministic identity check: no global/system git config.
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
}

func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

const taskFile = `---
id: %s
title: example task
project: inbox
status: todo
priority: medium
kind: task
created: 2026-11-01T09:00
updated: 2026-11-01T09:00
---

body
`

// initRepo creates a git repo at root with one committed task in inbox.
func initRepo(t *testing.T, root string, taskID string) {
	t.Helper()
	gitEnv(t)
	if err := os.MkdirAll(filepath.Join(root, "projects", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := fmt.Sprintf(taskFile, taskID)
	if err := os.WriteFile(filepath.Join(root, "projects", "inbox", taskID+"-example.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "init", "-b", "main")
	gitCmd(t, root, "add", "-A")
	gitCmd(t, root, "commit", "-qm", "add first task")
}

func writeMarker(t *testing.T, root, kind string) {
	t.Helper()
	marker := fmt.Sprintf("id: 11111111-2222-4333-8444-555555555555\ncreated: 2026-11-09\nkind: %s\n", kind)
	if err := os.WriteFile(filepath.Join(root, ".mtm-data"), []byte(marker), 0o644); err != nil {
		t.Fatal(err)
	}
}

func findingsOf(rep Report, level, check string) []Finding {
	var out []Finding
	for _, f := range rep.Findings {
		if f.Level == level && (check == "" || f.Check == check) {
			out = append(out, f)
		}
	}
	return out
}

func runDoctor(t *testing.T, root string) Report {
	t.Helper()
	return Run(Options{Root: root, StateDir: t.TempDir(), Now: time.Date(2026, 11, 9, 12, 0, 0, 0, time.Local)})
}

func TestHealthyRealRepo(t *testing.T) {
	gitEnv(t)
	root := t.TempDir()
	initRepo(t, root, "a1b2c3")
	writeMarker(t, root, "real")
	gitCmd(t, root, "remote", "add", "origin", filepath.Join(t.TempDir(), "origin.git"))
	gitCmd(t, root, "config", "user.name", "test")
	gitCmd(t, root, "config", "user.email", "t@example.com")
	gitCmd(t, root, "add", "-A")
	_ = gitCmd(t, root, "commit", "-qm", "marker")

	rep := runDoctor(t, root)
	if !rep.OK {
		data, _ := json.MarshalIndent(rep, "", "  ")
		t.Fatalf("healthy repo reported failing checks:\n%s", data)
	}
	if rep.Kind != "real" {
		t.Fatalf("kind = %q, want real", rep.Kind)
	}
}

func TestDemoDataInRealRepoIsFlagged(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root, "dn0055")
	writeMarker(t, root, "real")
	gitCmd(t, root, "commit", "-qm", "demo: seed example tasks", "--allow-empty")

	rep := runDoctor(t, root)
	if rep.OK {
		t.Fatal("demo-looking data in a real repo was not flagged")
	}
	demo := findingsOf(rep, LevelFail, "demo-data")
	if len(demo) < 2 {
		data, _ := json.MarshalIndent(rep.Findings, "", "  ")
		t.Fatalf("want both the task-id and commit-message findings, got:\n%s", data)
	}
}

func TestDemoDataInDemoRepoIsNotFlaggedAsFail(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root, "lt0001")
	writeMarker(t, root, "demo")

	rep := runDoctor(t, root)
	if fails := findingsOf(rep, LevelFail, "demo-data"); len(fails) > 0 {
		data, _ := json.MarshalIndent(fails, "", "  ")
		t.Fatalf("demo repo flagged for demo data:\n%s", data)
	}
}

func TestBrokenFrontmatterIsFlagged(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root, "a1b2c3")
	writeMarker(t, root, "real")
	bad := filepath.Join(root, "projects", "inbox", "zz9999-broken.md")
	if err := os.WriteFile(bad, []byte("not frontmatter at all\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rep := runDoctor(t, root)
	fails := findingsOf(rep, LevelFail, "frontmatter")
	if len(fails) != 1 || !strings.Contains(fails[0].Path, "zz9999-broken.md") {
		data, _ := json.MarshalIndent(rep.Findings, "", "  ")
		t.Fatalf("frontmatter failure not reported with path:\n%s", data)
	}
}

func TestConflictedSyncStateIsReported(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root, "a1b2c3")
	writeMarker(t, root, "real")
	gitDir := gitCmd(t, root, "rev-parse", "--absolute-git-dir")
	state := `{"conflicted":true,"files":["projects/inbox/a1b2c3-example.md"],"since":"2026-11-09T10:00:00+00:00","reason":"test"}`
	if err := os.WriteFile(filepath.Join(gitDir, "mtm-sync-state.json"), []byte(state), 0o600); err != nil {
		t.Fatal(err)
	}

	rep := runDoctor(t, root)
	// conflicted is a warning, not a failure
	if fails := findingsOf(rep, LevelFail, ""); len(fails) > 0 {
		data, _ := json.MarshalIndent(fails, "", "  ")
		t.Fatalf("conflicted sync should warn, not fail:\n%s", data)
	}
	warns := findingsOf(rep, LevelWarn, "sync")
	if len(warns) != 1 || !strings.Contains(warns[0].Message, "paused") {
		data, _ := json.MarshalIndent(rep.Findings, "", "  ")
		t.Fatalf("conflicted sync not reported:\n%s", data)
	}
}

func TestStaleTimerStateIsListed(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root, "a1b2c3")
	writeMarker(t, root, "real")
	stateDir := t.TempDir()
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.Local)
	stale := fmt.Sprintf(`{"task_id":"a1b2c3","mode":"timer","phase":"work","anchor":%q,"running":true,"accrued":0,"phase_elapsed":0,"phase_ends_at":"0001-01-01T00:00:00Z","phase_total":0,"phase_index":0,"pomodoros":0,"updated_at":%q}`,
		now.Add(-5*time.Hour).Format(time.RFC3339), now.Format(time.RFC3339))
	if err := os.WriteFile(filepath.Join(stateDir, "timer.json"), []byte(stale), 0o600); err != nil {
		t.Fatal(err)
	}

	rep := Run(Options{Root: root, StateDir: stateDir, Now: now})
	warns := findingsOf(rep, LevelWarn, "timer")
	if len(warns) != 1 || !strings.Contains(warns[0].Message, "stale") {
		data, _ := json.MarshalIndent(rep.Findings, "", "  ")
		t.Fatalf("stale timer not listed:\n%s", data)
	}
}

func TestMissingGitIdentityWarns(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root, "a1b2c3")
	writeMarker(t, root, "real")
	// gitEnv in initRepo points GIT_CONFIG_GLOBAL at an empty file, so no identity is set.
	rep := runDoctor(t, root)
	warns := findingsOf(rep, LevelWarn, "identity")
	if len(warns) == 0 {
		data, _ := json.MarshalIndent(rep.Findings, "", "  ")
		t.Fatalf("missing identity not warned about:\n%s", data)
	}
}

func TestMissingMarkerWarnsNotFails(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root, "a1b2c3")
	rep := runDoctor(t, root)
	if fails := findingsOf(rep, LevelFail, "marker"); len(fails) > 0 {
		t.Fatal("missing marker should warn, not fail")
	}
	if warns := findingsOf(rep, LevelWarn, "marker"); len(warns) != 1 {
		data, _ := json.MarshalIndent(rep.Findings, "", "  ")
		t.Fatalf("missing marker not warned about:\n%s", data)
	}
}

func TestNotAGitRepoFails(t *testing.T) {
	gitEnv(t)
	root := t.TempDir()
	rep := runDoctor(t, root)
	if fails := findingsOf(rep, LevelFail, "git"); len(fails) != 1 {
		data, _ := json.MarshalIndent(rep.Findings, "", "  ")
		t.Fatalf("non-repo not reported as fail:\n%s", data)
	}
	if rep.OK {
		t.Fatal("report ok on a non-repo")
	}
}
