package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// systemctlStub writes a stub shell script that logs its arguments to a
// file instead of touching the real systemd manager, and points the
// install path at a temp unit directory.
func systemctlStub(t *testing.T, logPath string, exitCode int) string {
	t.Helper()
	stub := filepath.Join(t.TempDir(), "systemctl-stub.sh")
	script := fmt.Sprintf("#!/bin/sh\necho \"$@\" >> %s\nexit %d\n", logPath, exitCode)
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MTM_SYSTEMCTL", stub)
	unitDir := t.TempDir()
	t.Setenv("MTM_SYSTEMD_USER_DIR", unitDir)
	return unitDir
}

func runSystemd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newSystemdCmd()
	cmd.SetArgs(args)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	return buf.String(), err
}

func TestSystemdInstallWritesUnitsAndEnables(t *testing.T) {
	log := filepath.Join(t.TempDir(), "stub.log")
	unitDir := systemctlStub(t, log, 0)

	out, err := runSystemd(t, "install")
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range []string{"mtm-sync.service", "mtm-sync.timer", "mtm-remind.service", "mtm-remind.timer"} {
		data, err := os.ReadFile(filepath.Join(unitDir, unit))
		if err != nil {
			t.Fatalf("unit %s not written: %v", unit, err)
		}
		if strings.HasSuffix(unit, ".timer") {
			if !strings.Contains(string(data), "OnUnitActiveSec=") {
				t.Errorf("timer %s missing OnUnitActiveSec:\n%s", unit, data)
			}
			continue
		}
		if !strings.Contains(string(data), "ExecStart=") || !strings.Contains(string(data), "mtm") {
			t.Errorf("service %s looks wrong:\n%s", unit, data)
		}
	}
	// The sync unit calls plain 'mtm sync' — nothing that forces past the
	// paused/conflicted refusal.
	data, _ := os.ReadFile(filepath.Join(unitDir, "mtm-sync.service"))
	if strings.Contains(string(data), "--force") || strings.Contains(string(data), "abort") {
		t.Errorf("sync unit forces past the paused state:\n%s", data)
	}
	if !strings.Contains(string(data), " sync\n") && !strings.Contains(string(data), " sync\r") {
		t.Errorf("sync unit does not invoke mtm sync:\n%s", data)
	}
	// The stub got the enable call with both timers.
	logData, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logData), "--user enable --now mtm-sync.timer mtm-remind.timer") {
		t.Errorf("stub log = %q, want the enable call with both timers", logData)
	}
	if !strings.Contains(out, "timers enabled") {
		t.Errorf("output = %q, want a confirmation", out)
	}
}

func TestSystemdInstallDryRunTouchesNothing(t *testing.T) {
	log := filepath.Join(t.TempDir(), "stub.log")
	unitDir := systemctlStub(t, log, 0)

	out, err := runSystemd(t, "install", "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range []string{"mtm-sync.service", "mtm-sync.timer", "mtm-remind.service", "mtm-remind.timer"} {
		if _, err := os.Stat(filepath.Join(unitDir, unit)); !os.IsNotExist(err) {
			t.Errorf("--dry-run wrote %s", unit)
		}
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Errorf("--dry-run called systemctl: %v", err)
	}
	if !strings.Contains(out, "would run") {
		t.Errorf("--dry-run output = %q, want a plan", out)
	}
}

func TestSystemdUninstallRemovesUnits(t *testing.T) {
	log := filepath.Join(t.TempDir(), "stub.log")
	unitDir := systemctlStub(t, log, 0)

	if _, err := runSystemd(t, "install"); err != nil {
		t.Fatal(err)
	}
	out, err := runSystemd(t, "uninstall")
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range []string{"mtm-sync.service", "mtm-sync.timer", "mtm-remind.service", "mtm-remind.timer"} {
		if _, err := os.Stat(filepath.Join(unitDir, unit)); !os.IsNotExist(err) {
			t.Errorf("uninstall left %s behind", unit)
		}
	}
	logData, _ := os.ReadFile(log)
	if !strings.Contains(string(logData), "--user disable --now mtm-sync.timer mtm-remind.timer") {
		t.Errorf("stub log = %q, want the disable call", logData)
	}
	if !strings.Contains(out, "removed") {
		t.Errorf("output = %q, want a removal list", out)
	}
}

func TestSystemdUninstallDryRunKeepsUnits(t *testing.T) {
	log := filepath.Join(t.TempDir(), "stub.log")
	unitDir := systemctlStub(t, log, 0)

	if _, err := runSystemd(t, "install"); err != nil {
		t.Fatal(err)
	}
	if _, err := runSystemd(t, "uninstall", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	for _, unit := range []string{"mtm-sync.service", "mtm-sync.timer", "mtm-remind.service", "mtm-remind.timer"} {
		if _, err := os.Stat(filepath.Join(unitDir, unit)); err != nil {
			t.Errorf("--dry-run removed %s", unit)
		}
	}
}

func TestSystemdInstallReportsSystemctlFailure(t *testing.T) {
	log := filepath.Join(t.TempDir(), "stub.log")
	systemctlStub(t, log, 1) // stub exits 1 with no output

	_, err := runSystemd(t, "install")
	if err == nil {
		t.Fatal("install with failing systemctl succeeded, want an error")
	}
}

func TestSystemdStatusReportsUnitState(t *testing.T) {
	log := filepath.Join(t.TempDir(), "stub.log")
	unitDir := systemctlStub(t, log, 0)

	out, err := runSystemd(t, "status")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "missing") {
		t.Errorf("status before install = %q, want 'missing' units", out)
	}
	if _, err := runSystemd(t, "install"); err != nil {
		t.Fatal(err)
	}
	out, err = runSystemd(t, "status")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "present") {
		t.Errorf("status after install = %q, want 'present' units", out)
	}
	_ = unitDir
}
