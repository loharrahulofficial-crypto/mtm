package actions

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/clock"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
)

// linkedStoreWithRepo returns an open store whose college/dbms project is
// linked to a fresh temp git repo seeded with commits at the given dates
// (RFC3339 with offset). The commits land on branch main.
func linkedStoreWithRepo(t *testing.T, commitDates []time.Time) (*store.Store, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(t.TempDir(), "index.json")
	s, err := store.Open(root, cache)
	if err != nil {
		t.Fatal(err)
	}
	s.Clock = clock.Fixed(time.Date(2026, 11, 7, 14, 0, 0, 0, time.Local))
	if err := s.SaveProject(core.DefaultProject("college/dbms")); err != nil {
		t.Fatal(err)
	}
	// Build the repo.
	repo := t.TempDir()
	gitEnv := append(os.Environ(),
		"GIT_AUTHOR_NAME=mtm", "GIT_AUTHOR_EMAIL=mtm@example.invalid",
		"GIT_COMMITTER_NAME=mtm", "GIT_COMMITTER_EMAIL=mtm@example.invalid")
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = gitEnv
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	git("init", "-q", "-b", "main")
	for i, d := range commitDates {
		f := filepath.Join(repo, fmt.Sprintf("f%d.txt", i))
		if err := os.WriteFile(f, []byte(fmt.Sprintf("content %d\n", i)), 0o644); err != nil {
			t.Fatal(err)
		}
		git("add", "-A")
		git("commit", "-q", "--date", d.Format(time.RFC3339), "-m", fmt.Sprintf("commit %d", i))
	}
	// Link it.
	p, err := s.GetProject("college/dbms")
	if err != nil {
		t.Fatal(err)
	}
	p.GitHub = "loharrahulofficial-crypto/mtm"
	p.RepoPath = repo
	if err := s.SaveProject(p); err != nil {
		t.Fatal(err)
	}
	return s, repo
}

func TestLoadGitHubLocalStats(t *testing.T) {
	now := time.Date(2026, 11, 7, 14, 0, 0, 0, time.Local)
	dates := []time.Time{
		now.AddDate(0, 0, -1),  // yesterday — streak 2
		now.AddDate(0, 0, -2),
		now.AddDate(0, 0, -8),  // 1 week back
		now.AddDate(0, 0, -9),
		now.AddDate(0, 0, -30), // 4 weeks back
	}
	s, _ := linkedStoreWithRepo(t, dates)
	defer func() { _ = s.Close() }()

	info, err := LoadGitHub(s, "college/dbms", now, false)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Linked || !info.LocalOK {
		t.Fatalf("linked=%v localOK=%v note=%q", info.Linked, info.LocalOK, info.LocalNote)
	}
	total := 0
	for _, n := range info.Weeks {
		total += int(n)
	}
	if total != len(dates) {
		t.Fatalf("window commits = %d, want %d (weeks: %v)", total, len(dates), info.Weeks)
	}
	if info.Streak != 2 {
		t.Fatalf("streak = %d, want 2 (yesterday+day before)", info.Streak)
	}
	if len(info.Subjects) == 0 {
		t.Fatal("no subjects")
	}
	// Subjects are newest first (the last-written commit is the newest).
	if !strings.HasPrefix(info.Subjects[0], "commit 4") {
		t.Fatalf("subjects = %v, want newest first", info.Subjects)
	}
	if info.LastCommit.Before(now.AddDate(0, 0, -2)) {
		t.Fatalf("last commit = %v, want within the last 2 days", info.LastCommit)
	}
	// No remote info requested.
	if info.RemoteOK || info.RemoteNote != "" {
		t.Fatalf("remote touched with fetchRemote=false: ok=%v note=%q", info.RemoteOK, info.RemoteNote)
	}
}

func TestLoadGitHubStreakAnchorYesterday(t *testing.T) {
	now := time.Date(2026, 11, 7, 14, 0, 0, 0, time.Local)
	// No commits today; yesterday and before are consecutive. A gap at
	// d-4 stops the run even though d-5 and d-6 have commits.
	dates := []time.Time{
		now.AddDate(0, 0, -1),
		now.AddDate(0, 0, -2),
		now.AddDate(0, 0, -3),
		now.AddDate(0, 0, -5),
		now.AddDate(0, 0, -6),
	}
	s, _ := linkedStoreWithRepo(t, dates)
	defer func() { _ = s.Close() }()
	info, err := LoadGitHub(s, "college/dbms", now, false)
	if err != nil {
		t.Fatal(err)
	}
	if info.Streak != 3 {
		t.Fatalf("streak = %d, want 3 (anchored on yesterday, gap at d-4)", info.Streak)
	}
}

func TestLoadGitHubNotLinked(t *testing.T) {
	s := openIdeaStore(t)
	defer func() { _ = s.Close() }()
	now := time.Date(2026, 11, 7, 14, 0, 0, 0, time.Local)
	info, err := LoadGitHub(s, "college/dbms", now, true)
	if err != nil {
		t.Fatal(err)
	}
	if info.Linked {
		t.Fatal("unlinked project reported as linked")
	}
	if info.RepoSlug != "" || info.RepoPath != "" {
		t.Fatalf("info = %+v, want empty slug/path", info)
	}
}

func TestLoadGitHubNonRepoPath(t *testing.T) {
	s := openIdeaStore(t)
	defer func() { _ = s.Close() }()
	p, err := s.GetProject("college/dbms")
	if err != nil {
		t.Fatal(err)
	}
	p.RepoPath = t.TempDir() // exists, not a repo
	if err := s.SaveProject(p); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 7, 14, 0, 0, 0, time.Local)
	info, err := LoadGitHub(s, "college/dbms", now, false)
	if err != nil {
		t.Fatal(err)
	}
	if info.LocalOK {
		t.Fatal("non-repo reported localOK")
	}
	if info.LocalNote == "" {
		t.Fatal("non-repo reported no local note")
	}
}

