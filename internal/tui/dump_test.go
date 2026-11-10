package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"

	"github.com/loharrahulofficial-crypto/mtm/internal/clock"
	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

// Golden-file verification for the --dump command. Every dump renders with a
// fixed clock (DumpFixedTime) and the fixed default theme; USER is pinned to
// keep the greeting deterministic; XDG/HOME redirect at temp dirs.
//
// Regenerate goldens with: MTM_UPDATE_GOLDEN=1 go test ./internal/tui/ -run TestDumpGolden

type fixtureTask struct {
	id, title, project, status, priority, kind, due string
	created, updated, completed                     string
	tags                                            []string
	parent                                          string
	milestone                                       string
	estimate, logged                                int
	weight                                          *float64
	score, scoreMax                                 *float64
	body                                            string
}

func fixedEnv(tb testing.TB) {
	tb.Helper()
	tb.Setenv("USER", "Rahul")
	tb.Setenv("HOME", tb.TempDir())
	tb.Setenv("XDG_CONFIG_HOME", tb.TempDir())
	tb.Setenv("XDG_DATA_HOME", tb.TempDir())
	tb.Setenv("XDG_CACHE_HOME", tb.TempDir())
	tb.Setenv("XDG_STATE_HOME", tb.TempDir())
}

type fixtureProject struct {
	name, color string
	course      *core.Course
}

