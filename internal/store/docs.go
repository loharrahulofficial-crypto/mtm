package store

// CRUD for the non-task document types: ideas (ideas/), milestones and
// dev-log entries (projects/<project>/milestones|devlog/). All are plain
// Markdown + YAML frontmatter — one file per item, sync-safe.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

// ---- ideas ---------------------------------------------------------------

// Ideas returns all indexed ideas (filesystem walk order).
func (s *Store) Ideas() []*core.Idea { return s.ideas }

// ResolveIdea finds an idea by id or unique id prefix (case-insensitive).
func (s *Store) ResolveIdea(prefix string) (*core.Idea, error) {
	if prefix == "" {
		return nil, errors.New("empty idea id")
	}
	prefix = strings.ToLower(prefix)
	var matches []*core.Idea
	for _, i := range s.ideas {
		if strings.HasPrefix(strings.ToLower(i.ID), prefix) {
			matches = append(matches, i)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no idea with id %q", prefix)
	case 1:
		return matches[0], nil
	default:
		ids := make([]string, len(matches))
		for i, m := range matches {
			ids[i] = m.ID
		}
		return nil, fmt.Errorf("ambiguous id prefix %q matches %s", prefix, strings.Join(ids, ", "))
	}
}

// CreateIdea writes a new idea file and indexes it.
func (s *Store) CreateIdea(i *core.Idea) error {
	if err := i.Validate(); err != nil {
		return err
	}
	if i.ID == "" {
		i.ID = s.uniqueID()
	}
	if i.Status == "" {
		i.Status = core.IdeaSeed
	}
	now := s.Now()
	if i.Created == "" {
		i.Created = s.Clock.StampAt(now)
	}
	i.Updated = s.Clock.StampAt(now)
	rel := filepath.ToSlash(filepath.Join("ideas", i.ID+"-"+core.Slugify(i.Title)+".md"))
	data, err := SerializeIdea(i)
	if err != nil {
		return err
	}
	path := s.absPath(rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := WriteFileAtomic(path, data, 0o644); err != nil {
		return err
	}
	s.ideas = append(s.ideas, i)
	s.iPaths[i.ID] = rel
	s.gen++
	return s.saveCache()
}

// UpdateIdea rewrites an existing idea file and refreshes the index.
func (s *Store) UpdateIdea(i *core.Idea) error {
	rel, ok := s.iPaths[i.ID]
	if !ok {
		return fmt.Errorf("idea %q is not in the index", i.ID)
	}
	if err := i.Validate(); err != nil {
		return err
	}
	i.Updated = s.Clock.Stamp()
	data, err := SerializeIdea(i)
	if err != nil {
		return err
	}
	if err := WriteFileAtomic(s.absPath(rel), data, 0o644); err != nil {
		return err
	}
	for j, existing := range s.ideas {
		if existing.ID == i.ID {
			s.ideas[j] = i
		}
	}
	s.gen++
	return s.saveCache()
}

// RemoveIdea deletes the idea file and drops it from the index.
func (s *Store) RemoveIdea(i *core.Idea) error {
	rel, ok := s.iPaths[i.ID]
	if !ok {
		return fmt.Errorf("idea %q is not in the index", i.ID)
	}
	if err := os.Remove(s.absPath(rel)); err != nil {
		return err
	}
	delete(s.iPaths, i.ID)
	for j, existing := range s.ideas {
		if existing.ID == i.ID {
			s.ideas = append(s.ideas[:j], s.ideas[j+1:]...)
			break
		}
	}
	s.gen++
	return s.saveCache()
}

// IdeaPath returns the absolute path of an idea by id.
func (s *Store) IdeaPath(id string) (string, error) {
	rel, ok := s.iPaths[id]
	if !ok {
		return "", fmt.Errorf("idea %q is not in the index", id)
	}
	return s.absPath(rel), nil
}

// ReparseIdea re-reads an idea file from disk (used by edit to pick up
// changes).
func (s *Store) ReparseIdea(i *core.Idea) error {
	rel, ok := s.iPaths[i.ID]
	if !ok {
		return fmt.Errorf("idea %q is not in the index", i.ID)
	}
	data, err := os.ReadFile(s.absPath(rel))
	if err != nil {
		return err
	}
	parsed, err := ParseIdea(data)
	if err != nil {
		return err
	}
	*i = *parsed
	s.gen++
	return nil
}

// ---- milestones -----------------------------------------------------------

// Milestones returns the indexed milestones; project "" means all of them.
// Order is filesystem walk order (sort by target for display).
func (s *Store) Milestones(project string) []*core.Milestone {
	if strings.TrimSpace(project) == "" {
		return s.milestones
	}
	var out []*core.Milestone
	for _, m := range s.milestones {
		if strings.EqualFold(m.Project, project) {
			out = append(out, m)
		}
	}
	return out
}

// ResolveMilestone finds a milestone by id or unique id prefix
// (case-insensitive). Matches across all projects.
func (s *Store) ResolveMilestone(prefix string) (*core.Milestone, error) {
	if prefix == "" {
		return nil, errors.New("empty milestone id")
	}
	prefix = strings.ToLower(prefix)
	var matches []*core.Milestone
	for _, m := range s.milestones {
		if strings.HasPrefix(strings.ToLower(m.ID), prefix) {
			matches = append(matches, m)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no milestone with id %q", prefix)
	case 1:
		return matches[0], nil
	default:
		ids := make([]string, len(matches))
		for i, m := range matches {
			ids[i] = m.ID
		}
		return nil, fmt.Errorf("ambiguous id prefix %q matches %s", prefix, strings.Join(ids, ", "))
	}
}

// CreateMilestone writes a new milestone file under
// projects/<project>/milestones/ and indexes it. The project must exist.
func (s *Store) CreateMilestone(m *core.Milestone) error {
	if err := validateProjectName(m.Project); err != nil {
		return err
	}
	if _, err := s.GetProject(m.Project); err != nil {
		return fmt.Errorf("project %q: %w — run `mtm project add %s`", m.Project, err, m.Project)
	}
	if err := m.Validate(); err != nil {
		return err
	}
	if m.ID == "" {
		m.ID = s.uniqueID()
	}
	now := s.Now()
	if m.Created == "" {
		m.Created = s.Clock.StampAt(now)
	}
	m.Updated = s.Clock.StampAt(now)
	rel := filepath.ToSlash(filepath.Join("projects", filepath.FromSlash(m.Project), "milestones",
		m.ID+"-"+core.Slugify(m.Title)+".md"))
	data, err := SerializeMilestone(m)
	if err != nil {
		return err
	}
	path := s.absPath(rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := WriteFileAtomic(path, data, 0o644); err != nil {
		return err
	}
	s.milestones = append(s.milestones, m)
	s.mPaths[m.ID] = rel
	s.gen++
	return s.saveCache()
}

// UpdateMilestone rewrites an existing milestone file and refreshes the index.
func (s *Store) UpdateMilestone(m *core.Milestone) error {
	rel, ok := s.mPaths[m.ID]
	if !ok {
		return fmt.Errorf("milestone %q is not in the index", m.ID)
	}
	if err := m.Validate(); err != nil {
		return err
	}
	m.Updated = s.Clock.Stamp()
	data, err := SerializeMilestone(m)
	if err != nil {
		return err
	}
	if err := WriteFileAtomic(s.absPath(rel), data, 0o644); err != nil {
		return err
	}
	for j, existing := range s.milestones {
		if existing.ID == m.ID {
			s.milestones[j] = m
		}
	}
	s.gen++
	return s.saveCache()
}

// MilestonePath returns the absolute path of a milestone by id.
func (s *Store) MilestonePath(id string) (string, error) {
	rel, ok := s.mPaths[id]
	if !ok {
		return "", fmt.Errorf("milestone %q is not in the index", id)
	}
	return s.absPath(rel), nil
}

// ReparseMilestone re-reads a milestone file from disk.
func (s *Store) ReparseMilestone(m *core.Milestone) error {
	rel, ok := s.mPaths[m.ID]
	if !ok {
		return fmt.Errorf("milestone %q is not in the index", m.ID)
	}
	data, err := os.ReadFile(s.absPath(rel))
	if err != nil {
		return err
	}
	parsed, err := ParseMilestone(data)
	if err != nil {
		return err
	}
	parsed.Project = m.Project
	*m = *parsed
	s.gen++
	return nil
}

// ---- dev-log ---------------------------------------------------------------

// DevLogs returns the indexed dev-log entries; project "" means all of
// them. Order is filesystem walk order — sort by date for display.
func (s *Store) DevLogs(project string) []*core.DevLogEntry {
	if strings.TrimSpace(project) == "" {
		return s.devlogs
	}
	var out []*core.DevLogEntry
	for _, d := range s.devlogs {
		if strings.EqualFold(d.Project, project) {
			out = append(out, d)
		}
	}
	return out
}

// CreateDevLog writes a new dev-log entry under
// projects/<project>/devlog/<YYYY-MM-DD>-<id>.md and indexes it.
func (s *Store) CreateDevLog(d *core.DevLogEntry) error {
	if err := validateProjectName(d.Project); err != nil {
		return err
	}
	if _, err := s.GetProject(d.Project); err != nil {
		return fmt.Errorf("project %q: %w — run `mtm project add %s`", d.Project, err, d.Project)
	}
	if err := d.Validate(); err != nil {
		return err
	}
	if d.ID == "" {
		d.ID = s.uniqueID()
	}
	now := s.Now()
	if d.Created == "" {
		d.Created = s.Clock.StampAt(now)
	}
	date := d.Created
	if t, err := core.ParseDue(d.Created, now.Location()); err == nil {
		date = t.Format("2006-01-02")
	} else if len(date) >= 10 {
		date = date[:10]
	}
	rel := filepath.ToSlash(filepath.Join("projects", filepath.FromSlash(d.Project), "devlog",
		core.DevLogFileName(date, d.ID)))
	data, err := SerializeDevLog(d)
	if err != nil {
		return err
	}
	path := s.absPath(rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := WriteFileAtomic(path, data, 0o644); err != nil {
		return err
	}
	s.devlogs = append(s.devlogs, d)
	s.dPaths[d.ID] = rel
	s.gen++
	return s.saveCache()
}

// UpdateDevLog rewrites an existing dev-log entry and refreshes the index.
func (s *Store) UpdateDevLog(d *core.DevLogEntry) error {
	rel, ok := s.dPaths[d.ID]
	if !ok {
		return fmt.Errorf("devlog entry %q is not in the index", d.ID)
	}
	if err := d.Validate(); err != nil {
		return err
	}
	data, err := SerializeDevLog(d)
	if err != nil {
		return err
	}
	if err := WriteFileAtomic(s.absPath(rel), data, 0o644); err != nil {
		return err
	}
	for j, existing := range s.devlogs {
		if existing.ID == d.ID {
			s.devlogs[j] = d
		}
	}
	s.gen++
	return s.saveCache()
}

// DevLogPath returns the absolute path of a dev-log entry by id.
func (s *Store) DevLogPath(id string) (string, error) {
	rel, ok := s.dPaths[id]
	if !ok {
		return "", fmt.Errorf("devlog entry %q is not in the index", id)
	}
	return s.absPath(rel), nil
}

// TemplateFile reads templates/<name>.md from the data repo. name must be a
// single clean path segment (no separators, no traversal).
func (s *Store) TemplateFile(name string) ([]byte, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		return nil, fmt.Errorf("template name %q must be a single file name", name)
	}
	if !strings.HasSuffix(name, ".md") {
		name += ".md"
	}
	path := filepath.Join(s.Root, "templates", name)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("templates/%s: %w", name, err)
	}
	return data, nil
}
