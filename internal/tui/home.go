package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

// homeData is the assembled read model for the Home dashboard.
type homeData struct {
	greeting     string
	user         string
	dateLine     string // "Sat 7 Nov"
	weekLine     string // "Week 11 of 16 · theme" or just theme
	now          time.Time
	dueToday     int
	overdue      int
	nextExam     *core.Task
	nextExamDays int // -1 when none

	todayRows []*core.Task
	exams     []examCard
	courses   []courseRow

	heatmap   [7 * 16]int
	heatWeeks [16]bool // which buckets have activity (always all true; kept simple)
	weekHours [7]float64
	weekTotal float64
}

type examCard struct {
	task     *core.Task
	daysLeft int
	weight   float64 // -1 when unset
	subDone  int
	subTotal int
}

type courseRow struct {
	name  string
	color string
	done  int
	total int
	frac  float64
	open  int
}

// homeSelectedTask tracks the Home-view cursor over today's rows.
func (m Model) homeSelectedTask() *core.Task {
	h := m.homeRows()
	if len(h) == 0 {
		return nil
	}
	if m.homeSel >= len(h) {
		if len(h) == 0 {
			return nil
		}
		return h[len(h)-1]
	}
	if m.homeSel < 0 {
		return h[0]
	}
	return h[m.homeSel]
}

// homeRows returns today's rows (unfinished first, then done) in display order.
func (m Model) homeRows() []*core.Task {
	return m.homeDataCached().todayRows
}

// homeDataCached returns the dashboard read model, building it on demand.
func (m Model) homeDataCached() homeData {
	if m.homeCache != nil {
		return *m.homeCache
	}
	return buildHomeData(m)
}

func buildHomeData(m Model) homeData {
	d := homeData{
		user:         homeUser(),
		dateLine:     m.clock.Now().Format("Mon 2 Jan"),
		nextExamDays: -1,
	}
	now := m.clock.Now()
	d.now = now

	// Week-of-semester for the brand line (sidebar shows the same).
	if cfg := m.cfg; cfg.UI.SemesterStart != "" {
		if start, err := time.ParseInLocation("2006-01-02", cfg.UI.SemesterStart, now.Location()); err == nil {
			week := int(now.Sub(start).Hours()/(24*7)) + 1
			weeks := cfg.UI.SemesterWeeks
			if weeks <= 0 {
				weeks = 16
			}
			if week < 1 {
				week = 1
			}
			if week > weeks {
				week = weeks
			}
			d.weekLine = fmt.Sprintf("Week %d of %d", week, weeks)
		}
	}
	if d.weekLine == "" {
		d.weekLine = m.palette.Name
	}

	isDone := func(p, st string) bool { return m.store.IsDoneStatus(p, st) }
	y, mo, dd := now.Date()
	todayStart := time.Date(y, mo, dd, 0, 0, 0, 0, now.Location())

	for _, t := range m.store.Tasks() {
		done := isDone(t.Project, t.Status)
		if t.Due != "" {
			dueDay := parseDayStart(t.Due, now.Location())
			if !dueDay.IsZero() {
				diff := int(dueDay.Sub(todayStart).Hours() / 24)
				switch {
				case !done && diff == 0:
					d.dueToday++
				case !done && diff < 0:
					d.overdue++
				}
				if !done && strings.EqualFold(t.Kind, "exam") && diff >= 0 {
					if d.nextExam == nil || diff < d.nextExamDays {
						d.nextExam = t
						d.nextExamDays = diff
					}
				}
			}
		}
		// Completion stamps feed heatmap + week hours. Tasks done out of
		// band (bulk imports) have no Completed — fall back to Updated.
		stamp := t.Completed
		if stamp == "" && isDone(t.Project, t.Status) {
			stamp = t.Updated
		}
		if stamp != "" {
			if ts, err := time.ParseInLocation(core.TimestampFormat, stamp, now.Location()); err == nil {
				dayIdx := int(todayStart.Sub(time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, now.Location())).Hours() / (24 * 7))
				if dayIdx >= 0 && dayIdx < 16 {
					col := 15 - dayIdx
					row := int(ts.Weekday()+6) % 7
					d.heatmap[row*16+col]++
				}
			}
		}
	}

	// Today timeline: tasks due today; untimed first, timed sorted.
	for _, t := range m.store.Tasks() {
		if t.Due == "" {
			continue
		}
		dueDay := parseDayStart(t.Due, now.Location())
		if dueDay.IsZero() || !sameDay(dueDay, now) {
			continue
		}
		d.todayRows = append(d.todayRows, t)
	}
	sortDayRows(d.todayRows, isDone)

	// Upcoming exams (this one's the nearest), nearest two for the card.
	for _, t := range m.store.Tasks() {
		if !strings.EqualFold(t.Kind, "exam") || isDone(t.Project, t.Status) {
			continue
		}
		dueDay := parseDayStart(t.Due, now.Location())
		if dueDay.IsZero() {
			continue
		}
		diff := int(dueDay.Sub(todayStart).Hours() / 24)
		if diff < 0 {
			continue
		}
		doneN, totalN, _ := core.SubtaskProgress(m.store.Tasks(), t.ID, isDone)
		w := -1.0
		if t.Weight != nil {
			w = *t.Weight
		}
		d.exams = append(d.exams, examCard{task: t, daysLeft: diff, weight: w, subDone: doneN, subTotal: totalN})
	}
	sortExams(d.exams)

	// Courses: one row per project with at least one task.
	projects, _ := m.store.ListProjects()
	for _, p := range projects {
		done, total := 0, 0
		for _, t := range m.store.Tasks() {
			if strings.EqualFold(t.Project, p.Name) {
				total++
				if isDone(t.Project, t.Status) {
					done++
				}
			}
		}
		if total == 0 {
			continue
		}
		color := theme.CourseColor(m.palette, p.Name, p.Color)
		frac := float64(done) / float64(total)
		d.courses = append(d.courses, courseRow{
			name:  p.Name,
			color: color,
			done:  done,
			total: total,
			frac:  frac,
			open:  total - done,
		})
	}
	sortCourses(d.courses)

	// This week: hours attributed to the completion day (local-naive).
	for i := 0; i < 7; i++ {
		day := todayStart.AddDate(0, 0, i-6)
		for _, t := range m.store.Tasks() {
			if t.TimeLoggedMinutes <= 0 {
				continue
			}
			stamp := t.Completed
			if stamp == "" && isDone(t.Project, t.Status) {
				stamp = t.Updated
			}
			if stamp == "" {
				continue
			}
			ts, err := time.ParseInLocation(core.TimestampFormat, stamp, now.Location())
			if err != nil {
				continue
			}
			if sameDay(ts, day) {
				d.weekHours[i] += float64(t.TimeLoggedMinutes) / 60.0
			}
		}
	}
	for _, h := range d.weekHours {
		d.weekTotal += h
	}

	return d
}

