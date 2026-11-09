package paths

// Admission check for commands that may only run inside a throwaway
// sandbox (demo seeding, the TUI dump). Unlike Sandboxed (which panics
// inside Resolve under tests/MTM_SANDBOX), AdmitSandbox returns an
// error so commands can refuse with a clear message and exit non-zero.
//
// Admission requires BOTH:
//  1. MTM_SANDBOX=1 in the environment, AND
//  2. every mtm root (and every extra target path the command writes
//     to) resolving under a temp dir, never under the real passwd home.

import (
	"fmt"
	"os"
	"path/filepath"
)

// underTempDir reports whether p (already cleaned/abs) sits under the
// OS temp dir or /tmp.
func underTempDir(p string) bool {
	tmp := []string{filepath.Clean(os.TempDir()), "/tmp"}
	abs, err := filepath.Abs(p)
	if err != nil {
		return false
	}
	for _, t := range tmp {
		tabs, err := filepath.Abs(t)
		if err == nil && underRoot(abs, tabs) {
			return true
		}
	}
	return false
}

// UnderRealHome reports whether p equals or sits under the real passwd
// home directory. Used as a belt-and-suspenders refusal for commands
// that must never touch the user's actual data, even if TMPDIR or a
// root override points there.
func UnderRealHome(p string) bool {
	home, err := realHomeDir()
	if err != nil {
		return false
	}
	return underRoot(p, home)
}

// AdmitSandbox verifies that this process may only touch throwaway
// directories. MTM_SANDBOX=1 must be set and every mtm root plus each
// extra path (e.g. a command's target argument) must resolve under a
// temp dir and never under the real passwd home. Returns nil when
// admitted; otherwise a descriptive error the command can print.
func AdmitSandbox(extra ...string) error {
	if os.Getenv("MTM_SANDBOX") != "1" {
		return fmt.Errorf("not sandboxed: set MTM_SANDBOX=1 and point HOME/XDG_*_HOME at a temp dir (see contrib/sandbox-env.sh)")
	}
	check := func(label, p string) error {
		abs, err := filepath.Abs(p)
		if err != nil {
			return fmt.Errorf("not sandboxed: cannot resolve %s %q: %w", label, p, err)
		}
		if UnderRealHome(abs) {
			return fmt.Errorf("not sandboxed: %s %s is under your real home directory", label, abs)
		}
		if !underTempDir(abs) {
			return fmt.Errorf("not sandboxed: %s %s does not resolve under a temp dir", label, abs)
		}
		return nil
	}
	for _, r := range roots {
		if err := check("mtm "+r.name+" dir", r.resolve()); err != nil {
			return err
		}
	}
	for _, p := range extra {
		if err := check("target", p); err != nil {
			return err
		}
	}
	return nil
}
