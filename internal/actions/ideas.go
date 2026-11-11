package actions

import (
	"fmt"
	"strings"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/parse"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
)

// Idea CRUD helpers shared by CLI and TUI. All logic lives here or in core;
// the UIs only render and collect input.

// AddIdea creates a seed idea with an optional rating (0 = unrated) and tags.
func AddIdea(s *store.Store, title string, tags []string, rating int) (*core.Idea, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, fmt.Errorf("idea needs a title")
	}
	if rating < 0 || rating > 5 {
		return nil, fmt.Errorf("rating must be 1-5 (or omitted), got %d", rating)
	}
	i := &core.Idea{
		Title:  title,
		Status: core.IdeaSeed,
		Tags:   uniqueTags(tags),
		Rating: rating,
	}
	if err := s.CreateIdea(i); err != nil {
		return nil, err
	}
	return i, nil
}

// SetIdeaStatus moves an idea to canonical (validated). Returns the canonical
// status.
func SetIdeaStatus(s *store.Store, i *core.Idea, status string) (string, error) {
	canonical := core.CanonicalIdeaStatus(status)
	if canonical == "" {
		return "", fmt.Errorf("invalid idea status %q (want %s)", status, strings.Join(core.IdeaStatuses, "|"))
	}
	i.Status = canonical
	if err := s.UpdateIdea(i); err != nil {
		return "", err
	}
	return canonical, nil
}

// NoteIdea appends a dated bullet to the idea's notes.
func NoteIdea(s *store.Store, i *core.Idea, text string, now time.Time) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("note text is required")
	}
	stamp := now.Format("2006-01-02")
	bullet := "- " + stamp + " " + text
	body := strings.TrimRight(i.Body, "\n")
	if body == "" {
		body = bullet
	} else {
		body = body + "\n" + bullet
	}
	i.Body = body + "\n"
	return s.UpdateIdea(i)
}

// PromoteResult reports what a promotion created.
type PromoteResult struct {
	Project        string
	MilestoneID    string
	MilestoneTitle string
	TaskIDs        []string
	IdeaID         string
}

// PromoteIdea turns an idea into a project:
//
//   - a new project named after name (or the idea's slugified title) with
//     the standard todo/doing/done workflow; it must not exist yet
//   - one initial milestone ("Initial milestone", planned, target today+30d)
//   - starter tasks from templates/<template>.md when template is non-empty
//     (one quick-add line each, linked to the initial milestone)
//   - the idea's status becomes promoted and promoted_to records the project.
//
// The idea file is kept. On error the result reports whatever was created
// so the caller can tell the user what exists; the idea is only marked
// promoted once everything else succeeded.
func PromoteIdea(s *store.Store, ideaPrefix, name, template string, now time.Time) (*PromoteResult, error) {
	i, err := s.ResolveIdea(ideaPrefix)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(i.Status, core.IdeaPromoted) {
		return nil, fmt.Errorf("idea %s is already promoted (to %s)", i.ID, i.PromotedTo)
	}
	project := strings.TrimSpace(name)
	if project == "" {
		project = core.Slugify(i.Title)
	}
	if strings.Contains(project, "/") {
		return nil, fmt.Errorf("project name %q must be a plain name (no slashes)", project)
	}
	if s.ProjectExists(project) {
		return nil, fmt.Errorf("project %q already exists", project)
	}
	res := &PromoteResult{Project: project, IdeaID: i.ID}
	// 1. Project with the standard workflow.
	if err := s.SaveProject(core.DefaultProject(project)); err != nil {
		return nil, fmt.Errorf("create project %q: %w", project, err)
	}
	// 2. Initial milestone.
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	ms := &core.Milestone{
		Title:       "Initial milestone",
		Target:      today.AddDate(0, 0, 30).Format("2006-01-02"),
		Status:      core.MilestonePlanned,
		Description: "Created by promoting idea " + i.ID + " (" + i.Title + ")",
		Project:     project,
	}
	if err := s.CreateMilestone(ms); err != nil {
		return res, fmt.Errorf("project %q was created but the milestone failed: %w", project, err)
	}
	res.MilestoneID = ms.ID
	res.MilestoneTitle = ms.Title
	// 3. Starter tasks from the template.
	if strings.TrimSpace(template) != "" {
		data, err := s.TemplateFile(template)
		if err != nil {
			return res, fmt.Errorf("project %q was created but reading the template failed: %w", project, err)
		}
		first := "todo"
		if p, err := s.GetProject(project); err == nil && len(p.Statuses) > 0 {
			first = p.Statuses[0].Name
		}
		for _, line := range templateTaskLines(data) {
			parsed := parse.QuickAdd(line, now)
			title := strings.TrimSpace(parsed.Title)
			if title == "" {
				continue
			}
			task := &core.Task{
				Title:           title,
				Project:         project,
				Status:          first,
				Priority:        parsed.Priority,
				Tags:            parsed.Tags,
				Due:             parsed.Due,
				Recurrence:      parsed.Recurrence,
				EstimateMinutes: parsed.EstimateMinutes,
				Kind:            core.KindTask,
				Milestone:       ms.ID,
			}
			if task.Priority == "" {
				task.Priority = core.PriorityMedium
			}
			if err := s.Create(task); err != nil {
				return res, fmt.Errorf("project %q was created but a starter task failed: %w", project, err)
			}
			res.TaskIDs = append(res.TaskIDs, task.ID)
		}
	}
	// 4. Record the promotion on the idea (kept, never deleted).
	i.Status = core.IdeaPromoted
	i.PromotedTo = project
	if err := s.UpdateIdea(i); err != nil {
		return res, fmt.Errorf("project %q was created but updating the idea failed: %w", project, err)
	}
	return res, nil
}

// templateTaskLines extracts task lines from a template file: non-empty
// lines, '#'-prefixed comments and an optional leading YAML frontmatter
// block are skipped; leading "- [ ] ", "- [x] ", "- " or "* " bullets are
// stripped. The rest is one quick-add line per entry.
func templateTaskLines(data []byte) []string {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	// Strip an optional leading frontmatter block.
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		end := -1
		for j := 1; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) == "---" {
				end = j
				break
			}
		}
		if end > 0 {
			lines = lines[end+1:]
		}
	}
	var out []string
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		for _, prefix := range []string{"- [ ] ", "- [x] ", "- [X] ", "- ", "* "} {
			if strings.HasPrefix(line, prefix) {
				line = strings.TrimSpace(line[len(prefix):])
				break
			}
		}
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

// NextMilestone returns the nearest not-done milestone across all projects
// that has a target date, plus whole days remaining (negative when overdue).
func NextMilestone(s *store.Store, now time.Time) (*core.Milestone, int, bool) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var best *core.Milestone
	bestDays := 0
	for _, m := range s.Milestones("") {
		if strings.EqualFold(m.Status, core.MilestoneDone) || m.Target == "" {
			continue
		}
		day, err := time.ParseInLocation("2006-01-02", m.Target, now.Location())
		if err != nil {
			continue
		}
		days := core.CalDays(today, day)
		if best == nil || days < bestDays {
			best, bestDays = m, days
		}
	}
	if best == nil {
		return nil, 0, false
	}
	return best, bestDays, true
}
