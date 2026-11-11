package ambient

import (
	"strings"
	"testing"
	"time"
)

// raster_test.go — §transitions-1: transitions blend pixels, not bytes.
// A synthetic pair of frames with different glyphs proves the blend is
// per-pixel: every cell's top and bottom halves equal the ideal blend of
// the normalized source pairs within ±2 per channel, and no half is ever
// brighter than both sources.

// normCell decodes one cell of a synthetic frame into a (top, bottom) pair.
// Unset colors default to the frame's base.
func normCell(fg, bg string, base string) ([3]uint8, [3]uint8) {
	fr, fgG, fgB := hexToRGB(defHex(fg, base))
	br, bgG, bgB := hexToRGB(defHex(bg, base))
	return [3]uint8{fr, fgG, fgB}, [3]uint8{br, bgG, bgB}
}

func defHex(v, base string) string {
	if v == "" {
		return base
	}
	return v
}

// decodeSynthPair parses one synthetic line into normalized (top, bottom)
// pairs using the same rules the transition engine applies.
func decodeSynthPair(line, base string) [][2][3]uint8 {
	type cell struct{ fg, bg, glyph string }
	cells := []cell{}
	fg, bg := "", ""
	rest := line
	for len(rest) > 0 {
		if strings.HasPrefix(rest, "\x1b[") {
			end := strings.Index(rest, "m")
			fg, bg = applySGRStrings(fg, bg, rest[:end+1])
			rest = rest[end+1:]
			continue
		}
		r, size := decodeRune(rest)
		if r == 0 {
			break
		}
		cells = append(cells, cell{fg, bg, string(r)})
		rest = rest[size:]
	}
	pairs := make([][2][3]uint8, 0, len(cells))
	for _, c := range cells {
		top, bot := normCell(c.fg, c.bg, base)
		switch c.glyph {
		case "▀":
		case "▄":
			top, bot = bot, top
		case "█":
			bot = top
		case " ":
			top = bot
		}
		pairs = append(pairs, [2][3]uint8{top, bot})
	}
	return pairs
}

func TestCrossfadeBlendsPerPixel(t *testing.T) {
	triplet := func(hex string) string {
		r, g, b := hexToRGB(hex)
		return rgbTriplet(r, g, b)
	}
	mk := func(cells [][3]string) []Line {
		// cells: {glyph, fg, bg} — fg/bg "" means unset
		var sb strings.Builder
		for _, c := range cells {
			if c[1] != "" {
				sb.WriteString("\x1b[38;2;" + triplet(c[1]) + "m")
			} else {
				sb.WriteString("\x1b[39m")
			}
			if c[2] != "" {
				sb.WriteString("\x1b[48;2;" + triplet(c[2]) + "m")
			} else {
				sb.WriteString("\x1b[49m")
			}
			sb.WriteString(c[0])
		}
		sb.WriteString("\x1b[0m")
		return []Line{sb.String()}
	}
	a := mk([][3]string{
		{" ", "", "#102030"},
		{"▀", "#404040", "#808080"},
		{"█", "#c0c0c0", ""},
		{"▄", "#ffffff", "#000000"},
	})
	b := mk([][3]string{
		{"▀", "#ff0000", "#00ff00"},
		{" ", "", "#112233"},
		{"▄", "#00ffff", "#ff00ff"},
		{"█", "#0f0f0f", ""},
	})
	baseA, baseB := baseHexFromLines(a), baseHexFromLines(b)
	srcA, srcB := decodeSynthPair(a[0], baseA), decodeSynthPair(b[0], baseB)
	lum := func(c [3]uint8) float64 {
		return 0.2126*float64(c[0]) + 0.7152*float64(c[1]) + 0.0722*float64(c[2])
	}
	for _, tt := range []float64{0.05, 0.25, 0.5, 0.75, 0.95} {
		e := EaseInOutCubic(tt)
		got := Crossfade(a, b, tt)
		if len(got) != 1 {
			t.Fatalf("t=%v: got %d lines, want 1", tt, len(got))
		}
		mix := func(x, y uint8) uint8 {
			return uint8(float64(x)*(1-e) + float64(y)*e + 0.5)
		}
		out := decodeSynthPair(got[0], baseA)
		for i, cell := range out {
			for half := 0; half < 2; half++ {
				wantR := mix(srcA[i][half][0], srcB[i][half][0])
				wantG := mix(srcA[i][half][1], srcB[i][half][1])
				wantB := mix(srcA[i][half][2], srcB[i][half][2])
				gotC := cell[half]
				for ch := 0; ch < 3; ch++ {
					d := int(gotC[ch]) - int([]uint8{wantR, wantG, wantB}[ch])
					if d < 0 {
						d = -d
					}
					if d > 2 {
						t.Fatalf("t=%v cell %d half %d ch %d: got %v want ~{%d %d %d}", tt, i, half, ch, gotC, wantR, wantG, wantB)
					}
				}
				la, lb := lum(srcA[i][half]), lum(srcB[i][half])
				hi := la
				if lb > hi {
					hi = lb
				}
				if l := lum(gotC); l > hi+2 {
					t.Fatalf("t=%v cell %d half %d: blend %v brighter than both sources (%v vs %v/%v)", tt, i, half, l, hi, la, lb)
				}
			}
		}
	}
}

