package actions

import (
	"testing"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

func TestFormatMinutes(t *testing.T) {
	for in, want := range map[int]string{0: "0m", 45: "45m", 60: "1h", 90: "1h 30m", 600: "10h", -5: "0m"} {
		if got := FormatMinutes(in); got != want {
			t.Errorf("FormatMinutes(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestCollectHours(t *testing.T) {
	s := openActionStore(t, "college/dbms")
	if err := s.SaveProject(&core.Project{Name: "college/os", Statuses: []core.ProjectStatus{{Name: "todo"}, {Name: "done", Done: true}}, Course: &core.Course{Code: "OS", Name: "Operating Systems"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveProject(&core.Project{Name: "other-proj", Statuses: []core.ProjectStatus{{Name: "todo"}, {Name: "done", Done: true}}}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 8, 14, 0, 0, 0, time.Local)
	weekStamp := now.AddDate(0, 0, -2).Format(core.TimestampFormat)
	oldStamp := now.AddDate(0, 0, -30).Format(core.TimestampFormat)
	mk := func(title, project string, logged, est int, stamp string) {
		t.Helper()
		task := &core.Task{
			Title:             title,
			Project:           project,
			Status:            "todo",
			Kind:              core.KindStudy,
			TimeLoggedMinutes: logged,
			EstimateMinutes:   est,
			Completed:         stamp,
		}
		if err := s.Create(task); err != nil {
			t.Fatal(err)
		}
	}
	mk("recent study", "college/dbms", 90, 120, weekStamp)
	mk("old study", "college/dbms", 45, 60, oldStamp)
	mk("os study", "college/os", 30, 0, weekStamp)
	mk("unattributed study", "college/os", 20, 0, "") // no completion stamp → counts toward all-time only
	mk("other project", "other-proj", 25, 10, weekStamp)

	rep, err := CollectHours(s, now)
	if err != nil {
		t.Fatal(err)
	}
	byCourse := map[string]HoursRollup{}
	for _, r := range rep.Courses {
		byCourse[r.Course] = r
	}
	d := byCourse["college/dbms"]
	if d.LoggedMinutes != 135 || d.WeekMinutes != 90 || d.EstimateMinutes != 180 {
		t.Errorf("dbms = %+v", d)
	}
	osR := byCourse["college/os"]
	if osR.LoggedMinutes != 50 || osR.WeekMinutes != 30 {
		t.Errorf("os = %+v", osR)
	}
	if rep.OtherLoggedMinutes != 25 || rep.OtherWeekMinutes != 25 {
		t.Errorf("other = %d/%d", rep.OtherLoggedMinutes, rep.OtherWeekMinutes)
	}
	// Courses are sorted by logged time descending.
	if rep.Courses[0].LoggedMinutes < rep.Courses[1].LoggedMinutes {
		t.Errorf("courses not sorted by logged minutes: %+v", rep.Courses)
	}
	// Tasks with time are listed, most minutes first.
	if len(d.Tasks) != 2 || d.Tasks[0].TimeLoggedMinutes < d.Tasks[1].TimeLoggedMinutes {
		t.Errorf("dbms tasks not sorted: %+v", d.Tasks)
	}
}
