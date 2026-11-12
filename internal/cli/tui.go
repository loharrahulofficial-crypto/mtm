package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/loharrahulofficial-crypto/mtm/internal/ambient"
	"github.com/loharrahulofficial-crypto/mtm/internal/caps"
	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/paths"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
	"github.com/loharrahulofficial-crypto/mtm/internal/tui"
)

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

// dumpPalette is the fixed palette the ambient dump uses (matches the
// default theme; rain_lightning off).
func dumpPalette() ambient.Palette {
	return ambient.Palette{
		Base: "#14282d", Surface: "#253c41", SurfaceRaised: "#2f4a50", SurfaceStrong: "#3d5960",
		Border: "#a1afa9", Text: "#e8dfc4", BrightText: "#fff2d5", MutedSoft: "#a1afa9",
		Accent: "#d3ae78", AccentSoft: "#8a744f", Success: "#a8b887", Warning: "#dfc181",
		Danger: "#d78f79", HeatWarm: "#dfc181", HeatHot: "#d3ae78", HeatOverdue: "#d78f79",
		Blue: "#91afc5", Cyan: "#86b8b5", Yellow: "#dfc181", Magenta: "#b5a0ba",
		Orange: "#d7a06c", Brown: "#b79b7f",
	}
}

func newTuiCmd() *cobra.Command {
	var dump bool
	var view, size string
	var noColor bool
	var sceneName string
	var frameT time.Duration
	var progress float64
	var renderMode, phase string
	var ppmPath string
	var ppmScale int
	var profilePath string
	var splashT float64
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Launch the interactive TUI (or, hidden, render one frame)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !dump {
				return tui.Run(profilePath)
			}
			// Hidden verification path: render one frame and print it. Refuses
			// to run outside a sandbox — this never touches real data.
			if err := paths.AdmitSandbox(); err != nil {
				return fmt.Errorf("refusing to dump: %v", err)
			}
			w, h, err := parseSize(size)
			if err != nil {
				return err
			}
			if sceneName != "" {
				// Ambient frame mode: one scene frame at the requested t/progress.
				if progress < 0 || progress > 1 {
					return fmt.Errorf("--progress %v out of range (0..1)", progress)
				}
				if frameT < 0 {
					return fmt.Errorf("--t %v must be >= 0", frameT)
				}
				ph := strings.ToLower(strings.TrimSpace(phase))
				if ph == "" {
					ph = "work"
				}
				// Honor ui.truecolor / MTM_TRUECOLOR so the dump shows the
				// same color mode the TUI will use.
				trueColorSetting := "auto"
				ditherSetting := "on"
				if cfg, err := config.Load(); err == nil {
					trueColorSetting = cfg.UI.TrueColor
					ditherSetting = cfg.UI.AmbientDither
				}
				trueColor, _ := caps.Effective(trueColorSetting, os.Getenv)
				cell := ambient.DetectCellPixels(os.Stdout)
				st := ambient.SceneState{
					Progress:       progress,
					Phase:          ambient.Phase(ph),
					Now:            tui.DumpFixedTime,
					Seed:           42,
					Palette:        dumpPalette(),
					PaletteLimited: !trueColor,
					Dither:         ditherSetting != "off" && !trueColor,
					FullFrame:      true,
					CellW:          cell.W,
					CellH:          cell.H,
				}
				s := ambient.New(sceneName, renderMode)
				if ppmPath != "" {
					if ppmScale < 1 || ppmScale > 20 {
						return fmt.Errorf("--ppm-scale %d out of range (1..20)", ppmScale)
					}
					aspect := ambient.SubpixelAspect(ambient.ResolveMode(renderMode), cell)
					if err := ambient.WritePPM(ppmPath, s.Frame(frameT, w, h, st), ppmScale, aspect); err != nil {
						return err
					}
					fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", ppmPath)
					return nil
				}
				frame := strings.Join(s.Frame(frameT, w, h, st), "\n")
				out := cmd.OutOrStdout()
				fmt.Fprint(out, frame)
				if !strings.HasSuffix(frame, "\n") {
					fmt.Fprintln(out)
				}
				return nil
			}
			if splashT >= 0 {
				// Splash frame mode: render the splash at t seconds. The
				// splash never reads the data repo, so a throwaway store
				// is enough.
				tmp, err := os.MkdirTemp("", "mtm-splash-dump-*")
				if err != nil {
					return err
				}
				defer func() { _ = os.RemoveAll(tmp) }()
				cfg, err := config.Load()
				if err != nil {
					cfg = config.Default()
				}
				cfg.DataRepo = tmp
				cfg.Sync.Auto = false
				s, err := store.Open(tmp, filepath.Join(tmp, "index.json"))
				if err != nil {
					return err
				}
				defer func() { _ = s.Close() }()
				frame, err := tui.RenderSplashDump(cfg, s, sync.New(tmp), w, h, splashT)
				if err != nil {
					return err
				}
				if ppmPath != "" {
					cell := ambient.DetectCellPixels(os.Stdout)
					aspect := ambient.SubpixelAspect(ambient.ResolveMode(cfg.UI.AmbientRender), cell)
					if err := ambient.WritePPM(ppmPath, strings.Split(strings.TrimSuffix(frame, "\n"), "\n"), ppmScale, aspect); err != nil {
						return err
					}
					fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", ppmPath)
					return nil
				}
				out := cmd.OutOrStdout()
				fmt.Fprint(out, frame)
				if !strings.HasSuffix(frame, "\n") {
					fmt.Fprintln(out)
				}
				return nil
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
	cmd.Flags().StringVar(&profilePath, "profile", "", "write a CPU profile to FILE while the TUI runs (sandbox-only, like MTM_PPROF=cpu:FILE)")
	cmd.Flags().BoolVar(&dump, "dump", false, "render one frame to stdout (hidden; requires MTM_SANDBOX=1 and temp XDG roots)")
	cmd.Flags().StringVar(&view, "view", "home", "view to dump: home|list|board|calendar|courses|focus|stats")
	cmd.Flags().StringVar(&size, "size", "120x40", "frame size as WxH, e.g. 120x40")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "strip ANSI escape sequences")
	cmd.Flags().StringVar(&sceneName, "scene", "", "dump an ambient scene frame instead of a TUI view: night|bonsai|ocean|rain|koi|snow|campfire")
	cmd.Flags().DurationVar(&frameT, "t", 12*time.Second, "animation time for the scene frame")
	cmd.Flags().Float64Var(&progress, "progress", 0.5, "scene progress 0..1 (pomodoro / tide / growth / clearing)")
	cmd.Flags().StringVar(&renderMode, "render", "halfblock", "ambient render mode: auto|halfblock|braille|ascii|quadrant|sextant")
	cmd.Flags().StringVar(&phase, "phase", "work", "scene phase: work|break|idle")
	cmd.Flags().StringVar(&ppmPath, "ppm", "", "also write the frame as a binary PPM image to FILE (tool-only)")
	cmd.Flags().IntVar(&ppmScale, "ppm-scale", 6, "PPM pixel scale per subcell (1..20)")
	cmd.Flags().Float64Var(&splashT, "splash-t", -1, "render the splash at t seconds (review mode, like --scene)")
	_ = cmd.Flags().MarkHidden("dump")
	_ = cmd.Flags().MarkHidden("profile")
	_ = cmd.Flags().MarkHidden("view")
	_ = cmd.Flags().MarkHidden("size")
	_ = cmd.Flags().MarkHidden("no-color")
	_ = cmd.Flags().MarkHidden("scene")
	_ = cmd.Flags().MarkHidden("t")
	_ = cmd.Flags().MarkHidden("progress")
	_ = cmd.Flags().MarkHidden("render")
	_ = cmd.Flags().MarkHidden("phase")
	_ = cmd.Flags().MarkHidden("ppm")
	_ = cmd.Flags().MarkHidden("ppm-scale")
	_ = cmd.Flags().MarkHidden("splash-t")
	return cmd
}
