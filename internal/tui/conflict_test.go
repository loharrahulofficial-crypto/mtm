package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
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

const taskFileName = "a1b2c3-test-task.md"

func taskContent(body string) string {
	return "---\nid: a1b2c3\ntitle: test task\nproject: inbox\nstatus: todo\npriority: medium\nkind: task\ncreated: 2026-11-07T10:00\nupdated: 2026-11-07T10:00\n---\n" + body + "\n"
}

// setupConflictedRepo manufactures a pull --rebase conflict on a data repo.
func setupConflictedRepo(t *testing.T) (a string) {
	t.Helper()
	base := t.TempDir()
	origin := filepath.Join(base, "origin.git")
	a = filepath.Join(base, "a")
	b := filepath.Join(base, "b")
	gitCmd(t, base, "test", "t@example.com", "init", "--bare", "-b", "main", origin)
	gitCmd(t, base, "test", "t@example.com", "clone", origin, a)
	gitCmd(t, base, "test", "t@example.com", "clone", origin, b)
	for _, d := range []string{a, b} {
		gitCmd(t, d, "test", "t@example.com", "config", "user.name", "test")
		gitCmd(t, d, "test", "t@example.com", "config", "user.email", "t@example.com")
	}
	plant := func(dir, content string) {
		mtdir := filepath.Join(dir, "projects", "inbox")
		if err := os.MkdirAll(mtdir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(mtdir, taskFileName), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	plant(a, taskContent("notes v1"))
	gitCmd(t, a, "test", "t@example.com", "add", "-A")
	gitCmd(t, a, "test", "t@example.com", "commit", "-m", "init")
	gitCmd(t, a, "test", "t@example.com", "push", "-u", "origin", "main")
	gitCmd(t, b, "test", "t@example.com", "pull", "--rebase")

	plant(a, taskContent("notes from a"))
	gitCmd(t, a, "test", "t@example.com", "add", "-A")
	gitCmd(t, a, "test", "t@example.com", "commit", "-m", "a change")

	plant(b, taskContent("notes from b"))
	gitCmd(t, b, "test", "t@example.com", "add", "-A")
	gitCmd(t, b, "test", "t@example.com", "commit", "-m", "b change")
	gitCmd(t, b, "test", "t@example.com", "push")

	// A pulls → conflict; sync pauses and marks mtm-sync-state.json.
	repoA := sync.New(a)
	if _, err := repoA.Sync("origin"); err != sync.ErrConflicted {
		t.Fatalf("expected ErrConflicted, got %v", err)
	}
	return a
}

func conflictedModel(t *testing.T, a string) Model {
	t.Helper()
	cfg := config.Default()
	cfg.DataRepo = a
	cfg.Sync.Auto = false
	cfg.DefaultProject = "inbox"
	s, err := store.Open(a, filepath.Join(t.TempDir(), "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := New(cfg, s, sync.New(a))
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 130, Height: 40})
	return updated.(Model)
}

func TestConflictScreenShownAutomatically(t *testing.T) {
	a := setupConflictedRepo(t)
	m := conflictedModel(t, a)
	if m.mode != modeConflict {
		t.Fatalf("mode = %v, want conflict screen", m.mode)
	}
	if len(m.conflicts) != 1 || m.conflicts[0] != "projects/inbox/"+taskFileName {
		t.Fatalf("conflicts = %v", m.conflicts)
	}
	if out := m.View(); !strings.Contains(out, "sync paused") {
		t.Fatal("conflict screen missing headline")
	}
}

func TestConflictKeepMineThenContinue(t *testing.T) {
	a := setupConflictedRepo(t)
	m := conflictedModel(t, a)

	// Keep mine (local answer).
	updated, _ := m.Update(keyMsg("m"))
	m = updated.(Model)
	if !m.conflictDone["projects/inbox/"+taskFileName] {
		t.Fatal("file not marked resolved")
	}
	// Continue the rebase; resumes sync.
	updated, cmd := m.Update(keyMsg("c"))
	m = updated.(Model)
	if m.mode != modeNormal {
		t.Fatalf("mode after continue = %v", m.mode)
	}
	if st, err := sync.New(a).LoadState(); err == nil && st.Conflicted {
		t.Fatal("conflicted state not cleared")
	}
	data, err := os.ReadFile(filepath.Join(a, "projects", "inbox", taskFileName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "notes from a") {
		t.Fatalf("keep-mine resolved to %q", data)
	}
	// The follow-up sync command must succeed (rebase is done; push goes out).
	if cmd != nil {
		if msg := cmd(); msg != nil {
			updated, _ := m.Update(msg)
			m = updated.(Model)
		}
	}
	if m.syncSt == syncConflict {
		t.Fatal("sync still conflicted after resume")
	}
}

func TestConflictKeepTheirs(t *testing.T) {
	a := setupConflictedRepo(t)
	m := conflictedModel(t, a)
	updated, _ := m.Update(keyMsg("r"))
	m = updated.(Model)
	_ = m // state checked on disk after continue
	updated, cmd := m.Update(keyMsg("c"))
	m = updated.(Model)
	if cmd != nil {
		if msg := cmd(); msg != nil {
			updated, _ := m.Update(msg)
			m = updated.(Model)
		}
	}
	data, err := os.ReadFile(filepath.Join(a, "projects", "inbox", taskFileName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "notes from b") {
		t.Fatalf("keep-theirs resolved to %q", data)
	}
}

func TestConflictContinueRefusedWhileUnresolved(t *testing.T) {
	a := setupConflictedRepo(t)
	m := conflictedModel(t, a)
	updated, _ := m.Update(keyMsg("c"))
	m = updated.(Model)
	if m.mode != modeConflict {
		t.Fatal("continue allowed with unresolved conflicts")
	}
	if !strings.Contains(m.statusMsg, "resolve") {
		t.Fatalf("statusMsg = %q", m.statusMsg)
	}
}