// sandboxAll redirects every mtm path root (HOME + all XDG dirs) into a
// throwaway dir so paths.Guard admits cache access under tests.
func sandboxAll(t *testing.T) {
	t.Helper()
	sandbox := t.TempDir()
	t.Setenv("HOME", sandbox)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(sandbox, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(sandbox, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(sandbox, "cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(sandbox, "state"))
}

// fakeGHScript writes a stub gh that appends every invocation to a counter
// file and answers auth/issue/pr queries with canned JSON.
func fakeGHScript(t *testing.T) (script, counter string) {
	t.Helper()
	dir := t.TempDir()
	counter = filepath.Join(dir, "gh-count")
	script = filepath.Join(dir, "gh")
	body := `#!/bin/sh
echo "$*" >> "` + counter + `"
case "$*" in
  "auth status"*) exit 0;;
  "issue list"*) printf '%s' '[{"number":12,"title":"Sync bug","url":"https://github.com/x/mtm/issues/12"},{"number":9,"title":"Waybar glitch","url":"https://github.com/x/mtm/issues/9"}]'; exit 0;;
  "pr list"*) printf '%s' '[{"number":18,"title":"board columns","url":"https://github.com/x/mtm/pull/18"}]'; exit 0;;
esac
exit 1
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	// Full sandbox: every mtm root redirected so paths.Guard admits the run.
	sandbox := t.TempDir()
	t.Setenv("HOME", sandbox)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(sandbox, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(sandbox, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(sandbox, "cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(sandbox, "state"))
	return script, counter
}

func ghCount(t *testing.T, counter string) int {
	t.Helper()
	data, err := os.ReadFile(counter)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return len(strings.Split(strings.TrimRight(string(data), "\n"), "\n"))
}

func TestLoadGitHubRemoteAndCacheTTL(t *testing.T) {
	now := time.Date(2026, 11, 7, 14, 0, 0, 0, time.Local)
	s, _ := linkedStoreWithRepo(t, []time.Time{now.AddDate(0, 0, -1)})
	defer func() { _ = s.Close() }()
	script, counter := fakeGHScript(t)
	t.Setenv("MTM_GH", script)

	// First call: cache miss, gh runs (auth + 2 list calls).
	info, err := LoadGitHub(s, "college/dbms", now, true)
	if err != nil {
		t.Fatal(err)
	}
	if !info.RemoteOK {
		t.Fatalf("remote not ok: note=%q", info.RemoteNote)
	}
	if info.OpenIssues != 2 || info.OpenPRs != 1 {
		t.Fatalf("counts = issues %d PRs %d, want 2/1", info.OpenIssues, info.OpenPRs)
	}
	if len(info.LatestIssues) != 2 || info.LatestIssues[0].Title != "Sync bug" {
		t.Fatalf("latest issues = %+v", info.LatestIssues)
	}
	first := ghCount(t, counter)
	if first == 0 {
		t.Fatal("gh never invoked on a cache miss")
	}

	// Second call 5 minutes later: cache valid, gh not invoked again.
	now2 := now.Add(5 * time.Minute)
	info, err = LoadGitHub(s, "college/dbms", now2, true)
	if err != nil {
		t.Fatal(err)
	}
	if !info.RemoteOK || info.OpenIssues != 2 {
		t.Fatalf("cached info = %+v", info)
	}
	if got := ghCount(t, counter); got != first {
		t.Fatalf("gh invoked again within the TTL (%d vs %d)", got, first)
	}

	// Third call 16 minutes later: cache expired, gh runs again.
	now3 := now.Add(16 * time.Minute)
	info, err = LoadGitHub(s, "college/dbms", now3, true)
	if err != nil {
		t.Fatal(err)
	}
	if !info.RemoteOK {
		t.Fatalf("remote after expiry = %+v note=%q", info, info.RemoteNote)
	}
	if got := ghCount(t, counter); got <= first {
		t.Fatalf("gh not invoked after TTL expiry (%d vs %d)", got, first)
	}
}

func TestLoadGitHubRemoteUnavailableDegrades(t *testing.T) {
	now := time.Date(2026, 11, 7, 14, 0, 0, 0, time.Local)
	s, _ := linkedStoreWithRepo(t, []time.Time{now.AddDate(0, 0, -1)})
	defer func() { _ = s.Close() }()

	// MTM_GH pointing at a missing binary → degraded note, no crash.
	t.Setenv("MTM_GH", filepath.Join(t.TempDir(), "no-such-gh"))
	sandboxAll(t)
	info, err := LoadGitHub(s, "college/dbms", now, true)
	if err != nil {
		t.Fatal(err)
	}
	if info.RemoteOK {
		t.Fatal("missing gh reported remote OK")
	}
	if !strings.Contains(info.RemoteNote, "not installed") {
		t.Fatalf("note = %q, want 'gh not installed'", info.RemoteNote)
	}

	// gh present but auth fails → auth note.
	dir := t.TempDir()
	script := filepath.Join(dir, "gh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MTM_GH", script)
	sandboxAll(t)
	info, err = LoadGitHub(s, "college/dbms", now, true)
	if err != nil {
		t.Fatal(err)
	}
	if info.RemoteOK {
		t.Fatal("unauthenticated gh reported remote OK")
	}
	if !strings.Contains(info.RemoteNote, "not authenticated") {
		t.Fatalf("note = %q, want auth note", info.RemoteNote)
	}
}

