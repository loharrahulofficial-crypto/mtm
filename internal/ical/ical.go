// Package ical generates RFC 5545 (iCalendar) output from mtm's task
// model. Dates stored local-naive are exported as floating local times
// (no TZID) unless a timezone is configured; DTSTAMP is always UTC.
//
// Output is strict RFC 5545: CRLF line endings, line folding at 75
// octets (never splitting a multibyte rune), TEXT escaping, and the
// required VCALENDAR/VEVENT properties. See Export for the flags.
package ical

import (
	"bytes"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

// FeedEvent is one VEVENT (built from a task).
type FeedEvent struct {
	TaskID    string
	Title     string
	Notes     string // optional description
	Project   string
	Due       string // stored format (date or timestamp)
	Completed bool
	Updated   string // stored format
	Kind      string // exam|assignment|task|...
	Exam      bool
}

// Options controls generation.
type Options struct {
	// Timezone is an IANA name (e.g. "Europe/London"); when set, events
	// carry TZID=<name> instead of floating local times.
	Timezone string
	// AlarmMinutes adds a VALARM trigger of -PT<n>M to every event when
	// > 0.
	AlarmMinutes int
	// IncludeTasks includes non-exam, non-done tasks with timed or
	// date-only due dates. Default true.
	IncludeTasks bool
	// IncludeExams includes exam tasks. Default true.
	IncludeExams bool
}

// CollectMilestones maps milestones to all-day feed events on their target
// date. Done milestones and milestones without a target never appear (the
// calendar is for what's ahead). UIDs derive from milestone ids.
func CollectMilestones(ms []*core.Milestone, include bool) []FeedEvent {
	var out []FeedEvent
	if !include {
		return out
	}
	for _, m := range ms {
		if strings.EqualFold(m.Status, core.MilestoneDone) {
			continue
		}
		if strings.TrimSpace(m.Target) == "" {
			continue
		}
		notes := m.Body
		if strings.TrimSpace(notes) == "" {
			notes = m.Description
		}
		out = append(out, FeedEvent{
			TaskID:  m.ID,
			Title:   "Milestone: " + m.Title,
			Notes:   notes,
			Project: m.Project,
			Due:     m.Target,
			Updated: m.Updated,
			Kind:    "milestone",
		})
	}
	return out
}

// Collect maps tasks to feed events, honoring show/hide toggles. Done
// items never appear (the calendar is for what's ahead).
func Collect(tasks []*core.Task, isDone func(project, status string) bool, includeTasks, includeExams bool) []FeedEvent {
	var out []FeedEvent
	for _, t := range tasks {
		isExam := strings.EqualFold(t.Kind, core.KindExam)
		if isDone != nil && isDone(t.Project, t.Status) {
			continue
		}
		if isExam && !includeExams {
			continue
		}
		if !isExam && !includeTasks {
			continue
		}
		if strings.TrimSpace(t.Due) == "" {
			continue
		}
		out = append(out, FeedEvent{
			TaskID:  t.ID,
			Title:   t.Title,
			Notes:   t.Body,
			Project: t.Project,
			Due:     t.Due,
			Updated: t.Updated,
			Kind:    t.Kind,
			Exam:    isExam,
		})
	}
	return out
}

// Calendar renders a complete VCALENDAR document for the events.
func Calendar(events []FeedEvent, opts Options) ([]byte, error) {
	var buf bytes.Buffer
	writeLine(&buf, "BEGIN:VCALENDAR")
	writeLine(&buf, "VERSION:2.0")
	writeLine(&buf, "PRODID:-//mtm//mtm//EN")
	writeLine(&buf, "CALSCALE:GREGORIAN")
	writeLine(&buf, "METHOD:PUBLISH")
	for _, ev := range events {
		writeEvent(&buf, ev, opts)
	}
	writeLine(&buf, "END:VCALENDAR")
	return buf.Bytes(), nil
}

func writeEvent(buf *bytes.Buffer, ev FeedEvent, opts Options) {
	// Stable UID from the task id: re-imports update instead of duplicating.
	uid := "mtm-" + ev.TaskID + "@mtm"
	dstamp := time.Now().UTC().Format("20060102T150405Z")
	writeLine(buf, "BEGIN:VEVENT")
	writeLine(buf, "UID:"+uid)
	writeLine(buf, "DTSTAMP:"+dstamp)
	if lastMod := formatDateUTC(ev.Updated); lastMod != "" {
		writeLine(buf, "LAST-MODIFIED:"+lastMod)
	}
	writeLine(buf, "SUMMARY:"+escapeText(ev.Title))
	if strings.TrimSpace(ev.Notes) != "" {
		writeLine(buf, "DESCRIPTION:"+escapeText(ev.Notes))
	}
	if ev.Project != "" {
		writeLine(buf, "CATEGORIES:"+escapeText(ev.Project))
	}
	dd, allDay, err := parseDue(ev.Due)
	if err != nil {
		// A task we can't understand doesn't go on the calendar.
		writeLine(buf, "STATUS:CANCELLED")
		writeLine(buf, "END:VEVENT")
		return
	}
	if allDay {
		writeLine(buf, "DTSTART;VALUE=DATE:"+dd.Format("20060102"))
		writeLine(buf, "DTEND;VALUE=DATE:"+dd.AddDate(0, 0, 1).Format("20060102"))
	} else {
		startFmt := "20060102T150405"
		if tz := opts.Timezone; tz != "" {
			writeLine(buf, "DTSTART;TZID="+paramText(tz)+":"+dd.Format(startFmt))
			writeLine(buf, "DTEND;TZID="+paramText(tz)+":"+dd.Add(time.Hour).Format(startFmt))
		} else {
			// Floating local time: no TZID, no trailing Z.
			writeLine(buf, "DTSTART:"+dd.Format(startFmt))
			writeLine(buf, "DTEND:"+dd.Add(time.Hour).Format(startFmt))
		}
	}
	writeLine(buf, "STATUS:CONFIRMED")
	writeLine(buf, "SEQUENCE:0")
	if opts.AlarmMinutes > 0 {
		writeLine(buf, "BEGIN:VALARM")
		writeLine(buf, "TRIGGER:-PT"+fmt.Sprint(opts.AlarmMinutes)+"M")
		writeLine(buf, "ACTION:DISPLAY")
		writeLine(buf, "DESCRIPTION:"+escapeText(ev.Title))
		writeLine(buf, "END:VALARM")
	}
	writeLine(buf, "END:VEVENT")
}

// parseDue parses a stored due stamp (timestamp or date-only).
func parseDue(s string) (time.Time, bool, error) {
	s = strings.TrimSpace(s)
	if len(s) > len("2006-01-02") {
		for _, layout := range []string{core.TimestampFormat, "2006-01-02 15:04"} {
			if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
				return t, false, nil
			}
		}
		return time.Time{}, false, fmt.Errorf("unparseable timestamp %q", s)
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("unparseable date %q", s)
	}
	return t, true, nil
}

