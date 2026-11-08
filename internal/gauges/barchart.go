package gauges

import (
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"
)

// Vertical bar chart shared by the Home dashboard and the course views.
// All rows come from one column grid: column i keeps the same left edge and
// center in the value row, every bar row, the axis row and the totals row.
// Bars hold a fixed cell width with a fixed gutter between columns; the grid
// is centered in the available width. Bar height is capped (default 8 rows)
// — the chart never stretches to fill a panel.
//
// Pure text geometry; colors are applied by the caller through the style
// callbacks (nil style means identity). Lines are returned ragged — callers
// pad/clip to their panel's inner width.

// verticalEighths index by (eighths in cell - 1); the partial top cap of a
// bar grows upward, so k eighths renders the lower k/8 block.
var verticalEighths = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇'}

// BarChartConfig configures RenderBarChart.
type BarChartConfig struct {
	// Values are the per-column quantities (hours). Never negative.
	Values []float64
	// Labels are the axis letters under each column (e.g. M T W T F S S).
	// Missing entries render as blank.
	Labels []string
	// ValueLabels override the labels above the bars (e.g. "1.5h"). When nil,
	// values are formatted as hours ("45m" / "1.5h" / "0").
	ValueLabels []string

	Width  int // available cells
	Height int // total row budget: value row + bars + axis row [+ total row]

	MaxBarRows int // cap on bar rows; values < 1 mean 8
	BarWidth   int // fixed bar cell width, clamped to [2,3]; 0 means 3
	Today      int // index of today's column, -1 for none (accent bar)

	ShowTotal   bool   // append the totals line
	TotalLabel  string // totals line content; default "%.1f h · last 7 days"
	ShowAverage bool   // draw a muted average rule when the grid is wide enough

	// Styles; nil → identity.
	Accent func(string) string // today's bar
	Bar    func(string) string // other bars
	Muted  func(string) string // value labels, axis letters, ticks, avg rule
	Bright func(string) string // total value
	Bg     func(string) string // gap/pad cells; nil → plain spaces
}

// barChartGrid is the shared column geometry.
type barChartGrid struct {
	pad       int   // left fill before column 0
	barW      int   // fixed bar cell width
	gap       int   // fixed gutter between columns
	colLeft   []int // left edge of each column
	colCenter []int // center cell of each column
	gridW     int   // cells occupied by the grid
}

func barChartLayout(n, width, barWidth int) barChartGrid {
	barW := barWidth
	if barW <= 0 {
		barW = 3
	}
	if barW < 2 {
		barW = 2
	}
	if barW > 3 {
		barW = 3
	}
	// Shrink gap, then bar width, until the grid fits.
	for _, cand := range []struct{ barW, gap int }{{barW, 3}, {barW, 2}, {2, 2}, {2, 1}} {
		gridW := n*cand.barW + (n-1)*cand.gap
		if gridW <= width {
			return layoutWith(n, width, cand.barW, cand.gap, gridW)
		}
	}
	return layoutWith(n, width, 2, 1, n*2+(n-1))

}

func layoutWith(n, width, barW, gap, gridW int) barChartGrid {
	pad := 0
	if width > gridW {
		pad = (width - gridW) / 2
	}
	g := barChartGrid{pad: pad, barW: barW, gap: gap, gridW: gridW}
	for i := 0; i < n; i++ {
		g.colLeft = append(g.colLeft, pad+i*(barW+gap))
		g.colCenter = append(g.colCenter, g.colLeft[i]+barW/2)
	}
	return g
}

func identity(s string) string { return s }

// bgRun paints a run of blank cells with the configured gap background.
func (c BarChartConfig) bgRun(s string) string {
	if c.Bg != nil {
		return c.Bg(s)
	}
	return s
}

func (c BarChartConfig) style(role func(string) string) func(string) string {
	if role != nil {
		return role
	}
	return identity
}

func formatHoursValue(v float64) string {
	switch {
	case v <= 0:
		return "0"
	case v < 1:
		return fmt.Sprintf("%dm", int(v*60+0.5))
	default:
		s := fmt.Sprintf("%.1f", v)
		s = strings.TrimSuffix(s, ".0")
		return s + "h"
	}
}

func chartValueLabels(c BarChartConfig) []string {
	if c.ValueLabels != nil {
		out := make([]string, len(c.Values))
		for i := range out {
			if i < len(c.ValueLabels) {
				out[i] = c.ValueLabels[i]
			}
		}
		return out
	}
	out := make([]string, len(c.Values))
	for i, v := range c.Values {
		out[i] = formatHoursValue(v)
	}
	return out
}

