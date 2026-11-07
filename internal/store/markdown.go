package store

import (
	"bytes"
	"fmt"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"gopkg.in/yaml.v3"
)

const fence = "---"

// ParseTask parses a Markdown task file (YAML frontmatter + body).
func ParseTask(data []byte) (*core.Task, error) {
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(data, []byte(fence+"\n")) {
		return nil, fmt.Errorf("missing frontmatter fence")
	}
	rest := data[len(fence)+1:]
	idx := bytes.Index(rest, []byte("\n"+fence+"\n"))
	var front, body []byte
	switch {
	case idx >= 0:
		front = rest[:idx]
		body = rest[idx+len("\n"+fence+"\n"):]
	case bytes.HasSuffix(rest, []byte("\n"+fence)):
		front = rest[:len(rest)-len("\n"+fence)]
		body = nil
	default:
		return nil, fmt.Errorf("missing closing frontmatter fence")
	}
	t := &core.Task{}
	if err := yaml.Unmarshal(front, t); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}
	t.Body = string(body)
	return t, nil
}

// SerializeTask renders a task back to its file form.
func SerializeTask(t *core.Task) ([]byte, error) {
	front, err := yaml.Marshal(t)
	if err != nil {
		return nil, fmt.Errorf("serialize frontmatter: %w", err)
	}
	var buf bytes.Buffer
	buf.WriteString(fence + "\n")
	buf.Write(front)
	buf.WriteString(fence + "\n")
	body := t.Body
	if body != "" {
		if body[len(body)-1] != '\n' {
			body += "\n"
		}
		buf.WriteString(body)
	}
	return buf.Bytes(), nil
}