// formatDateUTC converts a stored local stamp to a UTC stamp string;
// unparseable values yield "".
func formatDateUTC(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	for _, layout := range []string{core.TimestampFormat, "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t.UTC().Format("20060102T150405Z")
		}
	}
	return ""
}

func writeLine(buf *bytes.Buffer, line string) {
	// Fold to 75 octets per raw line. Continuation lines start with a
	// single space, so they carry at most 74 content octets. Never split
	// a multibyte rune.
	first := true
	for len(line) > 0 {
		prefix, limit := "", 75
		if !first {
			prefix, limit = " ", 74
		}
		if len(line) <= limit {
			buf.WriteString(prefix + line)
			line = ""
			break
		}
		cut := limit
		for cut > 0 && !utf8.RuneStart(line[cut]) {
			cut--
		}
		if cut == 0 {
			cut = limit // pathological: buffer as-is, let the validator complain
		}
		buf.WriteString(prefix + line[:cut])
		buf.WriteString("\r\n")
		line = line[cut:]
		first = false
	}
	buf.WriteString("\r\n")
}

// escapeText escapes a TEXT value per RFC 5545 §3.3.11.
func escapeText(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, ",", `\,`)
	s = strings.ReplaceAll(s, ";", `\;`)
	s = strings.ReplaceAll(s, "\r\n", `\n`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\n`)
	return s
}

// paramText emits an iCal parameter value: passed through when safe,
// double-quoted otherwise (RFC 5545 §3.1 — values with ':' ';' ',' '"' '^'
// or control characters cannot ride bare).
func paramText(s string) string {
	safe := len(s) > 0
	for _, r := range s {
		if r == ':' || r == ';' || r == ',' || r == '"' || r == '^' || r < 32 {
			safe = false
			break
		}
	}
	if safe {
		return s
	}
	quoted := strings.ReplaceAll(s, `"`, `^'`)
	return `"` + quoted + `"`
}
