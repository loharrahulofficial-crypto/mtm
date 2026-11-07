package sync

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitCmd(t *testing.T, dir, name, email string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME="+name, "GIT_AUTHOR_EMAIL="+email,
		"GIT_COMMITTER_NAME="+name, "GIT_COMMITTER_EMAIL="+email,
		"GIT_TERMINAL_PROMPT=0",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// setupRepo creates origin (bare) + two clones for conflict testing.
func setupClones(t *testing.T) (origin, a, b string) {
	t.Helper()
	base := t.TempDir()
	origin = filepath.Join(base, "origin.git")
	a = filepath.Join(base, "a")
	b = filepath.Join(base, "b")
	gitCmd(t, base, "test", "t@example.com", "init", "--bare", "-b", "main", origin)
	gitCmd(t, base, "test", "t@example.com", "clone", origin, a)
	gitCmd(t, a, "test", "t@example.com", "config", "user.name", "test")
	gitCmd(t, a, "test", "t@example.com", "config", "user.email", "t@example.com")
	if err := os.WriteFile(filepath.Join(a, "task.md"), []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, a, "test", "t@example.com", "add", "-A")
	gitCmd(t, a, "test", "t@example.com", "commit", "-m", "initial")
	gitCmd(t, a, "test", "t@example.com", "push", "-u", "origin", "main")
	gitCmd(t, base, "test", "t@example.com", "clone", origin, b)
	gitCmd(t, b, "test", "t@example.com", "config", "user.name", "test")
	gitCmd(t, b, "test", "t@example.com", "config", "user.email", "t@example.com")
	return origin, a, b
}

func TestCommitAllNothingToCommit(t *testing.T) {
	dir := t.TempDir()
	repo := New(dir)
	if err := repo.Init(); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, dir, "test", "t@example.com", "config", "user.name", "test")
	gitCmd(t, dir, "test", "t@example.com", "config", "user.email", "t@example.com")
	committed, err := repo.CommitAll("empty")
	if err != nil {
		t.Fatal(err)
	}
	if committed {
		t.Fatal("expected no commit on empty repo")
	}
}

