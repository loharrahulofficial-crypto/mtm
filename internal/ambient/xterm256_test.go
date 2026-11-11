package ambient

import (
	"math"
	"strings"
	"testing"
	"time"
)

// xterm256_test.go — §5.4: nearest-palette-entry quantization (cube + gray
// ramp, perceptual distance), never per-channel rounding; teal stays teal.

func TestXterm256CubeMapping(t *testing.T) {
	cases := []struct {
		in   [3]uint8
		want uint8
	}{
		{[3]uint8{0, 0, 0}, 16},
		{[3]uint8{255, 255, 255}, 231},
		{[3]uint8{255, 0, 0}, 196},
		{[3]uint8{0, 255, 0}, 46},
		{[3]uint8{0, 0, 255}, 21},
	}
	for _, c := range cases {
		got := nearestXterm256(c.in[0], c.in[1], c.in[2])
		if got != c.want {
			t.Errorf("nearestXterm256(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestNearestUsesGrayRampForMidtones(t *testing.T) {
	// mid grays should land on the 24-step ramp (232..255)
	for v := uint8(40); v <= 240; v += 40 {
		got := nearestXterm256(v, v, v)
		r, g, b := xterm256RGB(got)
		if absDiff(int(r), int(v)) > 26 || absDiff(int(g), int(v)) > 26 {
			t.Fatalf("gray %d mapped to idx %d (%d,%d,%d)", v, got, r, g, b)
		}
	}
}

func absDiff(a, b int) int {
	if a < b {
		return b - a
	}
	return a - b
}

// hueSat converts RGB to HSV and returns hue (degrees) and saturation.
func hueSat(r, g, b uint8) (float64, float64) {
	R, G, B := float64(r)/255, float64(g)/255, float64(b)/255
	mx, mn := R, R
	if G > mx {
		mx = G
	}
	if B > mx {
		mx = B
	}
	if G < mn {
		mn = G
	}
	if B < mn {
		mn = B
	}
	delta := mx - mn
	sat := 0.0
	if mx > 1e-9 {
		sat = delta / mx
	}
	h := 0.0
	switch {
	case delta < 1e-9:
		return 0, 0
	case mx == R:
		h = 60 * math.Mod((G-B)/delta, 6)
	case mx == G:
		h = 60 * ((B-R)/delta + 2)
	default:
		h = 60 * ((R-G)/delta + 4)
	}
	if h < 0 {
		h += 360
	}
	return h, sat
}

func hueDrift(a, b [3]uint8) float64 {
	h1, s1 := hueSat(a[0], a[1], a[2])
	h2, s2 := hueSat(b[0], b[1], b[2])
	if s1 < 0.10 || s2 < 0.10 {
		return 0 // near-gray pixels have no meaningful hue
	}
	d := math.Abs(h1 - h2)
	if d > 180 {
		d = 360 - d
	}
	return d
}

func TestDarkTealGradientStaysInFamily(t *testing.T) {
	ramps := BuildRamps(DefaultPalette())
	base := DefaultPalette().Base
	strong := DefaultPalette().SurfaceStrong
	for _, grad := range [][2]string{{ramps.SkyTop, ramps.SkyHorizon}, {base, strong}} {
		for i := 0; i <= 100; i++ {
			src := mustRGB(blendHex(grad[0], grad[1], float64(i)/100))
			srcArr := [3]uint8{src.r, src.g, src.b}
			snapR, snapG, snapB := xterm256RGB(nearestXterm256(src.r, src.g, src.b))
			snap := [3]uint8{snapR, snapG, snapB}
			if d := hueDrift(srcArr, snap); d > 30 {
				t.Fatalf("gradient %s→%s step %d (#%02x%02x%02x): snapped to #%02x%02x%02x, hue drifts %.1f° > 30°",
					grad[0], grad[1], i, src.r, src.g, src.b, snap[0], snap[1], snap[2], d)
			}
		}
	}
}

func TestLimitedOutputHasNo24BitSequences(t *testing.T) {
	st := stateFor(0.5, PhaseWork)
	st.PaletteLimited = true
	s := New("night", ModeHalfblock)
	frame := strings.Join(s.Frame(12*time.Second, 60, 18, st), "\n")
	if strings.Contains(frame, "38;2;") || strings.Contains(frame, "48;2;") {
		t.Fatal("limited-palette output contains 24-bit sequences")
	}
	if !strings.Contains(frame, "38;5;") {
		t.Fatal("limited-palette output contains no 256-color sequences")
	}
}

func TestTruecolorOutputHasNo256Sequences(t *testing.T) {
	st := stateFor(0.5, PhaseWork)
	st.PaletteLimited = false
	s := New("night", ModeHalfblock)
	frame := strings.Join(s.Frame(12*time.Second, 60, 18, st), "\n")
	if strings.Contains(frame, "38;5;") || strings.Contains(frame, "48;5;") {
		t.Fatal("truecolor output contains 256-color sequences")
	}
	if !strings.Contains(frame, "38;2;") {
		t.Fatal("truecolor output contains no 24-bit sequences")
	}
}
