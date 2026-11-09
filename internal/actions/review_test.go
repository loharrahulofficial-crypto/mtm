package actions

import (
	"testing"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

func TestCollectReviewSections(t *testing.T) {
	s := openActionStore(t, "college/dbms")
	now := time.Date(2026, 11, 8, 14, 0, 0, 0, time.Local) // a Sunday
	stamp := now.Format(core.TimestampFormat)
	yesterday := now.AddDate(0, 0, -1)
	yesterdayStr := yesterday.Format("2006-01-02")
	tomorrow := now.AddDate(0, 0, 1).Format("2006-01-02")

	mk := func(title, status, due, completed string) {
		t.Helper()
		task := &core.Task{Title: title, Project: "college/dbms", Status: status, Kind: core.KindTask,
			Due: due, Completed: completed, Created: stamp, Updated: stamp}
		if err := s.Create(task); err != nil {
			t.Fatal(err)
		}
	}
	mk("finished yesterday", "done", "", yesterday.Format(core.TimestampFormat))
	mk("slipped dated", "todo", yesterdayStr, "")
	mk("slipped timed", "todo", now.Format("2006-01-02")+"T09:00", "") // this morning → overdue
	mk("due today", "todo", now.Format("2006-01-02"), "")              // date-only today → upcoming, not overdue
	mk("due tomorrow", "todo", tomorrow, "")
	mk("exam soon", "todo", tomorrow+"T10:00", "")                      // exam-ish row, kind irrelevant to the report
	mk("later", "todo", now.AddDate(0, 0, 30).Format("2006-01-02"), "") // outside 7-day window
	mk("done overdue-but-done", "done", yesterdayStr, yesterday.Format(core.TimestampFormat))

	rep, err := CollectReview(s, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Finished) != 2 {
		t.Errorf("Finished = %d, want 2", len(rep.Finished))
	}
	var titles []string
	for _, tk := range rep.Overdue {
		titles = append(titles, tk.Title)
	}
	if len(rep.Overdue) != 2 {
		t.Fatalf("Overdue = %v, want [slipped dated, slipped timed]", titles)
	}
	for _, tk := range rep.Upcoming {
		titles = append(titles, tk.Title)
	}
	if len(rep.Upcoming) != 3 {
		t.Errorf("Upcoming = %v, want 3 rows", titles)
	}
	for _, tk := range rep.Upcoming {
		if tk.Title == "later" {
			t.Error("the 30-days-out task is outside the next-7-days window")
		}
	}
}

func TestReviewRescheduleAndArchive(t *testing.T) {
	s := openActionStore(t, "college/dbms")
	now := time.Date(2026, 11, 8, 14, 0, 0, 0, time.Local)
	task := &core.Task{Title: "slipped", Project: "college/dbms", Status: "todo", Kind: core.KindTask,
		Due: "2026-11-01", Created: now.Format(core.TimestampFormat), Updated: now.Format(core.TimestampFormat)}
	if err := s.Create(task); err != nil {
		t.Fatal(err)
	}
	if err := ReviewReschedule(s, task, "2026-11-15"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Resolve(task.ID)
	if got.Due != "2026-11-15" {
		t.Errorf("Due = %q, want 2026-11-15", got.Due)
	}
	if err := ReviewReschedule(s, task, "not a date"); err == nil {
		t.Fatal("invalid date accepted")
	}
	if err := ReviewArchive(s, task); err != nil {
		t.Fatal(err)
	}
	for _, tk := range s.Tasks() {
		if tk.ID == task.ID {
			t.Fatal("archived task still in the default index")
		}
	}
	if _, err := s.Resolve(task.ID); err == nil {
		t.Fatal("archived task still resolvable in the default index")
	}
}
