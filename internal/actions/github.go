package actions

// GitHub link (Phase 3, read-only): local git reads are always attempted
// first (no network); the remote facts (open issues/PRs) are fetched only
// when gh is installed and authenticated, with a timeout, cached in the
// cache dir for 15 minutes. Tokens are never stored; the linked repo is
// never written to.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/paths"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
)

// GitHubItem is one open issue or PR.
type GitHubItem struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url,omitempty"`
}

// GitHubInfo is the assembled read model for a linked project.
type GitHubInfo struct {
	Linked   bool
	RepoSlug string // owner/name when set in project.yaml
	RepoPath string // absolute path used for local git reads
	Branch   string

	// Local (present when the path is a git repo).
	LocalOK    bool
	LocalNote  string // why the local read failed (when !LocalOK)
	LastCommit time.Time
	Subjects   []string    // last 5 subjects, newest first
	Weeks      [12]float64 // commits per week, oldest → newest (12 buckets)
	Streak     int         // consecutive days with ≥1 commit (yesterday counts)

	// Remote (present only when gh is available and authenticated).
	RemoteOK     bool
	RemoteNote   string // muted "unavailable" reason when !RemoteOK
	OpenIssues   int
	OpenPRs      int
	LatestIssues []GitHubItem
	LatestPRs    []GitHubItem
}

// StreakWeeks is the span the sparkline covers (12 weeks).
const StreakWeeks = 12

// GitHub cache TTL (15 minutes).
const gitHubCacheTTL = 15 * time.Minute

type ghCacheFile struct {
	Version int                     `json:"version"`
	Entries map[string]ghCacheEntry `json:"entries"`
}

type ghCacheEntry struct {
	FetchedAt    time.Time    `json:"fetched_at"`
	OpenIssues   int          `json:"open_issues"`
	OpenPRs      int          `json:"open_prs"`
	LatestIssues []GitHubItem `json:"latest_issues,omitempty"`
	LatestPRs    []GitHubItem `json:"latest_prs,omitempty"`
}

const gitHubCacheVersion = 1

// gitHubCachePath resolves the cache file (never inside the data repo).
func gitHubCachePath() string { return filepath.Join(paths.CacheDir(), "github.json") }

// ghBin resolves the gh binary — MTM_GH overrides it (tests stub the network
// boundary with it).
func ghBin() string {
	if v := strings.TrimSpace(os.Getenv("MTM_GH")); v != "" {
		return v
	}
	return "gh"
}

// gitOut runs git in repoPath with a timeout and returns stdout.
func gitOut(ctx context.Context, repoPath string, args ...string) (string, error) {
	base := []string{"-C", repoPath}
	cmd := exec.CommandContext(ctx, "git", append(base, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("git %s timed out", strings.Join(args, " "))
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// LoadGitHub assembles the read model. fetchRemote=false skips the network
// entirely (local facts only). Never errors on git/gh failures — those land
// in LocalOK/RemoteOK instead.
func LoadGitHub(s *store.Store, project string, now time.Time, fetchRemote bool) (*GitHubInfo, error) {
	p, err := s.GetProject(project)
	if err != nil {
		return nil, err
	}
	info := &GitHubInfo{RepoSlug: strings.TrimSpace(p.GitHub), RepoPath: strings.TrimSpace(p.RepoPath)}
	if info.RepoSlug == "" && info.RepoPath == "" {
		return info, nil // not linked
	}
	info.Linked = true
	info.RepoPath = expandHome(info.RepoPath)

	// --- local facts -----------------------------------------------------
	if info.RepoPath == "" {
		info.LocalNote = "no repo_path set — run `mtm project link " + project + " --path <dir>`"
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		branch, err := gitOut(ctx, info.RepoPath, "branch", "--show-current")
		if err == nil {
			info.Branch = strings.TrimSpace(branch)
		}
		loc := now.Location()
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		windowStart := todayStart.AddDate(0, 0, -(int(StreakWeeks)-1)*7-6)
		commits, err := gitOut(ctx, info.RepoPath, "log", "--no-decorate", "--format=%aI",
			"--since="+windowStart.Format("2006-01-02T15:04:05"))
		if err != nil {
			info.LocalNote = err.Error()
		} else {
			info.LocalOK = true
			// Bucket commits per week (oldest → newest).
			for _, line := range strings.Split(strings.TrimRight(commits, "\n"), "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				ts, err := time.Parse(time.RFC3339, line)
				if err != nil {
					continue
				}
				tsLocal := ts.In(loc)
				day := time.Date(tsLocal.Year(), tsLocal.Month(), tsLocal.Day(), 0, 0, 0, 0, loc)
				back := int(todayStart.Sub(day).Hours() / (24 * 7))
				if back < 0 {
					back = 0
				}
				if back >= StreakWeeks {
					continue
				}
				info.Weeks[StreakWeeks-1-back]++
				if tsLocal.After(info.LastCommit) {
					info.LastCommit = tsLocal
				}
			}
			// Last 5 subjects (newest first), independent of the window.
			if out, err := gitOut(ctx, info.RepoPath, "log", "-5", "--no-decorate", "--format=%s"); err == nil {
				for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
					if s := strings.TrimSpace(line); s != "" {
						info.Subjects = append(info.Subjects, s)
					}
				}
			}
			// Streak: consecutive days with a commit, yesterday counts as
			// today when today has none yet (mtm stats semantics).
			info.Streak = commitStreak(ctx, info.RepoPath, loc, todayStart)
		}
		cancel()
	}

	// --- remote facts (gh), cached 15 min --------------------------------
	if !fetchRemote {
		return info, nil
	}
	if info.RepoSlug == "" {
		info.RemoteNote = "no --repo set — remote counts skipped (local git works)"
		return info, nil
	}
	// Cache first.
	if entry, ok := readGitHubCache(info.RepoSlug, now); ok {
		info.RemoteOK = true
		info.OpenIssues = entry.OpenIssues
		info.OpenPRs = entry.OpenPRs
		info.LatestIssues = entry.LatestIssues
		info.LatestPRs = entry.LatestPRs
		return info, nil
	}
	gh := ghBin()
	if _, err := exec.LookPath(gh); err != nil {
		info.RemoteNote = "gh not installed"
		return info, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if out, err := ghOut(ctx, gh, "auth", "status"); err != nil {
		_ = out
		info.RemoteNote = "gh not authenticated — run `gh auth login`"
		return info, nil
	}
	issues, err := ghList(ctx, gh, info.RepoSlug, "issue")
	if err != nil {
		info.RemoteNote = "issues unavailable: " + err.Error()
		return info, nil
	}
	prs, err := ghList(ctx, gh, info.RepoSlug, "pr")
	if err != nil {
		info.RemoteNote = "pull requests unavailable: " + err.Error()
		return info, nil
	}
	info.RemoteOK = true
	info.OpenIssues = len(issues)
	info.OpenPRs = len(prs)
	info.LatestIssues = latestN(issues, 3)
	info.LatestPRs = latestN(prs, 3)
	writeGitHubCache(info.RepoSlug, ghCacheEntry{
		FetchedAt:    now,
		OpenIssues:   info.OpenIssues,
		OpenPRs:      info.OpenPRs,
		LatestIssues: info.LatestIssues,
		LatestPRs:    info.LatestPRs,
	})
	return info, nil
}

// ghOut runs gh with a timeout and returns stdout+stderr combined (gh prints
// actionable errors there).
func ghOut(ctx context.Context, gh string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, gh, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return out.String(), fmt.Errorf("gh timed out")
		}
		return out.String(), err
	}
	return out.String(), nil
}

// ghList fetches open items of kind (issue|pr) for repo with a small JSON
// payload.
func ghList(ctx context.Context, gh, repo, kind string) ([]GitHubItem, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	out, err := ghOut(ctx, gh, kind, "list", "-R", repo, "--state", "open", "--limit", "100", "--json", "number,title,url")
	if err != nil {
		return nil, err
	}
	var items []GitHubItem
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		return nil, fmt.Errorf("gh %s list: unparseable JSON: %v", kind, err)
	}
	return items, nil
}

