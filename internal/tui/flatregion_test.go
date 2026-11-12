package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/loharrahulofficial-crypto/mtm/internal/ambient"
)

// flatregion_test.go — full-screen scene frames must never carry a
// single flat color across more than 20% of the frame height: no
// full-width horizontal band, and no column run, may be uniform.

// cellKey builds a per-cell effective-color key that matches what the
// user sees: '█' shows its foreground (the halfblock emitter backs solid
// cells with Base, so bg is not representative), ' ' backs with Base
// too, and everything else is the (glyph, fg, bg) triple. Two cells are
// "the same color" iff their keys match.
func cellKey(grid frameGrid, y, x int) string {
	ch, bg := grid.cell(y, x)
	if ch == "█" {
		return "█|" + grid.foreground(y, x)
	}
	if ch == " " {
		return " |" + bg
	}
	return ch + "|" + grid.foreground(y, x) + "|" + bg
}

// gridColorsAt builds a compact per-cell color key per row (rows x cols).
func gridColorsAt(grid frameGrid, rows, cols int, fallback string) [][]string {
	out := make([][]string, 0, rows)
	for y := 0; y < rows; y++ {
		row := make([]string, 0, cols)
		for x := 0; x < cols; x++ {
			row = append(row, cellKey(grid, y, x))
		}
		out = append(out, row)
	}
	return out
}

func TestScenesNoFlatRegion(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	for _, name := range ambient.Registered() {
		for _, size := range [][2]int{{100, 30}, {200, 50}} {
			w, h := size[0], size[1]
			bandH := int(float64(h) * 0.2)
			if bandH < 2 {
				bandH = 2
			}
			st := ambient.SceneState{
				Progress:  0.45,
				Phase:     ambient.PhaseWork,
				Now:       DumpFixedTime,
				Seed:      42,
				Palette:   ambient.DefaultPalette(),
				FullFrame: true,
			}
			frame := ambient.New(name, ambient.ModeHalfblock).Frame(3*time.Second, w, h, st)
			grid := parseFrame(strings.Join(frame, "\n"))
			rows, cols := grid.rows(), 0
			if rows != h {
				t.Fatalf("%s %dx%d: %d rows parsed, want %d", name, w, h, rows, h)
			}
			if rows > 0 {
				cols = len(grid.bg[0])
			}
			colors := gridColorsAt(grid, rows, cols, "#14282d")

			// 1) horizontal bands: sliding multiset of distinct colors
			window := map[string]int{}
			add := func(c string) { window[c]++ }
			del := func(c string) {
				window[c]--
				if window[c] == 0 {
					delete(window, c)
				}
			}
			for y := 0; y < bandH; y++ {
				for x := 0; x < cols; x++ {
					add(colors[y][x])
				}
			}
			if len(window) == 1 {
				t.Errorf("%s %dx%d: rows 0..%d are one flat color %q", name, w, h, bandH-1, soleKey(window))
			}
			for y := bandH; y < rows; y++ {
				for x := 0; x < cols; x++ {
					del(colors[y-bandH][x])
					add(colors[y][x])
				}
				if len(window) == 1 {
					t.Errorf("%s %dx%d: rows %d..%d are one flat color %q", name, w, h, y-bandH+1, y, soleKey(window))
				}
			}

			// 2) vertical runs: a single color must not stretch over
			// more than bandH cells in any column
			for x := 0; x < cols; x++ {
				run := 1
				for y := 1; y < rows; y++ {
					if colors[y][x] == colors[y-1][x] {
						run++
						if run > bandH {
							t.Errorf("%s %dx%d: column %d rows %d..%d are one flat color %q", name, w, h, x, y-run+1, y, colors[y][x])
							break
						}
						continue
					}
					run = 1
				}
			}
		}
	}
}

func soleKey(window map[string]int) string {
	for k := range window {
		return k
	}
	return ""
}
