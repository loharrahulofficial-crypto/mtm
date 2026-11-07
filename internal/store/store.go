// Package store reads and writes task Markdown files and keeps an
// in-memory index, optionally persisted to ~/.cache/mtm/index.json (D-006).
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/clock"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"gopkg.in/yaml.v3"
)

type Store struct {
	Root        string
	cachePath   string
	tasks       []*core.Task
	paths       map[string]string // task id -> path relative to Root
	projects    map[string]*core.Project
	diagnostics []string
	// Clock stamps created/updated (local-naive, D-004/D-011). Zero value
	// means the real system clock.
	Clock clock.Clock
}

type cacheFile struct {
	Version int          `json:"version"`
	Entries []cacheEntry `json:"entries"`
}

type cacheEntry struct {
	Path  string     `json:"path"`
	Mtime int64      `json:"mtime"`
	Size  int64      `json:"size"`
	Task  *core.Task `json:"task"`
}

const cacheVersion = 1

// Now returns the store's current local time: the injectable clock when
// present, the system clock otherwise. One source so stored stamps and any
// displayed clock agree (D-011).
func (s *Store) Now() time.Time {
	return s.Clock.Now() // zero-value Clock falls back to time.Now
}

// Open loads the data repo at root, rebuilding the index (with cache help).
// Pass includeArchived to also index archive/<project>/ tasks; they are
// marked Task.Archived and stay out of the default index otherwise.
func Open(root, cachePath string, includeArchived ...bool) (*Store, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("data repo not initialized at %s — run `mtm init` first", abs)
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("data repo path %s is not a directory", abs)
	}
	s := &Store{
		Root:      abs,
		cachePath: cachePath,
		paths:     map[string]string{},
		projects:  map[string]*core.Project{},
	}
	cache := s.loadCache()
	walk := func(base string, archived bool) {
		baseAbs := filepath.Join(abs, base)
		if _, statErr := os.Stat(baseAbs); statErr != nil {
			return
		}
		_ = filepath.WalkDir(baseAbs, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				s.diagnostics = append(s.diagnostics, fmt.Sprintf("%s: %v", path, err))
				return nil
			}
			if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
				return nil
			}
			rel, err := filepath.Rel(abs, path)
			if err != nil {
				return nil
			}
			key := filepath.ToSlash(rel)
			if t, ok := cache[key]; ok && t != nil {
				s.register(t, key)
				if archived {
					t.Archived = true
				}
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				s.diagnostics = append(s.diagnostics, fmt.Sprintf("read %s: %v", rel, err))
				return nil
			}
			t, err := ParseTask(data)
			if err != nil {
				s.diagnostics = append(s.diagnostics, fmt.Sprintf("parse %s: %v", rel, err))
				return nil
			}
			t.Archived = archived
			s.register(t, key)
			return nil
		})
	}
	walk("projects", false)
	if len(includeArchived) > 0 && includeArchived[0] {
		walk("archive", true)
	}
	return s, nil
}

func (s *Store) register(t *core.Task, rel string) {
	s.tasks = append(s.tasks, t)
	s.paths[t.ID] = rel
}

func (s *Store) loadCache() map[string]*core.Task {
	out := map[string]*core.Task{}
	data, err := os.ReadFile(s.cachePath)
	if err != nil {
		return out
	}
	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil || cf.Version != cacheVersion {
		return out
	}
	for _, e := range cf.Entries {
		fi, err := os.Stat(filepath.Join(s.Root, filepath.FromSlash(e.Path)))
		if err != nil {
			continue
		}
		if fi.ModTime().UnixNano() != e.Mtime || fi.Size() != e.Size {
			continue
		}
		out[e.Path] = e.Task
	}
	return out
}

func (s *Store) saveCache() error {
	if s.cachePath == "" {
		return nil
	}
	entries := make([]cacheEntry, 0, len(s.tasks))
	for _, t := range s.tasks {
		rel, ok := s.paths[t.ID]
		if !ok {
			continue
		}
		fi, err := os.Stat(s.absPath(rel))
		if err != nil {
			continue
		}
		entries = append(entries, cacheEntry{
			Path:  rel,
			Mtime: fi.ModTime().UnixNano(),
			Size:  fi.Size(),
			Task:  t,
		})
	}
	data, err := json.Marshal(cacheFile{Version: cacheVersion, Entries: entries})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.cachePath), 0o755); err != nil {
		return err
	}
	return WriteFileAtomic(s.cachePath, data, 0o600)
}

// Close persists the index cache. Best-effort: cache failures are not fatal.
func (s *Store) Close() error { return s.saveCache() }

