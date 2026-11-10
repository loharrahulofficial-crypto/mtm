// Package parse turns quick-add strings like
//
//	submit report friday 6pm !high #college @dbms ~2h every week
//
// into structured tasks. Flags always win over parsed values; unrecognized
// tokens are kept as part of the title.
package parse

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/text"
)

// Result is the parsed quick-add input. Due is either date-only
// ("2006-01-02") or a local-naive datetime ("2006-01-02T15:04").
type Result struct {
	Title           string
	Due             string
	Recurrence      string
	Priority        string
	Tags            []string
	Project         string
	EstimateMinutes int
}

var (
	nameToken   = regexp.MustCompile(`^[a-z0-9][a-z0-9/_.-]*$`)
	clockTimeRe = regexp.MustCompile(`^(0?[0-9]|1[0-9]|2[0-3]):([0-5][0-9])$`)
	amPmTimeRe  = regexp.MustCompile(`^([1-9]|1[0-2])(:([0-5][0-9]))?(am|pm)$`)
	estRe       = regexp.MustCompile(`^~(\d+(?:\.\d+)?)(h|hr|hrs|hour|hours|m|min|mins|minutes)$`)

	weekdayNames = map[string]time.Weekday{
		"sun": time.Sunday, "sunday": time.Sunday,
		"mon": time.Monday, "monday": time.Monday,
		"tue": time.Tuesday, "tues": time.Tuesday, "tuesday": time.Tuesday,
		"wed": time.Wednesday, "wednesday": time.Wednesday,
		"thu": time.Thursday, "thur": time.Thursday, "thurs": time.Thursday, "thursday": time.Thursday,
		"fri": time.Friday, "friday": time.Friday,
		"sat": time.Saturday, "saturday": time.Saturday,
	}

	monthNames = map[string]time.Month{
		"jan": time.January, "january": time.January,
		"feb": time.February, "february": time.February,
		"mar": time.March, "march": time.March,
		"apr": time.April, "april": time.April,
		"may": time.May,
		"jun": time.June, "june": time.June,
		"jul": time.July, "july": time.July,
		"aug": time.August, "august": time.August,
		"sep": time.September, "sept": time.September, "september": time.September,
		"oct": time.October, "october": time.October,
		"nov": time.November, "november": time.November,
		"dec": time.December, "december": time.December,
	}
)

type bareTime struct{ hour, minute int }

// QuickAdd parses input. now injects "today" (tests pass a fixed instant).
// Title may be empty — the caller decides how to handle that.
func QuickAdd(input string, now time.Time) Result {
	tokens := strings.Fields(input)
	r := Result{}
	var title []string
	bareTimeHour, bareTimeMinute, bareTimeSet := 0, 0, false
	i := 0
	for i < len(tokens) {
		tok := tokens[i]
		lower := strings.ToLower(tok)

		if prio, ok := priorityFromToken(lower); ok {
			r.Priority = prio
			i++
			continue
		}
		if name, ok := tokenValue(lower, "#"); ok && nameToken.MatchString(name) {
			if !containsFold(r.Tags, name) {
				r.Tags = append(r.Tags, name)
			}
			i++
			continue
		}
		if name, ok := tokenValue(lower, "@"); ok && nameToken.MatchString(name) {
			r.Project = name
			i++
			continue
		}
		if mins, ok := estimateFromToken(lower); ok {
			r.EstimateMinutes = mins
			i++
			continue
		}
		if phrase, consumed, ok := recurrenceFromTokens(tokens, i); ok {
			r.Recurrence = phrase
			i += consumed
			continue
		}
		if due, consumed, ok, bare := dueFromTokens(tokens, i, now); ok {
			r.Due = due
			if bare != nil {
				bareTimeHour, bareTimeMinute, bareTimeSet = bare.hour, bare.minute, true
			}
			i += consumed
			continue
		}
		title = append(title, tok)
		i++
	}

	// "gym every monday 7am": a bare time following a weekly-weekday
	// recurrence belongs to that weekday, not to the nearest day.
	if bareTimeSet && strings.HasPrefix(r.Recurrence, "weekly:") {
		if wd, ok := weekdayNames[strings.TrimPrefix(r.Recurrence, "weekly:")]; ok {
			delta := daysUntilWeekday(wd, now)
			day := dayStart(now).AddDate(0, 0, delta)
			dt := time.Date(day.Year(), day.Month(), day.Day(), bareTimeHour, bareTimeMinute, 0, 0, now.Location())
			if !dt.After(now) {
				dt = dt.AddDate(0, 0, 7)
			}
			r.Due = dt.Format(core.TimeFormat)
		}
	}

	r.Title = strings.Join(title, " ")
	return r
}

