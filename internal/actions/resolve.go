package actions

import (
	"fmt"
	"strings"

	"github.com/loharrahulofficial-crypto/mtm/internal/store"
)

// ResolveProject maps a user-supplied project argument to a project name:
// exact match first, then case-insensitive, then a unique match on the leaf
// directory name (dbms → college/dbms). Unknown or ambiguous names are
// errors listing the known projects. Used for exam targeting, where any
// project is acceptable.
func ResolveProject(s *store.Store, arg string) (string, error) {
	projects, warnings := s.ListProjects()
	_ = warnings // callers surface warnings themselves when listing tasks
	if len(projects) == 0 {
		return "", fmt.Errorf("no projects yet — run `mtm init` first")
	}
	arg = strings.TrimSpace(arg)
	names := make([]string, 0, len(projects))
	for _, p := range projects {
		names = append(names, p.Name)
	}
	for _, p := range projects { // exact
		if p.Name == arg {
			return p.Name, nil
		}
	}
	for _, p := range projects { // case-insensitive full path
		if strings.EqualFold(p.Name, arg) {
			return p.Name, nil
		}
	}
	var leaf []string // unique leaf (dbms → college/dbms)
	for _, p := range projects {
		base := p.Name
		if i := strings.LastIndex(base, "/"); i >= 0 {
			base = base[i+1:]
		}
		if strings.EqualFold(base, arg) {
			leaf = append(leaf, p.Name)
		}
	}
	switch len(leaf) {
	case 1:
		return leaf[0], nil
	case 0:
		return "", fmt.Errorf("unknown project %q — known: %s", arg, strings.Join(names, ", "))
	default:
		return "", fmt.Errorf("project %q is ambiguous — use the full path: %s", arg, strings.Join(leaf, ", "))
	}
}