// placeCentered composes a row: each placed item is centered on colCenter[i];
// gaps stay blank (painted when a Bg style is configured).
func placeCentered(c BarChartConfig, n, width int, g barChartGrid, items []string, styles []func(string) string) string {
	type placement struct {
		x    int
		text string
		bg   func(string) string
	}
	ps := []placement{}
	for i := 0; i < n; i++ {
		if i >= len(items) || items[i] == "" {
			continue
		}
		st := identity
		if i < len(styles) {
			st = styles[i]
		}
		lw := runewidth.StringWidth(items[i])
		x := g.colCenter[i] - lw/2
		if x < 0 {
			x = 0
		}
		ps = append(ps, placement{x, items[i], st})
	}
	var b strings.Builder
	cursor := 0
	for _, p := range ps {
		if p.x < cursor {
			p.x = cursor // overlap: clip to what remains rather than corrupt the grid
		}
		if p.x > cursor {
			b.WriteString(c.bgRun(strings.Repeat(" ", p.x-cursor)))
		}
		b.WriteString(p.bg(p.text))
		cursor = p.x + runewidth.StringWidth(p.text)
	}
	if pad := width - cursor; pad > 0 {
		b.WriteString(c.bgRun(strings.Repeat(" ", pad)))
	}
	return b.String()
}

// RenderBarChart renders the chart. At least 4 rows (value + 1 bar row +
// axis) plus an optional totals row are produced.
func RenderBarChart(c BarChartConfig) []string {
	n := len(c.Values)
	if n == 0 {
		return nil
	}
	width := c.Width
	if width < 1 {
		width = 1
	}
	maxBar := c.MaxBarRows
	if maxBar < 1 {
		maxBar = 8
	}
	g := barChartLayout(n, width, c.BarWidth)
	valueLabels := chartValueLabels(c)
	muted := c.style(c.Muted)
	bright := c.style(c.Bright)
	accent := c.style(c.Accent)
	bar := c.style(c.Bar)

	totalRows := 1 + 1 // value row + axis row
	if c.ShowTotal {
		totalRows++
	}
	budget := c.Height
	if budget <= 0 {
		budget = totalRows + maxBar
	}
	barRows := budget - totalRows
	if barRows > maxBar {
		barRows = maxBar
	}
	if barRows < 1 {
		barRows = 1
	}

	maxV := 0.0
	total := 0.0
	for _, v := range c.Values {
		if v > maxV {
			maxV = v
		}
		total += v
	}
	// Bar heights in eighths of a cell, capped at barRows*8.
	eighths := make([]int, n)
	for i, v := range c.Values {
		if v <= 0 || maxV <= 0 {
			continue
		}
		e := int(v/maxV*float64(barRows)*8 + 0.5)
		if e < 1 {
			e = 1 // any non-zero value shows at least one sliver
		}
		if e > barRows*8 {
			e = barRows * 8
		}
		eighths[i] = e
	}

	// Average, in eighths above the baseline (shown on the totals line
	// when the grid is wide enough to carry it).
	avg := total / float64(n)

	lines := []string{}
	valueStyles := make([]func(string) string, n)
	for i := range valueStyles {
		valueStyles[i] = muted
	}
	lines = append(lines, placeCentered(c, n, width, g, valueLabels, valueStyles))

	barRun := func(text string, i int) string {
		if i == c.Today {
			return accent(text)
		}
		return bar(text)
	}
	for r := 0; r < barRows; r++ {
		rowBottom := (barRows - r) * 8  // level (eighths from baseline) at the row's bottom edge
		rowTop := (barRows - 1 - r) * 8 // level at the row's top edge
		isBottom := r == barRows-1
		line := ""
		cursor := 0
		writeGap := func(x int) {
			if x > cursor {
				line += c.bgRun(strings.Repeat(" ", x-cursor))
				cursor = x
			}
		}
		for i := 0; i < n; i++ {
			left := g.colLeft[i]
			writeGap(left)
			e := eighths[i]
			var cell string
			switch {
			case e == 0:
				if isBottom {
					cell = muted(strings.Repeat("▁", g.barW)) // muted baseline tick for zero days
				} else {
					cell = c.bgRun(strings.Repeat(" ", g.barW))
				}
			case e >= rowBottom:
				cell = barRun(strings.Repeat("█", g.barW), i)
			case e > rowTop:
				// Partial top cap: grow the lower k/8 block.
				k := e - rowTop
				cell = barRun(strings.Repeat(string(verticalEighths[k-1]), g.barW), i)
			default:
				cell = c.bgRun(strings.Repeat(" ", g.barW))
			}
			line += cell
			cursor = left + g.barW
		}
		// Paint the remainder of the row so every cell carries a background.
		if pad := width - cursor; pad > 0 {
			line += c.bgRun(strings.Repeat(" ", pad))
		}
		lines = append(lines, line)
	}

	letterStyles := make([]func(string) string, n)
	for i := range letterStyles {
		letterStyles[i] = muted
	}
	letters := make([]string, n)
	for i := 0; i < n; i++ {
		if i < len(c.Labels) {
			letters[i] = c.Labels[i]
		}
	}
	lines = append(lines, placeCentered(c, n, width, g, letters, letterStyles))

	if c.ShowTotal {
		label := c.TotalLabel
		if label == "" {
			label = fmt.Sprintf("%.1f h · last 7 days", total)
		}
		brightPart := fmt.Sprintf("%.1f h", total)
		if c.TotalLabel == "" {
			suffix := "· last 7 days"
			if c.ShowAverage && avg > 0 && width >= 44 {
				suffix += fmt.Sprintf(" · avg %.1fh", avg)
			}
			lines = append(lines, " "+bright(brightPart)+" "+muted(suffix))
		} else {
			_ = brightPart
			lines = append(lines, " "+muted(label))
		}
	}
	return lines
}
