// Package dates is the single shared formatter for dates shown in the UI.
// Files keep ISO (core.TimestampFormat / "2006-01-02"); everything the user
// sees goes through here — absolute ("Mon 2 Nov, 10:00") or relative
// ("done 3d ago") where useful.
package dates

import (
	"strings"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

// CalDays returns whole calendar days from from's date to to's date.
// Delegates to core.CalDays (exact across DST transitions).
func CalDays(from, to time.Time) int {
	return core.CalDays(from, to)
}

// Parse reads a stored timestamp in any of the layouts the store writes.
func Parse(s string, loc *time.Location) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{core.TimestampFormat, "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// Human renders a moment as "Mon 2 Nov, 10:00" — or date-only ("Mon 2 Nov")
// when the value carries no clock time (midnight AND input was date-only
// cannot be distinguished here, so midnight stays date-only by convention).
func Human(t time.Time) string {
	if t.Hour() == 0 && t.Minute() == 0 {
		return HumanDay(t)
	}
	return t.Format("Mon 2 Jan, 15:04")
}

// HumanDay renders the day alone: "Mon 2 Nov".
func HumanDay(t time.Time) string {
	return t.Format("Mon 2 Jan")
}

// HumanStamp renders a stored timestamp for display; falls back to the raw
// value if it doesn't parse (never hide data).
func HumanStamp(s string, loc *time.Location) string {
	if t, ok := Parse(s, loc); ok {
		return Human(t)
	}
	return s
}

// HumanDayStamp is HumanStamp for a date column that should not grow a time.
func HumanDayStamp(s string, loc *time.Location) string {
	if t, ok := Parse(s, loc); ok {
		return HumanDay(t)
	}
	return s
}

// DaysAgo counts whole days from t to now, floor by calendar day.
func DaysAgo(now, t time.Time) int {
	return CalDays(t, now)
}

// RelDay renders "today", "1d ago" or "3d ago".
func RelDay(now, t time.Time) string {
	d := DaysAgo(now, t)
	switch {
	case d <= 0:
		return "today"
	case d == 1:
		return "1d ago"
	default:
		return itoa(d) + "d ago"
	}
}

// DoneLabel renders the muted completion marker for done tasks:
// "done today", "done 3d ago". Empty when the completion stamp doesn't parse.
func DoneLabel(completed string, now time.Time) string {
	t, ok := Parse(completed, now.Location())
	if !ok {
		return ""
	}
	return "done " + RelDay(now, t)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}