func TestSyncPushPullNoConflict(t *testing.T) {
	_, a, b := setupClones(t)
	// B makes a change and pushes.
	if err := os.WriteFile(filepath.Join(b, "task.md"), []byte("from b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repoB := New(b)
	if committed, err := repoB.CommitAll("b change"); err != nil || !committed {
		t.Fatalf("commit: committed=%v err=%v", committed, err)
	}
	res, err := repoB.Sync("origin")
	if err != nil || !res.Pushed {
		t.Fatalf("B sync: %+v err=%v", res, err)
	}
	// A syncs and should receive B's change.
	repoA := New(a)
	res, err = repoA.Sync("origin")
	if err != nil || !res.Pulled {
		t.Fatalf("A sync: %+v err=%v", res, err)
	}
	data, err := os.ReadFile(filepath.Join(a, "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "from b\n" {
		t.Fatalf("A did not pull B's change: %q", data)
	}
}

func TestSyncConflictPausesAndAborts(t *testing.T) {
	_, a, b := setupClones(t)
	// Both change the same line.
	if err := os.WriteFile(filepath.Join(a, "task.md"), []byte("from a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repoA := New(a)
	if _, err := repoA.CommitAll("a change"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b, "task.md"), []byte("from b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repoB := New(b)
	if _, err := repoB.CommitAll("b change"); err != nil {
		t.Fatal(err)
	}
	if _, err := repoB.Sync("origin"); err != nil {
		t.Fatal(err)
	}
	// A pulls → conflict expected.
	_, err := repoA.Sync("origin")
	if err != ErrConflicted {
		t.Fatalf("expected ErrConflicted, got %v", err)
	}
	st, err := repoA.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if !st.Conflicted {
		t.Fatal("state not marked conflicted")
	}
	if len(st.Files) == 0 {
		t.Fatal("no conflicted files recorded")
	}
	// While conflicted, Sync must refuse to do anything.
	if _, err := repoA.Sync("origin"); err != ErrConflicted {
		t.Fatalf("conflicted repo did not stay paused: %v", err)
	}
	// Abort returns the repo to a clean, syncable state.
	if err := repoA.Abort(); err != nil {
		t.Fatal(err)
	}
	st, err = repoA.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if st.Conflicted {
		t.Fatal("abort did not clear conflicted state")
	}
	data, err := os.ReadFile(filepath.Join(a, "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "from a\n" {
		t.Fatalf("abort did not restore A's version: %q", data)
	}
}

func TestAheadWithoutUpstream(t *testing.T) {
	dir := t.TempDir()
	repo := New(dir)
	if err := repo.Init(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "x"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, dir, "test", "t@example.com", "config", "user.name", "test")
	gitCmd(t, dir, "test", "t@example.com", "config", "user.email", "t@example.com")
	if _, err := repo.CommitAll("x"); err != nil {
		t.Fatal(err)
	}
	n, err := repo.Ahead()
	if err != nil || n != 0 {
		t.Fatalf("ahead with no upstream: n=%d err=%v", n, err)
	}
}

func TestHasRemote(t *testing.T) {
	dir := t.TempDir()
	repo := New(dir)
	if err := repo.Init(); err != nil {
		t.Fatal(err)
	}
	if repo.HasRemote("origin") {
		t.Fatal("unexpected remote")
	}
	if err := repo.AddRemote("origin", "https://example.com/repo.git"); err != nil {
		t.Fatal(err)
	}
	if !repo.HasRemote("origin") {
		t.Fatal("remote missing after add")
	}
	url, err := repo.RemoteURL("origin")
	if err != nil || url != "https://example.com/repo.git" {
		t.Fatalf("url=%q err=%v", url, err)
	}
}

func TestResolveConflictAndRebaseContinue(t *testing.T) {
	_, a, b := setupClones(t)
	// Both change the same line; B pushes; A pulls → conflict.
	if err := os.WriteFile(filepath.Join(a, "task.md"), []byte("from a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repoA := New(a)
	if _, err := repoA.CommitAll("a change"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b, "task.md"), []byte("from b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repoB := New(b)
	if _, err := repoB.CommitAll("b change"); err != nil {
		t.Fatal(err)
	}
	if _, err := repoB.Sync("origin"); err != nil {
		t.Fatal(err)
	}
	if _, err := repoA.Sync("origin"); err != ErrConflicted {
		t.Fatalf("expected ErrConflicted, got %v", err)
	}
	if !repoA.RebaseInProgress() {
		t.Fatal("expected a rebase in progress")
	}
	st, err := repoA.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Files) != 1 || st.Files[0] != "task.md" {
		t.Fatalf("conflicted files = %v", st.Files)
	}
	// Keep mine (local): during rebase, ours=remote so mine=--theirs.
	if err := repoA.ResolveConflict("task.md", true); err != nil {
		t.Fatal(err)
	}
	files, err := repoA.ConflictedFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("still conflicted after resolve: %v", files)
	}
	if err := repoA.RebaseContinue(); err != nil {
		t.Fatal(err)
	}
	if err := repoA.ClearState(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(a, "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "from a\n" {
		t.Fatalf("keepMine resolved to %q", data)
	}
	res, err := repoA.Sync("origin")
	if err != nil {
		t.Fatalf("sync after continue: %v", err)
	}
	if !res.Pushed {
		t.Fatal("expected push after rebase continue")
	}
}

func TestResolveConflictKeepTheirs(t *testing.T) {
	_, a, b := setupClones(t)
	if err := os.WriteFile(filepath.Join(a, "task.md"), []byte("from a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repoA := New(a)
	if _, err := repoA.CommitAll("a change"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b, "task.md"), []byte("from b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repoB := New(b)
	if _, err := repoB.CommitAll("b change"); err != nil {
		t.Fatal(err)
	}
	if _, err := repoB.Sync("origin"); err != nil {
		t.Fatal(err)
	}
	if _, err := repoA.Sync("origin"); err != ErrConflicted {
		t.Fatalf("expected ErrConflicted, got %v", err)
	}
	if err := repoA.ResolveConflict("task.md", false); err != nil {
		t.Fatal(err)
	}
	if err := repoA.RebaseContinue(); err != nil {
		t.Fatal(err)
	}
	if err := repoA.ClearState(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(a, "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "from b\n" {
		t.Fatalf("keepTheirs resolved to %q", data)
	}
}