func tokenValue(lower, prefix string) (string, bool) {
	if !strings.HasPrefix(lower, prefix) {
		return "", false
	}
	v := strings.TrimPrefix(lower, prefix)
	return v, v != ""
}

func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

func priorityFromToken(lower string) (string, bool) {
	switch lower {
	case "!low":
		return core.PriorityLow, true
	case "!med", "!medium":
		return core.PriorityMedium, true
	case "!high":
		return core.PriorityHigh, true
	case "!urgent", "!crit", "!critical":
		return core.PriorityUrgent, true
	}
	return "", false
}

func estimateFromToken(lower string) (int, bool) {
	m := estRe.FindStringSubmatch(lower)
	if m == nil {
		return 0, false
	}
	n, err := strconv.ParseFloat(m[1], 64)
	if err != nil || n < 0 {
		return 0, false
	}
	if m[2][0] == 'h' {
		return int(n * 60), true
	}
	return int(n), true
}

// ---- recurrence ----

func recurrenceFromTokens(tokens []string, i int) (string, int, bool) {
	lower := strings.ToLower(tokens[i])
	at := func(off int) string {
		if i+off < len(tokens) {
			return strings.ToLower(tokens[i+off])
		}
		return ""
	}

	switch lower {
	case "daily":
		return "daily", 1, true
	case "weekly":
		if at(1) == "on" && validWeekday(at(2)) {
			return "weekly:" + wdAbbrev(at(2)), 3, true
		}
		return "weekly", 1, true
	case "every":
		n, unit := at(1), at(2)
		// "every 2 weeks" / "every 3 days" / "every 1 day|week"
		if isDigits(n) && (unit == "day" || unit == "days" || unit == "week" || unit == "weeks") {
			count, _ := strconv.Atoi(n)
			if count == 1 {
				if unit[0] == 'd' {
					return "daily", 3, true
				}
				return "weekly", 3, true
			}
			if unit[0] == 'd' {
				return fmt.Sprintf("every %s", text.Count(count, "day", "days")), 3, true
			}
			return fmt.Sprintf("every %s", text.Count(count, "week", "weeks")), 3, true
		}
		// "every friday"
		if _, ok := weekdayNames[n]; ok {
			return "weekly:" + wdAbbrev(n), 2, true
		}
		// "every day" / "every week" ["on" <weekday>]
		if n == "day" {
			return "daily", 2, true
		}
		if n == "week" {
			if at(2) == "on" && validWeekday(at(3)) {
				return "weekly:" + wdAbbrev(at(3)), 4, true
			}
			return "weekly", 2, true
		}
	}
	return "", 0, false
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func validWeekday(s string) bool {
	_, ok := weekdayNames[s]
	return ok
}

func wdAbbrev(s string) string {
	full := []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}
	return full[weekdayNames[strings.ToLower(s)]][:3]
}

// ---- dates and times ----

// timeAt parses a time token starting at index i. The token may itself be a
// time ("6pm", "18:00") or a bare hour number with a following am/pm
// ("6 pm"). Returns the time-of-day and how many tokens it used (0 or 1
// extra token when am/pm follows a bare hour).
func timeAt(tokens []string, i int) (int, int, int, bool) {
	if i >= len(tokens) {
		return 0, 0, 0, false
	}
	lower := strings.ToLower(tokens[i])
	if m := amPmTimeRe.FindStringSubmatch(lower); m != nil {
		hour, _ := strconv.Atoi(m[1])
		minute := 0
		if m[3] != "" {
			minute, _ = strconv.Atoi(m[3])
		}
		hour = hourFromAmPm(hour, m[4])
		return hour, minute, 1, true
	}
	if m := clockTimeRe.FindStringSubmatch(lower); m != nil {
		hour, _ := strconv.Atoi(m[1])
		minute, _ := strconv.Atoi(m[2])
		return hour, minute, 1, true
	}
	if isDigits(lower) {
		hour, err := strconv.Atoi(lower)
		if err == nil && hour >= 1 && hour <= 12 && i+1 < len(tokens) && isAmPm(strings.ToLower(tokens[i+1])) {
			return hourFromAmPm(hour, strings.ToLower(tokens[i+1])), 0, 2, true
		}
	}
	return 0, 0, 0, false
}

