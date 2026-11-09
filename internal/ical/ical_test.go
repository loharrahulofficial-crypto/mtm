package ical

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

type icalDoc struct {
	properties []map[string]string
	events     []map[string]string
}

// unfoldAndValidate strictly checks the wire format, then parses it.
func unfoldAndValidate(t *testing.T, data []byte) icalDoc {
	t.Helper()
	if len(data) == 0 {
		t.Fatal("empty feed")
	}
	// CRLF everywhere; no bare LF or bare CR.
	stripped := bytes.ReplaceAll(data, []byte("\r\n"), nil)
	if bytes.Contains(stripped, []byte("\n")) || bytes.Contains(stripped, []byte("\r")) {
		t.Fatal("bare LF or CR outside CRLF line endings")
	}
	rawLines := bytes.Split(data, []byte("\r\n"))
	// A trailing CRLF produces one final empty element — allow exactly that.
	if len(rawLines) > 0 && len(rawLines[len(rawLines)-1]) == 0 {
		rawLines = rawLines[:len(rawLines)-1]
	}
	if len(rawLines) == 0 {
		t.Fatal("no lines in feed")
	}
	for i, l := range rawLines {
		if len(l) > 75 {
			t.Fatalf("line %d exceeds 75 octets (%d): %q", i+1, len(l), l)
		}
	}
	// Unfold.
	var lines []string
	var cur strings.Builder
	for i, raw := range rawLines {
		line := string(raw)
		if strings.HasPrefix(line, " ") {
			if cur.Len() == 0 {
				t.Fatalf("line %d is a continuation without a base line", i+1)
			}
			cur.WriteString(strings.TrimPrefix(line, " "))
			continue
		}
		if cur.Len() > 0 {
			lines = append(lines, cur.String())
			cur.Reset()
		}
		cur.WriteString(line)
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	if lines[0] != "BEGIN:VCALENDAR" {
		t.Fatalf("first line = %q, want BEGIN:VCALENDAR", lines[0])
	}
	if lines[len(lines)-1] != "END:VCALENDAR" {
		t.Fatalf("last line = %q, want END:VCALENDAR", lines[len(lines)-1])
	}
	doc := icalDoc{}
	var inEvent map[string]string
	for _, line := range lines[1 : len(lines)-1] {
		name, value, ok := splitProperty(line)
		if !ok {
			t.Fatalf("unparseable property line %q", line)
		}
		switch {
		case line == "BEGIN:VEVENT":
			inEvent = map[string]string{}
			continue
		case line == "END:VEVENT":
			if inEvent == nil {
				t.Fatal("END:VEVENT without BEGIN:VEVENT")
			}
			doc.events = append(doc.events, inEvent)
			inEvent = nil
			continue
		case line == "BEGIN:VALARM" || line == "END:VALARM":
			continue // VALARM content is validated inside events, ignored here
		}
		if inEvent != nil {
			if _, dup := inEvent[name]; dup {
				t.Fatalf("duplicate property %q in VEVENT", name)
			}
			inEvent[name] = value
			continue
		}
		doc.properties = append(doc.properties, map[string]string{name: value})
	}
	if inEvent != nil {
		t.Fatal("unterminated VEVENT")
	}
	// VCALENDAR must carry VERSION and PRODID.
	props := doc.Properties()
	if props["VERSION"] != "2.0" {
		t.Errorf("VCALENDAR VERSION = %q, want 2.0", props["VERSION"])
	}
	if props["PRODID"] == "" {
		t.Error("VCALENDAR missing PRODID")
	}
	return doc
}

func (d icalDoc) Properties() map[string]string {
	out := map[string]string{}
	for _, p := range d.properties {
		for k, v := range p {
			out[k] = v
		}
	}
	return out
}

func splitProperty(line string) (name, value string, ok bool) {
	// NAME[;PARAM=VALUE[...]]:VALUE — the name is up to the first ';' or ':'.
	i := strings.Index(line, ":")
	if i < 0 {
		return "", "", false
	}
	head := line[:i]
	name = head
	if j := strings.Index(head, ";"); j >= 0 {
		name = head[:j]
	}
	return name, line[i+1:], true
}

func vEventRequiredProps(t *testing.T, ev map[string]string) {
	t.Helper()
	for _, req := range []string{"UID", "DTSTAMP", "SUMMARY", "DTSTART"} {
		if ev[req] == "" {
			t.Errorf("VEVENT missing required property %q: %v", req, ev)
		}
	}
}

func feedTasks() []*core.Task {
	return []*core.Task{
		{ID: "a1b2c3", Title: "Submit problem set", Project: "inbox", Status: "todo", Priority: "high", Kind: core.KindTask,
			Due: "2026-11-09T14:00", Created: "2026-11-01T10:00", Updated: "2026-11-02T10:00"},
		{ID: "d4e5f6", Title: "DBMS midterm — relational algebra + SQL", Project: "college/dbms", Status: "todo", Priority: "urgent", Kind: core.KindExam,
			Due: "2026-11-14T09:00", Created: "2026-11-01T10:00", Updated: "2026-11-02T10:00", TimeLoggedMinutes: 30},
		{ID: "g7h8i9", Title: "Read ch.4", Project: "college/dbms", Status: "todo", Priority: "medium", Kind: core.KindTask,
			Due: "2026-11-09", Created: "2026-11-01T10:00", Updated: "2026-11-02T10:00"},
		{ID: "j0k1l2", Title: "Done — not on the calendar", Project: "inbox", Status: "done", Priority: "low", Kind: core.KindTask,
			Due: "2026-11-09T09:00", Created: "2026-11-01T10:00", Updated: "2026-11-02T10:00", Completed: "2026-11-02T10:00"},
		{ID: "m3n4o5", Title: "Text with, commas; semicolons and \\ backslash", Project: "inbox", Status: "todo", Priority: "low", Kind: core.KindTask,
			Due: "2026-11-10T10:00", Body: "line one\nline two, with comma", Created: "2026-11-01T10:00", Updated: "2026-11-02T10:00"},
	}
}

func TestCalendarValidatesStrict(t *testing.T) {
	events := Collect(feedTasks(), func(p, s string) bool { return s == "done" }, true, true)
	data, err := Calendar(events, Options{})
	if err != nil {
		t.Fatal(err)
	}
	doc := unfoldAndValidate(t, data)
	if len(doc.events) != 4 { // done item excluded
		t.Fatalf("events = %d, want 4 (done item excluded)", len(doc.events))
	}
	for i, ev := range doc.events {
		vEventRequiredProps(t, ev)
		if !strings.HasPrefix(ev["UID"], "mtm-") {
			t.Errorf("event %d UID = %q, want mtm- prefix", i, ev["UID"])
		}
	}
	// UIDs are stable for the same task id.
	events2 := Collect(feedTasks(), func(p, s string) bool { return s == "done" }, true, true)
	data2, _ := Calendar(events2, Options{})
	doc2 := unfoldAndValidate(t, data2)
	for i, ev := range doc.events {
		if ev["UID"] != doc2.events[i]["UID"] {
			t.Errorf("event %d UID changed between runs: %q vs %q", i, ev["UID"], doc2.events[i]["UID"])
		}
	}
}

func TestAllDayEvent(t *testing.T) {
	events := Collect(feedTasks(), func(p, s string) bool { return s == "done" }, true, true)
	data, _ := Calendar(events, Options{})
	if !bytes.Contains(data, []byte("DTSTART;VALUE=DATE:20261109\r\n")) {
		t.Errorf("all-day DTSTART missing/value wrong:\n%s", data)
	}
	if !bytes.Contains(data, []byte("DTEND;VALUE=DATE:20261110\r\n")) {
		t.Errorf("all-day DTEND missing/value wrong:\n%s", data)
	}
}

func TestTimedEventFloating(t *testing.T) {
	events := Collect(feedTasks(), func(p, s string) bool { return s == "done" }, true, true)
	data, _ := Calendar(events, Options{})
	doc := unfoldAndValidate(t, data)
	for _, ev := range doc.events {
		if strings.HasSuffix(ev["DTSTART"], "Z") || strings.Contains(ev["DTSTART"], "TZID") {
			t.Errorf("DTSTART %q carries a timezone; floating local time expected", ev["DTSTART"])
		}
	}
}

func TestTimezoneOptionUsesTZID(t *testing.T) {
	events := Collect(feedTasks(), func(p, s string) bool { return s == "done" }, true, true)
	data, _ := Calendar(events, Options{Timezone: "Europe/London"})
	unfoldAndValidate(t, data)
	// Three timed events (all-day events carry no TZID) — DTSTART+DTEND each.
	if got := bytes.Count(data, []byte("TZID=Europe/London")); got != 6 {
		t.Errorf("TZID appears %d times, want 6 (3 timed events x 2 lines)", got)
	}
	// The floating-time test proves the default path; here we only care
	// that TZID replaced it.
}

func TestEscaping(t *testing.T) {
	events := Collect(feedTasks(), func(p, s string) bool { return s == "done" }, true, true)
	data, _ := Calendar(events, Options{})
	found := false
	for _, raw := range bytes.Split(data, []byte("\r\n")) {
		if bytes.HasPrefix(raw, []byte("SUMMARY:")) && bytes.Contains(raw, []byte(`\,`)) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a SUMMARY with escaped comma:\n%s", data)
	}
	for _, raw := range bytes.Split(data, []byte("\r\n")) {
		if bytes.HasPrefix(raw, []byte("SUMMARY:")) && bytes.Contains(raw, []byte(`\;`)) {
			return // found — semicolon escape works too
		}
	}
	t.Errorf("expected a SUMMARY with escaped semicolon:\n%s", data)
}

func TestFoldingNeverSplitsRunes(t *testing.T) {
	long := FeedEvent{
		TaskID: "x1",
		Title:  strings.Repeat("日本語のタイトルです。とても長いタイトルです。", 6),
		Due:    "2026-11-09T10:00", Project: "inbox",
	}
	data, err := Calendar([]FeedEvent{long}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range bytes.Split(data, []byte("\r\n")) {
		if len(line) > 75 {
			t.Fatalf("line exceeds 75 octets (%d): %q", len(line), line)
		}
	}
	doc := unfoldAndValidate(t, data)
	if got := doc.events[0]["SUMMARY"]; !strings.HasPrefix(got, "日本語のタイトルです") {
		t.Fatalf("SUMMARY = %q, want the unfolded title", got)
	}
}

func TestAlarmAddedAndOmitted(t *testing.T) {
	events := Collect(feedTasks(), func(p, s string) bool { return s == "done" }, true, true)
	plain, _ := Calendar(events, Options{})
	if bytes.Contains(plain, []byte("VALARM")) {
		t.Fatal("VALARM present without --alarm")
	}
	withAlarm, _ := Calendar(events, Options{AlarmMinutes: 30})
	for _, needle := range [][]byte{[]byte("BEGIN:VALARM"), []byte("TRIGGER:-PT30M"), []byte("ACTION:DISPLAY")} {
		if !bytes.Contains(withAlarm, needle) {
			t.Errorf("alarm feed missing %q", needle)
		}
	}
}

func TestAlarmOnlyForIncluded(t *testing.T) {
	// Tasks only (no exams).
	events := Collect(feedTasks(), func(p, s string) bool { return s == "done" }, true, false)
	data, _ := Calendar(events, Options{})
	doc := unfoldAndValidate(t, data)
	for _, ev := range doc.events {
		if strings.Contains(ev["UID"], "d4e5f6") {
			t.Fatal("exam included despite includeExams=false")
		}
	}
	// Exams only.
	events = Collect(feedTasks(), func(p, s string) bool { return s == "done" }, false, true)
	data, _ = Calendar(events, Options{})
	doc = unfoldAndValidate(t, data)
	if len(doc.events) != 1 || !strings.Contains(doc.events[0]["UID"], "d4e5f6") {
		t.Fatalf("exams-only feed has %d events, want exactly the exam", len(doc.events))
	}
}

func TestFeedHandlerFuncServesOnlyTheFeed(t *testing.T) {
	feed := func() ([]byte, error) { return []byte("FEED-DATA\r\n"), nil }
	tests := []struct {
		name   string
		token  string
		method string
		path   string
		status int
		body   string
	}{
		{"bare path ok", "", http.MethodGet, "/mtm.ics", http.StatusOK, "FEED-DATA\r\n"},
		{"root refused", "", http.MethodGet, "/", http.StatusNotFound, ""},
		{"other path refused", "", http.MethodGet, "/etc/passwd", http.StatusNotFound, ""},
		{"post refused", "", http.MethodPost, "/mtm.ics", http.StatusMethodNotAllowed, ""},
		{"put refused", "", http.MethodPut, "/mtm.ics", http.StatusMethodNotAllowed, ""},
		{"token path ok", "secret", http.MethodGet, "/secret/mtm.ics", http.StatusOK, "FEED-DATA\r\n"},
		{"bare path refused with token", "secret", http.MethodGet, "/mtm.ics", http.StatusNotFound, ""},
		{"wrong token refused", "secret", http.MethodGet, "/other/mtm.ics", http.StatusNotFound, ""},
		{"head ok", "", http.MethodHead, "/mtm.ics", http.StatusOK, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(FeedHandlerFunc(tc.token, feed))
			defer srv.Close()
			req, _ := http.NewRequest(tc.method, srv.URL+tc.path, nil)
			resp, err := srv.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != tc.status {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.status)
			}
			if resp.StatusCode == http.StatusOK {
				if resp.Header.Get("Content-Type") != "text/calendar; charset=utf-8" {
					t.Errorf("Content-Type = %q, want text/calendar", resp.Header.Get("Content-Type"))
				}
				if tc.method == http.MethodHead {
					if resp.ContentLength != int64(len("FEED-DATA\r\n")) {
						t.Errorf("Content-Length = %d, want %d", resp.ContentLength, len("FEED-DATA\r\n"))
					}
				}
			}
			if resp.StatusCode == http.StatusMethodNotAllowed {
				if resp.Header.Get("Allow") != "GET, HEAD" {
					t.Errorf("Allow = %q, want GET, HEAD", resp.Header.Get("Allow"))
				}
			}
		})
	}
}

