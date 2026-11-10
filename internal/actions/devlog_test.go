package actions

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/clock"
)

func TestAddDevLog(t *testing.T) {
	s := openIdeaStore(t)
	defer func() { _ = s.Close() }()
	d, err := AddDevLog(s, "college/dbms", "3NF done", "then lossless joins", 45, "flow", []string{"study"})
	if err != nil {
		t.Fatal(err)
	}
	if d.ID == "" || d.Created == "" || d.Project != "college/dbms" {
		t.Fatalf("devlog = %+v", d)
	}
	if d.TimeSpentMinutes != 45 || d.Mood != "flow" || len(d.Tags) != 1 {
		t.Fatalf("devlog meta = %+v", d)
	}
	path, err := s.DevLogPath(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Base(path)
	if !strings.HasPrefix(base, time.Date(2026, 11, 9, 0, 0, 0, 0, time.Local).Format("2006-01-02")+"-") {
		t.Fatalf("devlog file name %q does not start with YYYY-MM-DD-", base)
	}
}

func TestGitLogSubjectsAgainstTempRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	repo := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=t@example.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	for _, msg := range []string{"init project", "add schema", "add queries"} {
		if err := os.WriteFile(filepath.Join(repo, "f.txt"), []byte(msg+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		git("add", "-A")
		git("commit", "-q", "-m", msg)
	}
	lines, err := GitLogSubjects(repo, time.Time{}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 3 {
		t.Fatalf("commits = %d, want 3 (%v)", len(lines), lines)
	}
	if !strings.Contains(lines[0], "add queries") || !strings.Contains(lines[2], "init project") {
		t.Fatalf("order/content wrong: %v", lines)
	}
	// Since-window filter: commits after a far-future cutoff exclude all.
	future := time.Now().Add(24 * time.Hour)
	lines, err = GitLogSubjects(repo, future, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 0 {
		t.Fatalf("future cutoff returned %v, want none", lines)
	}
	// A non-repo path is an error, not a panic.
	if _, err := GitLogSubjects(t.TempDir(), time.Time{}, 10); err == nil {
		t.Fatal("git log on a non-repo succeeded")
	}
}

func TestCommitsSincePrefersLastDevLog(t *testing.T) {
	s := openIdeaStore(t)
	defer func() { _ = s.Close() }()
	// Write a devlog dated in the past.
	s.Clock = clock.Fixed(time.Date(2026, 11, 1, 10, 0, 0, 0, time.Local))
	d, err := AddDevLog(s, "college/dbms", "week 1", "", 0, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = d
	s.Clock = clock.Fixed(time.Date(2026, 11, 9, 10, 0, 0, 0, time.Local))
	// No linked repo on the project → falls back to the data repo root,
	// which is not a git repo here → error, never a panic.
	_, err = CommitsSince(s, "college/dbms", s.Now(), 10)
	if err == nil {
		t.Fatal("CommitsSince on a non-git data repo succeeded")
	}
	// With a linked git repo, the window starts at the last devlog date.
	repo := t.TempDir()
	gitInit := exec.Command("git", "init", "-q", "-b", "main")
	gitInit.Dir = repo
	gitInit.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=t@example.com")
	if out, err := gitInit.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commit := exec.Command("git", "add", "-A")
	commit.Dir = repo
	if out, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	commit = exec.Command("git", "commit", "-q", "-m", "linked repo commit")
	commit.Dir = repo
	commit.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=t@example.com")
	if out, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
	p, err := s.GetProject("college/dbms")
	if err != nil {
		t.Fatal(err)
	}
	p.RepoPath = repo
	if err := s.SaveProject(p); err != nil {
		t.Fatal(err)
	}
	lines, err := CommitsSince(s, "college/dbms", s.Now(), 10)
	if err != nil {
		t.Fatal(err)
	}
	// The last devlog is dated 2026-11-01; the commit is "now"
	// (2026-11-09) → within the since window → included.
	if len(lines) != 1 || !strings.Contains(lines[0], "linked repo commit") {
		t.Fatalf("commits = %v, want the linked repo commit", lines)
	}
}

func TestCommitSummaryMarkdown(t *testing.T) {
	if got := CommitSummary(nil); got != "" {
		t.Fatalf("empty summary = %q, want empty", got)
	}
	got := CommitSummary([]string{"abc123 init", "def456 add queries"})
	if !strings.Contains(got, "## Commits") || !strings.Contains(got, "- abc123 init") || !strings.Contains(got, "- def456 add queries") {
		t.Fatalf("summary = %q", got)
	}
}
