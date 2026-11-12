package tui

import (
	"math"
	"strings"

	"github.com/loharrahulofficial-crypto/mtm/internal/ambient"
)

// wordmark.go — the hand-designed mtm wordmark glyphs.
//
// The M is built from diagonal strokes that meet at the center of the
// glyph (the old stems+center-leg M read as an H at a glance). Two
// sizes are used, both at bitmap pixels that are square ON SCREEN
// (see paintWordmark): an 11x11 set with 2-pixel strokes above
// w≥160 && h≥44, and a 7x9 set with 1-pixel strokes below that.
// A lowercase-style set offers the same letterforms at x-height for the
// `ui.wordmark: lower` setting (the default is caps, `ui.wordmark: caps`).
//
// Caps sets (hand-designed):
//
//	11x11   M ##.......## / ###.....### / ####...#### / ##.##.##.## /
//	          ##..###..## / ##...#...## / (then five rows of ##.......##)
//	        T ########### ×2 / then nine rows of ....###....
//	7x9     M #.....# / #.....# / #.#.#.# / #.#.#.# / #..#..# /
//	          #..#..# / #.....# / #.....# / #.....#
//	        T ####### ×2 / then seven rows of ..#..
//
// Letter gap: 3 subpixels. The block is placed so its vertical center sits
// at ~40% of the frame height (the sky band) — never on the horizon line.

// wordmarkGlyphs holds the per-letter glyphs for "mtm" so tests can assert
// the letters are distinct, legible shapes at every size.
type wordmarkGlyphs struct{ letters [][]string }

// wordmarkCaps11x11 — the reference set: 2-pixel stems, diagonals meeting
// at row 5 (55% of the glyph height).
var wordmarkCaps11x11 = wordmarkGlyphs{letters: [][]string{
	{"##.......##", "###.....###", "####...####", "##.##.##.##", "##..###..##", "##...#...##", "##.......##", "##.......##", "##.......##", "##.......##", "##.......##"},
	{"###########", "###########", "....###....", "....###....", "....###....", "....###....", "....###....", "....###....", "....###....", "....###....", "....###...."},
	{"##.......##", "###.....###", "####...####", "##.##.##.##", "##..###..##", "##...#...##", "##.......##", "##.......##", "##.......##", "##.......##", "##.......##"},
}}

// wordmarkCaps7x9 — hand-scaled: 1-pixel strokes, diagonals meeting at
// row 4 (the apex lights rows 2–5, 55% of the glyph height).
var wordmarkCaps7x9 = wordmarkGlyphs{letters: [][]string{
	{"#.....#", "#.....#", "#.#.#.#", "#.#.#.#", "#..#..#", "#..#..#", "#.....#", "#.....#", "#.....#"},
	{"#######", "#######", "...#...", "...#...", "...#...", "...#...", "...#...", "...#...", "...#..."},
	{"#.....#", "#.....#", "#.#.#.#", "#.#.#.#", "#..#..#", "#..#..#", "#.....#", "#.....#", "#.....#"},
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
		return wordmarkCaps11x11
	}
	if lower {
		if w >= 100 && h >= 24 {
			return wordmarkLower7x7
		}
		return wordmarkLower5x5
	}
	return wordmarkCaps7x9
}

// wordmarkPixelH is the on-screen height of one bitmap pixel in cells:
// every bitmap pixel is a square on screen, so its height is one cell
// width's worth of rows (cellW/cellH).
func wordmarkPixelH(cellW, cellH float64) float64 {
	if cellW <= 0 || cellH <= 0 {
		cellW, cellH = 9, 17
	}
	return cellW / cellH
}

// splashLayout holds the wordmark block's geometry for one frame.
type splashLayout struct {
	wm     wordmarkGlyphs
	x0     int     // first cell column of the wordmark
	top    float64 // fractional first row
	rows   float64 // fractional height in cells
	wordW  int     // total width in cells
	pixelH float64 // cell rows per bitmap pixel (cellW/cellH)
	tagY   int     // tagline row
}

// splashLayoutFor computes the wordmark block geometry: centered at
// 40% of the frame height, letter gap 3 subpixels, and the tagline at
// least 25% of the wordmark height below it (minimum 2 rows).
func splashLayoutFor(w, h int, cellW, cellH float64, lower bool) splashLayout {
	wm := wordmarkFor(w, h, lower)
	pixelH := wordmarkPixelH(cellW, cellH)
	rows := wordmarkRows(wm, cellW, cellH)
	top := wordmarkTop(h, rows)
	wordW := wordmarkWidthCells(wm)
	x0 := (w - wordW) / 2
	if x0 < 0 {
		x0 = 0
	}
	gap := int(math.Ceil(rows * 0.25))
	if gap < 2 {
		gap = 2
	}
	tagY := int(math.Ceil(top+rows)) + gap
	return splashLayout{wm: wm, x0: x0, top: top, rows: rows, wordW: wordW, pixelH: pixelH, tagY: tagY}
}

