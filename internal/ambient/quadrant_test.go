package ambient

import (
	"strings"
	"testing"
	"time"
)

// quadrant_test.go — invariants for the quadrant render mode: line count,
// cell width, unpainted frames, uniform rasters, limited palette, sparse
// ink and the glyph vocabulary.

func quadrantTestState() SceneState {
	return stateFor(0.5, PhaseWork)
}

func TestQuadrantLineCountAndWidth(t *testing.T) {
	for _, scene := range Registered() {
		s := New(scene, ModeQuadrant)
		lines := s.Frame(12*time.Second, 100, 30, quadrantTestState())
		if len(lines) != 30 {
			t.Fatalf("%s: got %d lines, want 30", scene, len(lines))
		}
		for i, l := range lines {
			if got := cellWidth(l); got != 100 {
				t.Fatalf("%s line %d: got %d cells, want 100", scene, i, got)
			}
		}
	}
}

func TestQuadrantAllInkZeroIsAllSpaces(t *testing.T) {
	const w, h = 24, 6
	px := make([]fpix, w*2*h*2)
	ink := make([]float32, len(px))
	lines := emitQuadrant(px, ink, w, h, "#14282d", emitOpts{})
	if len(lines) != h {
		t.Fatalf("got %d lines, want %d", len(lines), h)
	}
	for i, l := range lines {
		stripped := stripANSI(l)
		if got := cellWidth(l); got != w {
			t.Fatalf("line %d: got %d cells, want %d (%q)", i, got, w, stripped)
		}
		if strings.Trim(stripped, " ") != "" {
			t.Fatalf("line %d: expected only spaces, got %q", i, stripped)
		}
	}
}

func TestQuadrantUniformBrightRaster(t *testing.T) {
	const w, h = 24, 6
	px := make([]fpix, w*2*h*2)
	ink := make([]float32, len(px))
	for i := range px {
		px[i] = fpix{0.9, 0.8, 0.7, 1}
		ink[i] = 1
	}
	lines := emitQuadrant(px, ink, w, h, "#14282d", emitOpts{})
	if len(lines) != h {
		t.Fatalf("got %d lines, want %d", len(lines), h)
	}
	for i, l := range lines {
		if got := cellWidth(l); got != w {
			t.Fatalf("line %d: got %d cells, want %d", i, got, w)
		}
		for _, r := range stripANSI(l) {
			if r != ' ' && !isQuadrantGlyph(r) {
				t.Fatalf("line %d: glyph %q outside the quadrant set", i, r)
			}
		}
	}
}

func TestQuadrantLimitedPaletteHasNoTruecolor(t *testing.T) {
	st := quadrantTestState()
	st.PaletteLimited = true
	for _, dither := range []bool{false, true} {
		st.Dither = dither
		lines := New("ocean", ModeQuadrant).Frame(12*time.Second, 40, 14, st)
		if len(lines) != 14 {
			t.Fatalf("dither=%v: got %d lines, want 14", dither, len(lines))
		}
		for i, l := range lines {
			if strings.Contains(l, "38;2;") || strings.Contains(l, "48;2;") {
				t.Fatalf("dither=%v line %d: truecolor SGR leaked into limited output", dither, i)
			}
		}
	}
}

func TestQuadrantSingleInkedDot(t *testing.T) {
	const w, h = 24, 6
	px := make([]fpix, w*2*h*2)
	ink := make([]float32, len(px))
	for i := range px {
		px[i] = fpix{0.6, 0.5, 0.4, 1}
	}
	for cell := 0; cell < w*h; cell++ {
		// One of four subcells inked just above the 0.22 threshold.
		ink[(cell/w)*2*strideForTest(w)+(cell%w)*2] = 0.3
	}
	lines := emitQuadrant(px, ink, w, h, "#14282d", emitOpts{})
	if len(lines) != h {
		t.Fatalf("got %d lines, want %d", len(lines), h)
	}
	for i, l := range lines {
		if got := cellWidth(l); got != w {
			t.Fatalf("line %d: got %d cells, want %d", i, got, w)
		}
	}
}

func strideForTest(w int) int { return w * 2 }

func TestQuadrantGlyphVocabulary(t *testing.T) {
	for _, scene := range Registered() {
		for _, limited := range []bool{false, true} {
			st := quadrantTestState()
			st.PaletteLimited = limited
			lines := New(scene, ModeQuadrant).Frame(12*time.Second, 60, 18, st)
			for i, l := range lines {
				for _, r := range stripANSI(l) {
					if r == ' ' {
						continue
					}
					if !isQuadrantGlyph(r) {
						t.Fatalf("%s limited=%v line %d: glyph %q outside the 16 quadrant glyphs", scene, limited, i, r)
					}
				}
			}
		}
	}
}

func isQuadrantGlyph(r rune) bool {
	switch r {
	case '▘', '▝', '▖', '▗', '▀', '▄', '▌', '▐',
		'▚', '▞', '▛', '▜', '▙', '▟', '█',
		' ':
		return true
	}
	return false
}
