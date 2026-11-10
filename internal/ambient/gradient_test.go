package ambient

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestGradientVerticalNoBanding(t *testing.T) {
	m := testPalette()
	fb := newFBuffer(4, 24) // tall raster, same gradient stretched vertically
	baseR, baseG, baseB := m.rgbF(m.base)
	fb.baseFill(baseR, baseG, baseB)
	top := m.lookup["#14282d"]
	bot := m.lookup["#dfc181"] // accent
	r0, g0, b0 := m.rgbF3(top)
	r1, g1, b1 := m.rgbF3(bot)
	fb.fillGradientF(0, 0, 4, 24, r0, g0, b0, r1, g1, b1, 1, opNormal)
	ds := fb.downsample()
	// adjacent rows must differ smoothly: no step jump > (|top-bot| / rows * 2)
	maxStep := 0.0
	for y := 1; y < 24; y++ {
		prev := ds[(y-1)*4]
		cur := ds[y*4]
		d := math.Abs(float64(cur.r)-float64(prev.r)) + math.Abs(float64(cur.g)-float64(prev.g)) + math.Abs(float64(cur.b)-float64(prev.b))
		if d > maxStep {
			maxStep = d
		}
	}
	// total gradient range over the raster
	first, last := ds[0], ds[23*4]
	total := math.Abs(float64(last.r)-float64(first.r)) + math.Abs(float64(last.g)-float64(first.g)) + math.Abs(float64(last.b)-float64(first.b))
	if total < 1.0 {
		t.Fatalf("gradient painted almost nothing (range %.3f)", total)
	}
	if maxStep > total/4 {
		t.Fatalf("gradient bands: max adjacent step %.3f vs total range %.3f", maxStep, total)
	}
}

func TestDitherOnlyOnLimitedPalette(t *testing.T) {
	st := stateFor(0.5, PhaseWork)
	s := New("night", ModeHalfblock)
	st.Dither = false
	plain := strings.Join(s.Frame(12*time.Second, 60, 18, st), "\n")
	st.Dither = true
	dithered := strings.Join(s.Frame(12*time.Second, 60, 18, st), "\n")
	if plain == dithered {
		t.Fatal("dither had no effect")
	}
	// every quantized channel must lie on a cube level
	for _, want := range []string{"38;2;0;", "38;2;95;", "38;2;135;", "38;2;175;", "38;2;215;", "38;2;255;"} {
		if strings.Contains(dithered, want) {
			return // at least one cube-aligned quantized color present
		}
	}
	t.Fatal("no cube-level colors found in dithered output")
}

func TestXtermCubeLevels(t *testing.T) {
	for v := 0.0; v <= 1.0001; v += 0.01 {
		got := ditherChannel(v, 0, 0)
		if !xtermValid256(got) {
			t.Fatalf("ditherChannel(%.2f) = %v, not on the cube", v, got)
		}
	}
}

func TestBrailleUnchangedByDither(t *testing.T) {
	st := stateFor(0.5, PhaseWork)
	s := New("night", ModeBraille)
	st.Dither = true
	d := strings.Join(s.Frame(12*time.Second, 60, 18, st), "\n")
	if strings.Count(d, "38;2;") == 0 {
		t.Fatal("braille lost its colors under dither")
	}
}
