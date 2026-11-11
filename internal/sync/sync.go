// Package sync shells out to git (D-002) for auto-commit and auto-sync.
// A pull/rebase conflict pauses sync: the repo is marked conflicted in
// <gitdir>/mtm-sync-state.json and never auto-resolved (D-003).
package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Repo is a git repository at Path.
type Repo struct{ Path string }

// New returns a Repo rooted at path.
func New(path string) Repo { return Repo{Path: path} }

// Clone clones url into dest (which must not exist or be empty).
func Clone(url, dest string) error {
	parent := filepath.Dir(dest)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "clone", url, dest)
	cmd.Dir = parent
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// IsUnrelatedHistories reports whether err came from a pull/rebase that
// refused because the local and remote histories share no ancestors.
func IsUnrelatedHistories(err error) bool {
	return err != nil && strings.Contains(err.Error(), "unrelated histories")
}

// State is the persisted sync state. Conflicted pauses all auto-sync.
type State struct {
	Conflicted bool      `json:"conflicted"`
	Files      []string  `json:"files,omitempty"`
	Since      time.Time `json:"since,omitempty"`
	Reason     string    `json:"reason,omitempty"`
}

func (r Repo) git(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.Path
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(out))
	if err != nil {
		return trimmed, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, trimmed)
	}
	return trimmed, nil
}

func (r Repo) gitDefault(ctx context.Context, args ...string) (string, error) {
	ctx2, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return r.git(ctx2, args...)
}

// IsRepo reports whether Path is inside a git work tree.
func (r Repo) IsRepo() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := r.git(ctx, "rev-parse", "--is-inside-work-tree")
	return err == nil && out == "true"
}

// Init initializes a git repo with an initial branch named main.
func (r Repo) Init() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := r.git(ctx, "init", "-b", "main")
	return err
}

// GitDir returns the absolute git directory (handles worktrees).
func (r Repo) GitDir() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return r.git(ctx, "rev-parse", "--absolute-git-dir")
}

// ConfigGet returns the trimmed value of a git config key in this repo
// (repo-local, with global/system fallback), or "" when unset. Read-only.
func (r Repo) ConfigGet(key string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := r.git(ctx, "config", "--get", key)
	if err != nil {
		return ""
	}
	return out
}

// Remotes lists the configured remote names ("git remote"). Read-only.
func (r Repo) Remotes() []string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := r.git(ctx, "remote")
	if err != nil || out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// LogSubjects returns every commit subject in the repo (newest first),
// bounded at 1000 commits. Read-only.
func (r Repo) LogSubjects() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := r.git(ctx, "log", "--format=%s", "-n", "1000")
	if err != nil {
		// Unborn branch / no commits: not an error, just no subjects.
		return nil, nil
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// Branch returns the current branch name (works on an unborn branch).
func (r Repo) Branch() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return r.git(ctx, "symbolic-ref", "--short", "HEAD")
}

// HasRemote reports whether a remote exists.
func (r Repo) HasRemote(remote string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.git(ctx, "remote", "get-url", remote)
	return err == nil
}

// RemoteURL returns the URL of a remote.
func (r Repo) RemoteURL(remote string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return r.git(ctx, "remote", "get-url", remote)
}

// AddRemote adds or replaces a remote.
func (r Repo) AddRemote(remote, url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := r.git(ctx, "remote", "get-url", remote); err == nil {
		_, err = r.git(ctx, "remote", "set-url", remote, url)
		return err
	}
	_, err := r.git(ctx, "remote", "add", remote, url)
	return err
}

