// Package doctor implements `mtm doctor`: read-only health checks for
// the data repo. It never writes — every check either passes, warns,
// or fails; the command exits non-zero when any fail-level finding
// exists so scripts can gate on it.
package doctor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/ambient"
	"github.com/loharrahulofficial-crypto/mtm/internal/caps"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/demo"
	"github.com/loharrahulofficial-crypto/mtm/internal/marker"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

const (
	LevelFail = "fail"
	LevelWarn = "warn"
	LevelInfo = "info"
)

// Finding is one check result.
type Finding struct {
	Check   string `json:"check"`
	Level   string `json:"level"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

// Report is the full doctor result. OK is true when no fail-level
// finding exists.
type Report struct {
	Root     string    `json:"root"`
	Kind     string    `json:"kind"` // marker kind: real|demo|"unmarked"
	OK       bool      `json:"ok"`
	Findings []Finding `json:"findings"`
	// AmbientBytesPerFrame reports the SGR byte count of one halfblock
	// frame per scene at 100x30 (deterministic, default theme).
	AmbientBytesPerFrame map[string]int `json:"ambient_bytes_per_frame,omitempty"`
	// AmbientTier reports the adaptive-quality tier (0=full|1=low-ss|2=low-fps).
	AmbientTier     int    `json:"ambient_quality_tier"`
	AmbientTierName string `json:"ambient_quality_name"`
	// AmbientPaintNS is the slowest single measured reference frame
	// (halfblock, 100x30, fixed clock), in nanoseconds; AmbientPaintMs is
	// the same in milliseconds. AmbientTier reports this process's
	// adaptive-quality tier (a one-shot doctor run reports 0/full).
	AmbientPaintNS int64   `json:"ambient_paint_ns"`
	AmbientPaintMs float64 `json:"ambient_paint_ms"`
}

// Options carries the inputs Run needs; zero values default to the
// process environment (real clock, paths.StateDir()).
type Options struct {
	Root      string    // data repo
	StateDir  string    // timer state dir (defaults set by caller)
	ThemeDir  string    // omarchy state dir for the theme (defaults to theme.StateDir())
	TrueColor string    // ui.truecolor config value (auto|on|off); empty = auto
	Now       time.Time // for staleness math
}

var demoSubjectRe = regexp.MustCompile(`(?i)\bdemo\b`)

func (r *Report) add(level, check, path, format string, args ...any) {
	r.Findings = append(r.Findings, Finding{Check: check, Level: level, Path: path, Message: fmt.Sprintf(format, args...)})
}

// Run executes every check and returns the report. It is read-only.
func Run(opts Options) Report {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	rep := Report{Root: opts.Root, Kind: "unmarked", OK: true}
	finish := func() Report {
		for _, f := range rep.Findings {
			if f.Level == LevelFail {
				rep.OK = false
			}
		}
		return rep
	}

	// --- marker ---
	mk, merr := marker.Read(opts.Root)
	flagDemoAsFail := true // unmarked repos get demo-looking data flagged
	switch {
	case merr == nil:
		rep.Kind = mk.Kind
		flagDemoAsFail = mk.Kind != marker.KindDemo
		rep.add(LevelInfo, "marker", filepath.Join(opts.Root, marker.Filename), ".mtm-data ok (kind: %s, id %s)", mk.Kind, mk.ID)
	case errors.Is(merr, marker.ErrNotExist):
		rep.add(LevelWarn, "marker", filepath.Join(opts.Root, marker.Filename), "no .mtm-data marker — run `mtm init` to write one (kind: real)")
	default:
		rep.add(LevelWarn, "marker", filepath.Join(opts.Root, marker.Filename), "unreadable marker: %v", merr)
	}

	// --- ambient render cost (deterministic; one frame per scene) ---
	rep.AmbientBytesPerFrame = ambient.ReportBytes(ambient.ModeHalfblock, 100, 30)
	paintCosts := ambient.ReportFrameCost(ambient.ModeHalfblock, 100, 30)
	for _, cost := range paintCosts {
		if cost > rep.AmbientPaintNS {
			rep.AmbientPaintNS = cost
		}
	}
	rep.AmbientPaintMs = float64(rep.AmbientPaintNS) / 1e6
	rep.AmbientTier, rep.AmbientTierName = ambient.AmbientTier()

	// --- git repo ---
	repo := sync.New(opts.Root)
	if !repo.IsRepo() {
		rep.add(LevelFail, "git", opts.Root, "not a git repository — run `mtm init`")
	} else {
		rep.add(LevelInfo, "git", opts.Root, "data repo is a git repository")
	}

	// --- remote ---
	remotes := repo.Remotes()
	if len(remotes) == 0 {
		rep.add(LevelWarn, "remote", opts.Root, "no git remote configured — sync has nowhere to push")
	} else {
		rep.add(LevelInfo, "remote", opts.Root, "remote(s): %s", strings.Join(remotes, ", "))
	}

	// --- git identity ---
	name, email := repo.ConfigGet("user.name"), repo.ConfigGet("user.email")
	switch {
	case name == "" && email == "":
		rep.add(LevelWarn, "identity", opts.Root, "git identity not set (user.name and user.email) — commits will fail")
	case name == "":
		rep.add(LevelWarn, "identity", opts.Root, "git user.name not set")
	case email == "":
		rep.add(LevelWarn, "identity", opts.Root, "git user.email not set")
	default:
		rep.add(LevelInfo, "identity", opts.Root, "git identity: %s <%s>", name, email)
	}

	// --- sync state (paused / conflicted) ---
	st, serr := repo.LoadState()
	switch {
	case serr != nil:
		rep.add(LevelWarn, "sync", opts.Root, "cannot read sync state: %v", serr)
	case st.Conflicted:
		since := st.Since.Format("2006-01-02 15:04")
		rep.add(LevelWarn, "sync", opts.Root, "sync paused: conflicts since %s (%s) — fix the files or `mtm sync --abort`, then `mtm sync --continue`", since, strings.Join(st.Files, ", "))
	default:
		if repo.RebaseInProgress() {
			rep.add(LevelWarn, "sync", opts.Root, "a rebase is in progress — sync stays paused until it finishes (`mtm sync --continue` or `mtm sync --abort`)")
		}
		if ahead, err := repo.Ahead(); err == nil && ahead > 0 {
			rep.add(LevelInfo, "sync", opts.Root, "%d commit(s) ahead of upstream", ahead)
		}
	}

	// --- working-tree demo-data scan ---
	// Demo detection looks only at the current tree, never at history:
	// a demo-looking commit message in history is reported at most as an
	// info line, and only when the tree still contains demo data, so a
	// reverted demo commit is never reported. flagDemoAsFail (real or
	// unmarked repo) is computed above from the marker kind.
	demoLevel := LevelWarn
	if flagDemoAsFail {
		demoLevel = LevelFail
	}
	var demoTasks []string
	var demoProjects []string
	for _, base := range []string{"projects", "archive"} {
		walkBase := filepath.Join(opts.Root, base)
		if info, err := os.Stat(walkBase); err != nil || !info.IsDir() {
			continue
		}
		_ = filepath.WalkDir(walkBase, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				rep.add(LevelWarn, "frontmatter", path, "cannot read: %v", err)
				return nil
			}
			if d.IsDir() {
				rel, rerr := filepath.Rel(walkBase, path)
				if rerr == nil && demo.IsDemoProject(filepath.ToSlash(rel)) {
					demoProjects = append(demoProjects, filepath.ToSlash(rel))
				}
				return nil
			}
			if !strings.HasSuffix(d.Name(), ".md") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				rep.add(LevelWarn, "frontmatter", path, "cannot read: %v", err)
				return nil
			}
			rel, rerr := filepath.Rel(walkBase, path)
			seg := ""
			if rerr == nil {
				seg = filepath.ToSlash(filepath.Dir(rel))
			}
			// Milestones and dev-log entries are not task files; check
			// them with their own parsers so real repos with milestones
			// are not false-flagged.
			switch {
			case isUnder(seg, "milestones"):
				if _, err := store.ParseMilestone(data); err != nil {
					rep.add(LevelFail, "frontmatter", path, "invalid milestone frontmatter: %v", err)
				}
				return nil
			case isUnder(seg, "devlog"):
				if _, err := store.ParseDevLog(data); err != nil {
					rep.add(LevelFail, "frontmatter", path, "invalid dev-log frontmatter: %v", err)
				}
				return nil
			}
			t, err := store.ParseTask(data)
			if err != nil {
				rep.add(LevelFail, "frontmatter", path, "invalid task frontmatter: %v", err)
				return nil
			}
			for field, val := range map[string]string{"id": t.ID, "title": t.Title, "project": t.Project, "status": t.Status, "kind": t.Kind, "created": t.Created} {
				if strings.TrimSpace(val) == "" {
					rep.add(LevelFail, "frontmatter", path, "missing required field %q", field)
				}
			}
			if demo.IsDemoID(t.ID) {
				demoTasks = append(demoTasks, t.ID)
				if flagDemoAsFail {
					rep.add(demoLevel, "demo-data", path, "task id %s matches the demo corpus", t.ID)
				}
			}
			return nil
		})
	}
	for _, p := range demoProjects {
		if flagDemoAsFail {
			rep.add(demoLevel, "demo-data", filepath.Join(opts.Root, "projects", filepath.FromSlash(p)), "project %s is a demo corpus project", p)
		}
	}
	treeHasDemo := len(demoTasks) > 0 || len(demoProjects) > 0

	// --- commit messages that look like the demo seeder's ---
	// History alone proves nothing: the demo-seed incident was reverted.
	// Report demo-looking subjects only as an info line, and only when the
	// current tree still contains demo data.
	if repo.IsRepo() && treeHasDemo {
		if subjects, err := repo.LogSubjects(); err == nil {
			var demoSubjects []string
			for _, s := range subjects {
				if demoSubjectRe.MatchString(s) {
					demoSubjects = append(demoSubjects, s)
				}
			}
			if len(demoSubjects) > 0 {
				rep.add(LevelInfo, "demo-data", opts.Root, "%d commit message(s) look like the demo seeder; the current tree also contains demo data", len(demoSubjects))
			}
		}
	}
	if !flagDemoAsFail && treeHasDemo {
		rep.add(LevelInfo, "demo-data", opts.Root, "%d demo-corpus task(s), %d demo project(s) — expected for a demo repo", len(demoTasks), len(demoProjects))
	}

	// --- timer state (stale / corrupt) ---
	if opts.StateDir != "" {
		checkTimer(&rep, opts.StateDir, now)
	}

	// --- terminal color capability (informational) ---
	detected := caps.Detect(os.Getenv)
	trueColor, source := caps.Effective(opts.TrueColor, os.Getenv)
	mode := "truecolor"
	if !trueColor {
		mode = "xterm-256"
	}
	rep.add(LevelInfo, "terminal", "", "detected terminal: %s; color mode: %s (%s)", detected.Program, mode, source)

	// --- theme health: malformed roles recorded at load ---
	// mtm's palettes fall back to the default for a malformed role; the
	// bad role name lands in the diagnostics lists and is surfaced here.
	themeDir := opts.ThemeDir
	if themeDir == "" {
		themeDir = theme.StateDir()
	}
	theme.ResetDiagnostics()
	ambient.ResetDiagnostics()
	_, _ = theme.LoadFrom(themeDir)
	for _, d := range theme.Diagnostics() {
		rep.add(LevelWarn, "theme", "", "theme: %s", d)
	}
	for _, d := range ambient.Diagnostics() {
		rep.add(LevelWarn, "theme", "", "theme: %s", d)
	}

	return finish()
}

// isUnder reports whether slash-separated dir is or sits below a directory
// named name (e.g. projects/college/dbms/milestones is under "milestones").
func isUnder(dir, name string) bool {
	for _, part := range strings.Split(filepath.ToSlash(dir), "/") {
		if part == name {
			return true
		}
	}
	return false
}

func checkTimer(rep *Report, stateDir string, now time.Time) {
	path := filepath.Join(stateDir, "timer.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// No timer state is the common case — nothing to report.
		if _, cerr := os.Stat(path + ".corrupt"); cerr == nil {
			rep.add(LevelWarn, "timer", path+".corrupt", "a corrupt timer state file was moved aside — accrual from that session may be lost; check time_logged_minutes on the task")
		}
		return
	}
	if err != nil {
		rep.add(LevelWarn, "timer", path, "cannot read timer state: %v", err)
		return
	}
	var st core.TimerState
	if err := json.Unmarshal(data, &st); err != nil {
		rep.add(LevelWarn, "timer", path, "unparseable timer state: %v (move it aside or `mtm timer stop`)", err)
		return
	}
	view := st.View(now)
	if view.Stale {
		rep.add(LevelWarn, "timer", path, "stale timer: mode=%s task=%s accrued=%s — forgotten session? stop it with `mtm timer stop`", st.Mode, st.TaskID, formatDuration(view.Accrued))
		return
	}
	rep.add(LevelInfo, "timer", path, "active timer: mode=%s task=%s phase=%s accrued=%s", st.Mode, st.TaskID, st.Phase, formatDuration(view.Accrued))
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return d.Round(time.Second).String()
	}
	return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
}
