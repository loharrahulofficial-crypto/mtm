package ambient

import (
	"math"
	"testing"
)

// aspect_test.go — item 1 contract: subpixel aspect math, and shapes
// rendered through the aspect-corrected raster measure square/round on
// screen for every render mode and cell size 9x17, 8x16 and 10x22.

var cellSizes = []CellPixels{{W: 9, H: 17}, {W: 8, H: 16}, {W: 10, H: 22}}

var aspectModes = []string{ModeHalfblock, ModeSextant, ModeBraille, ModeQuadrant, ModeASCII}

func TestSubpixelAspect(t *testing.T) {
	for _, mode := range aspectModes {
		cols, rows := subcellsPerCell(mode)
		for _, cell := range cellSizes {
			got := SubpixelAspect(mode, cell)
			want := (cell.H / float64(rows)) / (cell.W / float64(cols))
			if math.Abs(got-want) > 1e-9 {
				t.Errorf("SubpixelAspect(%s, %+v) = %v, want %v", mode, cell, got, want)
			}
		}
	}
	// Unknown/invalid input falls back to the 9:17 default cell.
	if got := SubpixelAspect(ModeHalfblock, CellPixels{W: 0, H: 0}); math.Abs(got-(17.0/2)/9) > 1e-9 {
		t.Errorf("fallback aspect = %v, want %v", got, (17.0/2)/9)
	}
}

func TestParseXTWINOPSCellSize(t *testing.T) {
	for _, tc := range []struct {
		name  string
		reply string
		want  CellPixels
		ok    bool
	}{
		{"cell size reply", "\x1b[6;17;9t", CellPixels{W: 9, H: 17}, true},
		{"leading garbage", "noise\x1b[6;16;8t", CellPixels{W: 8, H: 16}, true},
		{"decimal dims", "\x1b[6;16.5;8.25t", CellPixels{W: 8.25, H: 16.5}, true},
		{"text area shape rejected", "\x1b[4;1080;1920t", CellPixels{}, false},
		{"empty", "", CellPixels{}, false},
		{"truncated", "\x1b[6;17", CellPixels{}, false},
		{"zero cell", "\x1b[6;0;0t", CellPixels{}, false},
	} {
		cp, ok := parseXTWINOPSCellSize(tc.reply)
		if ok != tc.ok {
			t.Errorf("%s: ok = %v, want %v", tc.name, ok, tc.ok)
			continue
		}
		if ok && cp != tc.want {
			t.Errorf("%s: got %+v, want %+v", tc.name, cp, tc.want)
		}
	}
}

// inkBBox returns the bounding box of raster pixels whose ink coverage
// exceeds ink0.
func inkBBox(px []float32, pw, ph int, ink0 float32) (x0, y0, x1, y1 int, ok bool) {
	x0, y0, x1, y1 = pw, ph, -1, -1
	for y := 0; y < ph; y++ {
		for x := 0; x < pw; x++ {
			if px[y*pw+x] > ink0 {
				if x < x0 {
					x0 = x
				}
				if x > x1 {
					x1 = x
				}
				if y < y0 {
					y0 = y
				}
				if y > y1 {
					y1 = y
				}
			}
		}
	}
	return x0, y0, x1, y1, x1 >= 0
}

// whiteBBox returns the bounding box of raster pixels close to white
// (the fills under test are painted opaque white).
func whiteBBox(px []fpix, pw, ph int) (x0, y0, x1, y1 int, ok bool) {
	x0, y0, x1, y1 = pw, ph, -1, -1
	for y := 0; y < ph; y++ {
		for x := 0; x < pw; x++ {
			c := px[y*pw+x]
			if c.r > 0.9 && c.g > 0.9 && c.b > 0.9 {
				if x < x0 {
					x0 = x
				}
				if x > x1 {
					x1 = x
				}
				if y < y0 {
					y0 = y
				}
				if y > y1 {
					y1 = y
				}
			}
		}
	}
	return x0, y0, x1, y1, x1 >= 0
}

