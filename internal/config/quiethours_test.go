package config

import (
	"testing"
	"time"
)

func TestParseQuietHours(t *testing.T) {
	for _, tc := range []struct {
		in       string
		wantErr  bool
		startMin int
		endMin   int
	}{
		{"", false, 0, 0},
		{"22:00-07:00", false, 22 * 60, 7 * 60},
		{"09:30-17:45", false, 9*60 + 30, 17*60 + 45},
		{"07:00-07:00", false, 7 * 60, 7 * 60},
		{"25:00-08:00", true, 0, 0},
		{"22:00-08", true, 0, 0},
		{"22-07", true, 0, 0},
		{"22:00-07:99", true, 0, 0},
		{"abc-def", true, 0, 0},
	} {
		w, err := ParseQuietHours(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseQuietHours(%q) accepted, want error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseQuietHours(%q) err = %v", tc.in, err)
			continue
		}
		if tc.in == "" {
			continue // zero window is the disabled sentinel
		}
		if got := int(w.Start.Minutes()); got != tc.startMin {
			t.Errorf("ParseQuietHours(%q).Start = %dm, want %dm", tc.in, got, tc.startMin)
		}
		if got := int(w.End.Minutes()); got != tc.endMin {
			t.Errorf("ParseQuietHours(%q).End = %dm, want %dm", tc.in, got, tc.endMin)
		}
	}
}

func TestQuietHoursInside(t *testing.T) {
	w, err := ParseQuietHours("22:00-07:00")
	if err != nil {
		t.Fatal(err)
	}
	at := func(hh, mm int) time.Time {
		return time.Date(2026, 11, 9, hh, mm, 0, 0, time.Local)
	}
	for _, tc := range []struct {
		now  time.Time
		want bool
	}{
		{at(23, 0), true},
		{at(2, 30), true},
		{at(6, 59), true},
		{at(7, 0), false},
		{at(7, 1), false},
		{at(12, 0), false},
		{at(21, 59), false},
		{at(22, 0), true},
	} {
		if got := w.InQuietHours(tc.now); got != tc.want {
			t.Errorf("InQuietHours(%s) = %v, want %v", tc.now.Format("15:04"), got, tc.want)
		}
	}
}

func TestQuietHoursNonMidnightWindow(t *testing.T) {
	w, err := ParseQuietHours("09:00-17:00")
	if err != nil {
		t.Fatal(err)
	}
	at := func(hh, mm int) time.Time {
		return time.Date(2026, 11, 9, hh, mm, 0, 0, time.Local)
	}
	for _, tc := range []struct {
		now  time.Time
		want bool
	}{
		{at(9, 0), true},
		{at(12, 0), true},
		{at(16, 59), true},
		{at(17, 0), false},
		{at(8, 59), false},
	} {
		if got := w.InQuietHours(tc.now); got != tc.want {
			t.Errorf("InQuietHours(%s) = %v, want %v", tc.now.Format("15:04"), got, tc.want)
		}
	}
}

func TestQuietHoursZeroWindowDisabled(t *testing.T) {
	disabled, err := ParseQuietHours("")
	if err != nil {
		t.Fatal(err)
	}
	zero, err := ParseQuietHours("00:00-00:00")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.Local)
	if disabled.InQuietHours(now) || zero.InQuietHours(now) {
		t.Error("a zero window must read as disabled")
	}
}
