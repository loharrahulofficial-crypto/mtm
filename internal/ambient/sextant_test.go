package ambient

import (
	"strings"
	"testing"
	"time"
)

// sextant_test.go — invariants for the sextant render mode: line count,
// cell width, unpainted frames, uniform rasters, limited palette, sparse
// ink and the glyph vocabulary.

func sextantTestState() SceneState {
	return stateFor(0.5, PhaseWork)
}

func TestSextantLineCountAndWidth(t *testing.T) {
	for _, scene := range Registered() {
		s := New(scene, ModeSextant)
		lines := s.Frame(12*time.Second, 100, 30, sextantTestState())
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

func TestSextantAllInkZeroIsAllSpaces(t *testing.T) {
	const w, h = 24, 6
	px := make([]fpix, w*2*h*3)
	ink := make([]float32, len(px))
	lines := emitSextant(px, ink, w, h, "#14282d", emitOpts{})
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

func TestSextantUniformBrightRaster(t *testing.T) {
	const w, h = 24, 6
	px := make([]fpix, w*2*h*3)
	ink := make([]float32, len(px))
	for i := range px {
		px[i] = fpix{0.9, 0.8, 0.7, 1}
		ink[i] = 1
	}
	lines := emitSextant(px, ink, w, h, "#14282d", emitOpts{})
	if len(lines) != h {
		t.Fatalf("got %d lines, want %d", len(lines), h)
	}
	for i, l := range lines {
		if got := cellWidth(l); got != w {
			t.Fatalf("line %d: got %d cells, want %d", i, got, w)
		}
		for _, r := range stripANSI(l) {
			if r != ' ' && !isSextantGlyph(r) {
				t.Fatalf("line %d: glyph %q outside the sextant set", i, r)
			}
		}
	}
}

func TestSextantLimitedPaletteHasNoTruecolor(t *testing.T) {
	st := sextantTestState()
	st.PaletteLimited = true
	for _, dither := range []bool{false, true} {
		st.Dither = dither
		lines := New("ocean", ModeSextant).Frame(12*time.Second, 40, 14, st)
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

func TestSextantSingleInkedDot(t *testing.T) {
	const w, h = 24, 6
	px := make([]fpix, w*2*h*3)
	ink := make([]float32, len(px))
	for i := range px {
		px[i] = fpix{0.6, 0.5, 0.4, 1}
	}
	stride := w * 2
	for cell := 0; cell < w*h; cell++ {
		// One of six subcells inked just above the 0.22 threshold.
		ink[(cell/w)*3*stride+(cell%w)*2] = 0.3
	}
	lines := emitSextant(px, ink, w, h, "#14282d", emitOpts{})
	if len(lines) != h {
		t.Fatalf("got %d lines, want %d", len(lines), h)
	}
	for i, l := range lines {
		if got := cellWidth(l); got != w {
			t.Fatalf("line %d: got %d cells, want %d", i, got, w)
		}
	}
}

func TestSextantGlyphVocabulary(t *testing.T) {
	for _, scene := range Registered() {
		for _, limited := range []bool{false, true} {
			st := sextantTestState()
			st.PaletteLimited = limited
			lines := New(scene, ModeSextant).Frame(12*time.Second, 60, 18, st)
			for i, l := range lines {
				for _, r := range stripANSI(l) {
					if r == ' ' {
						continue
					}
					if !isSextantGlyph(r) {
						t.Fatalf("%s limited=%v line %d: glyph %q outside the sextant set", scene, limited, i, r)
					}
				}
			}
		}
	}
}

// TestSextantGlyphTable pins the verified Unicode permutation and the
// mask round-trip: every pattern maps to its glyph and back, and the
// four mosaics Unicode skips come from ' ', '█', '▌', '▐'.
func TestSextantGlyphTable(t *testing.T) {
	want := map[uint8]rune{
		0: ' ', 63: '█', 21: '▌', 42: '▐',
		1: 0x1FB00, 2: 0x1FB01, 3: 0x1FB02, 4: 0x1FB03, // SEXTANT-1/2/12/3
		7:  0x1FB06, // SEXTANT-123
		20: 0x1FB13, // SEXTANT-35
		23: 0x1FB15, // SEXTANT-1235
		62: 0x1FB3B, // SEXTANT-23456 (all except TL)
	}
	for mask, glyph := range want {
		if sextantGlyphs[mask] != glyph {
			t.Fatalf("pattern %d: got %U, want %U", mask, sextantGlyphs[mask], glyph)
		}
		if sextantGlyphMask[glyph] != mask {
			t.Fatalf("glyph %U: mask got %d, want %d", glyph, sextantGlyphMask[glyph], mask)
		}
	}
	for i, g := range sextantGlyphs {
		if sextantGlyphMask[g] != uint8(i) {
			t.Fatalf("pattern %d (%U): mask round-trip failed", i, g)
		}
	}
}

// isSextantGlyph reports whether r is one of the 60 sextant glyphs
// (U+1FB00..U+1FB3B), '▌'/'▐' (column mosaics) or '█'.
func isSextantGlyph(r rune) bool {
	return r == '█' || r == '▌' || r == '▐' || (r >= 0x1FB00 && r <= 0x1FB3B)
}
