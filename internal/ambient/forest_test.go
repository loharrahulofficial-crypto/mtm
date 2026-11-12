package ambient

import (
	"math"
	"testing"
	"time"
)

// forest_test.go — the treeline must paint as one continuous gradient,
// never as stripy columns: deep inside a layer's fill (same layer in
// both adjacent sub-cells, well below its ridge line) the color matches
// exactly; the lake below the silhouette shows no vertical striping
// either (only a couple of bright feature pairs are allowed).

func TestTreelineAndLakeNoVerticalStripes(t *testing.T) {
	for _, size := range [][2]int{{100, 30}, {200, 50}} {
		w, h := size[0], size[1]
		st := stateFor(0.45, PhaseWork)
		st.FullFrame = true
		sc := New("night", ModeHalfblock)
		lay := layoutForState(st, float64(w), float64(h*2))
		H := float64(h * 2)
		layH := lay.Horizon
		sil := cachedPerlin(st.Seed, 101)
		rmp := BuildRamps(st.Palette)
		layers := nightTreelineLayers(H, layH, st.Palette.SurfaceStrong, st.Palette.Base, rmp)
		topmost := func(x, y float64) int {
			best := -1
			for i := range layers {
				if y <= layers[i].yBot && ridgeHeightAt(sil, x, layers[i].baseY, layers[i].params) <= y {
					best = i
				}
			}
			return best
		}
		inMist := func(y float64) bool {
			for i := 0; i < 3; i++ {
				c := layH + H*0.012 - 1.2 + float64(i)*1.3
				if math.Abs(y-c) < 1.1 {
					return true
				}
			}
			return false
		}
		r := sc.FramePixels(3*time.Second, w, h, st)
		pix := func(x, y int) [3]float32 {
			c := r.Pix[y*r.W+x]
			return [3]float32{c.r, c.g, c.b}
		}
		delta := func(a, b [3]float32) float64 {
			d := 0.0
			for i := range a {
				dd := absF(float64(a[i]-b[i])) * 255
				if dd > d {
					d = dd
				}
			}
			return d
		}
		maxInterior := 3.0
		for y := 0; y < r.H; y++ {
			if inMist(float64(y) + 0.5) {
				continue
			}
			yf := float64(y) + 0.5
			loudRow := 0
			for x := 1; x < r.W; x++ {
				xf := float64(x) + 0.5
				l0, l1 := topmost(xf-1, yf), topmost(xf, yf)
				if l0 < 0 || l0 != l1 {
					continue // between layers: exempt
				}
				baseY := layers[l0].baseY
				// x-edge: the ridge height changes between the two cells —
				// the feathered boundary column
				if math.Abs(ridgeHeightAt(sil, xf-1, baseY, layers[l0].params)-ridgeHeightAt(sil, xf, baseY, layers[l0].params)) > 0.6 {
					continue
				}
				// y-edge: near the layer's ridge line (top AA)
				if d0 := yf - ridgeHeightAt(sil, xf-1, baseY, layers[l0].params); d0 < 1.0 {
					continue
				}
				if d1 := yf - ridgeHeightAt(sil, xf, baseY, layers[l0].params); d1 < 1.0 {
					continue
				}
				if d := delta(pix(x-1, y), pix(x, y)); d > maxInterior {
					loudRow++
				}
			}
			// a few bright feature dabs (star glow) may sit in the
			// feather zone — that is not striping
			if loudRow > 6 {
				t.Errorf("%dx%d y=%d: %d interior pairs exceed %.1f — vertical striping in the fill", w, h, y, loudRow, maxInterior)
			}
		}
		// below the silhouette edge (treeline + its reflection band): the
		// lake must not stripe either. Bright features (glint path, star
		// reflections) may carry sharp edges; the moon's reflection
		// footprint is excluded entirely.
		prog := 0.45
		mR := lay.MinDim * 0.115
		if mR < 1.6 {
			mR = 1.6
		}
		mX := float64(w) * (0.10 + 0.80*prog)
		mY := layH*0.88 - math.Sin(math.Pi*prog)*layH*0.62
		mY = clampf(mY, mR+0.5, layH-mR*0.4)
		yRefl0 := layH + (layH-(mY+mR))*0.45
		yRefl1 := layH + (layH-(mY-mR))*0.45
		inMoonReflection := func(x, y float64) bool {
			if x >= mX-mR-1 && x <= mX+mR+1 && y >= yRefl0-1 && y <= yRefl1+1 {
				return true
			}
			// the glint path drifts under the moon across the water
			return math.Abs(x-mX) <= 12
		}
		for y := int(layH + H*0.10); y < r.H; y++ {
			for x := 1; x < r.W; x++ {
				d := delta(pix(x-1, y), pix(x, y))
				if d <= 26 {
					continue
				}
				if inMoonReflection(float64(x-1)+0.5, float64(y)+0.5) && inMoonReflection(float64(x)+0.5, float64(y)+0.5) {
					continue
				}
				c0, c1 := pix(x-1, y), pix(x, y)
				b := max3(c0[0], c0[1], c0[2], c1[0], c1[1], c1[2])
				if b > 0.52 { // bright feature (glint, star reflection)
					continue
				}
				t.Errorf("%dx%d y=%d x=%d: delta %.1f below the silhouette — vertical striping", w, h, y, x, d)
			}
		}
	}
}

func max3(a ...float32) float64 {
	m := float64(a[0])
	for _, v := range a[1:] {
		if float64(v) > m {
			m = float64(v)
		}
	}
	return m
}
