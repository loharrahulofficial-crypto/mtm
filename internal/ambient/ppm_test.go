package ambient

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// ppm_test.go — review dumps must show what the terminal shows:
// WritePPM draws each subpixel with its on-screen aspect, so a rendered
// circle comes out round in the PPM itself.

func TestWritePPMCircleIsRound(t *testing.T) {
	for _, mode := range []string{ModeHalfblock, ModeSextant, ModeQuadrant} {
		cell := DefaultCellPixels()
		asp := SubpixelAspect(mode, cell)
		p := paletteFor(DefaultPalette())
		b := newBuffer(mode, 60, 30, p, asp)
		b.fillCircleHexF(30, 15, 8, DefaultPalette().BrightText, 1, opNormal)
		R := Raster{W: b.sw, H: b.sh, Pix: append([]fpix{}, b.fb.downsample()...)}
		b.fb.release()
		lines := R.EmitHalfblock(DefaultPalette().Base, true, false, false)
		path := filepath.Join(t.TempDir(), "circle.ppm")
		const scale = 6
		if err := WritePPM(path, lines, scale, asp); err != nil {
			t.Fatal(err)
		}
		w, h, px := readPPMFile(t, path)
		// bounding box of pixels brighter than the dark base
		x0, y0, x1, y1 := w, h, -1, -1
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				c := px[y*w+x]
				if int(c.R)+int(c.G)+int(c.B) > 3*90 {
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
		if x1 < 0 {
			t.Fatalf("%s: no bright pixels in PPM", mode)
		}
		tw, th := float64(x1-x0+1), float64(y1-y0+1)
		ratio := tw / th
		// the circle is round in the grid; the PPM aspect must preserve it.
		// 12% slack absorbs bbox quantization at 6 px per subpixel.
		if math.Abs(ratio-1) > 0.12 {
			t.Errorf("%s: PPM circle bbox %.0fx%.0f px, ratio %.3f (want ~1)", mode, tw, th, ratio)
		}
		// and the halfblock subpixels must really be drawn with the
		// on-screen aspect: subPixH = round(scale*asp)
		if mode == ModeHalfblock {
			subPixH := int(math.Round(scale * asp))
			wantH := 30 * 2 * subPixH
			if h != wantH {
				t.Errorf("halfblock PPM height = %d, want %d (scale %d × asp %.3f)", h, wantH, scale, asp)
			}
		}
	}
}

func TestWritePPMAspectFallsBackToSquare(t *testing.T) {
	b := newBuffer(ModeHalfblock, 4, 2, paletteFor(DefaultPalette()), 1)
	b.fillRectHexF(0, 0, 4, 2, "#ffffff", 1, opNormal)
	R := Raster{W: b.sw, H: b.sh, Pix: append([]fpix{}, b.fb.downsample()...)}
	b.fb.release()
	lines := R.EmitHalfblock(DefaultPalette().Base, true, false, false)
	path := filepath.Join(t.TempDir(), "square.ppm")
	if err := WritePPM(path, lines, 2, 0); err != nil {
		t.Fatal(err)
	}
	w, h, _ := readPPMFile(t, path)
	if w != 8 || h != 8 {
		t.Errorf("square fallback PPM = %dx%d, want 8x8", w, h)
	}
}

// readPPMFile parses a binary P6 PPM (mirrors contrib/scenes-contact-sheet.py).
func readPPMFile(t *testing.T, path string) (int, int, []RGBPix) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	i := 0
	readToken := func() string {
		for i < len(data) {
			c := data[i]
			if c == '#' {
				for i < len(data) && data[i] != '\n' {
					i++
				}
				continue
			}
			if c == ' ' || c == '\n' || c == '\r' || c == '\t' {
				i++
				continue
			}
			break
		}
		start := i
		for i < len(data) {
			c := data[i]
			if c == ' ' || c == '\n' || c == '\r' || c == '\t' {
				break
			}
			i++
		}
		return string(data[start:i])
	}
	if magic := readToken(); magic != "P6" {
		t.Fatalf("not a P6 ppm: %q", magic)
	}
	var dims [2]int
	for k := range dims {
		v, err := parseUint(readToken())
		if err != nil {
			t.Fatal(err)
		}
		dims[k] = v
	}
	if maxv := readToken(); maxv != "255" {
		t.Fatalf("maxval %s, want 255", maxv)
	}
	i++ // single whitespace after maxval
	w, h := dims[0], dims[1]
	if len(data) < i+w*h*3 {
		t.Fatalf("truncated pixel data: %d bytes for %dx%d", len(data)-i, w, h)
	}
	pix := make([]RGBPix, w*h)
	for k := range pix {
		base := i + k*3
		pix[k] = RGBPix{data[base], data[base+1], data[base+2]}
	}
	return w, h, pix
}

func parseUint(s string) (int, error) {
	v := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("bad integer %q", s)
		}
		v = v*10 + int(r-'0')
	}
	return v, nil
}
