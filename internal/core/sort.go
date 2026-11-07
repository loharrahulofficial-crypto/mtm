package core

import (
	"fmt"
	"sort"
	"strings"
)

// Sort keys for task lists. Default order: not-done first, then due date
// (no-due last), then priority (urgent to low), then title.
const (
	SortDone     = "done"
	SortDue      = "due"
	SortPriority = "priority"
	SortTitle    = "title"
)

// DefaultSortKeys is used when config does not set sort:.
var DefaultSortKeys = []string{SortDone, SortDue, SortPriority, SortTitle}

// ValidSortKey reports whether k is a known sort key.
func ValidSortKey(k string) bool {
	switch k {
	case SortDone, SortDue, SortPriority, SortTitle:
		return true
	}
	return false
}

// ParseSortKeys validates config's sort: list. Empty/nil means the default.
func ParseSortKeys(keys []string) ([]string, error) {
	if len(keys) == 0 {
		return append([]string(nil), DefaultSortKeys...), nil
	}
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		k = strings.ToLower(strings.TrimSpace(k))
		if !ValidSortKey(k) {
			return nil, fmt.Errorf("unknown sort key %q (want done|due|priority|title)", k)
		}
		out = append(out, k)
	}
	return out, nil
}

var priorityRank = map[string]int{
	PriorityUrgent: 0,
	PriorityHigh:   1,
	PriorityMedium: 2,
	PriorityLow:    3,
}

func priorityRankOf(p string) int {
	if r, ok := priorityRank[strings.ToLower(p)]; ok {
		return r
	}
	return 4 // unknown priorities last
}

// SortTasks sorts tasks in place by the given keys and returns the slice.
// An empty keys slice means DefaultSortKeys. The final tie-break is the id
// so the order is total and stable.
func SortTasks(tasks []*Task, keys []string, isDone func(project, status string) bool) []*Task {
	if len(keys) == 0 {
		keys = DefaultSortKeys
	}
	doneOf := func(t *Task) bool {
		if isDone != nil {
			return isDone(t.Project, t.Status)
		}
		return IsDoneStatusFallback(t.Status)
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		a, b := tasks[i], tasks[j]
		for _, key := range keys {
			var cmp int
			switch key {
			case SortDone:
				cmp = boolRank(doneOf(a)) - boolRank(doneOf(b))
			case SortDue:
				cmp = dueRank(a.Due, b.Due)
			case SortPriority:
				cmp = priorityRankOf(a.Priority) - priorityRankOf(b.Priority)
			case SortTitle:
				cmp = strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
			}
			if cmp != 0 {
				return cmp < 0
			}
		}
		return a.ID < b.ID
	})
	return tasks
}

func boolRank(done bool) int {
	if done {
		return 1
	}
	return 0
}

// dueRank orders by due ascending, no-due last. The frontmatter layout
// (YYYY-MM-DD[THH:MM]) is lexicographically chronological.
func dueRank(a, b string) int {
	if a == "" && b == "" {
		return 0
	}
	if a == "" {
		return 1
	}
	if b == "" {
		return -1
	}
	return strings.Compare(a, b)
}