func TestFeedHandlerFuncFeedError(t *testing.T) {
	feed := func() ([]byte, error) { return nil, fmt.Errorf("boom") }
	srv := httptest.NewServer(FeedHandlerFunc("", feed))
	defer srv.Close()
	resp, err := srv.Client().Get(srv.URL + "/mtm.ics")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestIsLoopback(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "127.1.2.3", "::1", "localhost"} {
		if !IsLoopback(host) {
			t.Errorf("IsLoopback(%q) = false, want true", host)
		}
	}
	for _, host := range []string{"", "0.0.0.0", "192.168.1.5", "10.0.0.2", "example.com"} {
		if IsLoopback(host) {
			t.Errorf("IsLoopback(%q) = true, want false", host)
		}
	}
}

func TestWarnIfExposed(t *testing.T) {
	var buf bytes.Buffer
	WarnIfExposed(&buf, "127.0.0.1:8765", "")
	if buf.Len() != 0 {
		t.Errorf("loopback bind produced a warning: %q", buf.String())
	}
	buf.Reset()
	WarnIfExposed(&buf, "0.0.0.0:8765", "")
	if !bytes.Contains(buf.Bytes(), []byte("WARNING")) {
		t.Error("non-loopback bind produced no warning")
	}
	buf.Reset()
	WarnIfExposed(&buf, "0.0.0.0:8765", "secret")
	if !bytes.Contains(buf.Bytes(), []byte("WARNING")) {
		t.Error("non-loopback bind with token produced no warning")
	}
}

var _ = time.Local
