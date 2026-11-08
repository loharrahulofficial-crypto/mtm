package actions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
)

// SyllabusChapter is one chapter of a course syllabus: a task, plus one
// subtask per subtopic.
type SyllabusChapter struct {
	Title     string   `yaml:"title"`
	Subtopics []string `yaml:"subtopics"`
	// Topics is accepted as an alias for Subtopics.
	Topics []string `yaml:"topics"`
}

// SyllabusFile is the YAML shape of a syllabus/template file.
type SyllabusFile struct {
	Chapters []SyllabusChapter `yaml:"chapters"`
}

// ParseSyllabusYAML parses the structured format:
//
//	chapters:
//	  - title: "ER model"
//	    subtopics: [entities, relationships]
func ParseSyllabusYAML(data []byte) ([]SyllabusChapter, error) {
	var f SyllabusFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse syllabus YAML: %w", err)
	}
	chapters := normalizeChapters(f.Chapters)
	if len(chapters) == 0 {
		return nil, fmt.Errorf("syllabus needs at least one chapter (chapters: - title: ...)")
	}
	return chapters, nil
}

// ParseSyllabusMarkdown parses the loose format: every heading starts a
// chapter; list items under it become subtopics. `- [ ] item` checkboxes are
// accepted and the marker is stripped. Non-heading, non-list lines (prose)
// are ignored.
func ParseSyllabusMarkdown(data []byte) ([]SyllabusChapter, error) {
	var chapters []SyllabusChapter
	current := -1
	for i, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if title, ok := markdownHeading(trimmed); ok {
			chapters = append(chapters, SyllabusChapter{Title: title})
			current = len(chapters) - 1
			continue
		}
		if item, ok := markdownListItem(trimmed); ok {
			if current < 0 {
				return nil, fmt.Errorf("line %d: list item %q before any chapter heading", i+1, item)
			}
			chapters[current].Subtopics = append(chapters[current].Subtopics, item)
			continue
		}
		// Prose is ignored — syllabi often carry intro paragraphs.
	}
	chapters = normalizeChapters(chapters)
	if len(chapters) == 0 {
		return nil, fmt.Errorf("syllabus needs at least one chapter heading (# or ## ...)")
	}
	return chapters, nil
}

// normalizeChapters merges the topics alias, trims titles and drops chapters
// with an empty title (an error is reported for them).
func normalizeChapters(in []SyllabusChapter) []SyllabusChapter {
	var out []SyllabusChapter
	for _, ch := range in {
		ch.Title = strings.TrimSpace(ch.Title)
		subs := ch.Subtopics
		if len(subs) == 0 {
			subs = ch.Topics
		}
		var clean []string
		for _, s := range subs {
			if t := strings.TrimSpace(s); t != "" {
				clean = append(clean, t)
			}
		}
		ch.Subtopics = clean
		ch.Topics = nil
		if ch.Title == "" {
			continue
		}
		out = append(out, ch)
	}
	return out
}

// markdownHeading reports whether line is a Markdown heading and returns the
// heading text without the marker or any trailing #s.
func markdownHeading(line string) (string, bool) {
	if !strings.HasPrefix(line, "#") {
		return "", false
	}
	i := 0
	for i < len(line) && line[i] == '#' {
		i++
	}
	if i > 6 || i < len(line) && line[i] != ' ' {
		return "", false
	}
	title := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(line[i:]), "#"))
	if title == "" {
		return "", false
	}
	return title, true
}

// markdownListItem reports whether line is a list item and returns its text,
// stripping an optional checkbox marker ([ ] / [x]).
func markdownListItem(line string) (string, bool) {
	rest := ""
	switch {
	case strings.HasPrefix(line, "- "), strings.HasPrefix(line, "* "), strings.HasPrefix(line, "+ "):
		rest = strings.TrimSpace(line[2:])
	default:
		for i, r := range line {
			if r >= '0' && r <= '9' {
				continue
			}
			if (r == '.' || r == ')') && i+1 < len(line) && line[i+1] == ' ' {
				rest = strings.TrimSpace(line[i+2:])
			}
			break
		}
		if rest == "" {
			return "", false
		}
	}
	rest = strings.TrimPrefix(rest, "[ ] ")
	rest = strings.TrimPrefix(rest, "[x] ")
	rest = strings.TrimPrefix(rest, "[X] ")
	if rest == "" {
		return "", false
	}
	return rest, true
}

