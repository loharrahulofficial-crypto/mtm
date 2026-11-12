package tui

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

// wordmark_test.go — legibility contract for the hand-designed mtm
// wordmark: golden bitmaps, centered diagonals on the M, three-subpixel
// gaps, sky-band placement, and on-screen letter aspect at the three
// reference cell sizes.

var allWordmarks = map[string]wordmarkGlyphs{
	"caps11": wordmarkCaps11x11,
	"caps7":  wordmarkCaps7x9,
	"lower5": wordmarkLower5x5,
	"lower7": wordmarkLower7x7,
	"lower9": wordmarkLower9x9,
}

// wordmarkH is the H shape the M must differ from at every size (not
// shipped; test-local anchor for the "reads as H" regression).
var wordmarkH = map[string][]string{
	"caps11": {"##.......##", "##.......##", "##.......##", "##.......##", "##.......##", "###########", "##.......##", "##.......##", "##.......##", "##.......##", "##.......##"},
	"caps7":  {"#.....#", "#.....#", "#.....#", "#.....#", "#######", "#.....#", "#.....#", "#.....#", "#.....#"},
}

func litCount(a, b []string) int {
	n := 0
	for i := range a {
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				n++
			}
		}
	}
	return n
}

func TestWordmarkGoldenBitmaps(t *testing.T) {
	want := map[string][][]string{
		"caps11": {
			{"##.......##", "###.....###", "####...####", "##.##.##.##", "##..###..##", "##...#...##", "##.......##", "##.......##", "##.......##", "##.......##", "##.......##"},
			{"###########", "###########", "....###....", "....###....", "....###....", "....###....", "....###....", "....###....", "....###....", "....###....", "....###...."},
			{"##.......##", "###.....###", "####...####", "##.##.##.##", "##..###..##", "##...#...##", "##.......##", "##.......##", "##.......##", "##.......##", "##.......##"},
		},
		"caps7": {
			{"#.....#", "#.....#", "#.#.#.#", "#.#.#.#", "#..#..#", "#..#..#", "#.....#", "#.....#", "#.....#"},
			{"#######", "#######", "...#...", "...#...", "...#...", "...#...", "...#...", "...#...", "...#..."},
			{"#.....#", "#.....#", "#.#.#.#", "#.#.#.#", "#..#..#", "#..#..#", "#.....#", "#.....#", "#.....#"},
		},
		"lower5": {
			{"##.##", "#.#.#", "#.#.#", "#.#.#", "#.#.#"},
			{".#...", "####.", ".#...", ".#...", ".##.."},
			{"##.##", "#.#.#", "#.#.#", "#.#.#", "#.#.#"},
		},
		"lower7": {
			{"###.###", "#.#.#.#", "#.#.#.#", "#.#.#.#", "#.#.#.#", "#.#.#.#", "#.#.#.#"},
			{"...#...", ".#####.", "...#...", "...#...", "...#...", "...#...", "..###.."},
			{"###.###", "#.#.#.#", "#.#.#.#", "#.#.#.#", "#.#.#.#", "#.#.#.#", "#.#.#.#"},
		},
		"lower9": {
			{"####.####", "#...#...#", "#...#...#", "#...#...#", "#...#...#", "#...#...#", "#...#...#", "#...#...#", "#...#...#"},
			{"....#....", ".#######.", "....#....", "....#....", "....#....", "....#....", "....#....", "....#....", "...###..."},
			{"####.####", "#...#...#", "#...#...#", "#...#...#", "#...#...#", "#...#...#", "#...#...#", "#...#...#", "#...#...#"},
		},
	}
	for name, glyphs := range allWordmarks {
		wantLetters := want[name]
		if wantLetters == nil {
			t.Fatalf("missing golden spec for %s", name)
		}
		if len(glyphs.letters) != len(wantLetters) {
			t.Fatalf("%s: letter count = %d, want %d", name, len(glyphs.letters), len(wantLetters))
		}
		for i, letter := range glyphs.letters {
			if len(letter) != len(wantLetters[i]) {
				t.Fatalf("%s letter %d: row count = %d, want %d", name, i, len(letter), len(wantLetters[i]))
			}
			for r, row := range letter {
				if row != wantLetters[i][r] {
					t.Errorf("%s letter %d row %d = %q, want %q", name, i, r, row, wantLetters[i][r])
				}
			}
		}
	}
}

func TestWordmarkRowWidthsConsistent(t *testing.T) {
	for name, glyphs := range allWordmarks {
		width := len(glyphs.letters[0][0])
		for i, letter := range glyphs.letters {
			for r, row := range letter {
				if len(row) != width {
					t.Errorf("%s letter %d row %d: width %d, want %d", name, i, r, len(row), width)
				}
			}
		}
	}
}

