package cli

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var olderThanRe = regexp.MustCompile(`^(\d+)([dwh])$`)

// parseOlderThan accepts Go durations ("720h") and day/week shorthand ("30d", "2w").
func parseOlderThan(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, fmt.Errorf("--older-than needs a value, e.g. 30d")
	}
	if m := olderThanRe.FindStringSubmatch(s); m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, err
		}
		var unit time.Duration
		switch m[2] {
		case "h":
			unit = time.Hour
		case "d":
			unit = 24 * time.Hour
		case "w":
			unit = 7 * 24 * time.Hour
		}
		return time.Duration(n) * unit, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid --older-than %q (want e.g. 30d, 2w, 720h)", s)
	}
	return d, nil
}

func newArchiveCmd() *cobra.Command {
	var olderThan string
	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Move done tasks to archive/<project>/ (one commit)",
		Long: `Move all done tasks into archive/<project>/, keeping the files
byte-identical. Archived tasks stay out of the index by default; browse them
with 'mtm list --archived' and search them with 'mtm search --archived'.

  mtm archive                 archive every done task
  mtm archive --older-than 30d   only tasks completed over 30 days ago

Committed and synced once, like any other change. Nothing is deleted.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var d time.Duration
			var err error
			if olderThan != "" {
				d, err = parseOlderThan(olderThan)
				if err != nil {
					return err
				}
			}
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			printDiagnostics(s)

			moved, err := s.ArchiveDone(s.Now(), d, func(p, st string) bool { return s.IsDoneStatus(p, st) })
			if err != nil {
				return err
			}
			if moved == 0 {
				fmt.Println("nothing to archive")
				return nil
			}
			commitAndSync(cfg, s.Root, fmt.Sprintf("archive: %d done tasks", moved))
			fmt.Printf("archived %d done task(s) — see `mtm list --archived`\n", moved)
			return nil
		},
	}
	cmd.Flags().StringVar(&olderThan, "older-than", "", "only archive tasks completed more than this long ago (e.g. 30d, 2w)")
	return cmd
}
