package tui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/rivo/uniseg"

	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

// Frame invariants, verified at 6 sizes × every view with the populated
// fixture (73-task data never hits the tests — the two-task fixture is the
// same code path with less render work).
//
//   1. Every output line is exactly the terminal width in cells.
//   2. Every detected rounded panel border (╭…╮ / ╰…╯) spans exactly the
//      same cells on every row of the panel, and border width == panel
//      width.
//   3. No cell inside the frame carries the terminal default background
//      (the panel surfaces are painted end to end).
//   4. A "plain" panel row (its first cell is on the panel background) is
//      uniform except for intentional chips and highlights — every
//      off-background run is at most chipWidth cells. Highlight rows
//      (selected rows, tinted cells, card blocks) are exempt.
//
// Parser note: widths are measured per grapheme cluster with uniseg — the
// same rule lipgloss/x/ansi uses — so the parser agrees with the renderer
// exactly. No ANSI beyond CSI+SGR is expected in --dump output.

const chipWidth = 16

type frameGrid struct {
	// bg[r][c] = "" when the cell has the default background, otherwise the
	// SGR background as rendered ("rgb:r;g;b" / "256:n" / "ansi:n").
	bg [][]string
	ch [][]string
	// fg[r][c] = "" when the cell carries the default foreground.
	fg [][]string
}

func parseFrame(frame string) frameGrid {
	lines := strings.Split(frame, "\n")
	grid := frameGrid{}
	for _, line := range lines {
		bg := ""
		fg := ""
		rowBG := []string{}
		rowFG := []string{}
		rowCH := []string{}
		rest := line
		for len(rest) > 0 {
			if rest[0] == '\x1b' {
				// CSI: ESC [ <params> <final-letter>
				j := 1
				for j < len(rest) && !(rest[j] >= 'a' && rest[j] <= 'z') && !(rest[j] >= 'A' && rest[j] <= 'Z') {
					j++
				}
				if j+1 > len(rest) {
					j = len(rest) - 1
				}
				seq := rest[:j+1]
				rest = rest[j+1:]
				if strings.HasSuffix(seq, "m") {
					bg = sgrBackground(bg, seq)
					fg = sgrForeground(fg, seq)
				}
				continue
			}
			cluster, next, w, _ := uniseg.FirstGraphemeClusterInString(rest, -1)
			for k := 0; k < w; k++ {
				rowBG = append(rowBG, bg)
				rowFG = append(rowFG, fg)
				rowCH = append(rowCH, cluster)
			}
			rest = next
		}
		grid.bg = append(grid.bg, rowBG)
		grid.fg = append(grid.fg, rowFG)
		grid.ch = append(grid.ch, rowCH)
	}
	return grid
}

// sgrBackground applies an SGR sequence to the current background. Params are
// consumed the way ANSI parses them (38/48/58 eat their extended-colour
// arguments).
func sgrBackground(cur, seq string) string {
	params := strings.TrimSuffix(strings.TrimPrefix(seq, "\x1b["), "m")
	parts := strings.Split(params, ";")
	for i := 0; i < len(parts); i++ {
		switch p := parts[i]; p {
		case "", "0", "00", "49":
			cur = ""
		case "38", "58":
			i += skipColorArgs(parts, i)
		case "48":
			if i+1 < len(parts) {
				switch parts[i+1] {
				case "5":
					if i+2 < len(parts) {
						cur = "256:" + parts[i+2]
					}
					i += 2
				case "2":
					if i+4 < len(parts) {
						cur = "rgb:" + strings.Join(parts[i+2:i+5], ";")
					}
					i += 4
				default:
					i++
				}
			}
		}
	}
	return cur
}

// sgrForeground mirrors sgrBackground for the 38/58 foreground.
func sgrForeground(cur, seq string) string {
	params := strings.TrimSuffix(strings.TrimPrefix(seq, "\x1b["), "m")
	parts := strings.Split(params, ";")
	for i := 0; i < len(parts); i++ {
		switch p := parts[i]; p {
		case "", "0", "00", "39":
			cur = ""
		case "48":
			i += skipColorArgs(parts, i)
		case "38", "58":
			if i+1 < len(parts) {
				switch parts[i+1] {
				case "5":
					if i+2 < len(parts) {
						cur = "256:" + parts[i+2]
					}
					i += 2
				case "2":
					if i+4 < len(parts) {
						cur = "rgb:" + strings.Join(parts[i+2:i+5], ";")
					}
					i += 4
				default:
					i++
				}
			}
		}
	}
	return cur
}

func skipColorArgs(parts []string, i int) int {
	if i+1 >= len(parts) {
		return 0
	}
	switch parts[i+1] {
	case "5":
		return 2
	case "2":
		return 4
	default:
		return 1
	}
}

func (g frameGrid) cell(r, c int) (ch, bg string) {
	if r < 0 || r >= len(g.ch) {
		return "", ""
	}
	if c < 0 || c >= len(g.ch[r]) {
		return "", ""
	}
	return g.ch[r][c], g.bg[r][c]
}

