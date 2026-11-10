package core

import (
	"fmt"
	"strings"
	"time"
)

// Milestone lifecycle statuses.
const (
	MilestonePlanned = "planned"
	MilestoneActive  = "active"
	MilestoneDone    = "done"
	MilestoneMissed  = "missed"
)

var MilestoneStatuses = []string{MilestonePlanned, MilestoneActive, MilestoneDone, MilestoneMissed}

// Milestone is one file under projects/<project>/milestones/: YAML
// frontmatter + an optional Markdown description.
// The project comes from the file path and is never stored in the file.
type Milestone struct {
	ID          string `yaml:"id"`
	Title       string `yaml:"title"`
	Target      string `yaml:"target,omitempty"` // local-naive date 2006-01-02
	Status      string `yaml:"status"`           // planned|active|done|missed
	Description string `yaml:"description,omitempty"`
	Created     string `yaml:"created"`
	Updated     string `yaml:"updated"`
	Project     string `yaml:"-"` // set from the path by the store
	// Body is the Markdown description below the frontmatter fence.
	Body string `yaml:"-"`
}

// ValidMilestoneStatus reports whether s is a known status (case-insensitive).
func ValidMilestoneStatus(s string) bool {
	for _, v := range MilestoneStatuses {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

// CanonicalMilestoneStatus returns the canonical casing for a status, or "".
func CanonicalMilestoneStatus(s string) string {
	for _, v := range MilestoneStatuses {
		if strings.EqualFold(v, s) {
			return v
		}
	}
	return ""
}

// Validate checks the milestone is usable.
func (m *Milestone) Validate() error {
	if strings.TrimSpace(m.Title) == "" {
		return fmt.Errorf("milestone needs a title")
	}
	if m.Status == "" {
		return fmt.Errorf("milestone needs a status")
	}
	if !ValidMilestoneStatus(m.Status) {
		return fmt.Errorf("milestone status %q is not one of %s", m.Status, strings.Join(MilestoneStatuses, "|"))
	}
	m.Status = CanonicalMilestoneStatus(m.Status)
	if m.Target != "" {
		if _, err := time.ParseInLocation("2006-01-02", m.Target, time.UTC); err != nil {
			return fmt.Errorf("milestone target %q is not a date (want 2006-01-02)", m.Target)
		}
	}
	return nil
}

// MilestoneRisk classifies a milestone for the roadmap display.
const (
	MilestoneRiskOK      = 0 // on track (or done)
	MilestoneRiskAt      = 1 // due within 7 days and not done
	MilestoneRiskOverdue = 2 // target passed and not done
)

// MilestoneRisk reports the risk bucket for display. Done milestones are
// always OK; a missing target is OK (there is nothing to miss).
func MilestoneRisk(m *Milestone, now time.Time) int {
	if strings.EqualFold(m.Status, MilestoneDone) || m.Target == "" {
		return MilestoneRiskOK
	}
	loc := now.Location()
	day, err := time.ParseInLocation("2006-01-02", m.Target, loc)
	if err != nil {
		return MilestoneRiskOK
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	days := int(day.Sub(today).Hours() / 24)
	if days < 0 {
		return MilestoneRiskOverdue
	}
	if days <= 7 {
		return MilestoneRiskAt
	}
	return MilestoneRiskOK
}

// MilestoneScope returns the tasks a milestone covers: the tasks whose
// milestone links to id, plus every descendant of those tasks (subtasks of
// subtasks included). Indexed by id for sorting stability.
func MilestoneScope(all []*Task, milestoneID string) []*Task {
	seen := map[string]bool{}
	var out []*Task
	var add func(t *Task)
	add = func(t *Task) {
		if seen[t.ID] {
			return
		}
		seen[t.ID] = true
		out = append(out, t)
		for _, child := range all {
			if child.Parent == t.ID {
				add(child)
			}
		}
	}
	for _, t := range all {
		if strings.EqualFold(t.Milestone, milestoneID) {
			add(t)
		}
	}
	return out
}

// MilestoneProgress reports done/total and the fraction complete over the
// milestone's scope. Tasks with no scope report total == 0.
func MilestoneProgress(all []*Task, milestoneID string, isDone func(project, status string) bool) (done, total int, frac float64) {
	scope := MilestoneScope(all, milestoneID)
	for _, t := range scope {
		total++
		doneNow := IsDoneStatusFallback(t.Status)
		if isDone != nil {
			doneNow = isDone(t.Project, t.Status)
		}
		if doneNow {
			done++
		}
	}
	if total == 0 {
		return 0, 0, 0
	}
	return done, total, float64(done) / float64(total)
}