// ArchiveDone moves done task files from projects/<p>/ to archive/<p>/,
// keeping file contents byte-identical. Tasks whose Completed (or Updated as
// fallback) is unparseable are skipped. olderThan <= 0 archives all done
// tasks; otherwise only tasks finished more than olderThan ago. Archived
// tasks leave the default index. Returns the number moved.
func (s *Store) ArchiveDone(now time.Time, olderThan time.Duration, isDone func(project, status string) bool) (int, error) {
	if isDone == nil {
		isDone = func(_, status string) bool { return core.IsDoneStatusFallback(status) }
	}
	var moves []string
	for _, t := range s.tasks {
		rel, ok := s.paths[t.ID]
		if !ok || !strings.HasPrefix(rel, "projects/") {
			continue
		}
		if !isDone(t.Project, t.Status) {
			continue
		}
		if olderThan > 0 {
			stamp := t.Completed
			if stamp == "" {
				stamp = t.Updated
			}
			d, err := core.ParseDue(stamp, now.Location())
			if err != nil {
				continue // cannot tell how old it is; leave it
			}
			if now.Sub(d) < olderThan {
				continue // not old enough
			}
		}
		moves = append(moves, rel)
	}
	moved := 0
	for _, oldRel := range moves {
		if _, err := s.archiveOne(oldRel); err != nil {
			return moved, err
		}
		moved++
	}
	if moved > 0 {
		if err := s.saveCache(); err != nil {
			return moved, err
		}
	}
	return moved, nil
}

// archiveOne moves the task file at oldRel (a projects/-relative path) to
// archive/ and drops it from the default index. Returns the new relative
// path.
func (s *Store) archiveOne(oldRel string) (string, error) {
	oldPath := s.absPath(oldRel)
	newRel := "archive/" + strings.TrimPrefix(oldRel, "projects/")
	newPath := s.absPath(newRel)
	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		return "", fmt.Errorf("archive: mkdir %s: %w", filepath.Dir(newRel), err)
	}
	if _, err := os.Stat(newPath); err == nil {
		return "", fmt.Errorf("archive: %s already exists — resolve it manually", newRel)
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return "", fmt.Errorf("archive: move %s: %w", oldRel, err)
	}
	for i, t := range s.tasks {
		if s.paths[t.ID] != oldRel {
			continue
		}
		t.Archived = true
		s.paths[t.ID] = newRel
		s.tasks = append(s.tasks[:i], s.tasks[i+1:]...)
		break
	}
	return newRel, nil
}

// ArchiveTask moves a single task's file to archive/<project>/ and removes
// it from the default index. Used by the TUI's per-task archive action; the
// file stays byte-identical.
func (s *Store) ArchiveTask(t *core.Task) error {
	rel, ok := s.paths[t.ID]
	if !ok {
		return fmt.Errorf("task %q is not in the index", t.ID)
	}
	if !strings.HasPrefix(rel, "projects/") {
		return fmt.Errorf("task %q is already archived", t.ID)
	}
	if _, err := s.archiveOne(rel); err != nil {
		return err
	}
	return s.saveCache()
}

// Diagnostics lists files that could not be read/parsed during indexing.
func (s *Store) Diagnostics() []string { return s.diagnostics }

// Tasks returns all indexed tasks (order: filesystem walk order).
func (s *Store) Tasks() []*core.Task { return s.tasks }

func (s *Store) absPath(rel string) string {
	return filepath.Join(s.Root, filepath.FromSlash(rel))
}

// Path returns the absolute path of a task by id.
func (s *Store) Path(id string) (string, error) {
	rel, ok := s.paths[id]
	if !ok {
		return "", fmt.Errorf("task %q is not in the index", id)
	}
	return s.absPath(rel), nil
}