func latestN(items []GitHubItem, n int) []GitHubItem {
	if len(items) > n {
		return items[:n]
	}
	return items
}

func readGitHubCache(repo string, now time.Time) (ghCacheEntry, bool) {
	data, err := os.ReadFile(gitHubCachePath())
	if err != nil {
		return ghCacheEntry{}, false
	}
	var cf ghCacheFile
	if err := json.Unmarshal(data, &cf); err != nil || cf.Version != gitHubCacheVersion {
		return ghCacheEntry{}, false
	}
	entry, ok := cf.Entries[repo]
	if !ok {
		return ghCacheEntry{}, false
	}
	if now.Sub(entry.FetchedAt) > gitHubCacheTTL {
		return ghCacheEntry{}, false // stale
	}
	return entry, true
}

func writeGitHubCache(repo string, entry ghCacheEntry) {
	path := gitHubCachePath()
	cf := ghCacheFile{Version: gitHubCacheVersion, Entries: map[string]ghCacheEntry{}}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &cf) // merge best-effort; a corrupt cache is replaced
		if cf.Entries == nil {
			cf.Entries = map[string]ghCacheEntry{}
		}
		cf.Version = gitHubCacheVersion
	}
	cf.Entries[repo] = entry
	data, err := json.Marshal(cf)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = writeFileAtomicCache(path, data, 0o600)
}

func writeFileAtomicCache(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".github-*.json")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	return os.Rename(name, path)
}

// commitStreak counts consecutive days with ≥1 commit, anchored on today
// (or yesterday when today has no commits yet).
func commitStreak(ctx context.Context, repoPath string, loc *time.Location, todayStart time.Time) int {
	out, err := gitOut(ctx, repoPath, "log", "--no-decorate", "--format=%aI", "--since=180.days.ago")
	if err != nil {
		// Fallback for older git: fetch an ISO window we can express.
		out, err = gitOut(ctx, repoPath, "log", "--no-decorate", "--format=%aI", "--since="+todayStart.AddDate(0, 0, -180).Format("2006-01-02"))
		if err != nil {
			return 0
		}
	}
	days := map[int64]bool{}
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		ts, err := time.Parse(time.RFC3339, line)
		if err != nil {
			continue
		}
		day := time.Date(ts.In(loc).Year(), ts.In(loc).Month(), ts.In(loc).Day(), 0, 0, 0, 0, loc)
		days[day.Unix()] = true
	}
	anchor := todayStart
	if !days[anchor.Unix()] {
		anchor = anchor.AddDate(0, 0, -1) // yesterday counts as today
	}
	streak := 0
	for d := anchor; ; d = d.AddDate(0, 0, -1) {
		if !days[d.Unix()] {
			break
		}
		streak++
	}
	return streak
}

// expandHome expands a leading ~ (kept local; actions must not import config).
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
