package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/ical"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
)

func newIcalCmd() *cobra.Command {
	var (
		out     string
		project string
		include string
		alarm   string
		bind    string
		token   string
		serve   bool
	)
	cmd := &cobra.Command{
		Use:   "ical",
		Short: "Export the calendar (RFC 5545), or serve it read-only",
		Long: `Export your calendar as RFC 5545 (.ics): exams and timed tasks are
timed events, date-only tasks are all-day events. UIDs derive from task
ids, so re-imports update instead of duplicating. Dates stored
local-naive are exported as floating local times (no TZID) unless
ical.timezone is set in config.yaml.

  --out FILE          write to FILE instead of stdout (mode 0600)
  --include LIST      tasks, exams (default both; either one alone ok)
  --project X         restrict to one project
  --alarm 30m         add a VALARM 30 minutes before each event

  --serve             serve the feed read-only until interrupted
  --bind HOST:PORT    listen address (default 127.0.0.1:8765, loopback
                      only — non-loopback prints a loud warning)
  --token SECRET      require /SECRET/mtm.ics instead of the bare path;
                      anyone who can reach the (non-loopback) bind can
                      read the feed otherwise`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			includeTasks, includeExams, err := parseIcalInclude(include)
			if err != nil {
				return err
			}
			alarmMinutes, err := parseIcalAlarm(alarm)
			if err != nil {
				return err
			}
			if err := validateIcalToken(token); err != nil {
				return err
			}
			if serve && strings.TrimSpace(out) != "" && out != "-" {
				return fmt.Errorf("--serve cannot be combined with --out (the server generates the feed per request)")
			}
			feed := func() ([]byte, error) {
				_, s, err := openStore()
				if err != nil {
					return nil, err
				}
				defer func() { _ = s.Close() }()
				return collectIcalFeed(s, project, includeTasks, includeExams, alarmMinutes)
			}

			if serve {
				return serveIcalFeed(cmd, bind, token, feed)
			}
			data, err := feed()
			if err != nil {
				return err
			}
			if strings.TrimSpace(out) != "" {
				if err := os.WriteFile(out, data, 0o600); err != nil {
					return fmt.Errorf("write %s: %w", out, err)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "wrote %s (%d bytes)\n", out, len(data))
				return nil
			}
			_, err = cmd.OutOrStdout().Write(data)
			return err
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "write to FILE instead of stdout (mode 0600)")
	cmd.Flags().StringVar(&project, "project", "", "restrict to one project")
	cmd.Flags().StringVar(&include, "include", "tasks,exams", "comma-separated: tasks, exams")
	cmd.Flags().StringVar(&alarm, "alarm", "", "add a VALARM this far ahead of each event (e.g. 30m)")
	cmd.Flags().BoolVar(&serve, "serve", false, "serve the feed read-only until interrupted")
	cmd.Flags().StringVar(&bind, "bind", "127.0.0.1:8765", "listen address for --serve (loopback only by default)")
	cmd.Flags().StringVar(&token, "token", "", "require /TOKEN/mtm.ics when serving (letters, digits, . _ -)")
	return cmd
}

func collectIcalFeed(s *store.Store, project string, includeTasks, includeExams bool, alarmMinutes int) ([]byte, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	isDone := func(p, st string) bool { return s.IsDoneStatus(p, st) }
	tasks := s.Tasks()
	if strings.TrimSpace(project) != "" {
		filtered := make([]*core.Task, 0, len(tasks))
		for _, t := range tasks {
			if strings.EqualFold(t.Project, project) {
				filtered = append(filtered, t)
			}
		}
		tasks = filtered
	}
	events := ical.Collect(tasks, isDone, includeTasks, includeExams)
	return ical.Calendar(events, ical.Options{
		Timezone:     cfg.ICal.Timezone,
		AlarmMinutes: alarmMinutes,
	})
}

func parseIcalInclude(s string) (tasks, exams bool, err error) {
	tasks, exams = false, false
	for _, part := range strings.Split(s, ",") {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case "tasks":
			tasks = true
		case "exams":
			exams = true
		case "":
		default:
			return false, false, fmt.Errorf("unknown --include %q (want tasks and/or exams)", part)
		}
	}
	if !tasks && !exams {
		return false, false, fmt.Errorf("--include must name at least one of tasks, exams")
	}
	return tasks, exams, nil
}

func parseIcalAlarm(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("--alarm: %q is not a duration (e.g. 30m, 2h)", s)
	}
	minutes := int(d.Minutes())
	if minutes <= 0 {
		return 0, fmt.Errorf("--alarm: %q rounds to zero minutes", s)
	}
	return minutes, nil
}

var tokenPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func validateIcalToken(token string) error {
	if token == "" {
		return nil
	}
	if len(token) > 128 {
		return fmt.Errorf("--token: must be 128 characters or fewer")
	}
	if !tokenPattern.MatchString(token) {
		return fmt.Errorf("--token: only letters, digits, '.', '_' and '-' are allowed")
	}
	return nil
}

func serveIcalFeed(cmd *cobra.Command, bind, token string, feed func() ([]byte, error)) error {
	if strings.TrimSpace(bind) == "" {
		bind = "127.0.0.1:8765"
	}
	if _, _, err := net.SplitHostPort(bind); err != nil {
		return fmt.Errorf("--bind %q: %v", bind, err)
	}
	ical.WarnIfExposed(os.Stderr, bind, token)

	ln, err := net.Listen("tcp", bind)
	if err != nil {
		return fmt.Errorf("--bind %s: %w", bind, err)
	}
	path := ical.FeedPath
	if token != "" {
		path = "/" + token + ical.FeedPath
	}
	handler := ical.FeedHandlerFunc(token, feed)
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	fmt.Fprintf(cmd.ErrOrStderr(), "ical feed at http://%s%s — ctrl+c to stop (read-only; nothing else is served)\n", bind, path)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
