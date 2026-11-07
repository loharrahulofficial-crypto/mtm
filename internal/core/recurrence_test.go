package core

import (
	"testing"
	"time"
)

func TestNextOccurrence(t *testing.T) {
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.Local)
	mk := func(y int, m time.Month, d, h, mi int) time.Time {
		return time.Date(y, m, d, h, mi, 0, 0, time.Local)
	}
	cases := []struct {
		name  string
		recur string
		from  time.Time
		want  time.Time
	}{
		{"daily from yesterday keeps same clock time", "daily", mk(2026, 11, 6, 23, 59), mk(2026, 11, 7, 23, 59)},
		{"daily same time today", "daily", mk(2026, 11, 7, 23, 59), mk(2026, 11, 8, 23, 59)},
		{"daily skips overdue backlog", "daily", mk(2026, 11, 1, 9, 0), mk(2026, 11, 8, 9, 0)},
		{"weekly default 7d", "weekly", mk(2026, 11, 1, 9, 0), mk(2026, 11, 8, 9, 0)},
		{"weekly pinned monday", "weekly:mon", mk(2026, 10, 26, 9, 0), mk(2026, 11, 9, 9, 0)},
		{"weekly pinned friday", "weekly:fri", mk(2026, 11, 6, 18, 0), mk(2026, 11, 13, 18, 0)},
		{"every 2 weeks", "every 2 weeks", mk(2026, 10, 31, 10, 0), mk(2026, 11, 14, 10, 0)},
		{"every 3 days", "every 3 days", mk(2026, 11, 5, 10, 0), mk(2026, 11, 8, 10, 0)},
		{"every 10 days big step", "every 10 days", mk(2026, 10, 28, 10, 0), mk(2026, 11, 17, 10, 0)},
		{"zero from anchors to now", "daily", time.Time{}, mk(2026, 11, 8, 10, 0)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := NextOccurrence(c.recur, c.from, now)
			if err != nil {
				t.Fatal(err)
			}
			if !got.Equal(c.want) {
				t.Errorf("got %s, want %s", got.Format(TimeFormat), c.want.Format(TimeFormat))
			}
		})
	}
}

func TestNextOccurrenceErrors(t *testing.T) {
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.Local)
	for _, recur := range []string{"", "fortnightly", "every 0 weeks", "every banana", "weekly:nope"} {
		if _, err := NextOccurrence(recur, time.Time{}, now); err == nil {
			t.Errorf("recurrence %q: expected error", recur)
		}
	}
}

func TestSpawnRecurringOccurrence(t *testing.T) {
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.Local)
	fmtFloat := func(w float64) *float64 { return &w }
	origin := &Task{
		Title:             "Water plants",
		Project:           "home",
		Status:            "done",
		Priority:          "high",
		Tags:              []string{"plant"},
		Due:               "2026-11-07T09:00",
		Recurrence:        "daily",
		EstimateMinutes:   10,
		Kind:              KindTask,
		Weight:            fmtFloat(20),
		Score:             fmtFloat(18),
		Completed:         "2026-11-07T10:00",
		TimeLoggedMinutes: 35,
	}
	spawned, ok, err := SpawnRecurringOccurrence(origin, "todo", now)
	if err != nil || !ok {
		t.Fatalf("spawn failed: ok=%v err=%v", ok, err)
	}
	if spawned.Title != "Water plants" || spawned.Status != "todo" {
		t.Errorf("spawned task wrong: %+v", spawned)
	}
	if spawned.Due != "2026-11-08T09:00" {
		t.Errorf("due = %q, want 2026-11-08T09:00", spawned.Due)
	}
	if spawned.TimeLoggedMinutes != 0 || spawned.Completed != "" || spawned.Score != nil {
		t.Errorf("spawned task kept completion state: %+v", spawned)
	}
	if spawned.Weight == nil || *spawned.Weight != 20 {
		t.Errorf("weight not carried: %v", spawned.Weight)
	}
	if len(spawned.Tags) != 1 || spawned.Tags[0] != "plant" {
		t.Errorf("tags not carried: %v", spawned.Tags)
	}
	// Date-only tasks keep date-only style.
	origin.Due = "2026-11-07"
	spawned, ok, err = SpawnRecurringOccurrence(origin, "todo", now)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if spawned.Due != "2026-11-08" {
		t.Errorf("date-only due = %q, want 2026-11-08", spawned.Due)
	}
	// No recurrence → not ok.
	origin.Recurrence = ""
	if _, ok, err := SpawnRecurringOccurrence(origin, "todo", now); ok || err != nil {
		t.Errorf("no recurrence: ok=%v err=%v, want false nil", ok, err)
	}
}
