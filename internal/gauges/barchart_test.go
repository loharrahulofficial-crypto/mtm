package gauges

import (
	"fmt"
	"strings"
	"testing"
)

func TestBarChartLayoutGridFitsAndCenters(t *testing.T) {
	for _, w := range []int{40, 60, 100} {
		for _, n := range []int{7} {
			g := barChartLayout(n, w, 0)
			if g.gridW > w {
				t.Errorf("width %d: grid %d cells exceeds width", w, g.gridW)
			}
			for i := 0; i < n; i++ {
				want := g.colLeft[i] + g.barW/2
				if g.colCenter[i] != want {
					t.Errorf("width %d col %d: center = %d, want %d", w, i, g.colCenter[i], want)
				}
				if i > 0 && g.colLeft[i] <= g.colLeft[i-1] {
					t.Errorf("width %d col %d: columns not strictly increasing", w, i)
				}
			}
			// Grid is centered (or pinned left when it cannot be).
			if pad := (w - g.gridW) / 2; pad > 0 && g.pad != pad {
				t.Errorf("width %d: pad = %d, want %d", w, g.pad, pad)
			}
		}
	}
}

// cellOf returns the rune-cell x of the first occurrence of needle in line.
func cellOf(line, needle string) int {
	idx := strings.Index(line, needle)
	if idx < 0 {
		return -1
	}
	return len([]rune(line[:idx]))
}

func TestBarChartColumnAlignment(t *testing.T) {
	values := []float64{1.5, 0.5, 2.5, 0, 3.5, 2, 0.5}
	labels := []string{"A", "B", "C", "D", "E", "F", "G"}
	valueLabels := []string{"v1", "v2", "v3", "v4", "v5", "v6", "v7"}
	for _, w := range []int{40, 60, 100} {
		lines := RenderBarChart(BarChartConfig{
			Values:      values,
			Labels:      labels,
			ValueLabels: valueLabels,
			Width:       w,
			Height:      12,
			Today:       4,
			MaxBarRows:  8,
		})
		if len(lines) < 4 {
			t.Fatalf("width %d: got %d lines, want value+bars+axis", w, len(lines))
		}
		barRows := len(lines) - 2 // value + axis, no total row requested
		valueRow, axisRow := lines[0], lines[len(lines)-1]
		bottomBarRow := lines[barRows] // last bar row before the axis

		g := barChartLayout(len(values), w, 0)
		for i := 0; i < len(values); i++ {
			// Axis letter center.
			lx := cellOf(axisRow, labels[i])
			if lx < 0 {
				t.Fatalf("width %d col %d: axis letter %q not found in %q", w, i, labels[i], axisRow)
			}
			letterCenter := lx + 1/2 // single-cell letters
			// Value label center.
			vx := cellOf(valueRow, valueLabels[i])
			if vx < 0 {
				t.Fatalf("width %d col %d: value label %q not found in %q", w, i, valueLabels[i], valueRow)
			}
			valueCenter := vx + len([]rune(valueLabels[i]))/2
			// Bar span in the bottom row: the column's cell region must be
			// filled (bar, tick or avg rule), bounded by the column grid.
			left, barW := g.colLeft[i], g.barW
			runesBottom := []rune(bottomBarRow)
			if len(runesBottom) < left+barW {
				t.Fatalf("width %d col %d: bottom bar row too short (%d cells)", w, i, len(runesBottom))
			}
			span := strings.TrimSpace(string(runesBottom[left : left+barW]))
			if span == "" {
				t.Errorf("width %d col %d: bottom bar cell empty (want bar/tick), row = %q", w, i, bottomBarRow)
				continue
			}
			barCenter := left + barW/2
			if letterCenter != g.colCenter[i] {
				t.Errorf("width %d col %d: axis letter center = %d, grid center = %d", w, i, letterCenter, g.colCenter[i])
			}
			if valueCenter != g.colCenter[i] {
				t.Errorf("width %d col %d: value label center = %d, grid center = %d", w, i, valueCenter, g.colCenter[i])
			}
			if barCenter != g.colCenter[i] {
				t.Errorf("width %d col %d: bar center = %d, grid center = %d", w, i, barCenter, g.colCenter[i])
			}
		}
	}
}

func TestBarChartCapsHeightAtEightRows(t *testing.T) {
	lines := RenderBarChart(BarChartConfig{
		Values: []float64{1, 2, 3, 4, 5, 6, 7},
		Labels: []string{"M", "T", "W", "T", "F", "S", "S"},
		Width:  60, Height: 30, // generous budget: the cap must win
		MaxBarRows: 8,
	})
	// value row + 8 bar rows + axis row
	if len(lines) != 10 {
		t.Fatalf("got %d lines, want 10 (value + 8 capped bars + axis)", len(lines))
	}
}

