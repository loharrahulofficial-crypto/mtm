package actions

import (
	"testing"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/clock"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

func TestCollectStatsPeriodScope(t *testing.T) {
	s := openActionStore(t, "college/dbms")
	now := time.Date(2026, 11, 8, 14, 0, 0, 0, time.Local)
	inWeek := now.AddDate(0, 0, -2).Format(core.TimestampFormat)
	old := now.AddDate(0, 0, -40).Format(core.TimestampFormat)
	mk := func(title, status, stamp string, logged int) {
		t.Helper()
		task := &core.Task{Title: title, Project: "college/dbms", Status: status, Kind: core.KindTask,
			TimeLoggedMinutes: logged, Created: inWeek, Updated: inWeek, Completed: stamp}
		if err := s.Create(task); err != nil {
			t.Fatal(err)
		}
	}
	mk("done in week", "done", inWeek, 60)
	mk("done old", "done", old, 45)
	mk("not done", "todo", "", 30)

	since := now.AddDate(0, 0, -6)
	rep, err := CollectStats(s, now, since, "week", 15)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Completed != 1 {
		t.Errorf("Completed = %d, want 1 (period scope)", rep.Completed)
	}
	if got := rep.DayCompletions[len(rep.DayCompletions)-3]; got != 1 {
		t.Errorf("DayCompletions[-3] = %d, want 1 (two days ago)", got)
	}
	if rep.TotalHours() != 1.0 {
		t.Errorf("TotalHours = %v, want 1.0", rep.TotalHours())
	}
	if rep.BusiestCount != 1 || rep.BusiestDay.Format("2006-01-02") != now.AddDate(0, 0, -2).Format("2006-01-02") {
		t.Errorf("BusiestDay = %v (%d), want %s (1)", rep.BusiestDay, rep.BusiestCount, inWeek)
	}
}

func TestCollectStatsOnTime(t *testing.T) {
	s := openActionStore(t, "college/dbms")
	now := time.Date(2026, 11, 8, 14, 0, 0, 0, time.Local)
	stamp := now.Format(core.TimestampFormat)
	mk := func(title string, due, completed string) {
		t.Helper()
		task := &core.Task{Title: title, Project: "college/dbms", Status: "done", Kind: core.KindAssignment,
			Due: due, Completed: completed, Created: completed, Updated: completed}
		if err := s.Create(task); err != nil {
			t.Fatal(err)
		}
	}
	day := now.Format("2006-01-02")
	mk("on time dated", day, stamp)                                     // due today, done now → ok
	mk("late dated", now.AddDate(0, 0, -1).Format("2006-01-02"), stamp) // due yesterday → late
	mk("on time timed", day+"T15:00", stamp)                            // due 15:00, done 14:00 → ok
	mk("late timed", day+"T13:00", stamp)                               // due 13:00, done 14:00 → late
	mk("no due", "", stamp)                                             // excluded from rate

	since := now.AddDate(0, 0, -6)
	rep, err := CollectStats(s, now, since, "week", 15)
	if err != nil {
		t.Fatal(err)
	}
	if rep.WithDue != 4 {
		t.Errorf("WithDue = %d, want 4", rep.WithDue)
	}
	if rep.OnTime != 2 {
		t.Errorf("OnTime = %d, want 2", rep.OnTime)
	}
	if got := rep.OnTimePct(); got != 50 {
		t.Errorf("OnTimePct = %v, want 50", got)
	}
}

func TestCollectStatsEstimateAndOtherBucket(t *testing.T) {
	s := openActionStore(t, "college/dbms")
	if err := s.SaveProject(&core.Project{Name: "other-proj", Statuses: []core.ProjectStatus{
		{Name: "todo"}, {Name: "done", Done: true},
	}}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 8, 14, 0, 0, 0, time.Local)
	stamp := now.Format(core.TimestampFormat)
	mk := func(title string, project string, logged, est int) {
		t.Helper()
		task := &core.Task{Title: title, Project: project, Status: "done", Kind: core.KindAssignment,
			TimeLoggedMinutes: logged, EstimateMinutes: est, Completed: stamp, Created: stamp, Updated: stamp}
		if err := s.Create(task); err != nil {
			t.Fatal(err)
		}
	}
	mk("dbms one", "college/dbms", 60, 60)
	mk("dbms two", "college/dbms", 60, 0)
	mk("other one", "other-proj", 30, 30)
	mk("other two", "other-proj", 30, 0)
	mk("dbms three", "college/dbms", 120, 120)
	mk("dbms four", "college/dbms", 120, 120)

	since := now.AddDate(0, 0, -6)
	rep, err := CollectStats(s, now, since, "week", 15)
	if err != nil {
		t.Fatal(err)
	}
	if rep.OtherCompleted != 2 {
		t.Errorf("OtherCompleted = %d, want 2", rep.OtherCompleted)
	}
	if rep.OtherHours != 1.0 {
		t.Errorf("OtherHours = %v, want 1.0", rep.OtherHours)
	}
	// Global accuracy: estimated tasks = dbms one + dbms three + dbms four
	// + other one = 4 of 6 completed; logged 330 of est 330 → 100%.
	if rep.Estimate.Estimated != 4 || rep.Estimate.Total != 6 {
		t.Errorf("global estimate = %d/%d, want 4/6", rep.Estimate.Estimated, rep.Estimate.Total)
	}
	if rep.Estimate.Pct() != 100 {
		t.Errorf("global estimate pct = %v, want 100", rep.Estimate.Pct())
	}
	byCourse := map[string]StatsCourse{}
	for _, c := range rep.Courses {
		byCourse[c.Course] = c
	}
	d := byCourse["college/dbms"]
	if d.Completed != 4 || d.Estimate.Estimated != 3 {
		t.Errorf("dbms = %+v, want 4 completed, 3 estimated", d)
	}
	if !d.Estimate.Enough {
		t.Error("dbms Estimate.Enough = false with 3 estimated tasks, want true")
	}
}

func streakFixture(t *testing.T, daysDone []int, now time.Time) (streaks StreakReport) {
	t.Helper()
	s := openActionStore(t, "college/dbms")
	for _, off := range daysDone {
		stamp := now.AddDate(0, 0, -off).Format(core.TimestampFormat)
		task := &core.Task{Title: "done item", Project: "college/dbms", Status: "done", Kind: core.KindTask,
			Completed: stamp, Created: stamp, Updated: stamp}
		if err := s.Create(task); err != nil {
			t.Fatal(err)
		}
	}
	return Streaks(s, 15, now)
}

func TestStreaksCurrentAndLongest(t *testing.T) {
	now := time.Date(2026, 11, 8, 14, 0, 0, 0, time.Local)
	// Active yesterday and the two days before; nothing today; longest run 4.
	s := streakFixture(t, []int{1, 2, 3, 10, 11}, now)
	if s.Current != 3 {
		t.Errorf("Current = %d, want 3 (yesterday-anchored)", s.Current)
	}
	if s.Longest != 3 {
		t.Errorf("Longest = %d, want 3", s.Longest)
	}
	// Today active too: current counts today.
	s = streakFixture(t, []int{0, 1, 2, 10, 11, 12, 13}, now)
	if s.Current != 3 {
		t.Errorf("Current = %d, want 3", s.Current)
	}
	if s.Longest != 4 {
		t.Errorf("Longest = %d, want 4", s.Longest)
	}
	// Nothing active recently: current 0 even if history had a long run.
	s = streakFixture(t, []int{10, 11, 12, 13, 14}, now)
	if s.Current != 0 {
		t.Errorf("Current = %d, want 0", s.Current)
	}
	if s.Longest != 5 {
		t.Errorf("Longest = %d, want 5", s.Longest)
	}
	// Longest spans multiple runs.
	s = streakFixture(t, []int{0, 1, 5, 6, 7, 8, 9}, now)
	if s.Current != 2 {
		t.Errorf("Current = %d, want 2", s.Current)
	}
	if s.Longest != 5 {
		t.Errorf("Longest = %d, want 5", s.Longest)
	}
}

func TestStreaksMinutesAloneKeepDayActive(t *testing.T) {
	s := openActionStore(t, "college/dbms")
	now := time.Date(2026, 11, 8, 14, 0, 0, 0, time.Local)
	yesterday := now.AddDate(0, 0, -1)
	// Pin the clock for task creation: the store stamps Updated at
	// create time, so the wip tasks last worked yesterday.
	s.Clock = clock.Fixed(yesterday)
	mk := func(title string, logged int) {
		t.Helper()
		task := &core.Task{Title: title, Project: "college/dbms", Status: "todo", Kind: core.KindStudy,
			TimeLoggedMinutes: logged}
		if err := s.Create(task); err != nil {
			t.Fatal(err)
		}
	}
	mk("wip one", 20)
	mk("wip two", 10)
	// The day before: only 5 min < 15 minimum → inactive.
	s.Clock = clock.Fixed(now.AddDate(0, 0, -2))
	little := &core.Task{Title: "little work", Project: "college/dbms", Status: "todo", Kind: core.KindStudy,
		TimeLoggedMinutes: 5}
	if err := s.Create(little); err != nil {
		t.Fatal(err)
	}
	s.Clock = clock.Fixed(now)
	s2 := Streaks(s, 15, now)
	if s2.Current != 1 {
		t.Errorf("Current = %d, want 1 (minutes-only yesterday)", s2.Current)
	}
	s3 := Streaks(s, 0, now) // minutes disabled
	if s3.Current != 0 {
		t.Errorf("Current = %d with minutes disabled, want 0", s3.Current)
	}
}

func TestStreaksCompletionWithNoMinutes(t *testing.T) {
	s := openActionStore(t, "college/dbms")
	now := time.Date(2026, 11, 8, 14, 0, 0, 0, time.Local)
	yesterday := now.AddDate(0, 0, -1).Format(core.TimestampFormat)
	task := &core.Task{Title: "done no time logged", Project: "college/dbms", Status: "done", Kind: core.KindTask,
		Completed: yesterday, Created: yesterday, Updated: yesterday}
	if err := s.Create(task); err != nil {
		t.Fatal(err)
	}
	s2 := Streaks(s, 15, now)
	if s2.Current != 1 {
		t.Errorf("Current = %d, want 1 (a completion alone makes the day active)", s2.Current)
	}
}
