package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/loharrahulofficial-crypto/mtm/internal/store"
)

// Unit definitions; rendered with the absolute binary path at install time.
type systemdUnit struct {
	name    string
	content string
}

func renderUnits(mtmPath string) []systemdUnit {
	syncTimer := "[Unit]\nDescription=mtm: periodic sync of the data repo\n\n[Timer]\nOnBootSec=2min\nOnUnitActiveSec=15min\nPersistent=true\n\n[Install]\nWantedBy=timers.target\n"
	syncService := fmt.Sprintf("[Unit]\nDescription=mtm: sync the data repo\n\n[Service]\nType=oneshot\nExecStart=%s sync\n", mtmPath)
	remindTimer := "[Unit]\nDescription=mtm: reminder check every 5 minutes\n\n[Timer]\nOnBootSec=1min\nOnUnitActiveSec=5min\nPersistent=true\n\n[Install]\nWantedBy=timers.target\n"
	remindService := fmt.Sprintf("[Unit]\nDescription=mtm: send due reminders\n\n[Service]\nType=oneshot\nExecStart=%s remind\n", mtmPath)
	return []systemdUnit{
		{"mtm-sync.service", syncService},
		{"mtm-sync.timer", syncTimer},
		{"mtm-remind.service", remindService},
		{"mtm-remind.timer", remindTimer},
	}
}

// unitDir is the user unit directory; MTM_SYSTEMD_USER_DIR overrides it so
// tests (and unattended setups) never touch the real systemd user dir.
func unitDir() string {
	if d := strings.TrimSpace(os.Getenv("MTM_SYSTEMD_USER_DIR")); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err == nil {
		return filepath.Join(home, ".config", "systemd", "user")
	}
	return filepath.Join(os.TempDir(), "mtm-systemd-user")
}

// systemctlBin is the systemctl binary; MTM_SYSTEMCTL overrides it —
// tests point it at a stub that logs arguments instead of touching the
// real systemd manager.
func systemctlBin() string {
	if b := strings.TrimSpace(os.Getenv("MTM_SYSTEMCTL")); b != "" {
		return b
	}
	return "systemctl"
}

func runSystemctl(args ...string) (string, error) {
	bin := systemctlBin()
	out, err := exec.Command(bin, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func newSystemdCmd() *cobra.Command {
	var dryRun bool
	install := &cobra.Command{
		Use:   "install",
		Short: "Install and enable the user sync and reminder timers",
		Long: `Writes user units to the systemd user directory (default
~/.config/systemd/user; override with MTM_SYSTEMD_USER_DIR):

  mtm-sync.service / mtm-sync.timer     every 15 minutes (always plain
                                          'mtm sync' — refuses to run
                                          while the repo is conflicted/paused)
  mtm-remind.service / mtm-remind.timer every 5 minutes

Then enables both timers with systemctl --user. Nothing is written to the
data repo. --dry-run prints the plan without touching anything.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			mtmPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("locate mtm binary: %w", err)
			}
			units := renderUnits(mtmPath)
			dir := unitDir()
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "unit directory: %s\n\n", dir)
				for _, u := range units {
					fmt.Fprintf(cmd.OutOrStdout(), "--- %s ---\n%s\n", filepath.Join(dir, u.name), u.content)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would run: %s --user enable --now mtm-sync.timer mtm-remind.timer\n", systemctlBin())
				return nil
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("create %s: %w", dir, err)
			}
			for _, u := range units {
				if err := store.WriteFileAtomic(filepath.Join(dir, u.name), []byte(u.content), 0o644); err != nil {
					return fmt.Errorf("write %s: %w", filepath.Join(dir, u.name), err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", filepath.Join(dir, u.name))
			}
			out, err := runSystemctl("--user", "enable", "--now", "mtm-sync.timer", "mtm-remind.timer")
			if err != nil {
				if out != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\n", out)
				}
				return fmt.Errorf("enable timers with %s: %w", systemctlBin(), err)
			}
			if out != "" {
				fmt.Fprintln(cmd.OutOrStdout(), out)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "timers enabled — sync every 15m, reminders every 5m")
			return nil
		},
	}
	install.Flags().BoolVar(&dryRun, "dry-run", false, "print what would be written and enabled; change nothing")

	uninstall := &cobra.Command{
		Use:   "uninstall",
		Short: "Disable and remove the user sync and reminder timers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := unitDir()
			units := []string{"mtm-sync.service", "mtm-sync.timer", "mtm-remind.service", "mtm-remind.timer"}
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "would run: %s --user disable --now mtm-sync.timer mtm-remind.timer\n", systemctlBin())
				for _, u := range units {
					fmt.Fprintf(cmd.OutOrStdout(), "would remove %s\n", filepath.Join(dir, u))
				}
				return nil
			}
			out, err := runSystemctl("--user", "disable", "--now", "mtm-sync.timer", "mtm-remind.timer")
			if err != nil && out != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", out)
			}
			// Tolerate "not loaded" on systems where the timers were never
			// enabled; the file removal is the point of uninstall.
			for _, u := range units {
				path := filepath.Join(dir, u)
				if err := os.Remove(path); err != nil {
					if os.IsNotExist(err) {
						fmt.Fprintf(cmd.OutOrStdout(), "absent %s\n", path)
						continue
					}
					return fmt.Errorf("remove %s: %w", path, err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", path)
			}
			return nil
		},
	}
	uninstall.Flags().BoolVar(&dryRun, "dry-run", false, "print what would be disabled and removed; change nothing")

	status := &cobra.Command{
		Use:   "status",
		Short: "Show whether the user units exist and are enabled",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := unitDir()
			fmt.Fprintf(cmd.OutOrStdout(), "unit directory: %s\n", dir)
			units := []string{"mtm-sync.service", "mtm-sync.timer", "mtm-remind.service", "mtm-remind.timer"}
			for _, u := range units {
				_, statErr := os.Stat(filepath.Join(dir, u))
				exists := "missing"
				if statErr == nil {
					exists = "present"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %-18s %s\n", u, exists)
			}
			out, err := runSystemctl("--user", "is-enabled", "mtm-sync.timer", "mtm-remind.timer")
			switch {
			case err != nil && out == "":
				fmt.Fprintf(cmd.OutOrStdout(), "systemctl: not available (%v)\n", err)
			case err != nil:
				fmt.Fprintf(cmd.OutOrStdout(), "is-enabled: %s\n", strings.TrimSpace(out))
			default:
				fmt.Fprintf(cmd.OutOrStdout(), "is-enabled: %s\n", out)
			}
			return nil
		},
	}

	root := &cobra.Command{
		Use:   "systemd",
		Short: "Manage the user sync and reminder timers",
	}
	root.AddCommand(install, uninstall, status)
	return root
}
