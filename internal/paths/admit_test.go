package paths

import (
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

// setSandboxEnv points every mtm root at the given temp dir and sets
// MTM_SANDBOX. Pass "" for mtmSandbox to disable it.
func setSandboxEnv(t *testing.T, tmp, mtmSandbox string) {
	t.Helper()
	t.Setenv("MTM_SANDBOX", mtmSandbox)
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))
	t.Setenv("MTM_DATA_DIR", "")
	t.Setenv("MTM_CONFIG_DIR", "")
	t.Setenv("MTM_STATE_DIR", "")
}

func TestAdmitSandboxAdmissionMatrix(t *testing.T) {
	tmp := t.TempDir()

	// 1. MTM_SANDBOX=1 + all roots under a temp dir → admitted.
	setSandboxEnv(t, tmp, "1")
	if err := AdmitSandbox(); err != nil {
		t.Fatalf("valid sandbox refused: %v", err)
	}

	// 2. Without MTM_SANDBOX → refused even with temp roots.
	setSandboxEnv(t, tmp, "")
	if err := AdmitSandbox(); err == nil || !strings.Contains(err.Error(), "MTM_SANDBOX=1") {
		t.Fatalf("missing MTM_SANDBOX not refused properly: %v", err)
	}

	// 3. MTM_SANDBOX=1 but an XDG root is real → refused.
	setSandboxEnv(t, tmp, "1")
	t.Setenv("XDG_DATA_HOME", "/home")
	if err := AdmitSandbox(); err == nil {
		t.Fatal("real XDG_DATA_HOME admitted")
	}

	// 4. MTM_SANDBOX=1 with roots unset and HOME outside temp → refused.
	setSandboxEnv(t, tmp, "1")
	home, err := user.Current()
	if err != nil || home.HomeDir == "" {
		t.Skip("no passwd home")
	}
	t.Setenv("HOME", home.HomeDir)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	if err := AdmitSandbox(); err == nil {
		t.Fatal("real HOME with unset roots admitted")
	}

	// 5. Admitted, but an extra target under the real home → refused.
	setSandboxEnv(t, tmp, "1")
	if err := AdmitSandbox(filepath.Join(home.HomeDir, "mtm-admit-test-target")); err == nil {
		t.Fatal("extra target under real home admitted")
	}

	// 6. Admitted, extra target under temp → nil.
	setSandboxEnv(t, tmp, "1")
	if err := AdmitSandbox(filepath.Join(tmp, "target")); err != nil {
		t.Fatalf("temp target refused: %v", err)
	}

	// 7. MTM_DATA_DIR override pointing at the real home → refused.
	setSandboxEnv(t, tmp, "1")
	t.Setenv("MTM_DATA_DIR", filepath.Join(home.HomeDir, "mtm-admit-test-data"))
	if err := AdmitSandbox(); err == nil {
		t.Fatal("MTM_DATA_DIR under real home admitted")
	}
}

func TestUnderRealHome(t *testing.T) {
	home, err := user.Current()
	if err != nil || home.HomeDir == "" {
		t.Skip("no passwd home")
	}
	if !UnderRealHome(filepath.Join(home.HomeDir, "x")) {
		t.Fatal("path under real home not detected")
	}
	if UnderRealHome(t.TempDir()) {
		t.Fatal("temp path reported under real home")
	}
}
