package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

// heatmapFixed is the fixed clock used for heatmap assertions.
var heatmapFixed = time.Date(2026, 11, 7, 14, 32, 0, 0, time.UTC)

func heatmapTestModel(t *testing.T) Model {
	t.Helper()
	s := emptyFixture(t)
	t.Cleanup(func() { _ = s.Close() })
	return BuildDumpModel(dumpConfig(), s, sync.New(s.Root), "home", 120, 40, heatmapFixed)
}

// heatmapDataRows returns the rows of a rendered heatmap body that carry
// data cells (every weekday row renders cells; the month row and the
// legend never do).
func heatmapDataRows(body []string) []string {
	out := []string{}
	for _, l := range body {
		if strings.Contains(StripANSI(l), "▄") {
			out = append(out, l)
		}
	}
	return out
}

func TestHeatmapExactlySevenDataRows(t *testing.T) {
	m := heatmapTestModel(t)
	d := m.homeDataCached()
	for _, size := range []struct{ w, h int }{{60, 9}, {120, 9}, {200, 9}, {90, 7}} {
		body := heatmapBody(m, size.w, size.h, d)
		t.Logf("size %dx%d: %d lines", size.w, size.h, len(body))
		if size.h >= 9 && len(body) != 9 {
			t.Fatalf("size %dx%d: heatmap renders %d lines, want exactly 9 (month + 7 data rows + legend)", size.w, size.h, len(body))
		}
		data := heatmapDataRows(body)
		if len(data) != 7 {
			t.Fatalf("size %dx%d: %d data rows, want exactly 7", size.w, size.h, len(data))
		}
		for i, row := range data {
			cells := strings.Count(StripANSI(row), "▄")
			if cells != m.cfg.Heatmap.Weeks {
				t.Errorf("size %dx%d row %d: %d cells, want %d", size.w, size.h, i, cells, m.cfg.Heatmap.Weeks)
			}
		}
	}
}

func TestHeatmapWeekdayLabelsOnRowsOneThreeFiveSeven(t *testing.T) {
	m := heatmapTestModel(t)
	d := m.homeDataCached()
	body := heatmapBody(m, 120, 9, d)
	rows := heatmapDataRows(body)
	if len(rows) != 7 {
		t.Fatalf("%d data rows", len(rows))
	}
	want := map[int]string{0: "Mon", 2: "Wed", 4: "Fri", 6: "Sun"}
	for i, row := range rows {
		plain := StripANSI(row)
		label := strings.TrimSpace(plain[:3])
		if expected, ok := want[i]; ok {
			if label != expected {
				t.Errorf("data row %d: label %q, want %q", i, label, expected)
			}
			continue
		}
		if label != "" {
			t.Errorf("data row %d: unexpected label %q (weekday labels only on rows 1,3,5,7)", i, label)
		}
	}
}

func TestHeatmapMonthLabelsFitAtAllWidths(t *testing.T) {
	m := heatmapTestModel(t)
	d := m.homeDataCached()
	for w := 60; w <= 200; w++ {
		placements := heatmapMonthPlacements(heatmapFixed, 26, w)
		for i, p := range placements {
			if p.x < 0 {
				t.Errorf("w=%d: label %q at negative x %d", w, p.text, p.x)
				continue
			}
			if p.x+len(p.text) > w {
				t.Errorf("w=%d: label %q clipped (ends at %d > %d)", w, p.text, p.x+len(p.text), w)
			}
			if i > 0 {
				prev := placements[i-1]
				if p.x < prev.x+len(prev.text) {
					t.Errorf("w=%d: label %q overlaps previous label %q", w, p.text, prev.text)
				}
			}
		}
		// A dropped label must not leave a partial trace in the month row.
		body := heatmapBody(m, w, 9, d)
		monthRow := StripANSI(body[0])
		for _, p := range placements {
			if p.x+len(p.text) > len(monthRow) {
				t.Fatalf("w=%d: month row too short for label %q", w, p.text)
			}
			if got := monthRow[p.x : p.x+len(p.text)]; got != p.text {
				t.Errorf("w=%d: month row slice %q at x=%d, want %q", w, got, p.x, p.text)
			}
		}
	}
}

func TestHeatmapMonthLabelsDroppedNotClippedAtTinyWidths(t *testing.T) {
	for _, w := range []int{10, 20, 30, 40, 90, 119} {
		placements := heatmapMonthPlacements(heatmapFixed, 26, w)
		for _, p := range placements {
			if p.x+len(p.text) > w {
				t.Errorf("w=%d: label %q clipped at x=%d", w, p.text, p.x)
			}
		}
	}
}