// TestWordmarkLettersPairwiseDistinct: the wordmark reads "mtm" — the
// distinct letterforms must not blur into each other at any size.
// (Positions 0 and 2 are the same M by design.)
func TestWordmarkLettersPairwiseDistinct(t *testing.T) {
	for name, glyphs := range allWordmarks {
		letters := glyphs.letters
		unique := map[string]bool{}
		for _, letter := range letters {
			unique[strings.Join(letter, "\n")] = true
		}
		if len(unique) != 2 {
			t.Errorf("%s: %d distinct letters, want exactly 2 (m and t)", name, len(unique))
		}
		for i := 0; i+1 < len(letters); i++ {
			same := true
			for r := range letters[i] {
				if letters[i][r] != letters[i+1][r] {
					same = false
					break
				}
			}
			if same {
				t.Errorf("%s: adjacent letters %d and %d are identical", name, i, i+1)
			}
		}
	}
}

// TestWordmarkMCenterAndStems pins the legibility invariant that keeps
// the M from reading as an H: the diagonals meet in the center column,
// and the outer columns stay lit on every row.
func TestWordmarkMCenterAndStems(t *testing.T) {
	vertexRow := map[string][]int{"caps11": {5}, "caps7": {4, 5}}
	for name, glyphs := range allWordmarks {
		if !strings.HasPrefix(name, "caps") {
			continue // the vertex invariant describes the caps M
		}
		m := glyphs.letters[0]
		size := len(m[0])
		center := size / 2
		for i, row := range m {
			if row[0] != '#' {
				t.Errorf("%s M row %d: left outer column not lit", name, i)
			}
			if row[size-1] != '#' {
				t.Errorf("%s M row %d: right outer column not lit", name, i)
			}
		}
		for _, vr := range vertexRow[name] {
			if m[vr][center] != '#' {
				t.Errorf("%s M vertex row %d: center column %d not lit (diagonal meeting point)", name, vr, center)
			}
		}
	}
}

// TestWordmarkMDiffersFromH: the M must differ from the H it could be
// mistaken for by at least 6 pixels at every caps size.
func TestWordmarkMDiffersFromH(t *testing.T) {
	for name, glyphs := range allWordmarks {
		if !strings.HasPrefix(name, "caps") {
			continue
		}
		d := litCount(glyphs.letters[0], wordmarkH[name])
		if d < 6 {
			t.Errorf("%s: M differs from H in %d pixels, want >= 6", name, d)
		}
	}
}

// TestWordmarkMVReaches50Pct: the M's V must reach at least half the
// glyph height.
func TestWordmarkMVReaches50Pct(t *testing.T) {
	for name, glyphs := range allWordmarks {
		if !strings.HasPrefix(name, "caps") {
			continue
		}
		m := glyphs.letters[0]
		size := len(m[0])
		center := size / 2
		deepest := -1
		for i, row := range m {
			if row[center] == '#' {
				deepest = i
			}
		}
		if deepest < 0 {
			t.Fatalf("%s: center column never lit", name)
		}
		got := float64(deepest+1) / float64(len(m))
		if got < 0.5 {
			t.Errorf("%s: M's V reaches row %d of %d (%.0f%% of the glyph height), want >= 50%%", name, deepest+1, len(m), got*100)
		}
	}
}

// TestWordmarkGapIsThreeSubpixels: letters are spaced 3 subpixels apart.
func TestWordmarkGapIsThreeSubpixels(t *testing.T) {
	for _, name := range []string{"caps11", "caps7"} {
		g := allWordmarks[name]
		gap := wordmarkWidthCells(g) - len(g.letters)*len(g.letters[0][0])
		if gap != (len(g.letters)-1)*3 {
			t.Errorf("%s: total width %d leaves %d subpixel gap, want %d", name, wordmarkWidthCells(g), gap, (len(g.letters)-1)*3)
		}
	}
}

var referenceCells = [][2]float64{{9, 17}, {8, 16}, {10, 22}}

// bitmapAspect returns the aspect (w/h) of a glyph set's M bitmap.
func bitmapAspect(g wordmarkGlyphs) float64 {
	w := len(g.letters[0][0])
	h := len(g.letters[0])
	return float64(w) / float64(h)
}