// foreground reports the parsed foreground SGR at (r, c) ("" = default).
func (g frameGrid) foreground(r, c int) string {
	if r < 0 || r >= len(g.fg) {
		return ""
	}
	if c < 0 || c >= len(g.fg[r]) {
		return ""
	}
	return g.fg[r][c]
}

func (g frameGrid) rows() int { return len(g.ch) }

func TestFrameInvariants(t *testing.T) {
	sizes := []struct{ w, h int }{{80, 24}, {120, 40}, {200, 50}, {97, 31}, {143, 37}, {61, 16}}
	views := []string{"home", "list", "board", "calendar", "courses", "focus", "stats", "ideas", "project", "roadmap", "devlog"}
	for _, size := range sizes {
		for _, view := range views {
			states := []string{"empty", "populated"}
			if view == "project" || view == "roadmap" || view == "devlog" {
				// The rich fixture exercises the timeline bars, milestone
				// chips and glamour-rendered dev-log through the invariants.
				states = append(states, "projectTools")
			}
			for _, state := range states {
				t.Run(fmt.Sprintf("%s_%s_%dx%d", view, state, size.w, size.h), func(t *testing.T) {
					s := fixtures[state](t)
					defer func() { _ = s.Close() }()
					frame, err := RenderDump(dumpConfig(), s, sync.New(s.Root), view, size.w, size.h, false, DumpFixedTime)
					if err != nil {
						t.Fatal(err)
					}
					grid := parseFrame(frame)

					// (1) Every line exactly the terminal width.
					if grid.rows() != size.h {
						t.Fatalf("frame has %d rows, want %d", grid.rows(), size.h)
					}
					lines := strings.Split(frame, "\n")
					for i, l := range lines {
						if got := lipgloss.Width(l); got != size.w {
							t.Errorf("line %d width = %d cells, want %d", i, got, size.w)
						}
						if got := len(grid.bg[i]); got != size.w {
							t.Errorf("line %d parsed width = %d cells, want %d", i, got, size.w)
						}
					}

					// (3) No cell carries the default background.
					for r := 0; r < grid.rows(); r++ {
						for c, bg := range grid.bg[r] {
							if bg == "" {
								t.Errorf("cell (%d,%d) has the default background", r, c)
								return // one is enough to fail the frame
							}
						}
					}

					// (2) Bordered panels are rectangular: every ╭ finds its ╮,
					// every ╰ its ╯, and the left/right edges are solid on all
					// rows between the corners.
					checkPanelBorders(t, grid)

					// (4) Plain panel rows are uniform on the panel background
					// except for small intentional chips.
					checkPanelUniformity(t, grid)

					// (5) The selected row is one continuous raised block, end to end.
					// List only: board cards and the calendar today-pill are
					// deliberate filled chips on raised blocks, not rows.
					if state == "populated" && view == "list" {
						checkSelectedRowUniform(t, grid, size.w)
					}
				})
			}
		}
	}
}

func checkPanelBorders(t *testing.T, grid frameGrid) {
	t.Helper()
	for r := 0; r < grid.rows(); r++ {
		for c := 0; c < len(grid.ch[r]); c++ {
			ch, _ := grid.cell(r, c)
			if ch != "╭" {
				continue
			}
			// Find the matching ╮ on the same row.
			right := -1
			for c2 := c + 1; c2 < len(grid.ch[r]); c2++ {
				c2ch, _ := grid.cell(r, c2)
				if c2ch == "╮" {
					right = c2
					break
				}
			}
			if right < 0 {
				t.Errorf("panel top edge at (%d,%d) has no closing ╮", r, c)
				continue
			}
			// Walk down: left and right edges must be │ until ╰…╯.
			bot := -1
			for r2 := r + 1; r2 < grid.rows(); r2++ {
				lch, _ := grid.cell(r2, c)
				rch, _ := grid.cell(r2, right)
				if lch == "╰" {
					if rch != "╯" {
						t.Errorf("panel at (%d,%d): bottom-right corner = %q, want ╯ (left/right borders misaligned)", r, c, rch)
					}
					bot = r2
					break
				}
				if lch != "│" {
					t.Errorf("panel at (%d,%d): left edge broken at row %d (%q) — border shorter than the panel?", r, c, r2, lch)
					break
				}
				if rch != "│" {
					t.Errorf("panel at (%d,%d): right edge broken at row %d (%q) — border narrower than the panel?", r, c, r2, rch)
					break
				}
			}
			if bot < 0 {
				t.Errorf("panel at (%d,%d): no bottom edge ╰…╯ found", r, c)
			}
		}
	}
}

