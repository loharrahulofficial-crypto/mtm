// Package gauges implements the small high-resolution glyph graphics of the
// Study Desk design language: eighth-block progress bars, eighth-block
// sparklines, a 3-row block-digit font and the urgency heat model. All
// functions are pure; colors are applied by the caller.
package gauges

import (
	"strings"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

// Eighth-blocks for fractional cells (1/8 … full).
var eighths = []rune{'▏', '▎', '▍', '▌', '▋', '▊', '▉', '█'}

// Sparkline levels, lowest to highest.
var spark = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// Bar renders frac ∈ [0,1] into `cells` terminal cells: filled leading
// cells with eighth-block precision, the remainder a dim track ("░").
// fill and track are returned separately so callers can style them.
func Bar(frac float64, cells int) (fill, track string) {
	if cells < 1 {
		return "", ""
	}
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	eighthCount := int(frac * float64(cells) * 8)
	if frac > 0 && eighthCount == 0 {
		eighthCount = 1 // any visible progress shows at least one sliver
	}
	full := eighthCount / 8
	rem := eighthCount % 8
	var b strings.Builder
	for i := 0; i < full; i++ {
		b.WriteRune('█')
	}
	if rem > 0 {
		b.WriteRune(eighths[rem-1])
	}
	fill = b.String()
	used := full
	if rem > 0 {
		used++
	}
	track = strings.Repeat("░", cells-used)
	return fill, track
}

// Sparkline renders values as a single row of eighth-block levels.
// Negative values are clamped to zero. An all-zero (or empty) series
// renders as a flat floor line.
func Sparkline(values []float64) string {
	if len(values) == 0 {
		return ""
	}
	max := 0.0
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	var b strings.Builder
	for _, v := range values {
		if v < 0 {
			v = 0
		}
		idx := len(spark) - 1
		if max > 0 {
			idx = int(v / max * float64(len(spark)-1))
		} else {
			idx = 0
		}
		b.WriteRune(spark[idx])
	}
	return b.String()
}

// blockDigits is the 3-row font used for the exam countdown. Glyphs are
// 5 cells wide, joined by a 2-cell gap. All ten digits are distinct.
var blockDigits = map[rune][3]string{
	'0': {"╭───╮", "│   │", "╰───╯"},
	'1': {"  █  ", "  █  ", "  █  "},
	'2': {"╭───╮", "────╮", "╭────"},
	'3': {"╭───╮", "────╮", "────╯"},
	'4': {"│   │", "╰───╮", "    │"},
	'5': {"╭───╮", "╭────", "────╯"},
	'6': {"╭───╮", "╭────", "╰───╯"},
	'7': {"╭───╮", "    │", "    │"},
	'8': {"╭───╮", "├───┤", "╰───╯"},
	'9': {"╭───╮", "╰───╮", "────╯"},
}

// Digits renders s (digits only) as three rows of the block font.
// Unknown runes render as blank cells of the digit width.
func Digits(s string) [3]string {
	var out [3]string
	for row := 0; row < 3; row++ {
		parts := make([]string, 0, len(s))
		for _, r := range s {
			g, ok := blockDigits[r]
			if !ok {
				g = [3]string{"     ", "     ", "     "}
			}
			parts = append(parts, g[row])
		}
		out[row] = strings.Join(parts, "  ")
	}
	return out
}

// DigitWidth is the display width of n digits rendered by Digits.
func DigitWidth(n int) int {
	if n <= 0 {
		return 0
	}
	return n*5 + (n-1)*2
}

// Heat is the urgency level of a due date (or task generally).
type Heat int

const (
	HeatCalm    Heat = iota // > HeatWarmMaxDays out, or no due date / done
	HeatWarm                // due within HeatWarmMaxDays days
	HeatHot                 // due today
	HeatOverdue             // past due (not done)
)

// HeatWarmMaxDays is the outer bound of "warm": due dates within this many
// days render as warm. All heat thresholds live here — one place.
const HeatWarmMaxDays = 3

// DueHeat classifies a due string ("2006-01-02" or "2006-01-02T15:04")
// against now (local-naive). Done tasks are always HeatCalm. An empty due
// string is HeatCalm.
func DueHeat(due string, now time.Time, done bool) Heat {
	if done || strings.TrimSpace(due) == "" {
		return HeatCalm
	}
	day := parseDay(due, now.Location())
	if day.IsZero() {
		return HeatCalm
	}
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	days := int(day.Sub(today).Hours() / 24)
	switch {
	case days < 0:
		return HeatOverdue
	case days == 0:
		return HeatHot
	case days <= HeatWarmMaxDays:
		return HeatWarm
	default:
		return HeatCalm
	}
}

// DueLabel renders a short relative label: "overdue 2d", "today",
// "tomorrow", "in 3d", or the raw date for far-out dates.
func DueLabel(due string, now time.Time) (string, Heat) {
	day := parseDay(due, now.Location())
	if day.IsZero() {
		return due, HeatCalm
	}
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	days := int(day.Sub(today).Hours() / 24)
	switch {
	case days < 0:
		h := DueHeat(due, now, false)
		if days == -1 {
			return "overdue 1d", h
		}
		return "overdue " + itoa(-days) + "d", h
	case days == 0:
		return "today", HeatHot
	case days == 1:
		return "tomorrow", HeatWarm
	case days <= HeatWarmMaxDays:
		return "in " + itoa(days) + "d", HeatWarm
	default:
		return day.Format("Jan 2"), HeatCalm
	}
}

func parseDay(s string, loc *time.Location) time.Time {
	s = strings.TrimSpace(s)
	for _, layout := range []string{core.TimeFormat, "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
		}
	}
	return time.Time{}
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
