package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/gauges"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
)

func newTimerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "timer",
		Short: "Stopwatch on a task (accrues into time_logged_minutes)",
	}
	cmd.AddCommand(newTimerStartCmd(), newTimerStopCmd(), newTimerStatusCmd())
	return cmd
}

func newTimerStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start <id>",
		Short: "Start a stopwatch on a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			st, err := actions.StartTimer(s, cfg, args[0], core.TimerModeTimer)
			if err != nil {
				return err
			}
			t, err := s.Resolve(st.TaskID)
			if err != nil {
				return err
			}
			fmt.Printf("timer started on %s  %s — stop with `mtm timer stop`\n", shortID(t.ID), t.Title)
			return nil
		},
	}
}

func newTimerStopCmd() *cobra.Command {
	var full bool
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the timer and log the accrued minutes",
		Long: `Stops the active timer and writes the capped accrual into the task's
time_logged_minutes. A plain timer that ran past 4h logs at most 4h;
use --full to log everything.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			res, err := actions.StopTimer(s, full)
			if err != nil {
				return err
			}
			if res.Minutes > 0 {
				commitAndSync(cfg, s.Root, fmt.Sprintf("log: %s on %s", actions.FormatMinutes(res.Minutes), res.Title))
			}
			printStopResult(res)
			return nil
		},
	}
	cmd.Flags().BoolVar(&full, "full", false, "log everything, ignoring the 4h cap")
	return cmd
}

func newTimerStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the active timer",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			view, events, auto, err := actions.TickTimer(s, cfg, true)
			if err != nil {
				return err
			}
			if auto != nil {
				printStopResult(auto)
				return nil
			}
			_ = events
			if view.Phase == core.TimerPhaseIdle {
				fmt.Println("no timer running")
				return nil
			}
			printTimerView(s, view)
			return nil
		},
	}
}

func newPomoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pomo [id]",
		Short: "Pomodoro on a task (default 25m work / 5m break from config)",
		Long: `Runs a foreground pomodoro watcher for a task: 25m work, 5m break,
a long break every 4th round (configurable via pomodoro: in config.yaml).
Ctrl-C stops early and logs the accrued work minutes. Time accrues only
during work phases. With no argument, prints the current status.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			if len(args) == 0 {
				view, _, auto, err := actions.TickTimer(s, cfg, true)
				if err != nil {
					return err
				}
				if auto != nil {
					printStopResult(auto)
					return nil
				}
				if view.Phase == core.TimerPhaseIdle {
					fmt.Println("no timer running")
					return nil
				}
				printTimerView(s, view)
				return nil
			}
			return runPomoWatcher(cfg, s, args[0])
		},
	}
}

func printTimerView(s *store.Store, view core.TimerView) {
	title := view.TaskID
	if t, err := s.Resolve(view.TaskID); err == nil {
		title = t.Title
	}
	switch view.Phase {
	case core.TimerPhaseBreak:
		fmt.Printf("%s — break, %s left · %d pomodoro(s) done · accrued %s\n",
			shortID(view.TaskID)+" "+title,
			formatClock(view.Remaining), view.Pomodoros, actions.FormatMinutes(int(view.Accrued/time.Minute)))
	default:
		mode := view.Mode
		if mode == core.TimerModePomodoro && view.Total > 0 {
			fmt.Printf("%s — work, %s left of %s (pomodoro %d) · accrued %s\n",
				shortID(view.TaskID)+" "+title,
				formatClock(view.Remaining), formatClock(view.Total), view.PhaseIndex, actions.FormatMinutes(int(view.Accrued/time.Minute)))
			return
		}
		fmt.Printf("%s — running %s\n", shortID(view.TaskID)+" "+title, actions.FormatMinutes(int(view.Accrued/time.Minute)))
	}
}

func printStopResult(res *actions.TimerStopResult) {
	line := fmt.Sprintf("logged %s on %s  %s", actions.FormatMinutes(res.Minutes), shortID(res.TaskID), res.Title)
	if res.Pomodoros > 0 {
		line += fmt.Sprintf(" · %d pomodoro(s)", res.Pomodoros)
	}
	if res.Capped {
		line += " · capped at the 4h session cap (--full to log everything)"
	}
	if res.Auto {
		line += " · timer was stale (reboot/away) and stopped itself"
	}
	fmt.Println(line)
}

func formatClock(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d.Round(time.Second).Seconds())
	return fmt.Sprintf("%d:%02d", total/60, total%60)
}

// runPomoWatcher starts (or attaches to) a pomodoro for the task and renders
// a one-line live view until the session ends, is stopped (Ctrl-C), or is
// finalized elsewhere.
func runPomoWatcher(cfg config.Config, s *store.Store, arg string) error {
	t, err := s.Resolve(arg)
	if err != nil {
		return err
	}
	existing := actions.LoadTimerView(s)
	switch {
	case existing.Phase == core.TimerPhaseIdle:
		if _, err := actions.StartTimer(s, cfg, t.ID, core.TimerModePomodoro); err != nil {
			return err
		}
		fmt.Printf("pomodoro started on %s  %s\n", shortID(t.ID), t.Title)
	case existing.TaskID == t.ID:
		fmt.Printf("attached to running pomodoro on %s  %s\n", shortID(t.ID), t.Title)
	default:
		title := existing.TaskID
		if other, err := s.Resolve(existing.TaskID); err == nil {
			title = other.Title
		}
		return fmt.Errorf("timer already running on %s (%s) — stop it first with `mtm timer stop`", shortID(existing.TaskID), title)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	lastPhase := ""
	for {
		view, events, auto, err := actions.TickTimer(s, cfg, true)
		if err != nil {
			return err
		}
		if auto != nil {
			fmt.Println()
			printStopResult(auto)
			return nil
		}
		if view.Phase == core.TimerPhaseIdle {
			fmt.Println()
			fmt.Println("timer stopped")
			return nil
		}
		if view.Phase != lastPhase && lastPhase != "" {
			fmt.Println() // close the status line on a phase transition
		}
		lastPhase = view.Phase
		_ = events
		renderPomoLine(s, view)
		select {
		case <-ctx.Done():
			res, err := actions.StopTimer(s, false)
			fmt.Println()
			if err != nil {
				return err
			}
			if res.Minutes > 0 {
				commitAndSync(cfg, s.Root, fmt.Sprintf("log: %s on %s", actions.FormatMinutes(res.Minutes), res.Title))
			}
			printStopResult(res)
			return nil
		case <-ticker.C:
		}
	}
}

func renderPomoLine(s *store.Store, view core.TimerView) {
	fill, track := gauges.Bar(view.Progress, 24)
	label := "work"
	if view.Phase == core.TimerPhaseBreak {
		label = "break"
	}
	fmt.Printf("\r%s%s  %s · %s left of %s · pomodoro %d · accrued %s  ",
		fill, track, label, formatClock(view.Remaining), formatClock(view.Total),
		view.PhaseIndex, actions.FormatMinutes(int(view.Accrued/time.Minute)))
}