func checkPanelUniformity(t *testing.T, grid frameGrid) {
	t.Helper()
	// For each bordered panel, sample the panel background from the first
	// interior row's left cell, then check every interior row whose left
	// cell is on the panel background: off-background runs must be small
	// (chips). Rows whose left cell differs (highlights, selected rows,
	// tinted cells, card blocks) are exempt.
	for r := 0; r < grid.rows(); r++ {
		for c := 0; c < len(grid.ch[r]); c++ {
			ch, _ := grid.cell(r, c)
			if ch != "╭" {
				continue
			}
			right := -1
			for c2 := c + 1; c2 < len(grid.ch[r]); c2++ {
				c2ch, _ := grid.cell(r, c2)
				if c2ch == "╮" {
					right = c2
					break
				}
			}
			if right < 0 {
				continue
			}
			// Find the panel bottom.
			bot := -1
			for r2 := r + 1; r2 < grid.rows(); r2++ {
				lch, _ := grid.cell(r2, c)
				if lch == "╰" {
					bot = r2
					break
				}
			}
			if bot < 0 {
				continue
			}
			// Panel background = the most common background in the interior
			// (surface for every normal pane; base for focus). Sampling one
			// cell would misread a leading inset cell.
			counts := map[string]int{}
			for r2 := r + 1; r2 < bot; r2++ {
				for c2 := c + 1; c2 < right; c2++ {
					_, bg := grid.cell(r2, c2)
					counts[bg]++
				}
			}
			panelBG := ""
			best := -1
			for bg, n := range counts {
				if n > best {
					best = n
					panelBG = bg
				}
			}
			for r2 := r + 1; r2 < bot; r2++ {
				_, leftBG := grid.cell(r2, c+1)
				if leftBG != panelBG {
					continue // highlight row — exempt from uniformity
				}
				baseGutter := renderedBGProbe(theme.Defaults().Base)
				for _, run := range offBGRuns(grid, r2, c+1, right-1, panelBG, baseGutter) {
					if run.length > chipWidth && !run.uniform {
						t.Errorf("panel (%d,%d) row %d: off-background run of %d cells (plain rows may only carry chips ≤ %d or one deliberate highlight block)", r, c, r2, run.length, chipWidth)
					}
				}
			}
		}
	}
}

// renderedBGProbe returns the canonical background fingerprint (as the cell
// parser records it) for the given hex under the pinned TrueColor profile.
func renderedBGProbe(hex string) string {
	probe := lipgloss.NewStyle().Background(lipgloss.Color(hex)).Render(" ")
	for _, m := range sgrPatternAll.FindAllStringIndex(probe, -1) {
		seq := probe[m[0]:m[1]]
		if strings.Contains(seq, "48") {
			return sgrBackground("", seq)
		}
	}
	return ""
}

var sgrPatternAll = regexp.MustCompile("\x1b\\[[0-9;]*m")

type offRun struct {
	length  int
	uniform bool // one background dominates the run (≥75%) — a deliberate highlight block
}

// offBGRuns finds runs of off-panel-background cells in one row. Cells on
// the base gutter color are transparent: they belong to the grid system, not
// to the panel, so they neither extend nor break a run.
func offBGRuns(grid frameGrid, r, c0, c1 int, panelBG, baseGutter string) []offRun {
	runs := []offRun{}
	cur := 0
	counts := map[string]int{}
	flush := func() {
		if cur == 0 {
			return
		}
		best := 0
		for _, n := range counts {
			if n > best {
				best = n
			}
		}
		runs = append(runs, offRun{length: cur, uniform: best*4 >= cur*3})
		cur = 0
		counts = map[string]int{}
	}
	for c := c0; c <= c1; c++ {
		_, bg := grid.cell(r, c)
		if bg == baseGutter {
			continue // transparent gutter
		}
		if bg == panelBG {
			flush()
			continue
		}
		cur++
		counts[bg]++
	}
	flush()
	return runs
}

// checkSelectedRowUniform asserts that a row carrying the SurfaceRaised
// background (a selected list/home row, a selected card) carries it as one
// CONTINUOUS block — every cell between the first and last raised cell in
// the row must be raised too. Chips inside the row included: a single
// off-raised cell breaks the continuous highlight. Cells outside the block
// (sidebar, detail pane, gutters) are out of scope.
func checkSelectedRowUniform(t *testing.T, grid frameGrid, w int) {
	t.Helper()
	raised := renderedBGProbe(theme.Defaults().SurfaceRaised)
	if raised == "" {
		t.Fatal("could not probe raised background fingerprint")
	}
	checked := 0
	for r := 0; r < grid.rows(); r++ {
		first, last := -1, -1
		for c := 0; c < w; c++ {
			_, bg := grid.cell(r, c)
			if bg == raised {
				if first < 0 {
					first = c
				}
				last = c
			}
		}
		if first < 0 {
			continue
		}
		checked++
		for c := first; c <= last; c++ {
			_, bg := grid.cell(r, c)
			if bg != raised {
				t.Errorf("row %d: cell %d has background %q, want raised %q — the highlight must be continuous from cell %d to %d", r, c, bg, raised, first, last)
			}
		}
	}
	if checked == 0 {
		t.Error("no raised (selected) row found — the populated list fixture must have one")
	}
}
