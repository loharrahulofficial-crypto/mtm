package actions

// StatusSnapshot is the shared read model behind `mtm summary` and
// `mtm waybar`: today's open count, the overdue count, the nearest exam
// and the active timer.

import (
	"sort"
	"strings"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
)

// StatusSnapshot carries everything a shell or bar integration needs.
type StatusSnapshot struct {
	Today        int            // open tasks due today
	Overdue      int            // open tasks overdue (due date passed)
	NextExam     *core.Task     // nearest open exam, nil when none
	NextExamDays int            // days until NextExam (-1 when none)
	MoreExams    int            // additional open exams beyond the nearest
	Timer        core.TimerView // active timer view (phase idle = no session)
	// ProjectProgress carries per-project done/total (by project name).
	ProjectProgress []ProjectProgressRow
}

// ProjectProgressRow is one project's done/total line.
type ProjectProgressRow struct {
	Project string
	Done    int
	Total   int
}

// LoadStatusSnapshot builds the snapshot for now.
func LoadStatusSnapshot(s *store.Store, now time.Time) StatusSnapshot {
	isDone := func(p, st string) bool { return s.IsDoneStatus(p, st) }
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var snap StatusSnapshot
	snap.Timer = LoadTimerView(s)

	base := core.Filter{Now: now, DoneCheck: isDone}
	todayFilter := base
	todayFilter.Due = "today"
	overdueFilter := base
	overdueFilter.Due = "overdue"
	all := s.Tasks()
	snap.Today = len(todayFilter.Apply(all))
	snap.Overdue = len(overdueFilter.Apply(all))

	type examHit struct {
		task *core.Task
		days int
	}
	var exams []examHit
	for _, t := range all {
		if !strings.EqualFold(t.Kind, core.KindExam) || t.Due == "" {
			continue
		}
		d, err := core.ParseDue(t.Due, now.Location())
		if err != nil {
			continue
		}
		examDay := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())
		days := int(examDay.Sub(today).Hours() / 24)
		if days < 0 || isDone(t.Project, t.Status) {
			continue
		}
		exams = append(exams, examHit{t, days})
	}
	if len(exams) == 0 {
		snap.NextExamDays = -1
	} else {
		best := exams[0]
		for _, e := range exams[1:] {
			if e.days < best.days {
				best = e
			}
		}
		snap.NextExam = best.task
		snap.NextExamDays = best.days
		snap.MoreExams = len(exams) - 1
	}

	// Per-project progress, name-ordered.
	byProject := map[string]*ProjectProgressRow{}
	order := []string{}
	for _, t := range all {
		st := byProject[t.Project]
		if st == nil {
			st = &ProjectProgressRow{Project: t.Project}
			byProject[t.Project] = st
			order = append(order, t.Project)
		}
		st.Total++
		if isDone(t.Project, t.Status) {
			st.Done++
		}
	}
	snap.ProjectProgress = make([]ProjectProgressRow, 0, len(order))
	for _, name := range order {
		snap.ProjectProgress = append(snap.ProjectProgress, *byProject[name])
	}
	sort.Slice(snap.ProjectProgress, func(i, j int) bool { return snap.ProjectProgress[i].Project < snap.ProjectProgress[j].Project })
	return snap
}
