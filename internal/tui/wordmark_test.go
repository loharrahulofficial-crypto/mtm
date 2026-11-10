package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// wordmark_test.go — the splash wordmark must render "mtm", never "MIM":
// the t glyph needs a top bar over a narrower stem, and the three letters
// must be three distinct glyphs.

func letterInk(t *testing.T, g wordmarkGlyphs, idx int) []int {
	t.Helper()
	widths := []int{}
	for _, row := range g.letters[idx] {
		if lipgloss.Width(row) > 0 {
			// count non-space cells
			n := 0
			for _, r := range row {
				if r != ' ' {
					n++
				}
			}
			widths = append(widths, n)
		}
	}
	return widths
}

func TestWordmarkLettersDistinct(t *testing.T) {
	for _, name := range []string{"small", "large"} {
		g := wordmarkSmall
		if name == "large" {
			g = wordmarkLarge
		}
		// m, t, m: the m's are identical by design; t must differ from both.
		for i, prev := range g.letters[:2] {
			for j, l := range g.letters[:2] {
				if i != j {
					same := true
					for k := range l {
						if l[k] != prev[k] {
							same = false
						}
					}
					if same {
						t.Fatalf("%s: letters %d and %d are identical — the t is indistinguishable from the m", name, i, j)
					}
				}
			}
		}
	}
}

func TestWordmarkTReadsAsT(t *testing.T) {
	for _, name := range []string{"small", "large"} {
		g := wordmarkSmall
		if name == "large" {
			g = wordmarkLarge
		}
		// The t must have a full-width top bar and a strictly narrower stem,
		// so it can never collapse into an I.
		ink := letterInk(t, g, 1)
		if len(ink) < 2 {
			t.Fatalf("%s: t has %d rows, want at least 2", name, len(ink))
		}
		top := ink[0]
		for i, n := range ink[1:] {
			if n >= top {
				t.Fatalf("%s: t row %d has %d ink cells, not narrower than the top bar (%d) — reads as an I", name, i+1, n, top)
			}
		}
		// The two m letters must be identical (the wordmark is mtm, not mtX).
		for i := range g.letters[0] {
			if g.letters[0][i] != g.letters[2][i] {
				t.Fatalf("%s: m glyphs differ at row %d", name, i)
			}
		}
	}
}

func TestWordmarkLargeNeedsRoom(t *testing.T) {
	if _, large := wordmarkFor(80, 24); large {
		t.Fatal("80x24 selected the large wordmark")
	}
	if _, large := wordmarkFor(109, 27); large {
		t.Fatal("109x27 selected the large wordmark")
	}
	if _, large := wordmarkFor(110, 28); !large {
		t.Fatal("110x28 did not select the large wordmark")
	}
	if _, large := wordmarkFor(200, 50); !large {
		t.Fatal("200x50 did not select the large wordmark")
	}
}