// screenExtent converts a raster bounding box to screen pixels for the
// given mode and cell size.
func screenExtent(mode string, cell CellPixels, x0, y0, x1, y1 int) (w, h float64) {
	cols, rows := subcellsPerCell(mode)
	subW := cell.W / float64(cols)
	subH := cell.H / float64(rows)
	fss := float64(fSSV())
	w = float64(x1-x0+1) / fss * subW
	h = float64(y1-y0+1) / fss * subH
	return w, h
}

func TestRenderedMoonIsRoundOnScreen(t *testing.T) {
	for _, mode := range aspectModes {
		for _, cell := range cellSizes {
			asp := SubpixelAspect(mode, cell)
			p := paletteFor(DefaultPalette())
			b := newBuffer(mode, 44, 26, p, asp)
			mX, mY, mR := 22.0, 13.0, 10.0
			cols := sphereCols(DefaultPalette().BrightText, DefaultPalette().Base, DefaultPalette().Base)
			paintMoonSphere(b, mX, mY, mR, 0.5, 0, cols, haloCol(DefaultPalette().BrightText, DefaultPalette().Accent))
			// ink > 0.5 isolates the disc from the faint halo blob the
			// sphere paints into the ink raster.
			x0, y0, x1, y1, ok := inkBBox(b.fb.ink, b.fb.pw, b.fb.ph, 0.5)
			if !ok {
				t.Fatalf("%s %+v: moon left no ink", mode, cell)
			}
			w, h := screenExtent(mode, cell, x0, y0, x1, y1)
			b.fb.release()
			ratio := w / h
			if math.Abs(ratio-1) > 0.03 {
				t.Errorf("%s cell %.0fx%.0f: moon on-screen bbox %.1fx%.1f px, ratio %.3f (want within 3%% of 1)", mode, cell.W, cell.H, w, h, ratio)
			}
		}
	}
}

func TestRenderedSquareStaysSquare(t *testing.T) {
	for _, mode := range aspectModes {
		for _, cell := range cellSizes {
			asp := SubpixelAspect(mode, cell)
			p := paletteFor(DefaultPalette())
			b := newBuffer(mode, 40, 24, p, asp)
			// An ellipse with rx == ry is a circle in scene units, so it
			// must measure round on screen.
			b.fb.fillEllipseF(22, 13, 10, 10, 1, 1, 1, 1, opNormal)
			x0, y0, x1, y1, ok := whiteBBox(b.fb.px, b.fb.pw, b.fb.ph)
			if !ok {
				t.Fatalf("%s cell %.0fx%.0f: ellipse left no pixels", mode, cell.W, cell.H)
			}
			w, h := screenExtent(mode, cell, x0, y0, x1, y1)
			b.fb.release()
			if math.Abs(w/h-1) > 0.03 {
				t.Errorf("%s cell %.0fx%.0f: circle-ellipse on-screen bbox %.1fx%.1f px, ratio %.3f (want within 3%% of 1)", mode, cell.W, cell.H, w, h, w/h)
			}
		}
	}
}

func TestRenderedRectOnScreenSquare(t *testing.T) {
	// Scenes address y in raw subcells; primitives measure distances in
	// aspect-corrected terms, so a screen-square rect is requested with
	// a y span of xspan/asp. Verify the primitives honor that contract.
	for _, mode := range aspectModes {
		for _, cell := range cellSizes {
			asp := SubpixelAspect(mode, cell)
			p := paletteFor(DefaultPalette())
			b := newBuffer(mode, 40, 24, p, asp)
			b.fb.fillRectF(18, 9, 26, 9+8/asp, 1, 1, 1, 1, opNormal)
			x0, y0, x1, y1, ok := whiteBBox(b.fb.px, b.fb.pw, b.fb.ph)
			if !ok {
				t.Fatalf("%s cell %.0fx%.0f: rect left no pixels", mode, cell.W, cell.H)
			}
			w, h := screenExtent(mode, cell, x0, y0, x1, y1)
			b.fb.release()
			// 5% slack: the y axis is the coarsest (fewest subcells), and
			// bbox quantization adds ±1 raster pixel per axis.
			if math.Abs(w/h-1) > 0.05 {
				t.Errorf("%s cell %.0fx%.0f: rect on-screen bbox %.1fx%.1f px, ratio %.3f (want within 5%% of 1)", mode, cell.W, cell.H, w, h, w/h)
			}
		}
	}
}
