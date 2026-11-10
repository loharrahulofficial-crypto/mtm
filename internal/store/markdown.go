package store

import (
	"bytes"
	"fmt"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"gopkg.in/yaml.v3"
)

const fence = "---"

// parseFrontMatter splits a Markdown file into YAML frontmatter and a
// Markdown body, normalizing CRLF line endings.
func parseFrontMatter(data []byte, out any) (body string, err error) {
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(data, []byte(fence+"\n")) {
		return "", fmt.Errorf("missing frontmatter fence")
	}
	rest := data[len(fence)+1:]
	idx := bytes.Index(rest, []byte("\n"+fence+"\n"))
	var front, bodyBytes []byte
	switch {
	case idx >= 0:
		front = rest[:idx]
		bodyBytes = rest[idx+len("\n"+fence+"\n"):]
	case bytes.HasSuffix(rest, []byte("\n"+fence)):
		front = rest[:len(rest)-len("\n"+fence)]
	default:
		return "", fmt.Errorf("missing closing frontmatter fence")
	}
	if err := yaml.Unmarshal(front, out); err != nil {
		return "", fmt.Errorf("parse frontmatter: %w", err)
	}
	return string(bodyBytes), nil
}

// serializeFrontMatter renders v as YAML frontmatter + body back to file form.
func serializeFrontMatter(v any, body string) ([]byte, error) {
	front, err := yaml.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("serialize frontmatter: %w", err)
	}
	var buf bytes.Buffer
	buf.WriteString(fence + "\n")
	buf.Write(front)
	buf.WriteString(fence + "\n")
	if body != "" {
		if body[len(body)-1] != '\n' {
			body += "\n"
		}
		buf.WriteString(body)
	}
	return buf.Bytes(), nil
}

// ParseTask parses a Markdown task file (YAML frontmatter + body).
func ParseTask(data []byte) (*core.Task, error) {
	t := &core.Task{}
	body, err := parseFrontMatter(data, t)
	if err != nil {
		return nil, err
	}
	t.Body = body
	return t, nil
}

// SerializeTask renders a task back to its file form.
func SerializeTask(t *core.Task) ([]byte, error) {
	return serializeFrontMatter(t, t.Body)
}

// ParseIdea parses an ideas/<id>-<slug>.md file.
func ParseIdea(data []byte) (*core.Idea, error) {
	i := &core.Idea{}
	body, err := parseFrontMatter(data, i)
	if err != nil {
		return nil, err
	}
	i.Body = body
	return i, nil
}

// SerializeIdea renders an idea back to its file form.
func SerializeIdea(i *core.Idea) ([]byte, error) {
	return serializeFrontMatter(i, i.Body)
}

// ParseMilestone parses a projects/<p>/milestones/<id>-<slug>.md file.
func ParseMilestone(data []byte) (*core.Milestone, error) {
	m := &core.Milestone{}
	body, err := parseFrontMatter(data, m)
	if err != nil {
		return nil, err
	}
	m.Body = body
	return m, nil
}

// SerializeMilestone renders a milestone back to its file form.
func SerializeMilestone(m *core.Milestone) ([]byte, error) {
	return serializeFrontMatter(m, m.Body)
}

// ParseDevLog parses a projects/<p>/devlog/<YYYY-MM-DD>-<id>.md file.
func ParseDevLog(data []byte) (*core.DevLogEntry, error) {
	d := &core.DevLogEntry{}
	body, err := parseFrontMatter(data, d)
	if err != nil {
		return nil, err
	}
	d.Body = body
	return d, nil
}

// SerializeDevLog renders a dev-log entry back to its file form.
func SerializeDevLog(d *core.DevLogEntry) ([]byte, error) {
	return serializeFrontMatter(d, d.Body)
}
