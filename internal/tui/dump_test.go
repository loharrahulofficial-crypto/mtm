package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
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
		{id: "a00001", title: "Read ch.4 — storage models", project: "inbox", status: "todo", priority: "high", kind: "assignment", due: today + "T09:00", created: stamp, updated: stamp, estimate: 60},
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
	return fixtureStore(t, tasks,
		fixtureProject{name: "college/dbms", color: "blue", course: &core.Course{Code: "DBMS", Name: "Database Systems", Semester: "2026-odd", Credits: 4, TargetGrade: "A"}},
		fixtureProject{name: "college/os", color: "green", course: &core.Course{Code: "OS", Name: "Operating Systems", Semester: "2026-odd", Credits: 4, TargetGrade: "B+"}},
		fixtureProject{name: "college/maths", color: "yellow", course: &core.Course{Code: "MAM", Name: "Mathematics", Semester: "2026-odd", Credits: 3, TargetGrade: "A"}},
	)
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

var fixtures = map[string]func(t *testing.T) *store.Store{
	"empty":        emptyFixture,
	"populated":    twoTasksFixture,
	"manyTasks":    manyTasksFixture,
	"longTitles":   longTitlesFixture,
	"subtasks":     subtasksFixture,
	"courses":      coursesFixture,
	"boardColumns": boardColumnsFixture,
	"exams":        examsFixture,
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
	views := []string{"home", "list", "board", "calendar", "courses", "focus"}
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
			for _, state := range []string{"manyTasks", "longTitles", "subtasks", "courses", "boardColumns", "exams"} {
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
