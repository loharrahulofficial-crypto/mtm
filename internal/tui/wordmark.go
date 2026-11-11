package tui

import (
	"math"
	"strings"
)

// wordmark.go — the hand-designed mtm wordmark glyphs.
//
// The M is built from diagonal strokes that meet at the center of the
// glyph (the old stems+center-leg M read as an H at a glance). Three
// sizes are drawn by hand — never stretched from one another — and a
// lowercase-style set offers the same letterforms at x-height for the
// `ui.wordmark: lower` setting (the default is caps, `ui.wordmark: caps`).
//
// Caps sets (the base 5x5 bitmaps are the canonical spec shapes):
//
//	5x5   M #...# / ##.## / #.#.# / #...# / #...#
//	      T ##### / ..#.. / ..#.. / ..#.. / ..#..
//
// Size selection: 9x9 at w >= 160 && h >= 44, 7x7 at w >= 100 && h >= 24,
// otherwise 5x5. The block is placed so its vertical center sits at ~39%
// of the frame height (the sky band) — never on the horizon line.

// wordmarkGlyphs holds the per-letter glyphs for "mtm" so tests can assert
// the letters are distinct, legible shapes at every size.
type wordmarkGlyphs struct{ letters [][]string }

// wordmarkCaps5x5 is the spec base: M is corner stems + diagonals meeting
// at the center row, T is a full bar over a stem.
var wordmarkCaps5x5 = wordmarkGlyphs{letters: [][]string{
	{"#...#", "##.##", "#.#.#", "#...#", "#...#"},
	{"#####", "..#..", "..#..", "..#..", "..#.."},
	{"#...#", "##.##", "#.#.#", "#...#", "#...#"},
}}

// wordmarkCaps7x7 — hand-scaled: single-pixel diagonals from the corners
// meet at the center row (row 3).
var wordmarkCaps7x7 = wordmarkGlyphs{letters: [][]string{
	{"#.....#", "##...##", "#.#.#.#", "#..#..#", "#.....#", "#.....#", "#.....#"},
	{"#######", "...#...", "...#...", "...#...", "...#...", "...#...", "...#..."},
	{"#.....#", "##...##", "#.#.#.#", "#..#..#", "#.....#", "#.....#", "#.....#"},
}}

// wordmarkCaps9x9 — hand-scaled: diagonals from the corners meet at the
// center row (row 4).
var wordmarkCaps9x9 = wordmarkGlyphs{letters: [][]string{
	{"#.......#", "##.....##", "#.#...#.#", "#..#.#..#", "#...#...#", "#.......#", "#.......#", "#.......#", "#.......#"},
	{"#########", "....#....", "....#....", "....#....", "....#....", "....#....", "....#....", "....#....", "....#...."},
	{"#.......#", "##.....##", "#.#...#.#", "#..#.#..#", "#...#...#", "#.......#", "#.......#", "#.......#", "#.......#"},
}}

// wordmarkLower5x5 — x-height letterforms: m is two arch crowns over
// stems, t is a short bar over a stem with a rightward foot.
var wordmarkLower5x5 = wordmarkGlyphs{letters: [][]string{
	{"##.##", "#.#.#", "#.#.#", "#.#.#", "#.#.#"},
	{".#...", "####.", ".#...", ".#...", ".##.."},
	{"##.##", "#.#.#", "#.#.#", "#.#.#", "#.#.#"},
}}

// wordmarkLower7x7 — hand-scaled: two merged arch crowns (row 0) over
// stems, and a t with a foot row.
var wordmarkLower7x7 = wordmarkGlyphs{letters: [][]string{
	{"###.###", "#.#.#.#", "#.#.#.#", "#.#.#.#", "#.#.#.#", "#.#.#.#", "#.#.#.#"},
	{"...#...", ".#####.", "...#...", "...#...", "...#...", "...#...", "..###.."},
	{"###.###", "#.#.#.#", "#.#.#.#", "#.#.#.#", "#.#.#.#", "#.#.#.#", "#.#.#.#"},
}}

// wordmarkLower9x9 — hand-scaled: wide arch crowns over three stems, and a
// t with a 3-cell foot.
var wordmarkLower9x9 = wordmarkGlyphs{letters: [][]string{
	{"####.####", "#...#...#", "#...#...#", "#...#...#", "#...#...#", "#...#...#", "#...#...#", "#...#...#", "#...#...#"},
	{"....#....", ".#######.", "....#....", "....#....", "....#....", "....#....", "....#....", "....#....", "...###..."},
	{"####.####", "#...#...#", "#...#...#", "#...#...#", "#...#...#", "#...#...#", "#...#...#", "#...#...#", "#...#...#"},
}}

// wordmarkFor chooses the wordmark set for the terminal size and style.
// lower selects the x-height letterforms (ui.wordmark: lower).
func wordmarkFor(w, h int, lower bool) wordmarkGlyphs {
	if w >= 160 && h >= 44 {
		if lower {
			return wordmarkLower9x9
		}
		return wordmarkCaps9x9
	}
	if w >= 100 && h >= 24 {
		if lower {
			return wordmarkLower7x7
		}
		return wordmarkCaps7x7
	}
	if lower {
		return wordmarkLower5x5
	}
	return wordmarkCaps5x5
}

// glyphCells renders one spec row as terminal cells: '#' → '█',
// anything else → ' '. Spec rows are pure ASCII so len(row) == cells.
func glyphCells(row string) string {
	var sb strings.Builder
	for _, r := range row {
		if r == '#' {
			sb.WriteRune('█')
		} else {
			sb.WriteRune(' ')
		}
	}
	return sb.String()
}

// wordmarkTop returns the first terminal row of the wordmark. The vertical
// center of the block sits at ~39% of the frame (inside the sky band);
// the block is clamped to zero for the smallest frames and can never
// reach the horizon line (the full-screen horizon sits at ≥ 60% of the
// frame height).
func wordmarkTop(h, rows int) int {
	top := int(math.Round(float64(h)*0.39 - float64(rows)/2))
	if top < 0 {
		top = 0
	}
	maxTop := int(math.Floor(float64(h)*0.60)) - rows // last row stays above the horizon
	if top > maxTop {
		top = maxTop
	}
	if top < 0 {
		top = 0
	}
	return top
}
