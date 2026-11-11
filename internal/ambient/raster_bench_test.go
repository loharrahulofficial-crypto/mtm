package ambient

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// raster_bench_test.go — §7.3: report the cost of the raster transition
// pipeline against the old string-parse blend it replaces. The old
// algorithm is reproduced inline (referenceStringCrossfade) purely for
// the benchmark; it is not used anywhere in the binary.

func referenceStringCrossfade(from, to []Line, t float64) []Line {
	if t <= 0 {
		return from
	}
	if t >= 1 {
		return to
	}
	e := EaseInOutCubic(t)
	A := parseFrameReference(from)
	B := parseFrameReference(to)
	rows := len(A)
	if len(B) < rows {
		rows = len(B)
	}
	out := make([]Line, 0, rows)
	for y := 0; y < rows; y++ {
		ra := A[y]
		rb := B[y]
		w := len(ra)
		if len(rb) < w {
			w = len(rb)
		}
		var sb strings.Builder
		curFG, curBG := "", ""
		emit := func(fg, bg string) {
			if fg != curFG {
				if fg == "" {
					sb.WriteString("\x1b[39m")
				} else {
					sb.WriteString(fmt.Sprintf("\x1b[38;2;%sm", rgbTriplet(hexToRGB(fg))))
				}
				curFG = fg
			}
			if bg != curBG {
				if bg == "" {
					sb.WriteString("\x1b[49m")
				} else {
					sb.WriteString(fmt.Sprintf("\x1b[48;2;%sm", rgbTriplet(hexToRGB(bg))))
				}
				curBG = bg
			}
		}
		for x := 0; x < w; x++ {
			fg := lerpHexReference(ra[x].fg, rb[x].fg, e)
			bg := lerpHexReference(ra[x].bg, rb[x].bg, e)
			emit(fg, bg)
			if e < 0.5 {
				sb.WriteString(ra[x].glyph)
			} else {
				sb.WriteString(rb[x].glyph)
			}
		}
		sb.WriteString("\x1b[0m")
		out = append(out, sb.String())
	}
	return out
}

type referenceCell struct{ fg, bg, glyph string }

func parseFrameReference(lines []Line) [][]referenceCell {
	out := make([][]referenceCell, 0, len(lines))
	for _, line := range lines {
		row := []referenceCell{}
		fg, bg := "", ""
		rest := line
		for len(rest) > 0 {
			if strings.HasPrefix(rest, "\x1b[") {
				end := strings.Index(rest, "m")
				if end < 0 {
					break
				}
				fg, bg = applySGRStrings(fg, bg, rest[:end+1])
				rest = rest[end+1:]
				continue
			}
			r, size := decodeRune(rest)
			if r == 0 {
				break
			}
			row = append(row, referenceCell{fg, bg, string(r)})
			rest = rest[size:]
		}
		out = append(out, row)
	}
	return out
}

func lerpHexReference(a, b string, t float64) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	ar, ag, ab := hexToRGB(a)
	br, bg, bb := hexToRGB(b)
	mix := func(x, y uint8) uint8 {
		return uint8(float64(x)*(1-t) + float64(y)*t + 0.5)
	}
	return fmt.Sprintf("#%02x%02x%02x", mix(ar, br), mix(ag, bg), mix(ab, bb))
}

func benchFrames() (from, to []Line) {
	st := stateFor(0.5, PhaseWork)
	night := New("night", ModeHalfblock)
	ocean := New("ocean", ModeHalfblock)
	return night.Frame(12*time.Second, 100, 30, st), ocean.Frame(12*time.Second, 100, 30, st)
}

func BenchmarkCrossfadeOldStringBlend(b *testing.B) {
	from, to := benchFrames()
	b.ReportAllocs()
	b.SetBytes(int64(len(from) + len(to)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = referenceStringCrossfade(from, to, 0.5)
	}
}

func BenchmarkCrossfadeNewRasterBlend(b *testing.B) {
	from, to := benchFrames()
	b.ReportAllocs()
	b.SetBytes(int64(len(from) + len(to)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Crossfade(from, to, 0.5)
	}
}

func BenchmarkFramePixelsScene(b *testing.B) {
	s := New("night", ModeHalfblock)
	st := stateFor(0.5, PhaseWork)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.FramePixels(12*time.Second, 100, 30, st)
	}
}

func BenchmarkFrameToSceneCompare(b *testing.B) {
	s := New("night", ModeHalfblock)
	st := stateFor(0.5, PhaseWork)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.Frame(12*time.Second, 100, 30, st)
	}
}

func BenchmarkBlendRasterOnly(b *testing.B) {
	from, to := benchFrames()
	base := baseHexFromLines(from)
	ra := rasterFromLines(from, base)
	rb := rasterFromLines(to, baseHexFromLines(to))
	b.ReportAllocs()
	b.SetBytes(int64(len(ra.Pix) * 16))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := BlendRasters(ra, rb, 0.5)
		_ = m.EmitHalfblock(base, false, false, false)
	}
}
