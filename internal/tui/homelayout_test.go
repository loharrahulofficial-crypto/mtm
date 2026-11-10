package tui

import (
	"strings"
	"testing"

	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

func mainHOf(m Model) int { return m.height - 1 }

// bandWith finds the band containing panel title.
func findBand(bands [][]homeRect, title string) (int, []homeRect) {
	for bi, band := range bands {
		for _, r := range band {
			if r.title == title {
				return bi, band
			}
		}
	}
	return -1, nil
}

func TestHomeSideBySideBandsAtWideWidth(t *testing.T) {
	s := coursesFixture(t)
	defer func() { _ = s.Close() }()
	for _, size := range []struct {
		w, h     int
		detailOn bool
	}{{150, 40, false}, {175, 45, true}, {200, 50, true}} {
		m := BuildDumpModel(dumpConfig(), s, sync.New(s.Root), "home", size.w, size.h, DumpFixedTime)
		m.detailOn = size.detailOn
		_, mainW := mainPaneGeometry(m)
		bands := homeLayout(m, mainW, mainHOf(m))
		bi, mid := findBand(bands, "COURSES")
		if mid == nil {
			t.Fatalf("size %dx%d: no COURSES band", size.w, size.h)
		}
		if len(mid) != 2 {
			t.Fatalf("size %dx%d: COURSES band holds %d panels, want 2 (COURSES + THIS WEEK side by side)", size.w, size.h, len(mid))
		}
		if mid[0].title != "COURSES" || mid[1].title != "THIS WEEK" {
			t.Fatalf("size %dx%d: middle band = %s | %s, want COURSES | THIS WEEK", size.w, size.h, mid[0].title, mid[1].title)
		}
		if mid[0].y != mid[1].y {
			t.Errorf("size %dx%d: COURSES y=%d, THIS WEEK y=%d — not side by side", size.w, size.h, mid[0].y, mid[1].y)
		}
		if mid[0].h != mid[1].h {
			t.Errorf("size %dx%d: COURSES h=%d, THIS WEEK h=%d — panels must share the band height", size.w, size.h, mid[0].h, mid[1].h)
		}
		if mid[1].x <= mid[0].x {
			t.Errorf("size %dx%d: THIS WEEK panel overlaps COURSES", size.w, size.h)
		}
		// The chart panel width fits its content (bars + labels).
		if mid[1].w > 46 {
			t.Errorf("size %dx%d: THIS WEEK panel width = %d, want ≤ 46 (content-sized chart incl. totals)", size.w, size.h, mid[1].w)
		}
		_ = bi
		examsBI, _ := findBand(bands, "EXAMS")
		todayBI, _ := findBand(bands, "TODAY")
		heatBI, _ := findBand(bands, "HEATMAP")
		if examsBI < todayBI {
			t.Errorf("band order: TODAY at %d, EXAMS at %d — want TODAY then EXAMS", todayBI, examsBI)
		}
		if heatBI != len(bands)-1 {
			t.Errorf("HEATMAP is not the last band (index %d of %d)", heatBI, len(bands))
		}
	}
}

func TestHomeSideBySideStacksBelow110(t *testing.T) {
	s := coursesFixture(t)
	defer func() { _ = s.Close() }()
	for _, size := range []struct{ w, h int }{{100, 30}, {105, 35}, {92, 40}, {150, 40}} {
		m := BuildDumpModel(dumpConfig(), s, sync.New(s.Root), "home", size.w, size.h, DumpFixedTime)
		_, mainW := mainPaneGeometry(m)
		if mainW >= 110 {
			t.Fatalf("test setup: mainW=%d, want < 110", mainW)
		}
		bands := homeLayout(m, mainW, mainHOf(m))
		for _, band := range bands {
			if len(band) > 1 {
				t.Fatalf("size %dx%d: band %v is hstacked — stacked mode must keep full-width panels", size.w, size.h, band)
			}
		}
		// COURSES above THIS WEEK, each full width.
		_, courses := findBand(bands, "COURSES")
		_, week := findBand(bands, "THIS WEEK")
		if courses == nil || week == nil {
			t.Fatalf("size %dx%d: missing COURSES or THIS WEEK band", size.w, size.h)
		}
		if courses[0].y >= week[0].y {
			t.Errorf("size %dx%d: COURSES band must come before THIS WEEK (y=%d >= %d)", size.w, size.h, courses[0].y, week[0].y)
		}
	}
}

func TestHomeExamDigitsPresentForNearestExam(t *testing.T) {
	s := examsFixture(t)
	defer func() { _ = s.Close() }()
	frame, err := RenderDump(dumpConfig(), s, sync.New(s.Root), "home", 200, 50, true, DumpFixedTime)
	if err != nil {
		t.Fatal(err)
	}
	plain := StripANSI(frame)
	// The block-digit glyph rows for "6d" (exam in 6 days).
	if !strings.Contains(plain, "█▀▀▀") {
		t.Error("frame does not contain the countdown block digits for the nearest exam")
	}
	if !strings.Contains(plain, "in 6 days") {
		t.Error("frame does not contain the countdown caption \"in 6 days\"")
	}
	// The other exams stay as compact one-line rows beneath the card.
	if !strings.Contains(plain, "OS quiz — scheduling & deadlocks") {
		t.Error("frame does not list the second exam as a one-line row")
	}
}

func TestHomeExamCompactFallbackAtTinyHeight(t *testing.T) {
	s := examsFixture(t)
	defer func() { _ = s.Close() }()
	frame, err := RenderDump(dumpConfig(), s, sync.New(s.Root), "home", 120, 40, true, DumpFixedTime)
	if err != nil {
		t.Fatal(err)
	}
	// Scene pixels in the sidebar window can contain the digit pattern;
	// scope the check to the main pane.
	sideW := sideWidth(120, false)
	lines := strings.Split(frame, "\n")
	for i, l := range lines {
		lines[i] = stripAnsiPrefix(l, sideW+1)
	}
	plain := StripANSI(strings.Join(lines, "\n"))
	// At 120x40 the EXAMS band cannot hold the card; every exam shows a
	// compact row instead, and the digits are absent.
	if strings.Contains(plain, "█▀▀▀") {
		t.Error("compact fallback still shows the nearest exam's block digits")
	}
	if !strings.Contains(plain, "DBMS midterm") {
		t.Error("compact fallback dropped the nearest exam entirely")
	}
}

func TestHomeUpNextAbsorbsLeftoverRows(t *testing.T) {
	s := coursesFixture(t)
	defer func() { _ = s.Close() }()
	// At 200x50 there is room for everything; UP NEXT must show up to 8
	// (the fixture has 6 open tasks) instead of being squeezed out.
	m := BuildDumpModel(dumpConfig(), s, sync.New(s.Root), "home", 200, 50, DumpFixedTime)
	_, mainW := mainPaneGeometry(m)
	bands := homeLayout(m, mainW, mainHOf(m))
	_, upNext := findBand(bands, "UP NEXT")
	if upNext == nil {
		t.Fatal("UP NEXT band dropped at 200x50 — leftover rows must go to it")
	}
	if upNext[0].h < 8 {
		t.Errorf("UP NEXT height = %d at 200x50, want ≥ 8 (leftover rows belong to it)", upNext[0].h)
	}
	examsBI, _ := findBand(bands, "EXAMS")
	upBI, _ := findBand(bands, "UP NEXT")
	heatBI, _ := findBand(bands, "HEATMAP")
	if heatBI != len(bands)-1 {
		t.Errorf("HEATMAP is not the last band")
	}
	if upBI >= heatBI {
		t.Errorf("UP NEXT (band %d) must come before HEATMAP (band %d)", upBI, heatBI)
	}
	_ = examsBI
}

func TestHomeExamBodyFallsBackToCompactWhenShort(t *testing.T) {
	s := examsFixture(t)
	defer func() { _ = s.Close() }()
	m := BuildDumpModel(dumpConfig(), s, sync.New(s.Root), "home", 200, 50, DumpFixedTime)
	d := m.homeDataCached()

	// h too small for the card → compact rows for every exam.
	compact := examsBody(m, 60, examCardHeight(d.exams[0]), d)
	if strings.Contains(StripANSI(strings.Join(compact, "\n")), "█") {
		t.Error("compact fallback renders block digits")
	}
	if len(compact) != examCardHeight(d.exams[0]) {
		t.Logf("compact body = %d lines (h=%d)", len(compact), examCardHeight(d.exams[0]))
	}
	// Plenty of height → card for the nearest exam, one-liners beneath.
	full := examsBody(m, 100, 9, d)
	plain := StripANSI(strings.Join(full, "\n"))
	if !strings.Contains(plain, "█▀▀▀") {
		t.Error("tall EXAMS panel does not render the nearest exam's block digits")
	}
	if !strings.Contains(plain, "OS quiz — scheduling & deadlocks") {
		t.Error("tall EXAMS panel drops the second exam's one-line row")
	}
}

func TestHomeBarChartWidthFitsContent(t *testing.T) {
	s := coursesFixture(t)
	defer func() { _ = s.Close() }()
	m := BuildDumpModel(dumpConfig(), s, sync.New(s.Root), "home", 200, 50, DumpFixedTime)
	d := m.homeDataCached()
	chart := thisWeekBody(m, 42, 11, d)
	if len(chart) == 0 {
		t.Fatal("chart rendered no rows")
	}
	// Every row must fit the content width and the totals line must be
	// present (inside the panel).
	joined := StripANSI(strings.Join(chart, "\n"))
	if !strings.Contains(joined, "last 7 days") {
		t.Error("chart totals line missing")
	}
	for i, l := range chart {
		if w := len([]rune(StripANSI(l))); w > 42 {
			t.Errorf("chart row %d is %d cells wide, want ≤ 42", i, w)
		}
	}
}
