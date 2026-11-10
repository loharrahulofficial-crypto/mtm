package actions

// Serializers for the export read model (see export.go).

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

// ExportJSON renders the report as indented JSON with schema_version 2.
func ExportJSON(rep *ExportReport) ([]byte, error) {
	return json.MarshalIndent(rep, "", "  ")
}

// CSVColumns is the stable column order for --format csv. One row per
// task (subtasks included, parent referenced by id).
var CSVColumns = []string{
	"id", "title", "project", "status", "priority", "kind", "tags",
	"due", "reminder", "recurrence", "parent",
	"created", "updated", "completed",
	"estimate_minutes", "time_logged_minutes",
	"weight", "score", "score_max", "body",
}

// ExportCSV renders the task table with stable columns.
func ExportCSV(rep *ExportReport) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(CSVColumns); err != nil {
		return nil, err
	}
	for _, t := range rep.Tasks {
		row := []string{
			t.ID, t.Title, t.Project, t.Status, t.Priority, t.Kind,
			strings.Join(t.Tags, ";"),
			t.Due, t.Reminder, t.Recurrence, t.Parent,
			t.Created, t.Updated, t.Completed,
			fmt.Sprint(t.EstimateMinutes), fmt.Sprint(t.TimeLoggedMinutes),
			numPtrPlain(t.Weight), numPtrPlain(t.Score), numPtrPlain(t.ScoreMax),
			bodyForCSV(t.Body),
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

func numPtrPlain(p *float64) string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%g", *p)
}

func bodyForCSV(s string) string {
	// Keep one line per row; escape newlines for readability.
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", " "), "\n", " ")
}

// ExportMarkdown renders the readable report, grouped by course then by
// remaining project, with grades and hours inline.
func ExportMarkdown(rep *ExportReport, now time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# mtm export — %s\n\n", now.Format("2 Jan 2006"))
	fmt.Fprintf(&b, "%d tasks · %d done · %.1fh logged · generated %s\n\n",
		rep.Totals.Tasks, rep.Totals.Completed, float64(rep.Totals.HoursMinutes)/60, rep.GeneratedAt)

	projects := map[string][]*ExportTask{}
	for i := range rep.Tasks {
		projects[rep.Tasks[i].Project] = append(projects[rep.Tasks[i].Project], &rep.Tasks[i])
	}

	// Course sections first (sorted by name), then the rest.
	courseSet := map[string]ExportCourse{}
	for _, c := range rep.Courses {
		courseSet[c.Name] = c
	}
	var sectionNames []string
	var courseSections []string
	for name := range projects {
		if _, ok := courseSet[name]; ok {
			courseSections = append(courseSections, name)
		} else {
			sectionNames = append(sectionNames, name)
		}
	}
	sort.Strings(courseSections)
	sort.Strings(sectionNames)

	writeSection := func(name string, course *ExportCourse) {
		fmt.Fprintf(&b, "## %s\n\n", name)
		if course != nil {
			meta := fmt.Sprintf("course · %d done / %d tasks · %.1fh logged", course.Done, course.Total, float64(course.HoursMinutes)/60)
			if course.TargetGrade != "" {
				meta += " · target " + course.TargetGrade
			}
			fmt.Fprintf(&b, "*%s*\n", meta)
			if course.GradeStandingPct != nil {
				fmt.Fprintf(&b, "*grade standing %.1f%% · current %.1f%%*\n", *course.GradeStandingPct, *course.GradeCurrentPct)
			}
			fmt.Fprintln(&b)
		}
		tasks := projects[name]
		if course != nil {
			writeExamsSection(&b, rep, name)
		}
		writeTasksSection(&b, tasks, now, rep)
	}
	for _, name := range courseSections {
		c := courseSet[name]
		writeSection(name, &c)
	}
	for _, name := range sectionNames {
		writeSection(name, nil)
	}
	if len(rep.Tasks) == 0 {
		fmt.Fprint(&b, "_no tasks match the filters_\n\n")
	}
	return b.String()
}

func writeExamsSection(b *strings.Builder, rep *ExportReport, course string) {
	var mine []ExportExam
	for _, e := range rep.Exams {
		if e.Course == course {
			mine = append(mine, e)
		}
	}
	if len(mine) == 0 {
		return
	}
	fmt.Fprint(b, "### exams\n\n")
	for _, e := range mine {
		weight := ""
		if e.Weight != nil {
			weight = fmt.Sprintf(" · weight %.0f%%", *e.Weight)
		}
		fmt.Fprintf(b, "- %s — due %s%s · topics %d/%d\n", e.Title, humanDue(e.Due), weight, e.TopicsDone, e.TopicsTotal)
	}
	fmt.Fprintln(b)
}

func writeTasksSection(b *strings.Builder, tasks []*ExportTask, now time.Time, rep *ExportReport) {
	children := map[string][]*ExportTask{}
	var roots []*ExportTask
	byID := map[string]*ExportTask{}
	for _, t := range tasks {
		byID[t.ID] = t
	}
	for _, t := range tasks {
		if t.Parent != "" {
			if _, ok := byID[t.Parent]; ok {
				children[t.Parent] = append(children[t.Parent], t)
				continue
			}
		}
		roots = append(roots, t)
	}
	gradeByTask := map[string]ExportGrade{}
	for _, g := range rep.Grades {
		gradeByTask[g.TaskID] = g
	}

	fmt.Fprint(b, "### tasks\n\n")
	var writeOne func(t *ExportTask, depth int)
	writeOne = func(t *ExportTask, depth int) {
		indent := strings.Repeat("  ", depth)
		mark := " "
		if isDoneExport(t.Status) {
			mark = "x"
		}
		meta := []string{}
		if t.Priority != "" && t.Priority != "medium" {
			meta = append(meta, t.Priority)
		}
		if t.Due != "" {
			meta = append(meta, "due "+humanDue(t.Due))
		}
		if t.EstimateMinutes > 0 {
			meta = append(meta, fmt.Sprintf("est ~%s", actionsFormatMinutes(t.EstimateMinutes)))
		}
		if t.TimeLoggedMinutes > 0 {
			meta = append(meta, fmt.Sprintf("%s logged", actionsFormatMinutes(t.TimeLoggedMinutes)))
		}
		if g, ok := gradeByTask[t.ID]; ok && g.Scored {
			meta = append(meta, fmt.Sprintf("score %.0f%%", g.Pct))
		} else if ok && !g.Scored {
			meta = append(meta, "not graded")
		}
		suffix := ""
		if len(meta) > 0 {
			suffix = " · " + strings.Join(meta, " · ")
		}
		fmt.Fprintf(b, "%s- [%s] %s%s\n", indent, mark, t.Title, suffix)
		for _, child := range children[t.ID] {
			writeOne(child, depth+1)
		}
	}
	for _, t := range roots {
		writeOne(t, 0)
	}
	fmt.Fprintln(b)
}

func isDoneExport(status string) bool {
	return strings.EqualFold(status, "done") ||
		strings.EqualFold(status, "completed") ||
		strings.EqualFold(status, "cancelled") ||
		strings.EqualFold(status, "canceled")
}

func humanDue(due string) string {
	if due == "" {
		return ""
	}
	if len(due) > len("2006-01-02") {
		if ts, err := time.ParseInLocation(core.TimestampFormat, due, time.Local); err == nil {
			return ts.Format("Mon 2 Jan 15:04")
		}
		return due
	}
	if d, err := time.ParseInLocation("2006-01-02", due, time.Local); err == nil {
		return d.Format("Mon 2 Jan")
	}
	return due
}

// actionsFormatMinutes is FormatMinutes, unexported alias for markdown.
func actionsFormatMinutes(mins int) string {
	if mins <= 0 {
		return "0m"
	}
	h, m := mins/60, mins%60
	switch {
	case h == 0:
		return fmt.Sprintf("%dm", m)
	case m == 0:
		return fmt.Sprintf("%dh", h)
	default:
		return fmt.Sprintf("%dh %dm", h, m)
	}
}