// TestWordmarkLetterAspectOnScreen: after item 1, the on-screen letter
// aspect is within 5% of the bitmap's aspect — for the reference cell
// sizes and every shipped glyph set.
func TestWordmarkLetterAspectOnScreen(t *testing.T) {
	for _, name := range []string{"caps11", "caps7", "lower9"} {
		g := allWordmarks[name]
		for _, cell := range referenceCells {
			cellW, cellH := cell[0], cell[1]
			pixelH := wordmarkPixelH(cellW, cellH)
			p := Model{}.wordmarkRects(g, 0, 0, pixelH, 10, 1)
			if p == nil {
				t.Fatalf("%s %vx%v: no rects", name, cellW, cellH)
			}
			// bbox of the first letter's rects
			minX, minY := math.Inf(1), math.Inf(1)
			maxX, maxY := math.Inf(-1), math.Inf(-1)
			for _, r := range p.Rects {
				if r.X >= float64(len(g.letters[0][0])) {
					break // first letter only
				}
				minX = math.Min(minX, r.X)
				minY = math.Min(minY, r.Y)
				maxX = math.Max(maxX, r.X+r.W)
				maxY = math.Max(maxY, r.Y+r.H)
			}
			bw := (maxX - minX) * cellW
			bh := (maxY - minY) * cellH
			got := bw / bh
			want := bitmapAspect(g)
			if math.Abs(got-want)/want > 0.05 {
				t.Errorf("%s cell %gx%g: on-screen letter aspect %.3f, bitmap aspect %.3f (outside 5%%)", name, cellW, cellH, got, want)
			}
		}
	}
}

func TestWordmarkForSelectsByTerminalSize(t *testing.T) {
	sets := map[string]wordmarkGlyphs{
		"caps11x11": wordmarkCaps11x11,
		"caps7x9":   wordmarkCaps7x9,
		"lower5x5":  wordmarkLower5x5,
		"lower7x7":  wordmarkLower7x7,
		"lower9x9":  wordmarkLower9x9,
	}
	cases := []struct {
		w, h  int
		lower bool
		want  string
	}{
		{60, 15, false, "caps7x9"},
		{100, 24, false, "caps7x9"},
		{159, 43, false, "caps7x9"},
		{160, 44, false, "caps11x11"},
		{200, 50, false, "caps11x11"},
		{60, 15, true, "lower5x5"},
		{100, 24, true, "lower7x7"},
		{160, 44, true, "lower9x9"},
	}
	for _, tc := range cases {
		got := wordmarkFor(tc.w, tc.h, tc.lower)
		if !reflect.DeepEqual(got, sets[tc.want]) {
			t.Errorf("wordmarkFor(%d, %d, lower=%v) = %+v, want %s", tc.w, tc.h, tc.lower, got, tc.want)
		}
	}
}

func TestWordmarkLayoutSkyBandAndGap(t *testing.T) {
	for _, tc := range []struct{ w, h int }{{60, 15}, {80, 20}, {100, 24}, {120, 30}, {160, 44}, {180, 50}, {200, 50}} {
		cellW, cellH := 9.0, 17.0
		lay := splashLayoutFor(tc.w, tc.h, cellW, cellH, false)
		// vertical center at about 38-42% of the height
		center := lay.top + lay.rows/2
		frac := center / float64(tc.h)
		if frac < 0.36 || frac > 0.42 {
			t.Errorf("%dx%d: wordmark center at %.1f%% of the height, want within 36-42%%", tc.w, tc.h, frac*100)
		}
		// gap between the wordmark bottom and the tagline ≥ 25% of the
		// wordmark height, minimum 2 rows
		tagTop := lay.tagY
		wmBottom := int(math.Ceil(lay.top + lay.rows))
		gap := tagTop - wmBottom
		wantGap := int(math.Ceil(lay.rows * 0.25))
		if wantGap < 2 {
			wantGap = 2
		}
		if gap < wantGap {
			t.Errorf("%dx%d: gap wordmark bottom→tagline = %d rows, want >= %d (25%% of wordmark height)", tc.w, tc.h, gap, wantGap)
		}
		// the wordmark never reaches the horizon band
		if wmBottom > int(math.Floor(float64(tc.h)*0.60)) {
			t.Errorf("%dx%d: wordmark bottom row %d reaches past the horizon row %d", tc.w, tc.h, wmBottom, int(math.Floor(float64(tc.h)*0.60)))
		}
		// tagline sits above the hint row and below the wordmark
		if lay.tagY >= tc.h-2 {
			t.Errorf("%dx%d: tagline row %d collides with the hint row", tc.w, tc.h, lay.tagY)
		}
	}
	// reference cell sizes give a sane tagline position
	for _, cell := range referenceCells {
		lay := splashLayoutFor(120, 30, cell[0], cell[1], false)
		if lay.tagY <= 0 || lay.tagY >= 28 {
			t.Errorf("cell %gx%g: tagY = %d, want a sane row inside the frame", cell[0], cell[1], lay.tagY)
		}
	}
}
