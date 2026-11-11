package ambient

import (
	"math"
	"sync"
)

// xterm256.go — correct limited-palette (xterm-256) output.
//
// Every color maps to the *nearest* palette entry by perceptual distance
// (CIE76 Lab), chosen from the 6×6×6 cube plus the 24-step gray ramp —
// never per-channel rounding, so hues survive quantization (a dark teal
// gradient stays teal). Ordered Bayer dithering, when enabled, picks
// between the two nearest palette entries only.

const (
	xtermCubeStart = 16
	xtermGrayStart = 232
)

// xtermCubeLevels are the six cube levels as byte values.
var xtermCubeLevels = [6]uint8{0, 95, 135, 175, 215, 255}

// xterm256RGB returns the RGB of palette index idx (0..255).
func xterm256RGB(idx uint8) (uint8, uint8, uint8) {
	switch {
	case idx < 16:
		return xtermSystem[idx].r, xtermSystem[idx].g, xtermSystem[idx].b
	case idx < xtermGrayStart:
		v := int(idx) - xtermCubeStart
		c6 := v % 36
		return xtermCubeLevels[v/36], xtermCubeLevels[c6/6], xtermCubeLevels[c6%6]
	default:
		g := uint8(8 + (int(idx)-xtermGrayStart)*10)
		return g, g, g
	}
}

// xtermSystem holds the 16 system colors (standard xterm values).
var xtermSystem = [16]struct{ r, g, b uint8 }{
	{0, 0, 0}, {205, 0, 0}, {0, 205, 0}, {205, 205, 0},
	{0, 0, 238}, {205, 0, 205}, {0, 205, 205}, {229, 229, 229},
	{127, 127, 127}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0},
	{92, 92, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
}

type lab struct{ L, a, b float64 }

func rgbToLab(r, g, b uint8) lab {
	lin := func(v uint8) float64 {
		c := float64(v) / 255
		if c <= 0.04045 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	R, G, B := lin(r), lin(g), lin(b)
	X := R*0.4124564 + G*0.3575761 + B*0.1804375
	Y := R*0.2126729 + G*0.7151522 + B*0.0721750
	Z := R*0.0193339 + G*0.1191920 + B*0.9503041
	// D65 reference white
	Xn, Yn, Zn := 0.95047, 1.0, 1.08883
	f := func(t float64) float64 {
		if t > 0.008856 {
			return math.Cbrt(t)
		}
		return 7.787*t + 16.0/116.0
	}
	X, Y, Z = f(X/Xn), f(Y/Yn), f(Z/Zn)
	return lab{116*Y - 16, 500 * (X - Y), 200 * (Y - Z)}
}

var xtermLab [256]lab

func init() {
	for i := 0; i < 256; i++ {
		r, g, b := xterm256RGB(uint8(i))
		xtermLab[i] = rgbToLab(r, g, b)
	}
}

func labDist2(p lab) float64 { return p.L*p.L + p.a*p.a + p.b*p.b }

type labCacheKey struct{ r, g, b uint8 }
type labCacheEntry struct {
	first, second uint8
}

var labCache sync.Map

// nearestXterm256Pair returns the two nearest palette indices to (r,g,b)
// by CIE76 distance (first ≤ second). The candidate set is the 6×6×6
// cube plus the 24-step gray ramp (indices 16..255) — the 16 system
// colors are excluded because terminals theme them unpredictably.
// Cached.
func nearestXterm256Pair(r, g, b uint8) (uint8, uint8) {
	key := labCacheKey{r, g, b}
	if v, ok := labCache.Load(key); ok {
		e := v.(labCacheEntry)
		return e.first, e.second
	}
	l := rgbToLab(r, g, b)
	d := func(i int) float64 {
		dl := l.L - xtermLab[i].L
		daa := l.a - xtermLab[i].a
		db := l.b - xtermLab[i].b
		return dl*dl + daa*daa + db*db
	}
	best1, best2 := uint8(16), uint8(17)
	d1, d2 := math.Inf(1), math.Inf(1)
	for i := 16; i < 256; i++ {
		di := d(i)
		if di < d1 {
			d2, best2 = d1, best1
			d1, best1 = di, uint8(i)
			continue
		}
		if di < d2 {
			d2, best2 = di, uint8(i)
		}
	}
	labCache.Store(key, labCacheEntry{best1, best2})
	return best1, best2
}

// nearestXterm256 maps (r,g,b) to the nearest palette entry by perceptual
// distance (cube + gray ramp + system colors).
func nearestXterm256(r, g, b uint8) uint8 {
	f, _ := nearestXterm256Pair(r, g, b)
	return f
}

// nearestXterm256Dither maps (r,g,b) to a palette entry. With dither off,
// returns the nearest entry. With dither on, ordered-dithers between the
// two nearest entries only: frac is the normalized Lab distance from the
// nearer entry to the farther one; the Bayer threshold of (x, y) decides.
func nearestXterm256Dither(r, g, b uint8, x, y int, dither bool) uint8 {
	first, second := nearestXterm256Pair(r, g, b)
	if !dither || first == second {
		return first
	}
	l := rgbToLab(r, g, b)
	d1 := labDist2(subLab(l, xtermLab[first]))
	d2 := labDist2(subLab(l, xtermLab[second]))
	frac := d1 / (d1 + d2) // 0 → exactly first, 1 → exactly second
	if frac > bayerThreshold(x, y) {
		return second
	}
	return first
}

func subLab(a, b lab) lab { return lab{a.L - b.L, a.a - b.a, a.b - b.b} }

// appendXtermSGR appends "\x1b[38;5;Nm" (fg) or "\x1b[48;5;Nm".
func appendXtermSGR(dst []byte, fg bool, idx uint8) []byte {
	dst = append(dst, 0x1b, '[')
	if fg {
		dst = append(dst, '3', '8', ';', '5', ';')
	} else {
		dst = append(dst, '4', '8', ';', '5', ';')
	}
	dst = appendByte(dst, idx)
	return append(dst, 'm')
}