// CommitAll stages everything and commits. committed is false when there
// was nothing to commit.
func (r Repo) CommitAll(msg string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := r.git(ctx, "add", "-A"); err != nil {
		return false, err
	}
	out, err := r.git(ctx, "commit", "-m", msg)
	if err != nil {
		if strings.Contains(out, "nothing to commit") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ConflictedFiles lists unmerged paths (diff-filter=U).
func (r Repo) ConflictedFiles() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := r.git(ctx, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

func (r Repo) statePath() (string, error) {
	gitDir, err := r.GitDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(gitDir, "mtm-sync-state.json"), nil
}

// LoadState reads the persisted sync state.
func (r Repo) LoadState() (State, error) {
	p, err := r.statePath()
	if err != nil {
		return State{}, err
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return State{}, nil
	}
	if err != nil {
		return State{}, err
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return State{}, fmt.Errorf("parse %s: %w (delete the file to resume sync from a clean slate; conflicts are never auto-resolved)", p, err)
	}
	return st, nil
}

// SaveState persists the sync state atomically (temp+rename+fsync), so a
// crash mid-write never leaves a corrupt mtm-sync-state.json behind.
func (r Repo) SaveState(st State) error {
	p, err := r.statePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(p)
	tmp, err := os.CreateTemp(dir, ".mtm-sync-state-*")
	if err != nil {
		return fmt.Errorf("create temp sync state: %w", err)
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write sync state: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync sync state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close sync state: %w", err)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return fmt.Errorf("chmod sync state: %w", err)
	}
	if err := os.Rename(tmpName, p); err != nil {
		return fmt.Errorf("rename sync state: %w", err)
	}
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	ok = true
	return nil
}

// ClearState removes the persisted sync state.
func (r Repo) ClearState() error {
	p, err := r.statePath()
	if err != nil {
		return err
	}
	err = os.Remove(p)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Ahead returns how many local commits are not on the upstream branch.
// It returns 0 when there is no upstream configured; any other git error
// is reported, not silently treated as "in sync".
func (r Repo) Ahead() (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// No upstream → 0, by definition. Any other failure is an error.
	if _, err := r.git(ctx, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"); err != nil {
		return 0, nil
	}
	out, err := r.git(ctx, "rev-list", "--count", "@{upstream}..HEAD")
	if err != nil {
		return 0, fmt.Errorf("rev-list @{upstream}..HEAD: %w", err)
	}
	var n int
	_, err = fmt.Sscanf(out, "%d", &n)
	return n, err
}

// SyncResult describes what happened during Sync.
type SyncResult struct {
	Pulled bool
	Pushed bool
}

// ErrConflicted is returned when sync pauses due to a conflict (D-003).
var ErrConflicted = errors.New("sync paused: conflicts need manual resolution")

// HasRemoteBranch reports whether remote has a branch named branch.
func (r Repo) HasRemoteBranch(remote, branch string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := r.git(ctx, "ls-remote", "--heads", remote, branch)
	if err != nil {
		return false, err
	}
	return out != "", nil
}

func (r Repo) pullOrConflict(args ...string) (bool, error) {
	ctx := context.Background()
	if _, err := r.gitDefault(ctx, append([]string{"pull", "--rebase"}, args...)...); err != nil {
		files, ferr := r.ConflictedFiles()
		if ferr == nil && len(files) > 0 {
			_ = r.SaveState(State{Conflicted: true, Files: files, Since: time.Now(), Reason: err.Error()})
			return false, ErrConflicted
		}
		// Not a conflict (offline, auth, etc.) — leave state untouched.
		return false, err
	}
	return true, nil
}

// Sync pulls --rebase then pushes. It never auto-resolves a conflict: on
// conflict the repo is marked conflicted and ErrConflicted is returned.
// When no remote exists it is a no-op; when the remote lacks the current
// branch (fresh remote) the pull is skipped.
func (r Repo) Sync(remote string) (SyncResult, error) {
	var res SyncResult
	st, err := r.LoadState()
	if err != nil {
		return res, err
	}
	if st.Conflicted {
		return res, ErrConflicted
	}
	if !r.HasRemote(remote) {
		return res, nil
	}
	branch, err := r.Branch()
	if err != nil {
		return res, err
	}
	_, upstreamErr := func() (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return r.gitDefault(ctx, "rev-parse", "--abbrev-ref", "@{upstream}")
	}()
	if upstreamErr == nil {
		pulled, err := r.pullOrConflict()
		if err != nil {
			return res, err
		}
		res.Pulled = pulled
	} else {
		exists, err := r.HasRemoteBranch(remote, branch)
		if err != nil {
			return res, err
		}
		if exists {
			pulled, err := r.pullOrConflict(remote, branch)
			if err != nil {
				return res, err
			}
			res.Pulled = pulled
		} // fresh remote: nothing to pull
	}
	// Push, setting upstream on first push.
	if _, err := r.gitDefault(context.Background(), "rev-parse", "--abbrev-ref", "@{upstream}"); err != nil {
		if _, err := r.gitDefault(context.Background(), "push", "-u", remote, branch); err != nil {
			return res, err
		}
		res.Pushed = true
		return res, nil
	}
	if _, err := r.gitDefault(context.Background(), "push"); err != nil {
		return res, err
	}
	res.Pushed = true
	return res, nil
}

// RebaseInProgress reports whether a git rebase is in progress
// (rebase-merge or rebase-apply metadata exists in the git dir).
func (r Repo) RebaseInProgress() bool {
	gd, err := r.GitDir()
	if err != nil {
		return false
	}
	for _, d := range []string{"rebase-merge", "rebase-apply"} {
		if fi, err := os.Stat(filepath.Join(gd, d)); err == nil && fi.IsDir() {
			return true
		}
	}
	return false
}

// ResolveConflict stages a conflicted file choosing one side.
// During a rebase, "mine" (your local commit) is git's --theirs and "theirs"
// (the remote you rebase onto) is --ours. keepMine selects yours.
// The file must currently be unmerged; the chosen side is checked out and
// the path is staged. Never called implicitly by sync.
func (r Repo) ResolveConflict(path string, keepMine bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	side := "--ours" // remote version
	if keepMine {
		side = "--theirs" // local version
	}
	if _, err := r.git(ctx, "checkout", side, "--", path); err != nil {
		return err
	}
	_, err := r.git(ctx, "add", "--", path)
	return err
}

// AddConflictPath stages an edited conflicted file (used after $EDITOR).
func (r Repo) AddConflictPath(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := r.git(ctx, "add", "--", path)
	return err
}

// RebaseContinue continues an in-progress rebase accepting the default
// commit message (GIT_EDITOR=true, never opens an editor).
func (r Repo) RebaseContinue() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "rebase", "--continue")
	cmd.Dir = r.Path
	cmd.Env = append(os.Environ(), "GIT_EDITOR=true", "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git rebase --continue: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Abort stops an in-progress rebase and clears the conflicted state.
func (r Repo) Abort() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _ = r.git(ctx, "rebase", "--abort") // tolerate "no rebase in progress"
	return r.ClearState()
}

// Status describes the repo's sync status for `mtm sync --status`.
type Status struct {
	Initialized      bool
	Branch           string
	Remote           string
	RemoteURL        string
	RemoteConfigured bool
	Ahead            int
	State            State
}

// GetStatus gathers sync status; zero values when not a git repo.
func (r Repo) GetStatus(remote string) Status {
	st := Status{Initialized: r.IsRepo()}
	if !st.Initialized {
		return st
	}
	if b, err := r.Branch(); err == nil {
		st.Branch = b
	}
	st.Remote = remote
	if url, err := r.RemoteURL(remote); err == nil {
		st.RemoteURL = url
		st.RemoteConfigured = true
	}
	st.Ahead, _ = r.Ahead()
	st.State, _ = r.LoadState()
	return st
}