func TestBarChartScalesToMaxAndMinBar(t *testing.T) {
	values := []float64{0, 100, 25}
	lines := RenderBarChart(BarChartConfig{
		Values: values, Labels: []string{"A", "B", "C"},
		Width: 40, Height: 6, MaxBarRows: 8,
	})
	barRows := len(lines) - 2
	bottom := lines[barRows] // last bar row, right before the axis
	g := barChartLayout(3, 40, 0)
	// max column fills all rows: every bar row carries a full block there.
	for r := 1; r < len(lines)-1; r++ {
		cell := string([]rune(lines[r])[g.colLeft[1] : g.colLeft[1]+g.barW])
		for _, ch := range cell {
			if ch != '█' {
				t.Errorf("row %d: max bar cell = %q, want █", r, cell)
			}
		}
	}
	// tiny non-zero value still shows a sliver in the bottom row
	cell := string([]rune(bottom)[g.colLeft[2] : g.colLeft[2]+g.barW])
	if cell == strings.Repeat(" ", g.barW) {
		t.Errorf("25%%-of-max value rendered nothing in the bottom row")
	}
	// zero column: muted baseline tick on the bottom row only
	zero := string([]rune(bottom)[g.colLeft[0] : g.colLeft[0]+g.barW])
	if zero != strings.Repeat("▁", g.barW) {
		t.Errorf("zero column bottom tick = %q, want ▁×%d", zero, g.barW)
	}
	for r := 1; r < len(lines)-2; r++ {
		cell := string([]rune(lines[r])[g.colLeft[0] : g.colLeft[0]+g.barW])
		if strings.TrimSpace(cell) != "" {
			t.Errorf("row %d: zero column should be blank above the tick, got %q", r, cell)
		}
	}
}

func TestBarChartTopCapsAreEighthBlocks(t *testing.T) {
	// value chosen so the top cell is partial: 1 of 3 eighths over the full rows
	// max=8, barRows=4 → full cells = floor((v/8)*4). Use v=1.5 → 0.75 cell → 6/8 of one cell.
	lines := RenderBarChart(BarChartConfig{
		Values: []float64{1.5, 8}, Labels: []string{"A", "B"},
		Width: 20, Height: 6, MaxBarRows: 8,
	})
	g := barChartLayout(2, 20, 0)
	found := false
	for r := 1; r < len(lines)-1; r++ {
		cell := string([]rune(lines[r])[g.colLeft[0] : g.colLeft[0]+g.barW])
		for _, ch := range cell {
			if ch != ' ' && ch != '█' {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected an eighth-block top cap for the partial column, lines:\n%s", strings.Join(lines, "\n"))
	}
}

func TestBarChartTodayAccentAndTotalsLine(t *testing.T) {
	accentSeen := false
	lines := RenderBarChart(BarChartConfig{
		Values: []float64{1, 2}, Labels: []string{"A", "B"},
		Width: 20, Height: 7, MaxBarRows: 8, Today: 1,
		ShowTotal: true,
		Accent:    func(s string) string { accentSeen = true; return "<" + s + ">" },
		Bar:       func(s string) string { return s },
	})
	_ = accentSeen
	totalRow := lines[len(lines)-1]
	if !strings.Contains(totalRow, "3.0 h · last 7 days") {
		t.Errorf("totals row = %q, want it to contain %q", totalRow, "3.0 h · last 7 days")
	}
}

func TestBarChartAverageRule(t *testing.T) {
	withAvg := RenderBarChart(BarChartConfig{
		Values: []float64{1, 4, 1, 4, 1, 4, 1}, Labels: []string{"A", "B", "C", "D", "E", "F", "G"},
		Width: 60, Height: 10, MaxBarRows: 8, ShowAverage: true,
	})
	found := false
	for _, l := range withAvg {
		if strings.Contains(l, "┄") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a muted average rule (┄) when space allows, got:\n%s", strings.Join(withAvg, "\n"))
	}
	// Too narrow: no average rule even when requested.
	narrow := RenderBarChart(BarChartConfig{
		Values: []float64{1, 4, 1, 4, 1, 4, 1}, Labels: []string{"A", "B", "C", "D", "E", "F", "G"},
		Width: 30, Height: 6, MaxBarRows: 8, ShowAverage: true,
	})
	for _, l := range narrow {
		if strings.Contains(l, "┄") {
			t.Errorf("narrow chart shows an average rule: %q", l)
		}
	}
}

func ExampleRenderBarChart() {
	out := RenderBarChart(BarChartConfig{
		Values:      []float64{1.5, 0, 2.5, 0.75, 0, 1, 3},
		Labels:      []string{"M", "T", "W", "T", "F", "S", "S"},
		ValueLabels: []string{"1.5h", "0", "2.5h", "45m", "0", "1h", "3h"},
		Width:       40, Height: 11, MaxBarRows: 8, Today: 6,
		ShowTotal: true,
	})
	fmt.Println(strings.TrimRight(out[0], " "))
	fmt.Println(out[len(out)-1])
	// Output:
	// 1.5h   0   2.5h   45m    0    1h    3h
	//  8.8 h · last 7 days
}
