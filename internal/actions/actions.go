// Package actions holds the shared application-level operations used by both
// the CLI and the TUI: quick-add creation, done/status transitions with
// recurring-occurrence spawning, and commit+sync. Nothing in here formats
// user-facing output except terse result strings; the UIs render them.
package actions

import (
	"fmt"
	"strings"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/parse"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

// CommitResult reports what happened to a change's git bookkeeping.
type CommitResult struct {
	Committed bool
	Synced    bool
	Conflict  bool
	// Note is a one-line human explanation when something needs attention
	// (e.g. committed locally but not pushed). Empty on full success.
	Note string
}

// CommitAndSync auto-commits a data-repo change and best-effort syncs.
// Failures never roll back the change; they are reported in the Note.
func CommitAndSync(cfg config.Config, root string, msg string) CommitResult {
	if !cfg.Sync.Auto {
		return CommitResult{}
	}
	repo := sync.New(root)
	if !repo.IsRepo() {
		return CommitResult{Note: fmt.Sprintf("data repo at %s is not a git repository — change not committed", root)}
	}
	committed, err := repo.CommitAll(msg)
	if err != nil {
		return CommitResult{Note: fmt.Sprintf("auto-commit failed: %v", err)}
	}
	_, err = repo.Sync(cfg.Sync.Remote)
	if err != nil {
		if err == sync.ErrConflicted {
			return CommitResult{Committed: committed, Conflict: true, Note: "sync paused: conflicts need manual resolution — run `mtm sync --status`"}
		}
		return CommitResult{Committed: committed, Note: fmt.Sprintf("committed locally but not pushed (%v) — run `mtm sync` when online", err)}
	}
	return CommitResult{Committed: committed, Synced: true}
}

// SpawnNextOccurrence creates the follow-up for a recurring task that was
// just completed. It returns a one-line message describing what was spawned,
// or "" when there is nothing to spawn.
func SpawnNextOccurrence(s *store.Store, t *core.Task) string {
	if strings.TrimSpace(t.Recurrence) == "" {
		return ""
	}
	first := "todo"
	if p, err := s.GetProject(t.Project); err == nil && len(p.Statuses) > 0 {
		first = p.Statuses[0].Name
	}
	spawned, ok, err := core.SpawnRecurringOccurrence(t, first, s.Clock.Now())
	if err != nil {
		return fmt.Sprintf("could not compute next occurrence: %v", err)
	}
	if !ok {
		return ""
	}
	if err := s.Create(spawned); err != nil {
		return fmt.Sprintf("could not create next occurrence: %v", err)
	}
	return fmt.Sprintf("next occurrence %s  due %s", spawned.ID, spawned.Due)
}

// CompleteTask marks t done (the project's first done status), spawns the
// next occurrence of a recurring task, persists it and returns the spawn
// message. Mirrors `mtm done`.
func CompleteTask(s *store.Store, t *core.Task) (string, error) {
	p, err := s.GetProject(t.Project)
	if err != nil {
		return "", fmt.Errorf("project %q: %w", t.Project, err)
	}
	doneStatus := p.DoneStatus()
	if doneStatus == "" {
		return "", fmt.Errorf("project %q has no status marked done", t.Project)
	}
	t.Status = doneStatus
	t.Completed = s.Clock.Stamp()
	spawnMsg := SpawnNextOccurrence(s, t)
	if err := s.Update(t); err != nil {
		return "", err
	}
	return spawnMsg, nil
}

// SetStatus moves t to canonical (already validated by the caller against
// the project's workflow), maintains the Completed timestamp and spawns the
// next occurrence when entering a done status from a non-done one. Returns
// the spawn message, or "".
func SetStatus(s *store.Store, t *core.Task, canonical string) (string, error) {
	p, err := s.GetProject(t.Project)
	if err != nil {
		return "", fmt.Errorf("project %q: %w", t.Project, err)
	}
	wasDone := p.IsDoneStatus(t.Status)
	spawnMsg := ""
	t.Status = canonical
	if p.IsDoneStatus(canonical) {
		t.Completed = s.Clock.Stamp()
		if !wasDone {
			spawnMsg = SpawnNextOccurrence(s, t)
		}
	} else {
		t.Completed = ""
	}
	if err := s.Update(t); err != nil {
		return "", err
	}
	return spawnMsg, nil
}

// NextStatus returns the next non-done status after current in the project
// workflow, wrapping around; used by one-key "advance status" actions.
func NextStatus(p *core.Project, current string) string {
	if len(p.Statuses) == 0 {
		return current
	}
	idx := -1
	for i, st := range p.Statuses {
		if strings.EqualFold(st.Name, current) {
			idx = i
			break
		}
	}
	for i := 1; i <= len(p.Statuses); i++ {
		st := p.Statuses[(idx+i+len(p.Statuses))%len(p.Statuses)]
		if !st.Done {
			return st.Name
		}
	}
	return p.Statuses[(idx+1+len(p.Statuses))%len(p.Statuses)].Name
}

// QuickAddOverrides carries explicit user choices that win over the parsed
// natural-language values. The zero value means "everything parsed".
type QuickAddOverrides struct {
	Project  string   // "" = parsed, then cfg.DefaultProject
	Priority string   // "" = parsed, then medium
	Tags     []string // nil = parsed
	Due      string   // normalized via core.DueInput when DueSet
	DueSet   bool
	Kind     string   // "" = task
	Parent   string   // "" = no parent
	Weight   *float64 // nil = unset
	Now      time.Time
}

// QuickAddTask parses raw with the natural-language quick-add parser, applies
// overrides (flags win) and creates the task. Mirrors `mtm add` defaults.
func QuickAddTask(cfg config.Config, s *store.Store, raw string, ov QuickAddOverrides) (*core.Task, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("a title is required, e.g. \"submit report friday 6pm !high\"")
	}
	now := ov.Now
	if now.IsZero() {
		now = s.Clock.Now()
	}
	parsed := parse.QuickAdd(raw, now)
	title := parsed.Title
	if title == "" {
		return nil, fmt.Errorf("no title left after parsing — did you mean to write some words of the task itself?")
	}
	project := ov.Project
	if project == "" {
		project = parsed.Project
	}
	if project == "" {
		project = cfg.DefaultProject
	}
	priority := ov.Priority
	if priority == "" {
		priority = parsed.Priority
	}
	if priority == "" {
		priority = core.PriorityMedium
	}
	priority = strings.ToLower(strings.TrimSpace(priority))
	if !core.ValidPriority(priority) {
		return nil, fmt.Errorf("invalid priority %q (want low|medium|high|urgent)", priority)
	}
	var due string
	if ov.DueSet {
		normalized, err := core.DueInput(ov.Due)
		if err != nil {
			return nil, err
		}
		due = normalized
	} else {
		due = parsed.Due
	}
	tags := ov.Tags
	if tags == nil {
		tags = parsed.Tags
	}
	kind := strings.ToLower(strings.TrimSpace(ov.Kind))
	if kind == "" {
		kind = core.KindTask
	}
	if !core.ValidKind(kind) {
		return nil, fmt.Errorf("invalid kind %q (want task|assignment|exam|study|project)", kind)
	}
	p, err := s.GetProject(project)
	if err != nil {
		return nil, fmt.Errorf("project %q: %w — run `mtm project add %s`", project, err, project)
	}
	first := "todo"
	if len(p.Statuses) > 0 {
		first = p.Statuses[0].Name
	}
	t := &core.Task{
		Title:           title,
		Project:         project,
		Status:          first,
		Priority:        priority,
		Tags:            tags,
		Due:             due,
		Recurrence:      parsed.Recurrence,
		EstimateMinutes: parsed.EstimateMinutes,
		Kind:            kind,
		Parent:          ov.Parent,
		Weight:          ov.Weight,
	}
	if err := s.Create(t); err != nil {
		return nil, err
	}
	return t, nil
}