// TestFramePixelsMatchesFrame: the engine's raster API must see the same
// pixels the emitted halfblock frame encodes.
func TestFramePixelsMatchesFrame(t *testing.T) {
	for _, name := range Registered() {
		s := New(name, ModeHalfblock)
		st := stateFor(0.5, PhaseWork)
		const w, h = 32, 8
		rast := s.FramePixels(3*time.Second, w, h, st)
		if rast.W != w || rast.H != 2*h {
			t.Fatalf("%s: FramePixels raster is %dx%d, want %dx%d", name, rast.W, rast.H, w, 2*h)
		}
		frame := s.Frame(3*time.Second, w, h, st)
		_, _, decoded := DecodeFrame(frame)
		if len(decoded) != rast.W*rast.H {
			t.Fatalf("%s: decoded %d pixels, raster has %d", name, len(decoded), rast.W*rast.H)
		}
		for i := range decoded {
			dr, dg, db := decoded[i].R, decoded[i].G, decoded[i].B
			gr, gg, gb := rast.Pixel(i%rast.W, i/rast.W)
			for ch := 0; ch < 3; ch++ {
				d := int([]uint8{dr, dg, db}[ch]) - int([]uint8{gr, gg, gb}[ch])
				if d < 0 {
					d = -d
				}
				if d > 2 {
					t.Fatalf("%s: pixel %d ch %d: frame has %d, raster has %d", name, i, ch, []uint8{dr, dg, db}[ch], []uint8{gr, gg, gb}[ch])
				}
			}
		}
	}
}

// TestBlackFrameEmitsBlankBaseCells: every cell is a space on the frame's
// dominant background.
func TestBlackFrameEmitsBlankBaseCells(t *testing.T) {
	s := New("night", ModeHalfblock)
	st := stateFor(0.5, PhaseWork)
	frame := s.Frame(2*time.Second, 40, 12, st)
	black := BlackFrame(frame)
	if len(black) != len(frame) {
		t.Fatalf("BlackFrame rows = %d, want %d", len(black), len(frame))
	}
	base := baseHexFromLines(frame)
	br, bg, bb := hexToRGB(base)
	for i, l := range black {
		// strip SGR, every visible glyph must be a space
		for _, r := range stripEscapes(l) {
			if r != ' ' {
				t.Fatalf("line %d: BlackFrame contains glyph %q", i, r)
			}
		}
		// the base background must be what the cells sit on
		if !strings.Contains(l, "\x1b[48;2;"+rgbTriplet(br, bg, bb)+"m") {
			t.Fatalf("line %d: base background %s not present in %q", i, base, l)
		}
	}
}
