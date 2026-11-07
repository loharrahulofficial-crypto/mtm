package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
	"github.com/loharrahulofficial-crypto/mtm/internal/tui"
)

// sandboxGuard reports whether the environment is sandboxed for tests/dumps:
// MTM_SANDBOX=1, or all three XDG roots pointing at temp dirs. Anything else
// refuses to run — the dump command never touches real data or real config.
func sandboxed() bool {
	if os.Getenv("MTM_SANDBOX") == "1" {
		return true
	}
	for _, key := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME"} {
		v := os.Getenv(key)
		if v == "" {
			return false
		}
		abs, err := filepath.Abs(v)
		if err != nil {
			return false
		}
		if !strings.HasPrefix(abs, os.TempDir()) && !strings.HasPrefix(abs, "/tmp") {
			return false
		}
	}
	return true
}

// parseSize parses "120x40" into (120, 40).
func parseSize(s string) (int, int, error) {
	var w, h int
	if _, err := fmt.Sscanf(strings.TrimSpace(s), "%dx%d", &w, &h); err != nil {
		return 0, 0, fmt.Errorf("invalid --size %q (want 120x40)", s)
	}
	if w < 10 || w > 500 || h < 5 || h > 500 {
		return 0, 0, fmt.Errorf("--size %dx%d out of range (10..500 × 5..500)", w, h)
	}
	return w, h, nil
}

func newTuiCmd() *cobra.Command {
	var dump bool
	var view, size string
	var noColor bool
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Launch the interactive TUI (or, hidden, render one frame)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !dump {
				return tui.Run()
			}
			// Hidden verification path: render one frame and print it. Refuses
			// to run outside a sandbox — this never touches real data.
			if !sandboxed() {
				return fmt.Errorf("refusing to dump: not sandboxed (set MTM_SANDBOX=1 or point XDG_CONFIG_HOME/XDG_DATA_HOME/XDG_CACHE_HOME at temp dirs)")
			}
			w, h, err := parseSize(size)
			if err != nil {
				return err
			}
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			frame, err := tui.RenderDump(cfg, s, sync.New(s.Root), view, w, h, noColor, tui.DumpFixedTime)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprint(out, frame)
			if !strings.HasSuffix(frame, "\n") {
				fmt.Fprintln(out)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dump, "dump", false, "render one frame to stdout (hidden; requires MTM_SANDBOX=1 or temp XDG roots)")
	cmd.Flags().StringVar(&view, "view", "home", "view to dump: home|list|board|calendar|focus")
	cmd.Flags().StringVar(&size, "size", "120x40", "frame size as WxH, e.g. 120x40")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "strip ANSI escape sequences")
	_ = cmd.Flags().MarkHidden("dump")
	_ = cmd.Flags().MarkHidden("view")
	_ = cmd.Flags().MarkHidden("size")
	_ = cmd.Flags().MarkHidden("no-color")
	return cmd
}
