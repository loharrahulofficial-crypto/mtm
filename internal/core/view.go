package core

import (
	"fmt"
	"strings"
)

// ParseViewExpr parses a saved-view filter expression like
//
//	tag:college due:today priority:high
//
// Tokens are space-separated key:value pairs. Supported keys: tag, project,
// status, priority, due (today|week|overdue). A bare token "all" includes
// done tasks. Unknown keys are an error.
func ParseViewExpr(expr string) (Filter, error) {
	var f Filter
	for _, tok := range strings.Fields(expr) {
		lower := strings.ToLower(tok)
		if lower == "all" {
			f.IncludeCompleted = true
			continue
		}
		key, value, ok := strings.Cut(tok, ":")
		if !ok {
			return f, fmt.Errorf("view expression %q: token %q is not key:value", expr, tok)
		}
		value = strings.TrimSpace(value)
		switch strings.ToLower(key) {
		case "tag":
			f.Tag = value
		case "project":
			f.Project = value
		case "status":
			f.Status = value
		case "priority":
			value = strings.ToLower(value)
			if !ValidPriority(value) {
				return f, fmt.Errorf("view expression %q: unknown priority %q", expr, value)
			}
			f.Priority = value
		case "due":
			value = strings.ToLower(value)
			if !ValidDue(value) || value == "" {
				return f, fmt.Errorf("view expression %q: due must be today|week|overdue (got %q)", expr, value)
			}
			f.Due = value
		default:
			return f, fmt.Errorf("view expression %q: unknown key %q", expr, key)
		}
	}
	return f, nil
}
