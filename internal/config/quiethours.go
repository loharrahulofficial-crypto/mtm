package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// QuietHoursWindow is a parsed reminders.quiet_hours window ("22:00-07:00").
type QuietHoursWindow struct {
	Start time.Duration // minutes since midnight
	End   time.Duration
}

// ParseQuietHours parses "HH:MM-HH:MM". Windows may span midnight
// (22:00-07:00). An empty string returns (zero, nil) — quiet hours disabled.
func ParseQuietHours(s string) (QuietHoursWindow, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return QuietHoursWindow{}, nil
	}
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return QuietHoursWindow{}, fmt.Errorf("want HH:MM-HH:MM, got %q", s)
	}
	start, err := parseHHMM(parts[0])
	if err != nil {
		return QuietHoursWindow{}, fmt.Errorf("start: %v", err)
	}
	end, err := parseHHMM(parts[1])
	if err != nil {
		return QuietHoursWindow{}, fmt.Errorf("end: %v", err)
	}
	return QuietHoursWindow{Start: start, End: end}, nil
}

func parseHHMM(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("want HH:MM, got %q", s)
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 0, fmt.Errorf("hour %q out of range", parts[0])
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, fmt.Errorf("minute %q out of range", parts[1])
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute, nil
}

// InQuietHours reports whether now falls inside w. Windows where End <=
// Start span midnight. A zero window (disabled) is never inside.
func (w QuietHoursWindow) InQuietHours(now time.Time) bool {
	if w.Start == 0 && w.End == 0 {
		return false // disabled — treat 00:00-00:00 as disabled
	}
	minuteOfDay := time.Duration(now.Hour())*time.Hour + time.Duration(now.Minute())*time.Minute
	if w.End > w.Start {
		return minuteOfDay >= w.Start && minuteOfDay < w.End
	}
	return minuteOfDay >= w.Start || minuteOfDay < w.End
}