func hourFromAmPm(hour int, suffix string) int {
	if suffix == "am" {
		if hour == 12 {
			return 0
		}
		return hour
	}
	if hour == 12 {
		return 12
	}
	return hour + 12
}

func isAmPm(s string) bool { return s == "am" || s == "pm" }

func tokensSafe(tokens []string, i int) string {
	if i < 0 || i >= len(tokens) {
		return ""
	}
	return tokens[i]
}

func dayStart(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func dayNumber(tok string) (int, bool) {
	if !isDigits(tok) {
		return 0, false
	}
	n, err := strconv.Atoi(tok)
	if err != nil || n < 1 || n > 31 {
		return 0, false
	}
	return n, true
}

func yearNumber(tok string) (int, bool) {
	if len(tok) != 4 || !isDigits(tok) {
		return 0, false
	}
	n, err := strconv.Atoi(tok)
	if err != nil || n < 1900 || n > 2100 {
		return 0, false
	}
	return n, true
}

func daysUntilWeekday(wd time.Weekday, now time.Time) int {
	return (int(wd) - int(now.Weekday()) + 7) % 7
}

func isoDateTime(tok string, loc *time.Location) (time.Time, bool) {
	switch len(tok) {
	case len("2006-01-02"):
		t, err := time.ParseInLocation("2006-01-02", tok, loc)
		return t, err == nil
	case len(core.TimeFormat):
		t, err := time.ParseInLocation(core.TimeFormat, tok, loc)
		return t, err == nil
	default:
		return time.Time{}, false
	}
}

// withOptionalTime appends the time found at index j to `day` (date-only
// otherwise). Returns the formatted due, the total tokens consumed, and ok.
func withOptionalTime(day time.Time, tokens []string, j int, consumed int) (string, int) {
	hour, minute, n, ok := timeAt(tokens, j)
	if !ok {
		return day.Format("2006-01-02"), consumed
	}
	dt := time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, day.Location())
	return dt.Format(core.TimeFormat), consumed + n
}

