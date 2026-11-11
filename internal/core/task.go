// Package core holds the domain models and logic of mtm.
// CLI and TUI must not contain business logic; they call into core.
package core

import (
	"crypto/rand"
	"fmt"
	"strings"
	"unicode"
)

// Priorities (low|medium|high|urgent).
const (
	PriorityLow    = "low"
	PriorityMedium = "medium"
	PriorityHigh   = "high"
	PriorityUrgent = "urgent"
)

var Priorities = []string{PriorityLow, PriorityMedium, PriorityHigh, PriorityUrgent}

// Kinds (task|assignment|exam|study|project).
const (
	KindTask       = "task"
	KindAssignment = "assignment"
	KindExam       = "exam"
	KindStudy      = "study"
	KindProject    = "project"
)

var Kinds = []string{KindTask, KindAssignment, KindExam, KindStudy, KindProject}

// TimeFormat is the local-naive layout used everywhere in frontmatter (D-004).
const TimeFormat = "2006-01-02T15:04"

// TimestampFormat is the layout used for created/updated/completed.
const TimestampFormat = "2006-01-02T15:04"

// Task is one Markdown file on disk: YAML frontmatter + a Markdown body.
type Task struct {
	ID                string   `yaml:"id"`
	Title             string   `yaml:"title"`
	Project           string   `yaml:"project"`
	Status            string   `yaml:"status"`
	Priority          string   `yaml:"priority"`
	Tags              []string `yaml:"tags,omitempty"`
	Due               string   `yaml:"due,omitempty"`
	Reminder          string   `yaml:"reminder,omitempty"`
	Recurrence        string   `yaml:"recurrence,omitempty"`
	Parent            string   `yaml:"parent,omitempty"`
	Milestone         string   `yaml:"milestone,omitempty"`
	Created           string   `yaml:"created"`
	Updated           string   `yaml:"updated"`
	Completed         string   `yaml:"completed,omitempty"`
	EstimateMinutes   int      `yaml:"estimate_minutes,omitempty"`
	TimeLoggedMinutes int      `yaml:"time_logged_minutes,omitempty"`
	Weight            *float64 `yaml:"weight,omitempty"`
	Score             *float64 `yaml:"score,omitempty"`
	// ScoreMax is the maximum the recorded score is out of (default 100).
	ScoreMax *float64 `yaml:"score_max,omitempty"`
	Kind     string   `yaml:"kind"`
	// Body is the Markdown notes below the frontmatter fence. Not YAML.
	Body string `yaml:"-"`
	// Archived is set by the store when the task was loaded from archive/.
	Archived bool `yaml:"-"`
}

// NewID returns a random 6-hex-char id (matches the spec example).
func NewID() string {
	var b [3]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failure is unrecoverable for id generation
		panic(err)
	}
	return fmt.Sprintf("%x", b)
}

// Slugify converts a title into a filesystem-safe slug.
func Slugify(title string) string {
	var b strings.Builder
	lastDash := true // trim leading dashes
	for _, r := range strings.ToLower(title) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > 48 {
		// Truncate on a rune boundary so multibyte titles never split a rune.
		r := []rune(slug)
		if len(r) > 48 {
			r = r[:48]
		}
		slug = strings.TrimRight(strings.TrimSuffix(string(r), "-"), "-")
	}
	if slug == "" {
		slug = "task"
	}
	return slug
}

// ValidPriority reports whether p is a known priority.
func ValidPriority(p string) bool {
	for _, v := range Priorities {
		if v == p {
			return true
		}
	}
	return false
}

// ValidKind reports whether k is a known kind.
func ValidKind(k string) bool {
	for _, v := range Kinds {
		if v == k {
			return true
		}
	}
	return false
}

// IsDone is a convenience over project metadata; see store.IsDoneStatus.
// With no project info, the common convention is status == "done".
func IsDoneStatusFallback(status string) bool {
	s := strings.ToLower(status)
	return s == "done" || s == "completed" || s == "cancelled" || s == "canceled"
}
