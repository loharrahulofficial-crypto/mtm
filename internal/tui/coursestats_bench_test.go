package tui

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/loharrahulofficial-crypto/mtm/internal/clock"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

// benchStore2000 builds a store with 2000 tasks spread across 4 courses
// (500 each) plus an inbox.
func benchStore2000(tb testing.TB) *store.Store {
	tb.Helper()
	tb.Setenv("HOME", tb.TempDir())
	tb.Setenv("XDG_CONFIG_HOME", tb.TempDir())
	tb.Setenv("XDG_DATA_HOME", tb.TempDir())
	tb.Setenv("XDG_CACHE_HOME", tb.TempDir())
	tb.Setenv("XDG_STATE_HOME", tb.TempDir())
	ts := DumpFixedTime
	stamp := ts.Format(core.TimestampFormat)
	s, err := store.Open(tb.TempDir(), filepath.Join(tb.TempDir(), "index.json"))
	if err != nil {
		tb.Fatal(err)
	}
	s.Clock = clock.Fixed(ts)
	for _, p := range []string{"inbox", "college/dbms", "college/os", "college/maths", "college/algo"} {
		cfg := core.DefaultProject(p)
		if p != "inbox" {
			cfg.Course = &core.Course{Code: p[len("college/"):], Name: p, Semester: "2026-odd", Credits: 3, TargetGrade: "A"}
		}
		if err := s.SaveProject(cfg); err != nil {
			tb.Fatal(err)
		}
	}
	courses := []string{"college/dbms", "college/os", "college/maths", "college/algo"}
	for i := 0; i < 2000; i++ {
		project := "inbox"
		if i < 1800 {
			project = courses[i%len(courses)]
		}
		kind := core.KindTask
		if i%17 == 0 {
			kind = core.KindAssignment
		}
		if i%23 == 0 {
			kind = core.KindExam
		}
		status := "todo"
		if i%3 == 0 {
			status = "done"
		}
		est := 0
		if i%2 == 0 {
			est = 45 + i%120
		}
		logged := 0
		if i%2 == 0 {
			logged = i % 60
		}
		weight := float64(10 + i%40)
		task := &core.Task{
			Title:             fmt.Sprintf("benchmark task number %d with a reasonably long title to parse", i),
			Project:           project,
			Status:            status,
			Priority:          []string{"low", "medium", "high", "urgent"}[i%4],
			Kind:              kind,
			Created:           stamp,
			Updated:           stamp,
			EstimateMinutes:   est,
			TimeLoggedMinutes: logged,
		}
		if kind != core.KindTask && i%3 == 1 {
			task.Weight = &weight
		}
		if status == "done" {
			task.Completed = stamp
		}
		if err := s.Create(task); err != nil {
			tb.Fatal(err)
		}
	}
	return s
}

func benchModel(tb testing.TB, s *store.Store) Model {
	tb.Helper()
	cfg := dumpConfig()
	m := New(cfg, s, sync.New(s.Root))
	m.width, m.height = 120, 40
	m.view = viewCourses
	return m
}

// smallCoursesStore builds a minimal course fixture for the cache test.
func smallCoursesStore(t *testing.T, n int) *store.Store {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	ts := DumpFixedTime
	stamp := ts.Format(core.TimestampFormat)
	s, err := store.Open(t.TempDir(), filepath.Join(t.TempDir(), "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	s.Clock = clock.Fixed(ts)
	for _, p := range []string{"inbox", "college/dbms"} {
		cfg := core.DefaultProject(p)
		if p != "inbox" {
			cfg.Course = &core.Course{Code: "dbms", Name: "Database Systems", Semester: "2026-odd", Credits: 3, TargetGrade: "A"}
		}
		if err := s.SaveProject(cfg); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < n; i++ {
		status := "todo"
		if i%3 == 0 {
			status = "done"
		}
		task := &core.Task{
			Title:    fmt.Sprintf("task %d", i),
			Project:  "college/dbms",
			Status:   status,
			Priority: "medium",
			Kind:     core.KindTask,
			Created:  stamp,
			Updated:  stamp,
		}
		if status == "done" {
			task.Completed = stamp
		}
		if err := s.Create(task); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func BenchmarkComputeCourseCards2000Tasks(b *testing.B) {
	s := benchStore2000(b)
	defer func() { _ = s.Close() }()
	m := benchModel(b, s)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := computeCourseCards(m); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCollectCourseCards2000TasksCached(b *testing.B) {
	s := benchStore2000(b)
	defer func() { _ = s.Close() }()
	m := benchModel(b, s)
	if _, err := collectCourseCards(m); err != nil { // warm the cache
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cards, err := collectCourseCards(m)
		if err != nil {
			b.Fatal(err)
		}
		if len(cards) == 0 {
			b.Fatal("no cards")
		}
	}
}

// TestCollectCourseCardsCacheInvalidatesOnStoreChange uses a small fixture;
// the 2000-task scale lives in the benchmarks.
func TestCollectCourseCardsCacheInvalidatesOnStoreChange(t *testing.T) {
	s := smallCoursesStore(t, 60)
	defer func() { _ = s.Close() }()
	m := benchModel(t, s)
	first, err := collectCourseCards(m)
	if err != nil {
		t.Fatal(err)
	}
	second, err := collectCourseCards(m)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) == 0 || len(first) != len(second) {
		t.Fatalf("cards: %d then %d", len(first), len(second))
	}
	if &first[0] != &second[0] {
		t.Fatal("second call recomputed — cache not hit with no store mutation")
	}
	// A mutation bumps the generation and forces a recompute.
	if err := s.Update(s.Tasks()[0]); err != nil {
		t.Fatal(err)
	}
	third, err := collectCourseCards(m)
	if err != nil {
		t.Fatal(err)
	}
	if &third[0] == &first[0] {
		t.Fatal("store mutation did not invalidate the course-cards cache")
	}
}
