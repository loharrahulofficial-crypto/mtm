package demo

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/dates"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
)

var dateRe = regexp.MustCompile(`20\d\d-\d\d-\d\d(T\d\d:\d\d)?`)

// TestSeedAppliesFixedClockOffsets seeds with a fixed clock and checks
// the corpus's reference-day offsets land on the injected date.
// Regression for "demo dates were absolute, not relative to now".
func TestSeedAppliesFixedClockOffsets(t *testing.T) {
	now := time.Date(2026, 12, 25, 10, 0, 0, 0, time.Local)
	dir := t.TempDir()
	if err := Seed(dir, now); err != nil {
		t.Fatal(err)
	}
	read := func(rel string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	// The DBMS midterm is @+5d@ from the reference day (2026-11-08).
	mid := read("college/dbms/ex0001-dbms-midterm.md")
	wantDue := now.AddDate(0, 0, 5).Format("2006-01-02") + "T09:00"
	if !strings.Contains(mid, "due: "+wantDue) {
		t.Errorf("DBMS midterm due = not %q:\n%s", wantDue, mid)
	}
	// created is @-1d@ (created 2026-11-07 in the corpus).
	wantCreated := now.AddDate(0, 0, -1).Format("2006-01-02")
	if !strings.Contains(mid, "created: "+wantCreated) {
		t.Errorf("DBMS midterm created = not %q:\n%s", wantCreated, mid)
	}
	// The OS quiz is @+11d@.
	quiz := read("college/os/ex0002-os-quiz.md")
	wantQuiz := now.AddDate(0, 0, 11).Format("2006-01-02") + "T10:00"
	if !strings.Contains(quiz, "due: "+wantQuiz) {
		t.Errorf("OS quiz due = not %q:\n%s", wantQuiz, quiz)
	}
}

// TestSeedShiftsEverythingWithTheClock seeds twice at different clocks and
// proves every date moves by exactly the clock delta.
func TestSeedShiftsEverythingWithTheClock(t *testing.T) {
	nowA := time.Date(2026, 11, 8, 10, 0, 0, 0, time.Local)
	nowB := nowA.AddDate(0, 0, 47)
	collect := func(now time.Time) map[string]string {
		dir := t.TempDir()
		if err := Seed(dir, now); err != nil {
			t.Fatal(err)
		}
		out := map[string]string{}
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			if info.Name() == ".mtm-data" {
				return nil // the repo marker is expected to carry wall-clock dates
			}
			rel, _ := filepath.Rel(dir, path)
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			out[rel] = string(b)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	a, b := collect(nowA), collect(nowB)
	if len(a) != len(b) {
		t.Fatalf("file count differs: %d vs %d", len(a), len(b))
	}
	for rel, contentA := range a {
		contentB := b[rel]
		datesA, datesB := dateRe.FindAllString(contentA, -1), dateRe.FindAllString(contentB, -1)
		if len(datesA) != len(datesB) {
			t.Fatalf("%s: %d dates vs %d dates", rel, len(datesA), len(datesB))
		}
		for i := range datesA {
			da, _ := time.ParseInLocation("2006-01-02", datesA[i][:10], nowA.Location())
			db, _ := time.ParseInLocation("2006-01-02", datesB[i][:10], nowB.Location())
			if days := int(db.Sub(da).Hours() / 24); days != 47 {
				t.Errorf("%s: date %q → %q (delta %d days, want 47)", rel, datesA[i], datesB[i], days)
			}
		}
	}
}

// TestSeedCoversSixteenWeeksOfCompletions proves the seeded completion
// history spans at least the last 16 weeks (heatmap never empty).
func TestSeedCoversSixteenWeeksOfCompletions(t *testing.T) {
	now := time.Date(2026, 11, 8, 10, 0, 0, 0, time.Local)
	dir := t.TempDir()
	if err := Seed(dir, now); err != nil {
		t.Fatal(err)
	}
	uStamps := []string{}
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		m := regexp.MustCompile(`(?m)^status: done$`)
		if !m.Match(b) {
			return nil
		}
		stamp := regexp.MustCompile(`(?m)^(completed|updated): (20\d\d-\d\d-\d\d)`).FindStringSubmatch(string(b))
		if stamp != nil {
			uStamps = append(uStamps, stamp[2])
		}
		return nil
	})
	if len(uStamps) < 20 {
		t.Fatalf("only %d done stamps seeded, want a rich history", len(uStamps))
	}
	sort.Strings(uStamps)
	oldest, _ := time.Parse("2006-01-02", uStamps[0])
	span := int(now.Sub(oldest).Hours() / 24)
	if span < 16*7 {
		t.Errorf("completion history spans %d days, want >= %d (16 weeks)", span, 16*7)
	}
	t.Logf("completion history spans %d days over %d done tasks", span, len(uStamps))
}

// TestSeedRoundTripStore loads the seeded demo dataset into a store and
// confirms it parses cleanly (id, due, statuses, subtask counts).
func TestSeedRoundTripStore(t *testing.T) {
	dir := t.TempDir()
	if err := Seed(filepath.Join(dir, "projects"), time.Date(2026, 11, 8, 10, 0, 0, 0, time.Local)); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(dir, filepath.Join(dir, "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	tasks := st.Tasks()
	if len(tasks) < 70 {
		t.Fatalf("store loaded %d tasks, want >= 70 (the demo dataset)", len(tasks))
	}
	for _, tk := range tasks {
		if tk.Due != "" {
			if _, ok := dates.Parse(tk.Due, time.Local); !ok {
				t.Errorf("task %s due %q does not parse", tk.ID, tk.Due)
			}
		}
	}
	projects, warnings := st.ListProjects()
	if len(warnings) > 0 {
		t.Fatalf("project parse warnings: %v", warnings)
	}
	if len(projects) != 4 {
		t.Fatalf("projects = %d, want 4 (inbox + 3 courses)", len(projects))
	}
}
