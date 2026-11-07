package core

import "strings"

// ProjectStatus is one column/state of a project's custom workflow.
type ProjectStatus struct {
	Name  string `yaml:"name"`
	Done  bool   `yaml:"done"`
	Color string `yaml:"color,omitempty"`
}

// Project describes a project.yaml. The project name is the relative path of
// its directory under projects/ (e.g. "inbox" or "college/dbms"); it is not
// stored in the file itself.
type Project struct {
	Name        string          `yaml:"-"`
	Description string          `yaml:"description,omitempty"`
	Color       string          `yaml:"color,omitempty"`
	Statuses    []ProjectStatus `yaml:"statuses"`
}

// DefaultProject returns the standard todo/doing/done workflow.
func DefaultProject(name string) *Project {
	return &Project{
		Name: name,
		Statuses: []ProjectStatus{
			{Name: "todo"},
			{Name: "doing"},
			{Name: "done", Done: true},
		},
	}
}

// DoneStatus returns the first status marked done, or "" if none.
func (p *Project) DoneStatus() string {
	for _, s := range p.Statuses {
		if s.Done {
			return s.Name
		}
	}
	return ""
}

// StatusExists matches status names case-insensitively.
func (p *Project) StatusExists(status string) bool {
	for _, s := range p.Statuses {
		if strings.EqualFold(s.Name, status) {
			return true
		}
	}
	return false
}

// CanonicalStatus returns the project's canonical casing for a status,
// or "" if it does not exist.
func (p *Project) CanonicalStatus(status string) string {
	for _, s := range p.Statuses {
		if strings.EqualFold(s.Name, status) {
			return s.Name
		}
	}
	return ""
}

// IsDoneStatus reports whether status counts as done in this project.
func (p *Project) IsDoneStatus(status string) bool {
	for _, s := range p.Statuses {
		if strings.EqualFold(s.Name, status) {
			return s.Done
		}
	}
	return false
}

// Validate checks the project is usable.
func (p *Project) Validate() error {
	if len(p.Statuses) == 0 {
		return errNoStatuses
	}
	seen := map[string]bool{}
	for _, s := range p.Statuses {
		if strings.TrimSpace(s.Name) == "" {
			return errEmptyStatus
		}
		key := strings.ToLower(s.Name)
		if seen[key] {
			return errDupStatus
		}
		seen[key] = true
	}
	return nil
}

type projectError string

func (e projectError) Error() string { return string(e) }

const (
	errNoStatuses  = projectError("project needs at least one status")
	errEmptyStatus = projectError("project has an empty status name")
	errDupStatus   = projectError("project has a duplicate status name")
)
