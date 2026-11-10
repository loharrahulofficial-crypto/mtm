package ambient

import (
	"math"
	"testing"
)

func TestSubpixelMotionIsContinuous(t *testing.T) {
	m := testPalette()
	r, g, bl := m.rgbF3(m.lookup["#d3ae78"])
	hues := []float64{}
	alphas := []float64{}
	for _, fx := range []float64{2.0, 2.25, 2.5, 2.75, 3.0} {
		fb := newFBuffer(6, 3)
		br, bg, bb := m.rgbF(m.base)
		fb.baseFill(br, bg, bb)
		fb.fillCircleF(fx, 1.5, 0.9, r, g, bl, 1, opNormal)
		ds := fb.downsample()
		// centroid of painted coverage over the base
		var sum, weight float64
		for x := 0; x < 6; x++ {
			for y := 0; y < 3; y++ {
				p := ds[y*6+x]
				d := math.Abs(float64(p.r)-float64(br)) + math.Abs(float64(p.g)-float64(bg)) + math.Abs(float64(p.b)-float64(bb))
				sum += float64(x) * d
				weight += d
			}
		}
		hues = append(hues, sum/weight)
		alphas = append(alphas, weight)
	}
	for i := 1; i < len(hues); i++ {
		step := hues[i] - hues[i-1]
		if step <= 0.1 || step >= 0.6 {
			t.Fatalf("centroid jump %.3f — motion snapped instead of gliding (%v)", step, hues)
		}
	}
}
