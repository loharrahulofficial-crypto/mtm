package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func doctorEnv(t *testing.T, repo string) {
	t.Helper()
	cfgDir := t.TempDir()
	cfg := fmt.Sprintf("data_repo: %s\ndefault_project: inbox\n", repo)
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MTM_CONFIG_DIR", cfgDir)
	t.Setenv("MTM_STATE_DIR", t.TempDir())
	t.Setenv("MTM_DATA_DIR", "")
	// Every mtm root must resolve under temp: Resolve() guards all four.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))
	t.Setenv("GIT_AUTHOR_NAME", "test")
	t.Setenv("GIT_AUTHOR_EMAIL", "t@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "test")
	t.Setenv("GIT_COMMITTER_EMAIL", "t@example.com")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
}

func initDataRepo(t *testing.T, root, taskID string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "projects", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := fmt.Sprintf("---\nid: %s\ntitle: t\nproject: inbox\nstatus: todo\npriority: medium\nkind: task\ncreated: 2026-11-01T09:00\nupdated: 2026-11-01T09:00\n---\n\n", taskID)
	if err := os.WriteFile(filepath.Join(root, "projects", "inbox", taskID+"-t.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-b", "main")
	run("add", "-A")
	run("commit", "-qm", "add first task")
}

func runDoctorCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newDoctorCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestDoctorHealthyRepoJSON(t *testing.T) {
	root := t.TempDir()
	doctorEnv(t, root)
	initDataRepo(t, root, "a1b2c3")
	if err := os.WriteFile(filepath.Join(root, ".mtm-data"), []byte("id: 11111111-2222-4333-8444-555555555555\ncreated: 2026-11-09\nkind: real\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", root, "add", "-A")
	_ = cmd.Run()
	_ = exec.Command("git", "-C", root, "commit", "-qm", "marker").Run()
	_ = exec.Command("git", "-C", root, "remote", "add", "origin", filepath.Join(t.TempDir(), "o.git")).Run()
	_ = exec.Command("git", "-C", root, "config", "user.name", "test").Run()
	_ = exec.Command("git", "-C", root, "config", "user.email", "t@example.com").Run()

	out, err := runDoctorCmd(t, "--json")
	if err != nil {
		t.Fatalf("doctor on a healthy repo failed: %v\n%s", err, out)
	}
	var rep struct {
		Root string `json:"root"`
		Kind string `json:"kind"`
		OK   bool   `json:"ok"`
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("--json output did not parse: %v\n%s", err, out)
	}
	if !rep.OK || rep.Kind != "real" {
		t.Fatalf("unexpected report: %+v", &rep)
	}
}

func TestDoctorExitsNonZeroOnDemoDataInRealRepo(t *testing.T) {
	root := t.TempDir()
	doctorEnv(t, root)
	initDataRepo(t, root, "od0001")
	if err := os.WriteFile(filepath.Join(root, ".mtm-data"), []byte("id: 11111111-2222-4333-8444-555555555555\ncreated: 2026-11-09\nkind: real\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runDoctorCmd(t)
	if err == nil {
		t.Fatalf("doctor exited 0 on demo data in a real repo:\n%s", out)
	}
	if !strings.Contains(out, "od0001") {
		t.Fatalf("output does not name the demo-looking task:\n%s", out)
	}
}