// dueFromTokens tries to parse a date phrase at tokens[i]. On success it
// returns the formatted due and how many tokens were consumed.
//
// Semantics:
//
//	today / tomorrow [time]       (a past time today rolls to tomorrow)
//	tonight                        today 20:00
//	friday [time]                 nearest upcoming weekday (today if date-only)
//	next friday                   the Friday after the nearest one
//	in 3 days / in 2 weeks / in 2 months
//	14 nov [2027] [time]          bare date; a passed date means next year
//	2026-11-14 / 2026-11-14T15:04 single token
//	6pm / 18:00 [/ at 6pm]        time alone → nearest occurrence (today, else tomorrow)
//	noon / midnight
func dueFromTokens(tokens []string, i int, now time.Time) (string, int, bool, *bareTime) {
	lower := strings.ToLower(tokensSafe(tokens, i))
	today := dayStart(now)

	// "next <weekday>"
	if lower == "next" {
		wd, ok := weekdayNames[strings.ToLower(tokensSafe(tokens, i+1))]
		if !ok {
			return "", 0, false, nil
		}
		day := today.AddDate(0, 0, daysUntilWeekday(wd, now)+7)
		due, consumed := withOptionalTime(day, tokens, i+2, 2)
		return due, consumed, true, nil
	}

	// "in N <unit>"
	if lower == "in" {
		n := tokensSafe(tokens, i+1)
		unit := strings.ToLower(tokensSafe(tokens, i+2))
		if !isDigits(n) {
			return "", 0, false, nil
		}
		count, _ := strconv.Atoi(n)
		var day time.Time
		switch unit {
		case "d", "day", "days":
			day = today.AddDate(0, 0, count)
		case "w", "week", "weeks":
			day = today.AddDate(0, 0, 7*count)
		case "mo", "month", "months":
			day = today.AddDate(0, count, 0)
		default:
			return "", 0, false, nil
		}
		return day.Format("2006-01-02"), 3, true, nil
	}

	// today / tomorrow (past time today rolls to tomorrow)
	if lower == "today" || lower == "tomorrow" {
		add := 0
		if lower == "tomorrow" {
			add = 1
		}
		day := today.AddDate(0, 0, add)
		if add == 0 {
			if hour, minute, n, ok := timeAt(tokens, i+1); ok {
				dt := time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, now.Location())
				if !dt.After(now) {
					dt = dt.AddDate(0, 0, 1)
				}
				return dt.Format(core.TimeFormat), 1 + n, true, nil
			}
		}
		due, consumed := withOptionalTime(day, tokens, i+1, 1)
		return due, consumed, true, nil
	}
	if lower == "tonight" {
		return time.Date(today.Year(), today.Month(), today.Day(), 20, 0, 0, 0, now.Location()).Format(core.TimeFormat), 1, true, nil
	}

	// bare weekday
	if wd, ok := weekdayNames[lower]; ok {
		delta := daysUntilWeekday(wd, now)
		day := today.AddDate(0, 0, delta)
		if hour, minute, n, ok := timeAt(tokens, i+1); ok {
			dt := time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, now.Location())
			if !dt.After(now) {
				dt = dt.AddDate(0, 0, 7)
			}
			return dt.Format(core.TimeFormat), 1 + n, true, nil
		}
		return day.Format("2006-01-02"), 1, true, nil
	}

	// "14 nov" / "14 november 2027" [time]
	if dayN, ok := dayNumber(tokensSafe(tokens, i)); ok {
		month := monthNames[strings.ToLower(tokensSafe(tokens, i+1))]
		if month == 0 {
			return "", 0, false, nil
		}
		year := now.Year()
		consumed := 2
		if y, ok := yearNumber(tokensSafe(tokens, i+2)); ok {
			year = y
			consumed = 3
		}
		candidate := time.Date(year, month, dayN, 0, 0, 0, 0, now.Location())
		if candidate.Day() != dayN || candidate.Month() != month {
			return "", 0, false, nil // invalid like "31 feb"
		}
		if consumed == 2 && candidate.Before(today) {
			candidate = candidate.AddDate(1, 0, 0)
		}
		due, used := withOptionalTime(candidate, tokens, i+consumed, consumed)
		return due, used, true, nil
	}

	// single ISO token
	if t, ok := isoDateTime(tokensSafe(tokens, i), now.Location()); ok {
		if len(tokensSafe(tokens, i)) == len("2006-01-02") {
			return t.Format("2006-01-02"), 1, true, nil
		}
		return t.Format(core.TimeFormat), 1, true, nil
	}

	// "at <time>" or bare time ("6pm", "6 pm", "18:00")
	if lower == "at" {
		if hour, minute, n, ok := timeAt(tokens, i+1); ok {
			now2 := now
			dt := time.Date(today.Year(), today.Month(), today.Day(), hour, minute, 0, 0, now.Location())
			if !dt.After(now2) {
				dt = dt.AddDate(0, 0, 1)
			}
			return dt.Format(core.TimeFormat), 1 + n, true, &bareTime{hour, minute}
		}
		return "", 0, false, nil
	}
	if hour, minute, n, ok := timeAt(tokens, i); ok {
		dt := time.Date(today.Year(), today.Month(), today.Day(), hour, minute, 0, 0, now.Location())
		if !dt.After(now) {
			dt = dt.AddDate(0, 0, 1)
		}
		return dt.Format(core.TimeFormat), n, true, &bareTime{hour, minute}
	}

	// noon / midnight
	switch lower {
	case "noon":
		return nearestTime(today, 12, 0, now), 1, true, nil
	case "midnight":
		return nearestTime(today, 0, 0, now), 1, true, nil
	}

	return "", 0, false, nil
}

func nearestTime(today time.Time, hour, minute int, now time.Time) string {
	dt := time.Date(today.Year(), today.Month(), today.Day(), hour, minute, 0, 0, today.Location())
	if !dt.After(now) {
		dt = dt.AddDate(0, 0, 1)
	}
	return dt.Format(core.TimeFormat)
}