// LoadSyllabus loads a syllabus from a file path or from a reusable template
// stored in the data repo under templates/<name>.{yaml,yml,md,markdown}.
// The returned source string describes where it came from (for messages).
func LoadSyllabus(s *store.Store, fileOrTemplate string) ([]SyllabusChapter, string, error) {
	arg := strings.TrimSpace(fileOrTemplate)
	if arg == "" {
		return nil, "", fmt.Errorf("a syllabus file or template name is required")
	}
	if info, err := os.Stat(arg); err == nil && !info.IsDir() {
		data, err := os.ReadFile(arg)
		if err != nil {
			return nil, "", fmt.Errorf("read %s: %w", arg, err)
		}
		chapters, err := parseSyllabusFile(arg, data)
		return chapters, arg, err
	}
	if strings.ContainsAny(arg, "/\\") || strings.Contains(arg, "..") {
		return nil, "", fmt.Errorf("%q is not a file and not a template name (templates live in templates/<name>.yaml|md in the data repo)", arg)
	}
	for _, ext := range []string{".yaml", ".yml", ".md", ".markdown"} {
		path := filepath.Join(s.Root, "templates", arg+ext)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		chapters, err := parseSyllabusFile(path, data)
		return chapters, "templates/" + arg + ext, err
	}
	templates, _ := ListSyllabusTemplates(s)
	if len(templates) == 0 {
		return nil, "", fmt.Errorf("no syllabus file %q and no templates in templates/ — add one, e.g. templates/dbms.yaml", arg)
	}
	return nil, "", fmt.Errorf("no syllabus file or template %q — available templates: %s", arg, strings.Join(templates, ", "))
}

// ListSyllabusTemplates returns the template names (without extension) stored
// in the data repo's templates/ directory.
func ListSyllabusTemplates(s *store.Store) ([]string, error) {
	dir := filepath.Join(s.Root, "templates")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		switch ext {
		case ".yaml", ".yml", ".md", ".markdown":
			out = append(out, strings.TrimSuffix(e.Name(), ext))
		}
	}
	return out, nil
}

// parseSyllabusFile picks the parser by file extension.
func parseSyllabusFile(path string, data []byte) ([]SyllabusChapter, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return ParseSyllabusYAML(data)
	case ".md", ".markdown":
		return ParseSyllabusMarkdown(data)
	default:
		return nil, fmt.Errorf("unsupported syllabus format %q — use .yaml, .yml, .md or .markdown", filepath.Ext(path))
	}
}

// SyllabusResult summarizes an import: what was created and what already
// existed (and was therefore skipped).
type SyllabusResult struct {
	CreatedChapters []*core.Task
	CreatedSubtasks []*core.Task
	// Skipped lists chapter titles that were already present as tasks in the
	// course; their subtopics are still reconciled.
	Skipped []string
}

// ImportSyllabus creates one task per chapter and one subtask per subtopic
// under it. It is idempotent: a chapter whose title already exists as a
// non-archived task in the course is reused (not duplicated), and a subtopic
// that already exists as a child of that chapter is skipped.
func ImportSyllabus(s *store.Store, course string, chapters []SyllabusChapter) (*SyllabusResult, error) {
	name, err := ResolveCourse(s, course)
	if err != nil {
		return nil, err
	}
	p, err := s.GetProject(name)
	if err != nil {
		return nil, err
	}
	existing := CourseTasks(s, name)
	byTitle := func(title string) *core.Task {
		for _, t := range existing {
			if strings.EqualFold(t.Title, title) {
				return t
			}
		}
		return nil
	}
	res := &SyllabusResult{}
	for _, ch := range chapters {
		chapterTask := byTitle(ch.Title)
		if chapterTask == nil {
			chapterTask = &core.Task{
				Title:   ch.Title,
				Project: name,
				Status:  firstStatus(p),
				Kind:    core.KindTask,
				Tags:    []string{"chapter"},
			}
			if err := s.Create(chapterTask); err != nil {
				return res, err
			}
			res.CreatedChapters = append(res.CreatedChapters, chapterTask)
			existing = append(existing, chapterTask)
		} else {
			res.Skipped = append(res.Skipped, ch.Title)
		}
		children := map[string]bool{}
		for _, t := range s.Tasks() {
			if t.Parent == chapterTask.ID {
				children[strings.ToLower(t.Title)] = true
			}
		}
		for _, sub := range ch.Subtopics {
			if children[strings.ToLower(sub)] {
				continue
			}
			subTask := &core.Task{
				Title:   sub,
				Project: name,
				Status:  firstStatus(p),
				Kind:    core.KindTask,
				Parent:  chapterTask.ID,
			}
			if err := s.Create(subTask); err != nil {
				return res, err
			}
			res.CreatedSubtasks = append(res.CreatedSubtasks, subTask)
		}
	}
	return res, nil
}