func TestHeatmapStatsStripShownOnlyWhenWide(t *testing.T) {
	s := coursesFixture(t)
	defer func() { _ = s.Close() }()
	m := BuildDumpModel(dumpConfig(), s, sync.New(s.Root), "home", 160, 50, heatmapFixed)
	d := m.homeDataCached()
	body := heatmapBody(m, 160, 9, d)
	joined := StripANSI(strings.Join(body, "\n"))
	for _, want := range []string{"best day", "streak", "this period"} {
		if !strings.Contains(joined, want) {
			t.Errorf("wide heatmap missing stats strip entry %q", want)
		}
	}
	narrow := heatmapBody(m, 60, 9, d)
	narrowJoined := StripANSI(strings.Join(narrow, "\n"))
	for _, unwanted := range []string{"best day", "streak"} {
		if strings.Contains(narrowJoined, unwanted) {
			t.Errorf("narrow heatmap shows the stats strip entry %q — want blank space instead", unwanted)
		}
	}
}

func TestHeatStatsMath(t *testing.T) {
	// 2026-11-07 is a Saturday; index 0 = today.
	d := homeData{now: heatmapFixed}
	d.dayCounts[0] = 0 // nothing today yet
	d.dayCounts[1] = 2
	d.dayCounts[2] = 1
	d.dayCounts[3] = 0
	d.dayCounts[4] = 4
	d.dayCounts[40] = 9 // best of the window, outside the streak
	d.dayCounts[363] = 5
	st := heatStatsFor(d, 26)
	// Streak counts back from yesterday (today is blank): 2.
	if st.streak != 2 {
		t.Errorf("streak = %d, want 2", st.streak)
	}
	if st.bestCount != 9 {
		t.Errorf("bestCount = %d, want 9", st.bestCount)
	}
	wantDate := heatmapFixed.AddDate(0, 0, -40).Format("Mon 2 Jan")
	if st.bestDay != wantDate {
		t.Errorf("bestDay = %q, want %q", st.bestDay, wantDate)
	}
	// Total sums only the visible window (26 weeks = 182 days).
	if st.total != 2+1+4+9 {
		t.Errorf("total = %d, want %d", st.total, 2+1+4+9)
	}
}

func TestHeatmapGridNeverStretches(t *testing.T) {
	m := heatmapTestModel(t)
	d := m.homeDataCached()
	for _, w := range []int{60, 90, 120, 160, 200} {
		body := heatmapBody(m, w, 9, d)
		row := StripANSI(body[1]) // first data row (Mon)
		first, last := -1, -1
		for x, r := range []rune(row) {
			if r == '▄' {
				if first < 0 {
					first = x
				}
				last = x
			}
		}
		if first != heatLabelW {
			t.Errorf("w=%d: first cell at x=%d, want %d (label column)", w, first, heatLabelW)
		}
		gridRight := heatLabelW + m.cfg.Heatmap.Weeks*heatColW
		if last != gridRight-2 {
			t.Errorf("w=%d: last cell at x=%d, want %d (grid ends at %d, never stretched)", w, last, gridRight-2, gridRight)
		}
	}
}

func TestHeatmapWeeksConfigFlowsToGrid(t *testing.T) {
	m := heatmapTestModel(t)
	d := m.homeDataCached()

	m.cfg.Heatmap.Weeks = 52
	body := heatmapBody(m, 140, 9, d)
	if cells := strings.Count(StripANSI(body[1]), "▄"); cells != 52 {
		t.Errorf("weeks=52: %d cells per row, want 52", cells)
	}
	m.cfg.Heatmap.Weeks = 4
	body = heatmapBody(m, 40, 9, d)
	if cells := strings.Count(StripANSI(body[1]), "▄"); cells != 4 {
		t.Errorf("weeks=4: %d cells per row, want 4", cells)
	}
}

func TestHeatmapDumpFramePaintsGrid(t *testing.T) {
	s := coursesFixture(t)
	defer func() { _ = s.Close() }()
	frame, err := RenderDump(dumpConfig(), s, sync.New(s.Root), "home", 120, 40, true, DumpFixedTime)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, l := range strings.Split(frame, "\n") {
		if strings.Contains(l, "Mon") && strings.Count(l, "▄") >= 20 {
			found = true
			break
		}
	}
	if !found {
		t.Error("home dump does not contain a full heatmap data row")
	}
}
