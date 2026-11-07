package gauges

import (
	"strings"
	"testing"
	"time"
)

func TestBarZeroAndFull(t *testing.T) {
	fill, track := Bar(0, 10)
	if fill != "" || track != "░░░░░░░░░░" {
		t.Errorf("zero bar = fill %q track %q", fill, track)
	}
	fill, track = Bar(1, 10)
	if fill != "██████████" || track != "" {
		t.Errorf("full bar = fill %q track %q", fill, track)
	}
}

func TestBarEighthBlocks(t *testing.T) {
	// Quarter of 8 cells = 2 full cells
	fill, track := Bar(0.25, 8)
	if fill != "██" || track != "░░░░░░" {
		t.Errorf("0.25/8 = fill %q track %q", fill, track)
	}
	// 1/16 of a 8-cell bar = half a cell
	fill, track = Bar(1.0/16.0, 8)
	if !strings.HasPrefix(fill, "▌") || len([]rune(track)) != 7 {
		t.Errorf("sliver = fill %q track %q", fill, track)
	}
	// Track + fill always sum to cells (runes)
	for _, frac := range []float64{0, 0.01, 0.1, 0.35, 0.5, 0.75, 0.9, 0.99, 1} {
		fill, track := Bar(frac, 12)
		got := []rune(fill)
		if len(got)+len([]rune(track)) != 12 {
			t.Errorf("frac %v: fill %q (%d) + track %q (%d) != 12 cells", frac, fill, len(got), track, len([]rune(track)))
		}
	}
}

func TestBarClamps(t *testing.T) {
	fill, _ := Bar(2, 4)
	if fill != "████" {
		t.Errorf("over 1 = %q", fill)
	}
	fill, track := Bar(-1, 4)
	if fill != "" || track != "░░░░" {
		t.Errorf("negative = fill %q track %q", fill, track)
	}
	if f, tr := Bar(0.5, 0); f != "" || tr != "" {
		t.Errorf("zero cells = %q %q", f, tr)
	}
}

func TestSparkline(t *testing.T) {
	if got := Sparkline(nil); got != "" {
		t.Errorf("empty = %q", got)
	}
	if got := Sparkline([]float64{0, 0, 0}); got != "▁▁▁" {
		t.Errorf("all zero = %q", got)
	}
	got := Sparkline([]float64{0, 4, 8})
	if []rune(got)[0] != '▁' || []rune(got)[2] != '█' {
		t.Errorf("ramp = %q", got)
	}
	// Monotonic non-decreasing output for non-decreasing input
	got = Sparkline([]float64{1, 2, 3, 4, 5, 6, 7, 8})
	r := []rune(got)
	for i := 1; i < len(r); i++ {
		if strings.ContainsRune("▁▂▃▄▅▆▇█", r[i]) == false {
			t.Fatalf("bad spark char %q", r[i])
		}
		if r[i] < r[i-1] {
			t.Errorf("sparkline not monotonic: %q", got)
		}
	}
}

// TestBlockDigitsSnapshot pins the exact 3-row art for every glyph (0-9 and
// 'd'). The countdown reads at a glance only if each glyph is drawn
// deliberately; if this fails, someone changed the font without looking at it.
func TestBlockDigitsSnapshot(t *testing.T) {
	want := map[rune][3]string{
		'0': {"▄▀▀▀▄", "█   █", "▀▄▄▄▀"},
		'1': {"  █  ", "  █  ", "▄▄█▄▄"},
		'2': {"▀▀▀▀█", " ▄▀▀ ", "█▄▄▄▄"},
		'3': {"▀▀▀▀█", " ███ ", "▄▄▄▄█"},
		'4': {"█   █", "▀▀▀▀█", "    █"},
		'5': {"█▀▀▀▀", "████ ", "▄▄▄▄█"},
		'6': {"█▀▀▀ ", "█▀▀▀█", "▀▄▄▄▀"},
		'7': {"▀▀▀▀█", "    █", "    █"},
		'8': {"▄▀▀▀▄", "█████", "▀▄▄▄▀"},
		'9': {"▄▀▀▀▄", "▀▄▄▄▀", " ▄▄▄▀"},
		'd': {"    █", "▄▀▀▄█", "▀▄▄▀█"},
	}
	for r, wantRows := range want {
		g := Digits(string(r))
		for row := 0; row < 3; row++ {
			if g[row] != wantRows[row] {
				t.Errorf("digit %q row %d = %q, want %q", r, row, g[row], wantRows[row])
			}
			if len([]rune(g[row])) != 5 {
				t.Errorf("digit %q row %d = %d cells, want 5", r, row, len([]rune(g[row])))
			}
		}
	}
}