func fixtureStore(t *testing.T, tasks []fixtureTask, projects ...fixtureProject) *store.Store {
	t.Helper()
	fixedEnv(t)
	s, err := store.Open(t.TempDir(), filepath.Join(t.TempDir(), "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	s.Clock = clock.Fixed(DumpFixedTime)
	defaults := []fixtureProject{{name: "inbox", color: ""}}
	for _, p := range append(defaults, projects...) {
		proj := core.DefaultProject(p.name)
		proj.Color = p.color
		proj.Course = p.course
		if err := s.SaveProject(proj); err != nil {
			t.Fatal(err)
		}
	}
	for i, ft := range tasks {
		tk := &core.Task{
			ID:                ft.id,
			Title:             ft.title,
			Project:           ft.project,
			Status:            ft.status,
			Priority:          ft.priority,
			Kind:              ft.kind,
			Due:               ft.due,
			Created:           ft.created,
			Updated:           ft.updated,
			Completed:         ft.completed,
			Tags:              ft.tags,
			Parent:            ft.parent,
			Milestone:         ft.milestone,
			EstimateMinutes:   ft.estimate,
			TimeLoggedMinutes: ft.logged,
			Weight:            ft.weight,
			Score:             ft.score,
			ScoreMax:          ft.scoreMax,
			Body:              ft.body,
		}
		if tk.ID == "" {
			tk.ID = fmt.Sprintf("t%04d", i)
		}
		if err := s.Create(tk); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func emptyFixture(t *testing.T) *store.Store {
	return fixtureStore(t, nil)
}

func twoTasksFixture(t *testing.T) *store.Store {
	ts := DumpFixedTime
	stamp := ts.Format(core.TimestampFormat)
	today := ts.Format("2006-01-02")
	return fixtureStore(t, []fixtureTask{
		{id: "a00001", title: "Read ch.4 — storage models", project: "inbox", status: "todo", priority: "high", kind: "assignment", due: today + "T09:00", created: stamp, updated: stamp, estimate: 60, logged: 90},
		{id: "a00002", title: "Submit problem set", project: "inbox", status: "todo", priority: "medium", kind: "assignment", due: ts.AddDate(0, 0, 3).Format("2006-01-02"), created: stamp, updated: stamp, estimate: 45},
	})
}

func manyTasksFixture(t *testing.T) *store.Store {
	ts := DumpFixedTime
	stamp := ts.Format(core.TimestampFormat)
	tasks := []fixtureTask{}
	for i := 0; i < 50; i++ {
		due := ""
		if i%5 != 4 {
			due = ts.AddDate(0, 0, i%14-3).Format("2006-01-02") + "T10:00"
		}
		prios := []string{"urgent", "high", "medium", "low"}
		tasks = append(tasks, fixtureTask{
			id:       fmt.Sprintf("b%05d", i),
			title:    fmt.Sprintf("task number %02d with a reasonably long descriptive title", i),
			project:  "inbox",
			status:   "todo",
			priority: prios[i%4],
			kind:     "task",
			due:      due,
			created:  stamp,
			updated:  stamp,
		})
	}
	return fixtureStore(t, tasks)
}

func longTitlesFixture(t *testing.T) *store.Store {
	ts := DumpFixedTime
	stamp := ts.Format(core.TimestampFormat)
	longTitle := strings.Repeat("an extremely long title that keeps on going ", 8)
	return fixtureStore(t, []fixtureTask{
		{id: "c00001", title: longTitle, project: "inbox", status: "todo", priority: "high", kind: "assignment", due: ts.Format("2006-01-02") + "T09:00", created: stamp, updated: stamp},
		{id: "c00002", title: "short title", project: "inbox", status: "todo", priority: "medium", kind: "task", created: stamp, updated: stamp},
	})
}

func subtasksFixture(t *testing.T) *store.Store {
	ts := DumpFixedTime
	stamp := ts.Format(core.TimestampFormat)
	weight := 30.0
	tasks := []fixtureTask{
		{id: "d00000", title: "DBMS midterm — relational algebra + SQL", project: "college/dbms", status: "todo", priority: "urgent", kind: "exam", due: ts.AddDate(0, 0, 6).Format("2006-01-02") + "T09:00", created: stamp, updated: stamp, weight: &weight, estimate: 120, body: "Syllabus: ch 1-6. Focus on normalization + transactions."},
	}
	for i := 1; i <= 12; i++ {
		status := "todo"
		if i <= 7 {
			status = "done"
		}
		tasks = append(tasks, fixtureTask{
			id: fmt.Sprintf("d%05d", i), title: fmt.Sprintf("Revise ch.%d — topic %d", i, i),
			project: "college/dbms", status: status, priority: "medium", kind: "task",
			created: stamp, updated: stamp, parent: "d00000",
			completed: doneStamp(status, stamp),
		})
	}
	return fixtureStore(t, tasks,
		fixtureProject{name: "college/dbms", color: "blue", course: &core.Course{Code: "DBMS", Name: "Database Systems", Semester: "2026-odd", Credits: 4, TargetGrade: "A"}},
		fixtureProject{name: "college/os", color: "green", course: &core.Course{Code: "OS", Name: "Operating Systems", Semester: "2026-odd", Credits: 4, TargetGrade: "B+"}},
		fixtureProject{name: "college/maths", color: "yellow", course: &core.Course{Code: "MAM", Name: "Mathematics", Semester: "2026-odd", Credits: 3, TargetGrade: "A"}},
	)
}

// seedIdeas writes a fixed set of idea files into root/ideas. Used by the
// ideas-view fixtures and by the Home blank-ratio test's fixture.
func seedIdeas(t *testing.T, root string) {
	t.Helper()
	ts := DumpFixedTime
	slug := func(s string) string {
		out := []rune{}
		lastDash := true
		for _, r := range strings.ToLower(s) {
			switch {
			case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
				out = append(out, r)
				lastDash = false
			default:
				if !lastDash {
					out = append(out, '-')
					lastDash = true
				}
			}
		}
		trim := strings.Trim(string(out), "-")
		if trim == "" {
			return "idea"
		}
		return trim
	}
	mk := func(id, status string, rating int, title string, createdBack, updatedBack int, tags []string, promotedTo, body string) {
		t.Helper()
		created := ts.AddDate(0, 0, createdBack).Format(core.TimestampFormat)
		updated := ts.AddDate(0, 0, updatedBack).Format(core.TimestampFormat)
		var b strings.Builder
		b.WriteString("---\nid: " + id + "\ntitle: " + title + "\nstatus: " + status + "\n")
		if rating > 0 {
			b.WriteString(fmt.Sprintf("rating: %d\n", rating))
		}
		if len(tags) > 0 {
			b.WriteString("tags:\n")
			for _, tag := range tags {
				b.WriteString("- " + tag + "\n")
			}
		}
		b.WriteString("created: " + created + "\nupdated: " + updated + "\n")
		if promotedTo != "" {
			b.WriteString("promoted_to: " + promotedTo + "\n")
		}
		b.WriteString("---\n" + body)
		if err := os.WriteFile(filepath.Join(root, "ideas", id+"-"+slug(title)+".md"), []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "ideas"), 0o755); err != nil {
		t.Fatal(err)
	}
	mk("i00001", "exploring", 5, "Local-first sync engine", -40, -2, []string{"sync", "go"}, "", "CRDT or oplog?\nCompare Automerge and ElectricSQL.\n")
	mk("i00002", "seed", 0, "Offline flashcard deck for exam prep", -3, -1, []string{"study"}, "", "")
	mk("i00003", "seed", 3, "Pomodoro ambient scenes", -20, -10, []string{"tui"}, "", "Rain, cafe, library.\n")
	mk("i00004", "parked", 2, "Paperless todo pipeline", -90, -30, []string{"paper"}, "", "Waiting on a scanner.\n")
	mk("i00005", "promoted", 4, "AI syllabus import", -120, -50, []string{"ai"}, "college/dbms", "Shipped as mtm syllabus import.\n")
	mk("i00006", "dropped", 1, "Voice memos to tasks", -200, -80, []string{"audio"}, "", "Privacy wore me down.\n")
}

// reopenStore closes s and re-opens the same repo with a fresh index.
func reopenStore(t *testing.T, s *store.Store) *store.Store {
	t.Helper()
	root := s.Root
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	opened, err := store.Open(root, filepath.Join(t.TempDir(), "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	opened.Clock = clock.Fixed(DumpFixedTime)
	return opened
}

func ideasFixture(t *testing.T) *store.Store {
	ts := DumpFixedTime
	stamp := ts.Format(core.TimestampFormat)
	tasks := []fixtureTask{
		{id: "e00001", title: "Submit problem set 4", project: "inbox", status: "todo", priority: "medium", kind: "assignment", due: ts.AddDate(0, 0, 2).Format("2006-01-02"), created: stamp, updated: stamp},
	}
	s := fixtureStore(t, tasks)
	seedIdeas(t, s.Root)
	return reopenStore(t, s)
}

func f64(f float64) *float64 { return &f }

func coursesFixture(t *testing.T) *store.Store {
	ts := DumpFixedTime
	stamp := ts.Format(core.TimestampFormat)
	today := ts.Format("2006-01-02")
	tasks := []fixtureTask{
		{id: "e00001", title: "Read ch.4 — storage models", project: "college/dbms", status: "todo", priority: "high", kind: "assignment", due: today + "T09:00", created: stamp, updated: stamp, estimate: 60, weight: f64(30), score: f64(27), scoreMax: f64(30)},
		{id: "e00002", title: "OS lab report — page replacement", project: "college/os", status: "doing", priority: "high", kind: "assignment", due: today + "T14:00", created: stamp, updated: stamp, estimate: 90, tags: []string{"os", "lab"}, weight: f64(20), score: f64(8), scoreMax: f64(10)},
		{id: "e00003", title: "Maths revision — integration by parts", project: "college/maths", status: "todo", priority: "urgent", kind: "assignment", due: ts.AddDate(0, 0, -1).Format("2006-01-02"), created: stamp, updated: stamp, weight: f64(30), score: f64(27), scoreMax: f64(30)},
		{id: "e00020", title: "DBMS midterm — relational algebra + SQL", project: "college/dbms", status: "todo", priority: "urgent", kind: "exam", due: ts.AddDate(0, 0, 6).Format("2006-01-02") + "T09:00", created: stamp, updated: stamp, estimate: 120, weight: f64(30)},
		{id: "e00021", title: "OS final — scheduling + deadlocks", project: "college/os", status: "todo", priority: "high", kind: "exam", due: ts.AddDate(0, 0, 13).Format("2006-01-02") + "T10:00", created: stamp, updated: stamp, estimate: 90, weight: f64(40)},
		{id: "e00022", title: "Maths final — series convergence", project: "college/maths", status: "todo", priority: "high", kind: "exam", due: ts.AddDate(0, 0, 19).Format("2006-01-02") + "T10:00", created: stamp, updated: stamp, estimate: 90, weight: f64(40)},
	}
	// Completed history for courses + heatmap.
	for i := 0; i < 12; i++ {
		proj := []string{"college/dbms", "college/os", "college/maths"}[i%3]
		stampBack := ts.AddDate(0, 0, -i).Format(core.TimestampFormat)
		stampDay := stampBack[:10]
		status := "done"
		tasks = append(tasks, fixtureTask{
			id: fmt.Sprintf("e%05d", i+10), title: fmt.Sprintf("completed item %d", i), project: proj,
			status: status, priority: "medium", kind: "task",
			created: stampBack, updated: stampBack, completed: stampBack, logged: 60 + i*5, due: stampDay,
		})
	}
	s := fixtureStore(t, tasks,
		fixtureProject{name: "college/dbms", color: "blue", course: &core.Course{Code: "DBMS", Name: "Database Systems", Semester: "2026-odd", Credits: 4, TargetGrade: "A"}},
		fixtureProject{name: "college/os", color: "green", course: &core.Course{Code: "OS", Name: "Operating Systems", Semester: "2026-odd", Credits: 4, TargetGrade: "B+"}},
		fixtureProject{name: "college/maths", color: "yellow", course: &core.Course{Code: "MAM", Name: "Mathematics", Semester: "2026-odd", Credits: 3, TargetGrade: "A"}},
	)
	// Seed ideas so the Home blank-ratio test exercises the IDEAS band.
	seedIdeas(t, s.Root)
	return reopenStore(t, s)
}

// Phase 4 data: two exams across two courses, topic progress on the nearest
// one — what Home should surface and what the calendar marks with ✦. The
// near exam is single-digit days-left, the far one two-digit.
func examsFixture(t *testing.T) *store.Store {
	ts := DumpFixedTime
	stamp := ts.Format(core.TimestampFormat)
	dbmsWeight := 30.0
	osWeight := 10.0
	tasks := []fixtureTask{
		{id: "x00001", title: "DBMS midterm — relational algebra + SQL", project: "college/dbms", status: "todo", priority: "urgent", kind: "exam", due: ts.AddDate(0, 0, 6).Format("2006-01-02") + "T09:00", created: stamp, updated: stamp, weight: &dbmsWeight, estimate: 120, body: "Syllabus: ch 1-6. Focus on normalization + transactions."},
		{id: "x00002", title: "OS quiz — scheduling & deadlocks", project: "college/os", status: "todo", priority: "high", kind: "exam", due: ts.AddDate(0, 0, 12).Format("2006-01-02") + "T10:00", created: stamp, updated: stamp, weight: &osWeight, estimate: 45},
	}
	// Topic progress for the DBMS exam (topics bar): 4 of 6 revised.
	for i := 1; i <= 6; i++ {
		status := "todo"
		if i <= 4 {
			status = "done"
		}
		tasks = append(tasks, fixtureTask{
			id: fmt.Sprintf("x%05d", 10000+i), title: fmt.Sprintf("Revise ch.%d", i),
			project: "college/dbms", status: status, priority: "medium", kind: "task",
			created: stamp, updated: stamp, parent: "x00001",
			completed: doneStamp(status, stamp),
		})
	}
	// One maths task so all three courses appear on Home.
	tasks = append(tasks, fixtureTask{
		id: "x00090", title: "Maths — integration by parts set", project: "college/maths", status: "todo",
		priority: "medium", kind: "assignment", due: ts.AddDate(0, 0, 2).Format("2006-01-02") + "T18:00",
		created: stamp, updated: stamp,
	})
	return fixtureStore(t, tasks,
		fixtureProject{name: "college/dbms", color: "blue", course: &core.Course{Code: "DBMS", Name: "Database Systems", Semester: "2026-odd", Credits: 4, TargetGrade: "A"}},
		fixtureProject{name: "college/os", color: "green", course: &core.Course{Code: "OS", Name: "Operating Systems", Semester: "2026-odd", Credits: 4, TargetGrade: "B+"}},
		fixtureProject{name: "college/maths", color: "yellow", course: &core.Course{Code: "MAM", Name: "Mathematics", Semester: "2026-odd", Credits: 3, TargetGrade: "A"}},
	)
}

func doneStamp(status, stamp string) string {
	if status == "done" {
		return stamp
	}
	return ""
}

func boardColumnsFixture(t *testing.T) *store.Store {
	ts := DumpFixedTime
	stamp := ts.Format(core.TimestampFormat)
	today := ts.Format("2006-01-02")
	return fixtureStore(t, []fixtureTask{
		{id: "m00001", title: "Write lecture notes on B-trees", project: "inbox", status: "todo", priority: "high", kind: "assignment", due: today + "T10:00", created: stamp, updated: stamp},
		{id: "m00002", title: "Prepare problem set 4", project: "inbox", status: "doing", priority: "medium", kind: "assignment", due: today + "T15:00", created: stamp, updated: stamp},
		{id: "m00003", title: "Submit lab report — page replacement", project: "inbox", status: "doing", priority: "urgent", kind: "assignment", due: ts.AddDate(0, 0, 2).Format("2006-01-02"), created: stamp, updated: stamp},
		{id: "m00004", title: "Read ch.2 of the OS book", project: "inbox", status: "todo", priority: "low", kind: "task", created: stamp, updated: stamp},
	})
}

// doneTasksFixture: inbox tasks with a done one (completed today, spaces
// in the title) — the shared done-display fixture.
func doneTasksFixture(t *testing.T) *store.Store {
	ts := DumpFixedTime
	stamp := ts.Format(core.TimestampFormat)
	today := ts.Format("2006-01-02")
	cam := 30.0
	_ = cam
	return fixtureStore(t, []fixtureTask{
		{id: "e00001", title: "Write the plan now", project: "inbox", status: "done", priority: "high", kind: "task", due: today + "T09:00", created: stamp, updated: stamp, completed: stamp},
		{id: "e00002", title: "Keep the remaining thing open", project: "inbox", status: "todo", priority: "medium", kind: "task", due: ts.AddDate(0, 0, 2).Format("2006-01-02"), created: stamp, updated: stamp},
	})
}

// projectToolsFixture: a full project page's worth of data — two milestones
// (one active, one planned), two dev-log entries, linked tasks, a temp git
// repo with fixed-date commits (deterministic sparkline/streak) and a fake
// gh on MTM_GH so the remote counts are canned. Everything date-related is
// anchored to DumpFixedTime.
func projectToolsFixture(t *testing.T) *store.Store {
	ts := DumpFixedTime
	stamp := ts.Format(core.TimestampFormat)
	stampAt := func(daysBack int) string { return ts.AddDate(0, 0, daysBack).Format(core.TimestampFormat) }

	// Temp git repo with fixed commit dates (author date drives %aI).
	repo := t.TempDir()
	gitEnv := append(os.Environ(),
		"GIT_AUTHOR_NAME=mtm", "GIT_AUTHOR_EMAIL=mtm@example.invalid",
		"GIT_COMMITTER_NAME=mtm", "GIT_COMMITTER_EMAIL=mtm@example.invalid")
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = gitEnv
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	git("init", "-q", "-b", "main")
	commits := []struct{ day, msg string }{
		{"2026-09-10T10:00:00+00:00", "docs: syllabus import design"},
		{"2026-10-06T10:00:00+00:00", "store: index ideas dir"},
		{"2026-10-14T10:00:00+00:00", "tui: board card drag-and-drop"},
		{"2026-10-14T18:00:00+00:00", "timer: stale accrual cap"},
		{"2026-10-22T10:00:00+00:00", "sync: rework conflict screen"},
		{"2026-10-22T15:00:00+00:00", "fix: duplicate task on merge"},
		{"2026-10-28T10:00:00+00:00", "gauge: thin bars everywhere"},
		{"2026-10-31T10:00:00+00:00", "grades: target met math"},
		{"2026-11-02T10:00:00+00:00", "stats: streak from completion day"},
		{"2026-11-05T10:00:00+00:00", "ideas: incubation cards"},
		{"2026-11-06T10:00:00+00:00", "ical: milestone events"},
	}
	for i, c := range commits {
		if err := os.WriteFile(filepath.Join(repo, fmt.Sprintf("f%d.txt", i)), []byte(c.msg+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		git("add", "-A")
		git("commit", "-q", "--date", c.day, "-m", c.msg)
	}

	tasks := []fixtureTask{
		{id: "p00001", title: "Revise normalization", project: "college/dbms", status: "doing", priority: "high", kind: "study", created: stamp, updated: stamp, milestone: "ms0001"},
		{id: "p00002", title: "Problem set 5", project: "college/dbms", status: "todo", priority: "medium", kind: "assignment", due: "2026-11-13", created: stamp, updated: stamp, milestone: "ms0001"},
		{id: "p00003", title: "DB design doc", project: "college/dbms", status: "done", priority: "medium", kind: "task", created: stampAt(-4), updated: stampAt(-3), completed: stampAt(-3), milestone: "ms0002"},
		{id: "p00004", title: "OS lab report", project: "college/os", status: "todo", priority: "low", kind: "task", created: stamp, updated: stamp},
	}
	s := fixtureStore(t, tasks,
		fixtureProject{name: "college/dbms", color: "blue", course: &core.Course{Code: "DBMS", Name: "Database Systems", Semester: "2026-odd", Credits: 4, TargetGrade: "A"}},
		fixtureProject{name: "college/os", color: "green", course: &core.Course{Code: "OS", Name: "Operating Systems", Semester: "2026-odd", Credits: 4, TargetGrade: "B+"}},
	)
	root := s.Root
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	write := func(rel, content string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("projects/college/dbms/milestones/ms0001-v0-1-study-mvp.md",
		"---\nid: ms0001\ntitle: v0.1 — study MVP\ntarget: 2026-11-19\nstatus: active\ndescription: Course companion features\ncreated: "+stampAt(-18)+"\nupdated: "+stampAt(-2)+"\n---\nScope: quick-add, grades, syllabus import.\n")
	write("projects/college/dbms/milestones/ms0002-v1-0-public-beta.md",
		"---\nid: ms0002\ntitle: v1.0 — public beta\ntarget: 2027-01-30\nstatus: planned\ndescription: Open-source release\ncreated: "+stampAt(-6)+"\nupdated: "+stampAt(-6)+"\n---\n")
	write("projects/college/dbms/devlog/2026-11-04-dl00001.md",
		"---\nid: dl00001\ntitle: Week 2 wrap\nmood: flow\ntime_spent_minutes: 90\ntags:\n- study\ncreated: "+stampAt(-3)+"\n---\nER diagrams, then 3NF. Lost an hour to the foreign-key rabbit hole but the schema clicked.\n")
	write("projects/college/dbms/devlog/2026-11-01-dl00002.md",
		"---\nid: dl00002\ntitle: ER diagrams\ntime_spent_minutes: 45\ntags:\n- study\ncreated: "+stampAt(-6)+"\n---\nCrow's foot notation is opinionated and I respect it.\n")

	// Link the repo + a fake gh on MTM_GH (fixed issue/PR payloads).
	pyaml := filepath.Join(root, "projects", "college", "dbms", "project.yaml")
	data, err := os.ReadFile(pyaml)
	if err != nil {
		t.Fatal(err)
	}
	var proj map[string]any
	if err := yaml.Unmarshal(data, &proj); err != nil {
		t.Fatal(err)
	}
	proj["github"] = "loharrahulofficial-crypto/mtm"
	proj["repo_path"] = repo
	out, err := yaml.Marshal(proj)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pyaml, out, 0o644); err != nil {
		t.Fatal(err)
	}

	fakeGH := filepath.Join(t.TempDir(), "gh")
	script := "#!/bin/sh\n" +
		"case \"$*\" in\n" +
		"  \"auth status\"*) echo \"✓ Logged in to github.com\"; exit 0;;\n" +
		"  \"issue list\"*) printf '%s' '[{\"number\":12,\"title\":\"Sync: duplicate task on merge\",\"url\":\"https://github.com/x/mtm/issues/12\"},{\"number\":9,\"title\":\"Waybar icon glitch on resumed timer\",\"url\":\"https://github.com/x/mtm/issues/9\"}]'; exit 0;;\n" +
		"  \"pr list\"*) printf '%s' '[{\"number\":18,\"title\":\"board: column virtualization\",\"url\":\"https://github.com/x/mtm/pull/18\"}]'; exit 0;;\n" +
		"esac\n" +
		"exit 1\n"
	if err := os.WriteFile(fakeGH, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MTM_GH", fakeGH)

	return reopenStore(t, s)
}

var fixtures = map[string]func(t *testing.T) *store.Store{
	"empty":        emptyFixture,
	"populated":    twoTasksFixture,
	"doneTasks":    doneTasksFixture,
	"manyTasks":    manyTasksFixture,
	"longTitles":   longTitlesFixture,
	"subtasks":     subtasksFixture,
	"courses":      coursesFixture,
	"boardColumns": boardColumnsFixture,
	"exams":        examsFixture,
	"ideas":        ideasFixture,
	"projectTools": projectToolsFixture,
}

func dumpConfig() config.Config {
	cfg := config.Default()
	cfg.Sync.Auto = false
	cfg.DataRepo = filepath.Join(os.TempDir(), "mtm-dump-unused")
	return cfg
}

func renderFixture(t *testing.T, view, size, state string) string {
	t.Helper()
	s := fixtures[state](t)
	var w, h int
	if _, err := fmt.Sscanf(size, "%dx%d", &w, &h); err != nil {
		t.Fatalf("bad size %q", size)
	}
	frame, err := RenderDump(dumpConfig(), s, sync.New(s.Root), view, w, h, true, DumpFixedTime)
	if err != nil {
		t.Fatalf("RenderDump(%s, %s, %s): %v", view, size, state, err)
	}
	_ = s.Close()
	return frame
}

func TestDumpGolden(t *testing.T) {
	views := []string{"home", "list", "board", "calendar", "courses", "focus", "stats", "ideas", "project", "roadmap", "devlog"}
	sizes := []string{"80x24", "120x40", "200x50"}
	states := []string{"empty", "populated"}

	update := os.Getenv("MTM_UPDATE_GOLDEN") == "1"

	dir := filepath.Join("testdata", "dump")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, view := range views {
		for _, size := range sizes {
			for _, state := range states {
				t.Run(view+"_"+size+"_"+state, func(t *testing.T) {
					golden := filepath.Join(dir, view+"-"+size+"-"+state+".golden")
					got := renderFixture(t, view, size, state)
					if update {
						if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
							t.Fatal(err)
						}
						return
					}
					want, err := os.ReadFile(golden)
					if err != nil {
						t.Fatalf("missing golden %s (run MTM_UPDATE_GOLDEN=1 to create)", golden)
					}
					if got != string(want) {
						t.Fatalf("dump mismatch in %s\n--- got ---\n%s\n--- want ---\n%s", golden, got, want)
					}
				})
			}
			// The ≥150-column (three-band) Home layout with the Phase 4
			// exam cards: block digits, topics bar, both exams visible.
			if view == "home" && size == "200x50" {
				t.Run(view+"_"+size+"_exams", func(t *testing.T) {
					golden := filepath.Join(dir, view+"-"+size+"-exams.golden")
					got := renderFixture(t, view, size, "exams")
					if update {
						if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
							t.Fatal(err)
						}
						return
					}
					want, err := os.ReadFile(golden)
					if err != nil {
						t.Fatalf("missing golden %s (run MTM_UPDATE_GOLDEN=1 to create)", golden)
					}
					if got != string(want) {
						t.Fatalf("dump mismatch in %s\n--- got ---\n%s\n--- want ---\n%s", golden, got, want)
					}
				})
			}
			// Richer states at the canonical size.
			for _, state := range []string{"manyTasks", "longTitles", "subtasks", "courses", "boardColumns", "exams", "ideas", "projectTools"} {
				t.Run(view+"_"+size+"_"+state, func(t *testing.T) {
					if size != "120x40" {
						t.Skip("rich states only at 120x40")
					}
					golden := filepath.Join(dir, view+"-"+size+"-"+state+".golden")
					got := renderFixture(t, view, size, state)
					if update {
						if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
							t.Fatal(err)
						}
						return
					}
					want, err := os.ReadFile(golden)
					if err != nil {
						t.Fatalf("missing golden %s (run MTM_UPDATE_GOLDEN=1 to create)", golden)
					}
					if got != string(want) {
						t.Fatalf("dump mismatch in %s\n--- got ---\n%s\n--- want ---\n%s", golden, got, want)
					}
				})
			}
		}
	}
}

func TestRenderDumpFrameShape(t *testing.T) {
	s := fixtures["populated"](t)
	defer func() { _ = s.Close() }()
	frame, err := RenderDump(dumpConfig(), s, sync.New(s.Root), "list", 80, 24, false, DumpFixedTime)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(frame, "\n")
	if len(lines) != 24 {
		t.Errorf("%d lines, want 24", len(lines))
	}
	for i, l := range lines {
		if w := lipglossWidthT(l); w != 80 {
			t.Errorf("line %d = %d cells", i, w)
		}
	}
	// Colored output carries ANSI escapes; --no-color strips them.
	plain, err := RenderDump(dumpConfig(), s, sync.New(s.Root), "list", 80, 24, true, DumpFixedTime)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain, "\x1b[") {
		t.Fatal("--no-color output still contains ANSI")
	}
	if !strings.Contains(plain, "STS") {
		t.Fatal("plain output missing list chrome")
	}
}

func TestRenderDumpUnknownView(t *testing.T) {
	s := fixtures["empty"](t)
	defer func() { _ = s.Close() }()
	if _, err := RenderDump(dumpConfig(), s, sync.New(s.Root), "nope", 80, 24, true, DumpFixedTime); err == nil {
		t.Fatal("unknown view accepted")
	}
}

func lipglossWidthT(s string) int { return lipgloss.Width(s) }
