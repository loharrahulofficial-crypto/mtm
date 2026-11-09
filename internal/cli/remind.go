package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
)

func newRemindCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "remind",
		Short: "Send notifications for tasks whose reminder time has passed",
		Long: `One-shot reminder check (run it from cron/systemd every few minutes):
notifies for tasks whose reminder stamp has passed, plus — when
reminders.due_soon_minutes is set — tasks due within that window.

State lives in the state directory, never in the data repo: every task
and occurrence is notified at most once. reminders.quiet_hours (e.g.
"22:00-07:00") suppresses everything inside the window.

--dry-run prints what would be sent without sending or recording anything.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()

			now := s.Now()
			var rep *actions.ReminderResult
			if dryRun {
				rep, err = actions.RunRemindersDry(s, cfg, now, nil)
			} else {
				send := func(title, body string) error {
					return actions.NotifyErr(title, body)
				}
				rep, err = actions.RunReminders(s, cfg, now, send)
			}
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if rep.Quiet {
				fmt.Fprintln(out, "quiet hours — nothing sent")
				return nil
			}
			if len(rep.Sent) == 0 {
				fmt.Fprintln(out, "nothing to remind about")
				return nil
			}
			verb := "would send"
			if !dryRun {
				verb = "sent"
			}
			fmt.Fprintf(out, "%s %d reminder(s):\n", verb, len(rep.Sent))
			for _, c := range rep.Sent {
				fmt.Fprintf(out, "  %s — %s\n", c.Title, c.Body)
			}
			if rep.Skipped > 0 {
				fmt.Fprintf(out, "  (%d already sent earlier, skipped)\n", rep.Skipped)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would be sent; send and record nothing")
	return cmd
}
