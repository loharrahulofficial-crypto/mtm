package tui

import (
	"strings"
	"testing"

	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

// Spec 0b.1: the Courses detail pane never silently drops sections. If the
// content is taller than the pane, every section is reachable by scrolling
// and muted ▲/▼ indicators mark the clipped edges — at short pane sizes.
func TestCoursesDetailScrollReachesEverySection(t *testing.T) {
	sectionKeys := []string{
		"DBMS — Database Systems", // header
		"grade",                   // grade gauge line
		"GRADED ITEMS",            // table
		"HOURS",                   // hours section
		"EXAMS",                   // exams section
		"NEXT TASKS",              // next tasks section
	}
	for _, size := range [][2]int{{80, 24}, {200, 30}} {
		name := func() string {
			if size[0] == 80 {
				return "80x24"
			}
			return "200x30"
		}()
		t.Run(name, func(t *testing.T) {
			s := fixtures["courses"](t)
			defer func() { _ = s.Close() }()
			m := BuildDumpModel(dumpConfig(), s, sync.New(s.Root), "courses", size[0], size[1], DumpFixedTime)
			m.focus = focusMain
			seen := map[string]bool{}
			bottom := false
			frames := 0
			for i := 0; i < 40; i++ {
				frame := StripANSI(m.View())
				frames++
				for _, key := range sectionKeys {
					if strings.Contains(frame, key) {
						seen[key] = true
					}
				}
				if i > 0 {
					if !strings.Contains(frame, "▲ more") {
						t.Errorf("scrolled frame at offset %d is missing the ▲ more indicator:\n%s", m.coursesScroll, frame)
					}
				}
				before := m.coursesScroll
				updated, _ := m.Update(keyMsg("ctrl+d"))
				m = updated.(Model)
				if m.coursesScroll == before {
					bottom = true
					frame = StripANSI(m.View())
					if strings.Contains(frame, "▼ more") {
						t.Errorf("bottom frame still shows ▼ more:\n%s", frame)
					}
					break
				}
			}
			if !bottom {
				t.Fatal("paging never reached the bottom of the dashboard — sections may be unreachable")
			}
			for _, key := range sectionKeys {
				if !seen[key] {
					t.Errorf("section %q never visible while paging with ctrl+d — its content is unreachable", key)
				}
			}
		})
	}
}

// At the default scroll the ▲ indicator must be absent, and paging up from
// the top must not move the scroll or show stale indicators.
func TestCoursesDetailScrollResetAndTop(t *testing.T) {
	s := fixtures["courses"](t)
	defer func() { _ = s.Close() }()
	m := BuildDumpModel(dumpConfig(), s, sync.New(s.Root), "courses", 80, 24, DumpFixedTime)
	frame := StripANSI(m.View())
	if strings.Contains(frame, "▲ more") {
		t.Fatal("▲ more shown at the top of the dashboard")
	}
	updated, _ := m.updateCoursesKey(aPageUp)
	m = updated.(Model)
	if m.coursesScroll != 0 {
		t.Fatalf("coursesScroll = %d after paging up at the top, want 0", m.coursesScroll)
	}
	// Moving the course selection resets the scroll to the top.
	updated, _ = m.Update(keyMsg("ctrl+d"))
	m = updated.(Model)
	if m.coursesScroll == 0 {
		t.Fatal("ctrl+d did not scroll the dashboard")
	}
	updated, _ = m.updateCoursesKey(aDown)
	m = updated.(Model)
	if m.courseSel == 0 || m.coursesScroll != 0 {
		t.Fatalf("selection/scroll after j = %d/%d, want a different course at scroll 0", m.courseSel, m.coursesScroll)
	}
}
