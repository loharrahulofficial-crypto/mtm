package dates

import (
	"testing"
	"time"
)

func TestHuman(t *testing.T) {
	cases := []struct {
		in   time.Time
		want string
	}{
		{time.Date(2026, 11, 2, 10, 0, 0, 0, time.Local), "Mon 2 Nov, 10:00"},
		{time.Date(2026, 11, 2, 0, 0, 0, 0, time.Local), "Mon 2 Nov"},
		{time.Date(2026, 12, 25, 9, 5, 0, 0, time.Local), "Fri 25 Dec, 09:05"},
	}
	for _, c := range cases {
		if got := Human(c.in); got != c.want {
			t.Errorf("Human(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestHumanStamp(t *testing.T) {
	loc := time.Local
	if got := HumanStamp("2026-11-02T10:00", loc); got != "Mon 2 Nov, 10:00" {
		t.Errorf("HumanStamp with time = %q", got)
	}
	if got := HumanStamp("2026-11-02", loc); got != "Mon 2 Nov" {
		t.Errorf("HumanStamp date-only = %q", got)
	}
	// Garbage passes through — never hide stored data.
	if got := HumanStamp("not a date", loc); got != "not a date" {
		t.Errorf("HumanStamp garbage = %q", got)
	}
}

func TestHumanDayStamp(t *testing.T) {
	if got := HumanDayStamp("2026-11-02T18:30", time.Local); got != "Mon 2 Nov" {
		t.Errorf("HumanDayStamp = %q", got)
	}
}

func TestDoneLabel(t *testing.T) {
	now := time.Date(2026, 11, 8, 14, 32, 0, 0, time.Local)
	cases := []struct{ stamp, want string }{
		{"2026-11-08T09:00", "done today"},
		{"2026-11-07T23:00", "done 1d ago"},
		{"2026-11-05T12:00", "done 3d ago"},
		{"", ""},
		{"garbage", ""},
	}
	for _, c := range cases {
		if got := DoneLabel(c.stamp, now); got != c.want {
			t.Errorf("DoneLabel(%q) = %q, want %q", c.stamp, got, c.want)
		}
	}
}

func TestRelDay(t *testing.T) {
	now := time.Date(2026, 11, 8, 0, 0, 0, 0, time.Local)
	if got := RelDay(now, now); got != "today" {
		t.Errorf("RelDay(now, now) = %q", got)
	}
	if got := RelDay(now, now.AddDate(0, 0, -1)); got != "1d ago" {
		t.Errorf("RelDay(-1d) = %q", got)
	}
	if got := RelDay(now, now.AddDate(0, 0, -9)); got != "9d ago" {
		t.Errorf("RelDay(-9d) = %q", got)
	}
}
