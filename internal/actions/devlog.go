package actions

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
)

// DevLog helpers shared by CLI and TUI.

// AddDevLog creates a dev-log entry in project. body may be empty.
// timeSpentMinutes carries the optional timer-attached elapsed time.
func AddDevLog(s *store.Store, project, title, body string, timeSpentMinutes int, mood string, tags []string) (*core.DevLogEntry, error) {
	project = strings.TrimSpace(project)
	title = strings.TrimSpace(title)
	if project == "" {
		return nil, fmt.Errorf("devlog needs a project")
	}
	if title == "" {
		return nil, fmt.Errorf("devlog entry needs a title")
	}
	resolved, err := ResolveProject(s, project)
	if err != nil {
		return nil, err
	}
	d := &core.DevLogEntry{
		Title:            title,
		Body:             body,
		TimeSpentMinutes: timeSpentMinutes,
		Mood:             strings.TrimSpace(mood),
		Tags:             uniqueTags(tags),
		Project:          resolved,
	}
	if err := s.CreateDevLog(d); err != nil {
		return nil, err
	}
	return d, nil
}

// gitLogTimeout bounds every git invocation used by the devlog/github
// features: local repos answer quickly; a hung repo must never stall a CLI
// command or the TUI.
const gitLogTimeout = 3 * time.Second

// GitLogSubjects runs `git -C repoPath log` since since (or the last max
// commits when since is zero) and returns "<short-hash> <subject>" lines.
// repoPath may point at the linked repo or any git repo on disk; nothing is
// ever written there.
func GitLogSubjects(repoPath string, since time.Time, max int) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitLogTimeout)
	defer cancel()
	args := []string{"-C", repoPath, "log", "--no-decorate", "--format=%h %s"}
	if !since.IsZero() {
		args = append(args, "--since="+since.Format("2006-01-02 15:04:05"))
	} else {
		if max <= 0 {
			max = 10
		}
		args = append(args, fmt.Sprintf("-n%d", max))
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("git log timed out after %s", gitLogTimeout)
		}
		return nil, fmt.Errorf("git log: %s", strings.TrimSpace(stderr.String()))
	}
	var out []string
	for _, line := range strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out, nil
}

// CommitSummary renders commit lines as a Markdown section for a devlog
// body, or "" when there are no commits.
func CommitSummary(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Commits\n\n")
	for _, l := range lines {
		b.WriteString("- " + l + "\n")
	}
	return b.String()
}

// CommitsSince picks the right window for --from-commits: since the most
// recent devlog entry for the project (any date), else the last 10 commits.
func CommitsSince(s *store.Store, project string, now time.Time, max int) ([]string, error) {
	since := time.Time{}
	latest := ""
	for _, d := range s.DevLogs(project) {
		if d.Created > latest {
			latest = d.Created
		}
	}
	if latest != "" {
		stamp := latest
		if t, ok := parseStampOK(stamp); ok {
			since = t
		}
	}
	return GitLogSubjects(devLogRepoPath(s, project), since, max)
}

// devLogRepoPath returns the project's linked repo path when set, else the
// data repo root.
func devLogRepoPath(s *store.Store, project string) string {
	if p, err := s.GetProject(project); err == nil && strings.TrimSpace(p.RepoPath) != "" {
		return expandHome(p.RepoPath)
	}
	return s.Root
}

// parseStampOK parses the layouts the store writes.
func parseStampOK(s string) (time.Time, bool) {
	for _, layout := range []string{core.TimeFormat, "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, strings.TrimSpace(s), time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// expandHome expands a leading ~ (kept local to avoid importing config here).
func expandHome(p string) string {
	p = strings.TrimSpace(p)
	if p == "~" {
		if h, err := os.UserHomeDir(); err == nil {
			return h
		}
		return p
	}
	if strings.HasPrefix(p, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			return h + p[1:]
		}
		return p
	}
	return p
}
