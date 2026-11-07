package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_AUTHOR_NAME", "test")
	t.Setenv("GIT_AUTHOR_EMAIL", "t@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "test")
	t.Setenv("GIT_COMMITTER_EMAIL", "t@example.com")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
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

func sandbox(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	return os.Getenv("XDG_DATA_HOME")
}

// seedRemote creates a bare repo on disk containing one commit.
func seedRemote(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	bare := filepath.Join(base, "origin.git")
	seed := filepath.Join(base, "seed")
	gitCmd(t, base, "init", "--bare", "-b", "main", bare)
	gitCmd(t, base, "clone", bare, seed)
	if err := os.WriteFile(filepath.Join(seed, "seed.txt"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, seed, "add", "-A")
	gitCmd(t, seed, "commit", "-m", "seed commit")
	gitCmd(t, seed, "push", "-u", "origin", "main")
	return bare
}

func TestInitCloneExistingRemote(t *testing.T) {
	gitEnv(t)
	sandbox(t)
	seed := seedRemote(t)
	root := filepath.Join(os.Getenv("XDG_DATA_HOME"), "mtm")

	if err := runInit(seed); err != nil {
		t.Fatal(err)
	}
	// Cloned repo should contain the seed commit.
	if _, err := os.Stat(filepath.Join(root, "seed.txt")); err != nil {
		t.Fatalf("clone did not happen: %v", err)
	}
	log := gitCmd(t, root, "log", "--oneline")
	if !strings.Contains(log, "seed commit") {
		t.Fatalf("seed commit missing from clone:\n%s", log)
	}
	// Layout should exist inside the clone.
	if _, err := os.Stat(filepath.Join(root, "projects", "inbox", "project.yaml")); err != nil {
		t.Fatalf("inbox project missing: %v", err)
	}
	// Remote must be set.
	if url := gitCmd(t, root, "remote", "get-url", "origin"); url != seed {
		t.Fatalf("origin = %q, want %q", url, seed)
	}
}

func TestInitRemoteOnAlreadyInitializedRepo(t *testing.T) {
	gitEnv(t)
	sandbox(t)
	seed := seedRemote(t)
	root := filepath.Join(os.Getenv("XDG_DATA_HOME"), "mtm")

	// Local init first (empty local repo, no remote).
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}
	// Now point it at the existing remote. Histories are unrelated, so the
	// push must not happen, but init itself must succeed and origin must be set.
	if err := runInit(seed); err != nil {
		t.Fatal(err)
	}
	if url := gitCmd(t, root, "remote", "get-url", "origin"); url != seed {
		t.Fatalf("origin = %q, want %q", url, seed)
	}
}

func TestInitRemoteOnEmptyRemotePushesUpstream(t *testing.T) {
	gitEnv(t)
	sandbox(t)
	base := t.TempDir()
	bare := filepath.Join(base, "empty.git")
	gitCmd(t, base, "init", "--bare", "-b", "main", bare)
	root := filepath.Join(os.Getenv("XDG_DATA_HOME"), "mtm")

	// Local init first without a remote, then attach the (empty) remote.
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}
	if err := runInit(bare); err != nil {
		t.Fatal(err)
	}
	// The remote should now have main.
	out := gitCmd(t, base, "ls-remote", bare)
	if !strings.Contains(out, "refs/heads/main") {
		t.Fatalf("remote main missing after push-upstream:\n%s", out)
	}
	// Ahead count should be zero.
	syncStatus := gitCmd(t, root, "rev-list", "--count", "@{upstream}..HEAD")
	if syncStatus != "0" {
		t.Fatalf("ahead = %s, want 0", syncStatus)
	}
}

func TestInitRemoteIdempotent(t *testing.T) {
	gitEnv(t)
	sandbox(t)
	base := t.TempDir()
	bare := filepath.Join(base, "empty.git")
	gitCmd(t, base, "init", "--bare", "-b", "main", bare)
	root := filepath.Join(os.Getenv("XDG_DATA_HOME"), "mtm")
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}
	if err := runInit(bare); err != nil {
		t.Fatal(err)
	}
	if err := runInit(bare); err != nil {
		t.Fatal(fmt.Sprintf("second init --remote should be idempotent: %v", err))
	}
	if url := gitCmd(t, root, "remote", "get-url", "origin"); url != bare {
		t.Fatalf("origin = %q, want %q", url, bare)
	}
}
