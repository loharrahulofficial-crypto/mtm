package cli

import (
	"fmt"

	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

func newLogCmd() *cobra.Command {
	var (
		tags        []string
		mood        string
		withTimer   bool
		fromCommits bool
	)
	cmd := &cobra.Command{
		Use:   "log <project> [text...]",
		Short: "Append a dev-log entry to a project (no text opens $EDITOR)",
		Long: `Write a dev-log entry under projects/<project>/devlog/<YYYY-MM-DD>-<id>.md.

  mtm log college/dbms "3NF, then lossless joins"   # one entry, body = text
  mtm log college/dbms                              # scaffold opens in $EDITOR
  mtm log college/dbms --with-timer "evening block" # attach the running timer's elapsed time
  mtm log college/dbms --from-commits               # scaffold prefilled with commits
                                                      # since the last devlog entry
Without text, the editor scaffold is "# Title" followed by Markdown notes.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			now := s.Now()

			project, err := actions.ResolveProject(s, args[0])
			if err != nil {
				return err
			}
			minutes := 0
			if withTimer {
				if v := actions.LoadTimerView(s); v.Phase != core.TimerPhaseIdle && v.Accrued > 0 {
					minutes = int(v.Accrued.Minutes())
				} else {
					note("no active timer — time not attached")
				}
			}
			text := strings.TrimSpace(strings.Join(args[1:], " "))
			if text != "" {
				title, body := splitLogText(text, now)
				if fromCommits {
					lines, err := actions.CommitsSince(s, project, now, 10)
					if err != nil {
						return err
					}
					body = appendSection(body, actions.CommitSummary(lines))
				}
				d, err := actions.AddDevLog(s, project, title, body, minutes, mood, tags)
				if err != nil {
					return err
				}
				commitAndSync(cfg, s.Root, "log: "+d.Title)
				fmt.Printf("devlog %s  %s  %s\n", d.ID, d.Created[:10], d.Title)
				return nil
			}
			// Editor flow.
			scaffold := "# \n\n"
			if fromCommits || suggestFromCommits(cfg) {
				lines, err := actions.CommitsSince(s, project, now, 10)
				if err == nil {
					scaffold = "# \n\n" + actions.CommitSummary(lines)
				} else {
					note("commits unavailable: %v", err)
				}
			}
			data, path, err := editNewFile(cfg, "mtm-devlog-*.md", []byte(scaffold))
			if err != nil {
				return err
			}
			title, body, ok := parseHashTitle(data)
			if !ok {
				return fmt.Errorf("empty dev-log entry discarded (no `# Title` line; scaffold kept at %s)", path)
			}
			d, err := actions.AddDevLog(s, project, title, body, minutes, mood, tags)
			if err != nil {
				return err
			}
			commitAndSync(cfg, s.Root, "log: "+d.Title)
			fmt.Printf("devlog %s  %s  %s\n", d.ID, d.Created[:10], d.Title)
			return nil
		},
	}
	cmd.Flags().StringSliceVarP(&tags, "tag", "t", nil, "tags (comma-separated or repeated)")
	cmd.Flags().StringVar(&mood, "mood", "", "optional mood (one word)")
	cmd.Flags().BoolVar(&withTimer, "with-timer", false, "attach the running timer's elapsed time to the entry")
	cmd.Flags().BoolVar(&fromCommits, "from-commits", false, "prefill the entry with commits since the last devlog entry")
	return cmd
}

func splitLogText(text string, now time.Time) (title, body string) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	trimmed := strings.TrimSpace(text)
	if i := strings.Index(trimmed, "\n"); i >= 0 {
		title = strings.TrimSpace(trimmed[:i])
		body = strings.TrimSpace(trimmed[i+1:]) + "\n"
	} else {
		title = trimmed
	}
	if title == "" {
		title = "devlog " + now.Format("2006-01-02")
	}
	return title, body
}

func appendSection(body, section string) string {
	if section == "" {
		return body
	}
	body = strings.TrimRight(body, "\n")
	if body == "" {
		return section
	}
	return body + "\n\n" + section
}

// suggestFromCommits reports the opt-in config flag: prefill the $EDITOR
// scaffold with recent commits. Default off.
func suggestFromCommits(cfg config.Config) bool { return cfg.Devlog.SuggestFromCommits }

func newLogsCmd() *cobra.Command {
	var since string
	cmd := &cobra.Command{
		Use:   "logs <project>",
		Short: "Read the dev-log (reverse-chronological)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()

			project, err := actions.ResolveProject(s, args[0])
			if err != nil {
				return err
			}
			sinceTime, err := parseSinceFlag(since)
			if err != nil {
				return err
			}
			var entries []*core.DevLogEntry
			for _, d := range s.DevLogs(project) {
				if !sinceTime.IsZero() {
					ts, ok := parseStamp(d.Created)
					if !ok || ts.Before(sinceTime) {
						continue
					}
				}
				entries = append(entries, d)
			}
			if len(entries) == 0 {
				fmt.Println("no dev-log entries — write one with `mtm log " + project + " \"...\"`")
				return nil
			}
			// Created has minute precision, so ties are possible; break them
			// deterministically (id descending).
			sort.Slice(entries, func(a, b int) bool {
				if entries[a].Created != entries[b].Created {
					return entries[a].Created > entries[b].Created
				}
				return entries[a].ID > entries[b].ID
			})
			for _, d := range entries {
				meta := []string{}
				if d.TimeSpentMinutes > 0 {
					meta = append(meta, actions.FormatMinutes(d.TimeSpentMinutes))
				}
				if d.Mood != "" {
					meta = append(meta, "mood: "+d.Mood)
				}
				if len(d.Tags) > 0 {
					meta = append(meta, strings.Join(d.Tags, ","))
				}
				suffix := ""
				if len(meta) > 0 {
					suffix = "  —  " + strings.Join(meta, " · ")
				}
				date := d.Created
				if len(date) >= 10 {
					date = date[:10]
				}
				fmt.Printf("%s  %s%s\n", date, d.Title, suffix)
				if body := strings.TrimSpace(d.Body); body != "" {
					for _, line := range strings.Split(body, "\n") {
						fmt.Printf("    %s\n", line)
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "only entries from this window (e.g. 7d, 2w, 24h, or a Go duration)")
	return cmd
}

// parseSinceFlag reads "Nd", "Nw", "Nh" or a Go duration ("168h").
func parseSinceFlag(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	loc := time.Local
	now := time.Now().In(loc)
	n := 0
	if _, err := fmt.Sscanf(s, "%d", &n); err == nil && n > 0 {
		unit := strings.TrimSpace(s[strings.LastIndexAny(s, "0123456789")+1:])
		switch unit {
		case "d":
			return now.AddDate(0, 0, -n), nil
		case "w":
			return now.AddDate(0, 0, -7*n), nil
		case "h":
			return now.Add(-time.Duration(n) * time.Hour), nil
		}
	}
	if d, err := time.ParseDuration(s); err == nil {
		return now.Add(-d), nil
	}
	return time.Time{}, fmt.Errorf("--since %q not understood (want e.g. 7d, 2w, 24h, 48h30m)", s)
}

// parseStamp reads the layouts the store writes (a local copy keeps the
// dependency direction one-way: core must not import dates, dates imports
// core).
func parseStamp(s string) (time.Time, bool) {
	for _, layout := range []string{core.TimeFormat, "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, strings.TrimSpace(s), time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
