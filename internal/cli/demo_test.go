package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// sandboxedEnv redirects HOME + XDG roots into temp dirs AND sets
// MTM_SANDBOX=1 — the environment `make demo` runs under.
func sandboxedEnv(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("MTM_SANDBOX", "1")
	t.Setenv("HOME", filepath.Join(tmp, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))
	t.Setenv("MTM_DATA_DIR", "")
	t.Setenv("MTM_CONFIG_DIR", "")
	t.Setenv("MTM_STATE_DIR", "")
	return tmp
}

func runDemoSeed(args ...string) error {
	cmd := newDemoSeedCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	return cmd.Execute()
}

// TestDemoSeedRefusesOutsideSandbox: no MTM_SANDBOX, real-looking XDG
// roots — the command must refuse and must not create the target.
func TestDemoSeedRefusesOutsideSandbox(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MTM_SANDBOX", "")
	u, err := user.Current()
	if err != nil || u.HomeDir == "" {
		t.Skip("no passwd home")
	}
	// Real-looking XDG roots (the environment that caused the incident).
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(u.HomeDir, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(u.HomeDir, ".local", "share"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(u.HomeDir, ".cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(u.HomeDir, ".local", "state"))

	target := filepath.Join(tmp, "projects")
	err = runDemoSeed(target)
	if err == nil {
		t.Fatal("demo-seed ran outside a sandbox")
	}
	if !strings.Contains(err.Error(), "not sandboxed") {
		t.Fatalf("wrong error: %v", err)
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatal("target was created despite refusal")
	}
}

// TestDemoSeedRefusesUnderRealHome: MTM_SANDBOX=1 with valid temp roots,
// but a target under the real home directory — must refuse before any
// write. Cleanup removes the target dir as a backstop in case the guard
// ever regresses.
func TestDemoSeedRefusesUnderRealHome(t *testing.T) {
	sandboxedEnv(t)
	u, err := user.Current()
	if err != nil || u.HomeDir == "" {
		t.Skip("no passwd home")
	}
	name := fmt.Sprintf("mtm-demo-seed-test-%d", os.Getpid())
	base := filepath.Join(u.HomeDir, name)
	t.Cleanup(func() { _ = os.RemoveAll(base) })
	target := filepath.Join(base, "projects")
	err = runDemoSeed(target)
	if err == nil {
		t.Fatal("demo-seed under the real home directory was admitted")
	}
	if !strings.Contains(err.Error(), "real home") && !strings.Contains(err.Error(), "not sandboxed") {
		t.Fatalf("wrong error: %v", err)
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatal("target was created despite refusal")
	}
}

// TestDemoSeedSucceedsInSandbox: full sandbox env, seeds the corpus and
// writes a kind: demo marker at the repo root.
func TestDemoSeedSucceedsInSandbox(t *testing.T) {
	tmp := sandboxedEnv(t)
	data := filepath.Join(tmp, "data", "mtm")
	target := filepath.Join(data, "projects")
	if err := runDemoSeed(target); err != nil {
		t.Fatalf("sandboxed demo-seed failed: %v", err)
	}
	for _, rel := range []string{
		"college/dbms/dn0001-done-dn0001.md",
		"college/os/ex0002-os-quiz.md",
		"inbox/td0007-mess-bill.md",
	} {
		if _, err := os.Stat(filepath.Join(target, rel)); err != nil {
			t.Errorf("expected seeded file %s: %v", rel, err)
		}
	}
	// Marker written at the repo root with kind demo.
	raw, err := os.ReadFile(filepath.Join(data, ".mtm-data"))
	if err != nil {
		t.Fatalf("marker not written: %v", err)
	}
	var mk struct {
		ID      string `yaml:"id"`
		Created string `yaml:"created"`
		Kind    string `yaml:"kind"`
	}
	if err := yaml.Unmarshal(raw, &mk); err != nil {
		t.Fatalf("marker not YAML: %v", err)
	}
	if mk.Kind != "demo" {
		t.Fatalf("marker kind = %q, want demo", mk.Kind)
	}
	if mk.ID == "" || mk.Created == "" {
		t.Fatalf("marker missing id or created: %+v", mk)
	}
}

// TestDemoSeedRefusesRealMarker: a repo whose .mtm-data says kind: real
// is never seeded, even inside a sandbox.
func TestDemoSeedRefusesRealMarker(t *testing.T) {
	tmp := sandboxedEnv(t)
	data := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(data, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := "id: 11111111-2222-4333-8444-555555555555\ncreated: 2026-11-09\nkind: real\n"
	if err := os.WriteFile(filepath.Join(data, ".mtm-data"), []byte(marker), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(data, "projects")
	err := runDemoSeed(target)
	if err == nil {
		t.Fatal("demo-seed wrote into a kind: real repo")
	}
	if !strings.Contains(err.Error(), "kind: real") {
		t.Fatalf("wrong error: %v", err)
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatal("target was created despite the real marker")
	}
}

// TestDemoSeedAcceptsDemoMarker: an existing kind: demo marker does not
// block a re-seed.
func TestDemoSeedAcceptsDemoMarker(t *testing.T) {
	tmp := sandboxedEnv(t)
	data := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(data, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := "id: 11111111-2222-4333-8444-555555555555\ncreated: 2026-11-09\nkind: demo\n"
	if err := os.WriteFile(filepath.Join(data, ".mtm-data"), []byte(marker), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runDemoSeed(filepath.Join(data, "projects")); err != nil {
		t.Fatalf("demo-seed refused an existing demo repo: %v", err)
	}
}