// wordmarkWidthCells is the total width in cells, with 3-subpixel gaps.
func wordmarkWidthCells(g wordmarkGlyphs) int {
	if len(g.letters) == 0 {
		return 0
	}
	return len(g.letters)*len(g.letters[0][0]) + (len(g.letters)-1)*3
}

// wordmarkRows is the wordmark block's height in (float) cells.
func wordmarkRows(g wordmarkGlyphs, cellW, cellH float64) float64 {
	if len(g.letters) == 0 || len(g.letters[0]) == 0 {
		return 0
	}
	return float64(len(g.letters[0])) * wordmarkPixelH(cellW, cellH)
}

// wordmarkTop returns the first (fractional) cell row of the wordmark.
// The block's vertical center sits at 40% of the frame height; the block
// is clamped to zero for the smallest frames and can never reach the
// horizon line (the full-screen horizon sits at ≥ 60% of the frame).
func wordmarkTop(h int, rows float64) float64 {
	top := float64(h)*0.40 - rows/2
	if top < 0 {
		top = 0
	}
	maxTop := math.Floor(float64(h)*0.60) - rows
	if top > maxTop {
		top = maxTop
	}
	if top < 0 {
		top = 0
	}
	return top
}

// wordmarkRects builds the lit bitmap pixels for this frame — every lit
// pixel is a square on screen (1 cell wide, cellW/cellH rows tall), the
// letters rise with the appear animation and the pulse shimmer, and the
// whole wordmark fades with the dissolve tail. Coordinates are in cells.
func (m Model) wordmarkRects(g wordmarkGlyphs, x0 int, top, pixelH float64, t, fade float64) *ambient.WordmarkPaint {
	rects := []ambient.WordmarkRect{}
	letterDur, step, start := 0.55, 0.28, 0.9
	for i, letter := range g.letters {
		appear := clampUnit((t-(start+float64(i)*step))/letterDur, 0, 1)
		if appear <= 0 {
			continue
		}
		rize := ambient.EaseOutBack(appear)
		yOff := (1 - rize) * 2.0 // cells
		bright := clampUnit(rize, 0, 1)
		landT := t - (start + float64(i)*step + letterDur*0.55)
		if landT > 0 && landT < 1.2 {
			pulse := ambient.Envelope(landT, 0, 0.25, 0.55, 1.1)
			bright = clampUnit(rize+pulse*0.35, 0, 1)
		}
		bright *= fade
		lw := len(letter[0]) // spec rows are pure ASCII: bytes == cells
		for row, line := range letter {
			for col, ch := range line {
				if ch != '#' {
					continue
				}
				rects = append(rects, ambient.WordmarkRect{
					X:     float64(x0 + i*(lw+3) + col),
					Y:     top + float64(row)*pixelH + yOff,
					W:     1,
					H:     pixelH,
					Alpha: bright,
				})
			}
		}
	}
	if len(rects) == 0 {
		return nil
	}
	return &ambient.WordmarkPaint{
		Rects:       rects,
		BrightText:  m.palette.BrightText,
		Base:        m.palette.Base,
		ShadowAlpha: 0.45,
		MinContrast: 3.0,
	}
}

// relLumHex returns the WCAG relative luminance of a #rrggbb color.
func relLumHex(hex string) float64 {
	var v [3]float64
	s := strings.TrimPrefix(hex, "#")
	if len(s) != 6 {
		return 0
	}
	for i := 0; i < 3; i++ {
		n := 0
		for j := 0; j < 2; j++ {
			c := s[2*i+j]
			vv := 0
			switch {
			case c >= '0' && c <= '9':
				vv = int(c - '0')
			case c >= 'a' && c <= 'f':
				vv = int(c-'a') + 10
			case c >= 'A' && c <= 'F':
				vv = int(c-'A') + 10
			}
			n = n*16 + vv
		}
		f := float64(n) / 255
		if f <= 0.03928 {
			f /= 12.92
		} else {
			f = math.Pow((f+0.055)/1.055, 2.4)
		}
		v[i] = f
	}
	return 0.2126*v[0] + 0.7152*v[1] + 0.0722*v[2]
}
