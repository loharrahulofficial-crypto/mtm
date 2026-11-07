package parse

import (
	"testing"
	"time"
)

// fixedNow is a Saturday: 2026-11-07 10:00 local.
var fixedNow = time.Date(2026, 11, 7, 10, 0, 0, 0, time.Local)

func TestQuickAdd(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  Result
	}{
		{"plain title", "buy milk", Result{Title: "buy milk"}},
		{"empty input", "", Result{}},
		{"multi-space", "  buy   milk  ", Result{Title: "buy milk"}},

		// dates
		{"friday date", "submit report friday", Result{Title: "submit report", Due: "2026-11-13"}},
		{"friday datetime", "submit report friday 6pm", Result{Title: "submit report", Due: "2026-11-13T18:00"}},
		{"weekday abbrev", "submit fri 5pm", Result{Title: "submit", Due: "2026-11-13T17:00"}},
		{"tomorrow date", "call mum tomorrow", Result{Title: "call mum", Due: "2026-11-08"}},
		{"tomorrow datetime", "call mum tomorrow 6pm", Result{Title: "call mum", Due: "2026-11-08T18:00"}},
		{"today datetime future", "meet today 6pm", Result{Title: "meet", Due: "2026-11-07T18:00"}},
		{"today datetime past", "meeting today 8am", Result{Title: "meeting", Due: "2026-11-08T08:00"}},
		{"tonight", "study tonight", Result{Title: "study", Due: "2026-11-07T20:00"}},
		{"bare time future today", "standup 5pm", Result{Title: "standup", Due: "2026-11-07T17:00"}},
		{"bare time past today", "standup 9am", Result{Title: "standup", Due: "2026-11-08T09:00"}},
		{"12pm", "12pm lunch", Result{Title: "lunch", Due: "2026-11-07T12:00"}},
		{"12am past", "12am review", Result{Title: "review", Due: "2026-11-08T00:00"}},
		{"at time", "call at 3pm", Result{Title: "call", Due: "2026-11-07T15:00"}},
		{"in 3 days", "exam in 3 days", Result{Title: "exam", Due: "2026-11-10"}},
		{"in 2 weeks", "in 2 weeks review", Result{Title: "review", Due: "2026-11-21"}},
		{"in 2 months", "in 2 months taxes", Result{Title: "taxes", Due: "2027-01-07"}},
		{"14 nov past", "prep 3 nov", Result{Title: "prep", Due: "2027-11-03"}},
		{"14 nov future", "pay fees 30 nov", Result{Title: "pay fees", Due: "2026-11-30"}},
		{"day month", "14 nov pay fees", Result{Title: "pay fees", Due: "2026-11-14"}},
		{"day month year", "14 november 2027", Result{Title: "", Due: "2027-11-14"}},
		{"day month time", "14 nov 6pm review", Result{Title: "review", Due: "2026-11-14T18:00"}},
		{"iso date", "review 2026-11-14", Result{Title: "review", Due: "2026-11-14"}},
		{"iso datetime", "review 2026-11-14T15:30", Result{Title: "review", Due: "2026-11-14T15:30"}},
		{"next friday", "buy friday plan", Result{Title: "buy plan", Due: "2026-11-13"}},
		{"next friday literal", "buy next friday", Result{Title: "buy", Due: "2026-11-20"}},
		{"next monday", "standup next monday", Result{Title: "standup", Due: "2026-11-16"}},
		{"next monday with time", "standup next monday 9am", Result{Title: "standup", Due: "2026-11-16T09:00"}},

		// priorities
		{"priority high", "!high submit taxes", Result{Title: "submit taxes", Priority: "high"}},
		{"priority med", "!med submit taxes", Result{Title: "submit taxes", Priority: "medium"}},
		{"priority urgent", "!urgent submit taxes", Result{Title: "submit taxes", Priority: "urgent"}},
		{"priority low", "!low submit taxes", Result{Title: "submit taxes", Priority: "low"}},
		{"priority med and urgent last wins", "!med !urgent x", Result{Title: "x", Priority: "urgent"}},

		// tags
		{"one tag", "#home cleaning", Result{Title: "cleaning", Tags: []string{"home"}}},
		{"two tags", "#home #urgent chores", Result{Title: "chores", Tags: []string{"home", "urgent"}}},
		{"duplicate tags deduped", "#a #a #b x", Result{Title: "x", Tags: []string{"a", "b"}}},
		{"tag with dash underscore", "#valid-tag_x crash", Result{Title: "crash", Tags: []string{"valid-tag_x"}}},

		// projects
		{"project", "read ch3 @college/dbms", Result{Title: "read ch3", Project: "college/dbms"}},
		{"project last wins", "@a @b x", Result{Title: "x", Project: "b"}},

		// estimates
		{"estimate hours", "~2h write report", Result{Title: "write report", EstimateMinutes: 120}},
		{"estimate minutes", "~45m tidy desk", Result{Title: "tidy desk", EstimateMinutes: 45}},
		{"estimate fractional hours", "~1.5h deep work", Result{Title: "deep work", EstimateMinutes: 90}},
		{"estimate unrecognized stays title", "~2x milk", Result{Title: "~2x milk"}},

		// recurrence
		{"daily word", "gym daily", Result{Title: "gym", Recurrence: "daily"}},
		{"every day", "water plants every day", Result{Title: "water plants", Recurrence: "daily"}},
		{"every 1 day", "water plants every 1 day", Result{Title: "water plants", Recurrence: "daily"}},
		{"weekly word", "report weekly", Result{Title: "report", Recurrence: "weekly"}},
		{"every week", "gym every week", Result{Title: "gym", Recurrence: "weekly"}},
		{"every monday", "team sync every monday", Result{Title: "team sync", Recurrence: "weekly:mon"}},
		{"every friday", "squash every friday", Result{Title: "squash", Recurrence: "weekly:fri"}},
		{"weekly on monday", "team sync weekly on monday", Result{Title: "team sync", Recurrence: "weekly:mon"}},
		{"every 2 weeks", "pay rent every 2 weeks", Result{Title: "pay rent", Recurrence: "every 2 weeks"}},
		{"every 3 days", "meds every 3 days", Result{Title: "meds", Recurrence: "every 3 days"}},

		// combined
		{"full mix", "submit essay friday 6pm !high #exam @college/dbms ~2h",
			Result{Title: "submit essay", Due: "2026-11-13T18:00", Priority: "high", Tags: []string{"exam"}, Project: "college/dbms", EstimateMinutes: 120}},
		{"combined recurrence", "team standup monday 9am every week @work !med #team",
			Result{Title: "team standup", Due: "2026-11-09T09:00", Recurrence: "weekly", Priority: "medium", Tags: []string{"team"}, Project: "work"}},
		{"all markers", "in 3 days @proj !low ~10m do thing", Result{Title: "do thing", Due: "2026-11-10", Priority: "low", Project: "proj", EstimateMinutes: 10}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := QuickAdd(c.input, fixedNow)
			if got.Title != c.want.Title ||
				got.Due != c.want.Due ||
				got.Priority != c.want.Priority ||
				got.Recurrence != c.want.Recurrence ||
				got.Project != c.want.Project ||
				got.EstimateMinutes != c.want.EstimateMinutes ||
				len(got.Tags) != len(c.want.Tags) {
				if got.Title != c.want.Title {
					t.Errorf("title: got %q, want %q", got.Title, c.want.Title)
				}
				if got.Due != c.want.Due {
					t.Errorf("due: got %q, want %q", got.Due, c.want.Due)
				}
				if got.Priority != c.want.Priority {
					t.Errorf("priority: got %q, want %q", got.Priority, c.want.Priority)
				}
				if got.Recurrence != c.want.Recurrence {
					t.Errorf("recurrence: got %q, want %q", got.Recurrence, c.want.Recurrence)
				}
				if got.Project != c.want.Project {
					t.Errorf("project: got %q, want %q", got.Project, c.want.Project)
				}
				if got.EstimateMinutes != c.want.EstimateMinutes {
					t.Errorf("estimate: got %d, want %d", got.EstimateMinutes, c.want.EstimateMinutes)
				}
				if len(got.Tags) != len(c.want.Tags) {
					t.Errorf("tags: got %v, want %v", got.Tags, c.want.Tags)
					return
				}
				return
			}
			for i := range got.Tags {
				if got.Tags[i] != c.want.Tags[i] {
					t.Errorf("tags: got %v, want %v", got.Tags, c.want.Tags)
					break
				}
			}
		})
	}
}
