package core

import (
	"fmt"
	"strings"
)

// DevLogEntry is one file under projects/<project>/devlog/:
// <YYYY-MM-DD>-<id>.md with YAML frontmatter + a Markdown body.
// The project comes from the file path and is never stored in the file.
type DevLogEntry struct {
	ID    string `yaml:"id"`
	Title string `yaml:"title"`
	Mood  string `yaml:"mood,omitempty"`
	// TimeSpentMinutes is the optional wall-clock time this entry covers
	// (when started from a running timer, the elapsed minutes).
	TimeSpentMinutes int      `yaml:"time_spent_minutes,omitempty"`
	Tags             []string `yaml:"tags,omitempty"`
	Created          string   `yaml:"created"`
	Project          string   `yaml:"-"` // set from the path by the store
	// Body is the Markdown notes below the frontmatter fence.
	Body string `yaml:"-"`
}

// DevLogFileName builds the canonical file name for an entry.
func DevLogFileName(date, id string) string {
	return date + "-" + id + ".md"
}

// Validate checks the entry is usable.
func (d *DevLogEntry) Validate() error {
	if strings.TrimSpace(d.Title) == "" {
		return fmt.Errorf("devlog entry needs a title")
	}
	if d.TimeSpentMinutes < 0 {
		return fmt.Errorf("devlog time_spent_minutes must be >= 0, got %d", d.TimeSpentMinutes)
	}
	return nil
}