func homeUser() string {
	u := os.Getenv("USER")
	if u == "" {
		return "there"
	}
	r := []rune(u)
	r[0] = []rune(strings.ToUpper(string(r[0])))[0]
	return string(r)
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func parseDayStart(due string, loc *time.Location) time.Time {
	d := strings.TrimSpace(due)
	for _, layout := range []string{core.TimeFormat, "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, d, loc); err == nil {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
		}
	}
	return time.Time{}
}

// sortDayRows orders: unfinished first (timed by time, then untimed), done last.
func sortDayRows(rows []*core.Task, isDone func(p, st string) bool) {
	// Insertion sort (rows are tiny).
	key := func(t *core.Task) (int, int) {
		done := 1
		if !isDone(t.Project, t.Status) {
			done = 0
		}
		min := 24 * 60
		if d, err := core.ParseDue(t.Due, time.Local); err == nil && len(t.Due) > len("2006-01-02") {
			min = d.Hour()*60 + d.Minute()
		}
		return done, min
	}
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0; j-- {
			d1, m1 := key(rows[j])
			d0, m0 := key(rows[j-1])
			if d1 < d0 || (d1 == d0 && m1 < m0) {
				rows[j], rows[j-1] = rows[j-1], rows[j]
				continue
			}
			break
		}
	}
}

func sortExams(exams []examCard) {
	for i := 1; i < len(exams); i++ {
		for j := i; j > 0; j-- {
			if exams[j].daysLeft < exams[j-1].daysLeft {
				exams[j], exams[j-1] = exams[j-1], exams[j]
				continue
			}
			break
		}
	}
}

func sortCourses(rows []courseRow) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0; j-- {
			if rows[j].total > rows[j-1].total {
				rows[j], rows[j-1] = rows[j-1], rows[j]
				continue
			}
			break
		}
	}
}
