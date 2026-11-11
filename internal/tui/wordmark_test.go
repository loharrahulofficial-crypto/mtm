package tui

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

// wordmark_test.go — legibility contract for the hand-designed mtm
// wordmark: distinct letters, centered diagonals on the M, golden
// bitmaps, and sky-band placement.

var allWordmarks = map[string]wordmarkGlyphs{
	"caps5":  wordmarkCaps5x5,
	"caps7":  wordmarkCaps7x7,
	"caps9":  wordmarkCaps9x9,
	"lower5": wordmarkLower5x5,
	"lower7": wordmarkLower7x7,
	"lower9": wordmarkLower9x9,
}

// wordmarkH is the H shape the M must differ from at every size (not
// shipped; test-local anchor for the "reads as H" regression).
var wordmarkH = map[string][]string{
	"caps5": {"#...#", "#...#", "#####", "#...#", "#...#"},
	"caps7": {"#.....#", "#.....#", "#.....#", "#######", "#.....#", "#.....#", "#.....#"},
	"caps9": {"#.......#", "#.......#", "#.......#", "#.......#", "#########", "#.......#", "#.......#", "#.......#", "#.......#"},
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
		"caps5": {
			{"#...#", "##.##", "#.#.#", "#...#", "#...#"},
			{"#####", "..#..", "..#..", "..#..", "..#.."},
			{"#...#", "##.##", "#.#.#", "#...#", "#...#"},
		},
		"caps7": {
			{"#.....#", "##...##", "#.#.#.#", "#..#..#", "#.....#", "#.....#", "#.....#"},
			{"#######", "...#...", "...#...", "...#...", "...#...", "...#...", "...#..."},
			{"#.....#", "##...##", "#.#.#.#", "#..#..#", "#.....#", "#.....#", "#.....#"},
		},
		"caps9": {
			{"#.......#", "##.....##", "#.#...#.#", "#..#.#..#", "#...#...#", "#.......#", "#.......#", "#.......#", "#.......#"},
			{"#########", "....#....", "....#....", "....#....", "....#....", "....#....", "....#....", "....#....", "....#...."},
			{"#.......#", "##.....##", "#.#...#.#", "#..#.#..#", "#...#...#", "#.......#", "#.......#", "#.......#", "#.......#"},
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
		if name == "" || glyphs.letters == nil {
			t.Fatalf("empty wordmark set %q", name)
		}
		wantLetters := want[name]
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
		size := len(glyphs.letters[0])
		for i, letter := range glyphs.letters {
			for r, row := range letter {
				if len(row) != size {
					t.Errorf("%s letter %d row %d: width %d, want %d", name, i, r, len(row), size)
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
// the M from reading as an H: the diagonals meet in the center column
// (on the vertex row), and the outer columns stay lit on every row.
// Vertex rows are the rows where the diagonals converge: 5x5 → row 2,
// 7x7 → row 3, 9x9 → row 4.
func TestWordmarkMCenterAndStems(t *testing.T) {
	vertexRow := map[string]int{"caps5": 2, "caps7": 3, "caps9": 4}
	for name, glyphs := range allWordmarks {
		if !strings.HasPrefix(name, "caps") {
			continue // the vertex invariant describes the caps M
		}
		m := glyphs.letters[0]
		size := len(m)
		center := size / 2
		for i, row := range m {
			if row[0] != '#' {
				t.Errorf("%s M row %d: left outer column not lit", name, i)
			}
			if row[size-1] != '#' {
				t.Errorf("%s M row %d: right outer column not lit", name, i)
			}
		}
		vr := vertexRow[name]
		if m[vr][center] != '#' {
			t.Errorf("%s M vertex row %d: center column %d not lit (diagonal meeting point)", name, vr, center)
		}
	}
}

func TestWordmarkMDiffersFromH(t *testing.T) {
	for name, glyphs := range allWordmarks {
		if !strings.HasPrefix(name, "caps") {
			continue // H comparison covers the caps M, the shape that read as H
		}
		d := litCount(glyphs.letters[0], wordmarkH[name])
		if d < 4 {
			t.Errorf("%s: M differs from H in %d pixels, want >= 4", name, d)
		}
	}
}

func TestGlyphCellsRendersSpecBitmaps(t *testing.T) {
	replacer := strings.NewReplacer("#", "█", ".", " ")
	for name, glyphs := range allWordmarks {
		for i, letter := range glyphs.letters {
			for r, row := range letter {
				want := replacer.Replace(row)
				if got := glyphCells(row); got != want {
					t.Errorf("%s letter %d row %d: glyphCells(%q) = %q, want %q", name, i, r, row, got, want)
				}
			}
		}
	}
}

func TestWordmarkForSelectsByTerminalSize(t *testing.T) {
	sets := map[string]wordmarkGlyphs{
		"caps5x5":  wordmarkCaps5x5,
		"caps7x7":  wordmarkCaps7x7,
		"caps9x9":  wordmarkCaps9x9,
		"lower5x5": wordmarkLower5x5,
		"lower7x7": wordmarkLower7x7,
		"lower9x9": wordmarkLower9x9,
	}
	cases := []struct {
		w, h  int
		lower bool
		want  string
	}{
		{60, 15, false, "caps5x5"},
		{99, 23, false, "caps5x5"},
		{100, 24, false, "caps7x7"},
		{159, 43, false, "caps7x7"},
		{160, 44, false, "caps9x9"},
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

func TestWordmarkTopStaysInSkyBand(t *testing.T) {
	// feasible (h, rows) pairs: 5x5 any h, 7x7 from h=16, 9x9 from h=26
	pairs := [][2]int{}
	for _, h := range []int{12, 16, 20, 24, 30, 40, 44, 50} {
		pairs = append(pairs, [2]int{h, 5})
		if h >= 16 {
			pairs = append(pairs, [2]int{h, 7})
		}
		if h >= 26 {
			pairs = append(pairs, [2]int{h, 9})
		}
	}
	for _, p := range pairs {
		h, rows := p[0], p[1]
		top := wordmarkTop(h, rows)
		if top < 0 {
			t.Errorf("h=%d rows=%d: top %d < 0", h, rows, top)
		}
		bottom := top + rows
		horizonRow := int(math.Floor(float64(h) * 0.60)) // full-screen horizon sits at ≥ 60%
		if bottom > horizonRow {
			t.Errorf("h=%d rows=%d: bottom %d reaches past the horizon row %d", h, rows, bottom, horizonRow)
		}
		center := float64(top) + float64(rows)/2
		frac := center / float64(h)
		if frac > 0.45 {
			t.Errorf("h=%d rows=%d: wordmark center at %.0f%% of the height, want ≤ 45%%", h, rows, frac*100)
		}
	}
	// default sizes: the center sits inside 36-42% of the height
	for _, tc := range []struct{ w, h int }{{80, 20}, {120, 30}, {180, 50}} {
		g := wordmarkFor(tc.w, tc.h, false)
		rows := len(g.letters[0])
		top := wordmarkTop(tc.h, rows)
		frac := (float64(top) + float64(rows)/2) / float64(tc.h)
		if frac < 0.36 || frac > 0.42 {
			t.Errorf("wordmarkFor(%d, %d): center at %.1f%% of height, want within 36-42%%", tc.w, tc.h, frac*100)
		}
	}
}