func TestDigitsRenderThreeRows(t *testing.T) {
	for _, d := range "0123456789d" {
		g := Digits(string(d))
		for row := 0; row < 3; row++ {
			if len([]rune(g[row])) != 5 {
				t.Errorf("digit %q row %d = %q, want 5 cells", d, row, g[row])
			}
		}
	}
	g := Digits("12")
	for row := 0; row < 3; row++ {
		if len([]rune(g[row])) != DigitWidth(2) {
			t.Errorf("digits 12 row %d width = %d, want %d", row, len([]rune(g[row])), DigitWidth(2))
		}
	}
	// every glyph distinct
	seen := map[[3]string]rune{}
	for _, d := range "0123456789d" {
		g := Digits(string(d))
		if prev, dup := seen[g]; dup {
			t.Errorf("glyphs %q and %q render identically", prev, d)
		}
		seen[g] = d
	}
}

func TestDigitWidth(t *testing.T) {
	if got := DigitWidth(1); got != 5 {
		t.Errorf("DigitWidth(1) = %d, want 5", got)
	}
	if got := DigitWidth(3); got != 19 {
		t.Errorf("DigitWidth(3) = %d, want 19", got)
	}
	if got := DigitWidth(0); got != 0 {
		t.Errorf("DigitWidth(0) = %d, want 0", got)
	}
}

func TestDueHeatThresholds(t *testing.T) {
	now := time.Date(2026, 11, 7, 14, 32, 0, 0, time.Local)
	day := func(offset int) string {
		return now.AddDate(0, 0, offset).Format("2006-01-02")
	}
	cases := []struct {
		due  string
		done bool
		want Heat
	}{
		{"", false, HeatCalm},
		{day(-3), false, HeatOverdue},
		{day(-1), false, HeatOverdue},
		{day(0), false, HeatHot},
		{day(1), false, HeatWarm},
		{day(HeatWarmMaxDays), false, HeatWarm},
		{day(HeatWarmMaxDays + 1), false, HeatCalm},
		{day(30), false, HeatCalm},
		{day(-5), true, HeatCalm}, // done tasks never hot
		{day(0), true, HeatCalm},
		{"garbage", false, HeatCalm},
	}
	for _, c := range cases {
		if got := DueHeat(c.due, now, c.done); got != c.want {
			t.Errorf("DueHeat(%q, done=%v) = %v, want %v", c.due, c.done, got, c.want)
		}
	}
}

func TestDueLabel(t *testing.T) {
	now := time.Date(2026, 11, 7, 14, 32, 0, 0, time.Local)
	day := func(offset int) string {
		return now.AddDate(0, 0, offset).Format("2006-01-02")
	}
	for offset, want := range map[int]string{-1: "overdue 1d", -3: "overdue 3d", 0: "today", 1: "tomorrow", 3: "in 3d"} {
		got, _ := DueLabel(day(offset), now)
		if got != want {
			t.Errorf("DueLabel(%d) = %q, want %q", offset, got, want)
		}
	}
	got, heat := DueLabel(day(30), now)
	if heat != HeatCalm || got == "" {
		t.Errorf("far future = %q heat %v", got, heat)
	}
}
