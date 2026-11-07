package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
)

func TestSandboxedGuard(t *testing.T) {
	// MTM_SANDBOX=1 always works.
	t.Setenv("MTM_SANDBOX", "1")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	if !sandboxed() {
		t.Fatal("MTM_SANDBOX=1 not accepted")
	}
	// All XDG roots under temp dirs works.
	t.Setenv("MTM_SANDBOX", "")
	os.Unsetenv("MTM_SANDBOX")
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("XDG_DATA_HOME", tmp)
	t.Setenv("XDG_CACHE_HOME", tmp)
	if !sandboxed() {
		t.Fatal("temp XDG roots not accepted")
	}
	// Real-looking XDG roots refuse.
	t.Setenv("XDG_CONFIG_HOME", "/home/rahul/.config")
	t.Setenv("XDG_DATA_HOME", "/home/rahul/.local/share")
	t.Setenv("XDG_CACHE_HOME", "/home/rahul/.cache")
	if sandboxed() {
		t.Fatal("real XDG roots accepted")
	}
	// Missing XDG roots refuse.
	os.Unsetenv("XDG_CONFIG_HOME")
	os.Unsetenv("XDG_DATA_HOME")
	os.Unsetenv("XDG_CACHE_HOME")
	if sandboxed() {
		t.Fatal("missing XDG roots accepted")
	}
}

func TestParseSize(t *testing.T) {
	w, h, err := parseSize("120x40")
	if err != nil || w != 120 || h != 40 {
		t.Fatalf("parseSize(120x40) = %d,%d,%v", w, h, err)
	}
	for _, bad := range []string{"", "big", "120", "120*40", "5x5", "999x999", "120X40x1"} {
		if _, _, err := parseSize(bad); err == nil {
			t.Errorf("parseSize(%q) accepted", bad)
		}
	}
}

func TestTuiDumpRefusesWithoutSandbox(t *testing.T) {
	os.Unsetenv("MTM_SANDBOX")
	os.Unsetenv("XDG_CONFIG_HOME")
	os.Unsetenv("XDG_DATA_HOME")
	os.Unsetenv("XDG_CACHE_HOME")
	cmd := newTuiCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--dump", "--view", "list"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("dump ran without sandbox")
	}
	if !strings.Contains(err.Error(), "not sandboxed") {
		t.Fatalf("wrong error: %v", err)
	}
}

func TestTuiDumpRendersFrame(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MTM_SANDBOX", "1")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))
	// Initialize a minimal data repo.
	root := filepath.Join(tmp, "data", "mtm")
	if err := os.MkdirAll(filepath.Join(root, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(root, filepath.Join(tmp, "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveProject(core.DefaultProject("inbox")); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	cmd := newTuiCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--dump", "--view", "list", "--no-color", "--size", "80x24"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "STS") || !strings.Contains(out, "DUE") {
		t.Fatalf("frame missing list chrome:\n%s", out)
	}
	if strings.Contains(out, "\x1b[") {
		t.Fatal("--no-color frame still contains ANSI escapes")
	}
}