// Resolve finds a task by id or unique id prefix (case-insensitive).
func (s *Store) Resolve(prefix string) (*core.Task, error) {
	if prefix == "" {
		return nil, errors.New("empty task id")
	}
	prefix = strings.ToLower(prefix)
	var matches []*core.Task
	for _, t := range s.tasks {
		if strings.HasPrefix(strings.ToLower(t.ID), prefix) {
			matches = append(matches, t)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no task with id %q", prefix)
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

// Create writes a new task file and indexes it.
func (s *Store) Create(t *core.Task) error {
	if err := validateProjectName(t.Project); err != nil {
		return err
	}
	if _, err := s.GetProject(t.Project); err != nil {
		return fmt.Errorf("project %q: %w — run `mtm project add %s`", t.Project, err, t.Project)
	}
	if t.ID == "" {
		t.ID = s.uniqueID()
	}
	now := s.Now()
	if t.Created == "" {
		t.Created = s.Clock.StampAt(now)
	}
	t.Updated = s.Clock.StampAt(now)
	if t.Kind == "" {
		t.Kind = core.KindTask
	}
	rel := filepath.ToSlash(filepath.Join("projects", filepath.FromSlash(t.Project),
		t.ID+"-"+core.Slugify(t.Title)+".md"))
	data, err := SerializeTask(t)
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
	s.register(t, rel)
	return s.saveCache()
}

// Update rewrites an existing task file and refreshes the index.
func (s *Store) Update(t *core.Task) error {
	rel, ok := s.paths[t.ID]
	if !ok {
		return fmt.Errorf("task %q is not in the index", t.ID)
	}
	t.Updated = s.Clock.Stamp()
	data, err := SerializeTask(t)
	if err != nil {
		return err
	}
	if err := WriteFileAtomic(s.absPath(rel), data, 0o644); err != nil {
		return err
	}
	for i, existing := range s.tasks {
		if existing.ID == t.ID {
			s.tasks[i] = t
		}
	}
	return s.saveCache()
}

// Remove deletes the task file and drops it from the index.
func (s *Store) Remove(t *core.Task) error {
	rel, ok := s.paths[t.ID]
	if !ok {
		return fmt.Errorf("task %q is not in the index", t.ID)
	}
	if err := os.Remove(s.absPath(rel)); err != nil {
		return err
	}
	delete(s.paths, t.ID)
	for i, existing := range s.tasks {
		if existing.ID == t.ID {
			s.tasks = append(s.tasks[:i], s.tasks[i+1:]...)
			break
		}
	}
	return s.saveCache()
}

// Reparse re-reads a task file from disk (used by edit to pick up changes).
func (s *Store) Reparse(t *core.Task) error {
	rel, ok := s.paths[t.ID]
	if !ok {
		return fmt.Errorf("task %q is not in the index", t.ID)
	}
	data, err := os.ReadFile(s.absPath(rel))
	if err != nil {
		return err
	}
	parsed, err := ParseTask(data)
	if err != nil {
		return err
	}
	*t = *parsed
	return nil
}

func (s *Store) uniqueID() string {
	for {
		id := core.NewID()
		if _, exists := s.paths[id]; !exists {
			return id
		}
	}
}

// validateProjectName rejects absolute paths and traversal.
func validateProjectName(name string) error {
	if name == "" {
		return errors.New("empty project name")
	}
	if filepath.IsAbs(name) || strings.HasPrefix(name, "/") {
		return fmt.Errorf("project name %q must be relative", name)
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(name)))
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("project name %q escapes the repo", name)
	}
	if clean != filepath.ToSlash(name) {
		return fmt.Errorf("project name %q is not clean (use %q)", name, clean)
	}
	return nil
}

// GetProject loads a project's project.yaml (cached per Store instance).
func (s *Store) GetProject(name string) (*core.Project, error) {
	if err := validateProjectName(name); err != nil {
		return nil, err
	}
	if p, ok := s.projects[name]; ok {
		return p, nil
	}
	path := filepath.Join(s.Root, "projects", filepath.FromSlash(name), "project.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read project.yaml: %w", err)
	}
	p := &core.Project{Name: name}
	if err := yaml.Unmarshal(data, p); err != nil {
		return nil, fmt.Errorf("parse project.yaml: %w", err)
	}
	p.Name = name
	if err := p.Validate(); err != nil {
		return nil, err
	}
	s.projects[name] = p
	return p, nil
}

// SaveProject writes project.yaml atomically and refreshes the cache.
func (s *Store) SaveProject(p *core.Project) error {
	if err := validateProjectName(p.Name); err != nil {
		return err
	}
	if err := p.Validate(); err != nil {
		return err
	}
	dir := filepath.Join(s.Root, "projects", filepath.FromSlash(p.Name))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(p)
	if err != nil {
		return err
	}
	if err := WriteFileAtomic(filepath.Join(dir, "project.yaml"), data, 0o644); err != nil {
		return err
	}
	s.projects[p.Name] = p
	return nil
}

// ReloadProject drops the cached copy and re-reads project.yaml from disk.
// Use after external edits (e.g. $EDITOR) to validate what the user saved.
func (s *Store) ReloadProject(name string) error {
	delete(s.projects, name)
	_, err := s.GetProject(name)
	return err
}

// ProjectExists reports whether a project directory has a project.yaml.
func (s *Store) ProjectExists(name string) bool {
	_, err := s.GetProject(name)
	return err == nil
}

// ListProjects returns all projects that have a project.yaml, plus warnings
// for ones that failed to parse (never silently skipped).
func (s *Store) ListProjects() ([]*core.Project, []string) {
	var out []*core.Project
	var warnings []string
	projectRoot := filepath.Join(s.Root, "projects")
	_ = filepath.WalkDir(projectRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "project.yaml" {
			return nil
		}
		name, err := filepath.Rel(projectRoot, filepath.Dir(path))
		if err != nil {
			return nil
		}
		p, err := s.GetProject(filepath.ToSlash(name))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("project %s: %v", name, err))
			return nil
		}
		out = append(out, p)
		return nil
	})
	return out, warnings
}

// TaskCount counts indexed tasks belonging to a project.
func (s *Store) TaskCount(project string) int {
	n := 0
	for _, t := range s.tasks {
		if t.Project == project {
			n++
		}
	}
	return n
}

// RemoveProject deletes a project directory (and its tasks) from disk.
// Confirmation is the caller's responsibility.
func (s *Store) RemoveProject(name string) error {
	if err := validateProjectName(name); err != nil {
		return err
	}
	dir := filepath.Join(s.Root, "projects", filepath.FromSlash(name))
	return os.RemoveAll(dir)
}

// IsDoneStatus reports whether status is done in project, falling back to
// the common convention when the project is missing or unparseable.
func (s *Store) IsDoneStatus(project, status string) bool {
	if p, err := s.GetProject(project); err == nil {
		return p.IsDoneStatus(status)
	}
	return core.IsDoneStatusFallback(status)
}
