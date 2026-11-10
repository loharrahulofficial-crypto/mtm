package core

import (
	"fmt"
	"strings"
)

// Idea lifecycle statuses.
const (
	IdeaSeed      = "seed"
	IdeaExploring = "exploring"
	IdeaParked    = "parked"
	IdeaPromoted  = "promoted"
	IdeaDropped   = "dropped"
)

var IdeaStatuses = []string{IdeaSeed, IdeaExploring, IdeaParked, IdeaPromoted, IdeaDropped}

// IdeaStatusesActive excludes the terminal statuses (promoted|dropped).
var IdeaStatusesActive = []string{IdeaSeed, IdeaExploring, IdeaParked}

// Idea is one file under ideas/: YAML frontmatter + Markdown notes.
type Idea struct {
	ID      string   `yaml:"id"`
	Title   string   `yaml:"title"`
	Status  string   `yaml:"status"`
	Tags    []string `yaml:"tags,omitempty"`
	Created string   `yaml:"created"`
	Updated string   `yaml:"updated"`
	// Rating is 1-5; 0 means unrated.
	Rating     int      `yaml:"rating,omitempty"`
	Links      []string `yaml:"links,omitempty"`
	PromotedTo string   `yaml:"promoted_to,omitempty"`
	// Body is the Markdown notes below the frontmatter fence. Not YAML.
	Body string `yaml:"-"`
}

// ValidIdeaStatus reports whether s is a known idea status (case-insensitive).
func ValidIdeaStatus(s string) bool {
	for _, v := range IdeaStatuses {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

// CanonicalIdeaStatus returns the canonical casing for a status, or "".
func CanonicalIdeaStatus(s string) string {
	for _, v := range IdeaStatuses {
		if strings.EqualFold(v, s) {
			return v
		}
	}
	return ""
}

// NextIdeaStatus cycles the active lifecycle seed → exploring → parked →
// seed. Terminal statuses (promoted|dropped) cycle back into seed.
func NextIdeaStatus(s string) string {
	seq := IdeaStatusesActive
	for i, v := range seq {
		if strings.EqualFold(v, s) {
			return seq[(i+1)%len(seq)]
		}
	}
	return IdeaSeed
}

// Validate checks the idea is usable.
func (i *Idea) Validate() error {
	if strings.TrimSpace(i.Title) == "" {
		return fmt.Errorf("idea needs a title")
	}
	if i.Status == "" {
		return fmt.Errorf("idea needs a status")
	}
	if !ValidIdeaStatus(i.Status) {
		return fmt.Errorf("idea status %q is not one of %s", i.Status, strings.Join(IdeaStatuses, "|"))
	}
	i.Status = CanonicalIdeaStatus(i.Status)
	if i.Rating < 0 || i.Rating > 5 {
		return fmt.Errorf("idea rating must be 0 (unrated) or 1-5, got %d", i.Rating)
	}
	return nil
}

// ParseRating validates a CLI-supplied rating (1-5; "0" is not accepted on
// the CLI — unrated is the absence of a rating).
func ParseRating(s string) (int, error) {
	var n int
	if _, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &n); err != nil {
		return 0, fmt.Errorf("rating must be a number 1-5, got %q", s)
	}
	if n < 1 || n > 5 {
		return 0, fmt.Errorf("rating must be 1-5, got %d", n)
	}
	return n, nil
}
