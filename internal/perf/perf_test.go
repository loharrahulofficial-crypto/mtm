package perf

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func resetEnv(t *testing.T) {
	t.Helper()
	t.Setenv("MTM_PPROF", "")
	ResetForTest()
}

func TestCountView(t *testing.T) {
	resetEnv(t)
	CountView(120)
	CountView(80)
	if got := Views(); got != 2 {
		t.Fatalf("Views = %d, want 2", got)
	}
	if got := ViewBytes(); got != 200 {
		t.Fatalf("ViewBytes = %d, want 200", got)
	}
}

func TestSetupNoRequestIsNoop(t *testing.T) {
	resetEnv(t)
	stop, err := Setup("")
	if err != nil {
		t.Fatal(err)
	}
	stop()
}

func TestSetupRefusesOutsideSandbox(t *testing.T) {
	resetEnv(t)
	// Sandbox flag unset: must refuse before touching anything.
	t.Setenv("MTM_SANDBOX", "")
	cfg := filepath.Join(os.TempDir(), "mtm-perf-test.pprof")
	_, err := Setup(cfg)
	if err == nil {
		t.Fatal("expected refusal outside the sandbox")
	}
	if !strings.Contains(err.Error(), "refusing to profile") {
		t.Fatalf("error = %v", err)
	}
	if _, statErr := os.Stat(cfg); !os.IsNotExist(statErr) {
		t.Fatal("profile file must not be created outside the sandbox")
	}
}

func TestSetupWritesProfileAndStats(t *testing.T) {
	resetEnv(t)
	root := t.TempDir()
	t.Setenv("MTM_SANDBOX", "1")
	t.Setenv("HOME", filepath.Join(root, "home"))
	for _, k := range []string{"XDG_DATA_HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME"} {
		t.Setenv(k, filepath.Join(root, k))
	}
	t.Setenv("OMARCHY_STATE_HOME", filepath.Join(root, "omarchy"))
	out := filepath.Join(root, "mtm.cpu.pprof")
	stop, err := Setup(out)
	if err != nil {
		t.Fatal(err)
	}
	CountView(42)
	stop()
	data, err := os.ReadFile(out + ".stats.json")
	if err != nil {
		t.Fatal(err)
	}
	var stats Stats
	if err := json.Unmarshal(data, &stats); err != nil {
		t.Fatal(err)
	}
	if stats.Views != 1 || stats.ViewBytes != 42 {
		t.Fatalf("stats = %+v, want views=1 bytes=42", stats)
	}
	if stats.Mallocs == 0 {
		t.Fatal("stats.Mallocs must be populated")
	}
	info, err := os.Stat(out)
	if err != nil {
		t.Fatal("cpu profile not written:", err)
	}
	if info.Size() == 0 {
		t.Fatal("cpu profile is empty")
	}
}

func TestSetupRejectsUnknownKind(t *testing.T) {
	resetEnv(t)
	t.Setenv("MTM_SANDBOX", "1")
	t.Setenv("MTM_PPROF", "heap:/tmp/mtm-perf-heap.pprof")
	_, err := Setup("")
	if err == nil || !strings.Contains(err.Error(), "unknown profile kind") {
		t.Fatalf("err = %v", err)
	}
}
