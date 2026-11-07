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
		return State{}, fmt.Errorf("parse %s: %w", p, err)
	}
	return st, nil
}

// SaveState persists the sync state.
func (r Repo) SaveState(st State) error {
	p, err := r.statePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
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
// It returns 0 when there is no upstream.
func (r Repo) Ahead() (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := r.git(ctx, "rev-list", "--count", "@{upstream}..HEAD")
	if err != nil {
		return 0, nil // no upstream configured
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

// Sync pulls --rebase then pushes. It never auto-resolves a conflict: on
// conflict the repo is marked conflicted and ErrConflicted is returned.
// When no remote exists it is a no-op.
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
	ctx := context.Background()
	if _, err := r.gitDefault(ctx, "pull", "--rebase", remote); err != nil {
		files, ferr := r.ConflictedFiles()
		if ferr == nil && len(files) > 0 {
			_ = r.SaveState(State{Conflicted: true, Files: files, Since: time.Now(), Reason: err.Error()})
			return res, ErrConflicted
		}
		// Not a conflict (offline, auth, etc.) — leave state untouched.
		return res, err
	}
	res.Pulled = true
	// Push, setting upstream on first push.
	if _, err := r.gitDefault(ctx, "rev-parse", "--abbrev-ref", "@{upstream}"); err != nil {
		branch, berr := r.Branch()
		if berr != nil {
			return res, berr
		}
		if _, err := r.gitDefault(ctx, "push", "-u", remote, branch); err != nil {
			return res, err
		}
		res.Pushed = true
		return res, nil
	}
	if _, err := r.gitDefault(ctx, "push"); err != nil {
		return res, err
	}
	res.Pushed = true
	return res, nil
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
	Initialized bool
	Branch      string
	Remote      string
	RemoteURL   string
	Ahead       int
	State       State
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
	}
	st.Ahead, _ = r.Ahead()
	st.State, _ = r.LoadState()
	return st
}
