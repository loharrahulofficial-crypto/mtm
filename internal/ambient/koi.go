package ambient

import (
	"math"
	"time"
)

// koiScene paints a pond: depth-graded water bands with parallax sine
// lanes, caustic shimmer, lily pads, two big koi with smooth body
// undulation and fins, expanding ripples, and petals drifting down.
type koiScene struct {
	mode string
}

func (s *koiScene) Name() string { return "koi" }

func (s *koiScene) Frame(t time.Duration, w, h int, state SceneState) []Line {
	return renderFrame(s, s.mode, t, w, h, state, false)
}

func (s *koiScene) Reward(t time.Duration, w, h int, state SceneState) []Line {
	return renderFrame(s, s.mode, t, w, h, state, true)
}

func (s *koiScene) FramePixels(t time.Duration, w, h int, state SceneState) Raster {
	return renderRaster(s, t, w, h, state)
}

func (s *koiScene) paint(b *buffer, p *paletteModel, t time.Duration, w, h int, state SceneState) {
	W := float64(sw(b))
	H := float64(sh(b))
	base := state.Palette.Base
	bright := state.Palette.BrightText
	rmp := BuildRamps(state.Palette)
	ts := tSecF(t)

	// --- water: four depth-graded bands with faint caustic lanes
	waterCols := []string{
		rmp.Mid,
		blendHex(state.Palette.Cyan, rmp.Mid, 0.35),
		blendHex(state.Palette.Blue, base, 0.35),
		blendHex(state.Palette.Blue, base, 0.62),
	}
	bandH := H / 4
	for i, col := range waterCols {
		y0 := float64(i) * bandH
		b.fillGradientHex(0, y0, W, y0+float64(bandH)+0.8, col, blendHex(col, base, 0.35), 1, opNormal)
	}
	// parallax lanes: long faint sine highlights drifting at different speeds
	lanes := []struct {
		yFrac, amp, freq, speed float64
		alpha                   float64
	}{
		{0.18, 0.35, 0.22, 0.35, 0.35},
		{0.38, 0.5, 0.16, 0.26, 0.3},
		{0.58, 0.65, 0.12, 0.2, 0.28},
		{0.78, 0.8, 0.09, 0.15, 0.25},
	}
	lanesN := cachedPerlin(state.Seed, 71)
	for li, ln := range lanes {
		for x := 0.0; x < W; x += 0.8 {
			y := ln.yFrac*H + math.Sin(x*ln.freq+ts*ln.speed+float64(li)*9.1)*ln.amp + lanesN.At(x*0.06, float64(li)*13.7)*0.6
			b.strokeHexF(x, y, x+0.8, y, 0.3, blendHex(state.Palette.Cyan, base, 0.35+float64(3-li)*0.1), ln.alpha, opNormal)
		}
	}
	// caustic shimmer: sparse twinkling light dots, two depths
	shimmer := cachedPerlin(state.Seed, 73)
	for i := 0; i < 26; i++ {
		r := newRNG(state.Seed, uint64(i)*733+29)
		sx := r.float64() * W
		sy := (0.12 + r.float64()*0.8) * H
		tw := shimmer.At(ts*0.5+float64(i)*3.1, float64(i)*1.7)
		if tw <= 0.25 {
			continue
		}
		a := (tw - 0.25) * 0.5
		b.fillCircleHexF(sx, sy, 0.36, blendHex(state.Palette.Cyan, state.Palette.BrightText, 0.3), a, opAdd)
	}

	// --- lily pads on the surface
	b.beginLayer(layerFoliage, 0, 0, sw(b), sh(b))
	for i := 0; i < 5; i++ {
		lr := newRNG(state.Seed, uint64(i)*412+19)
		px := lr.rangeF(0.06, 0.94) * W
		py := lr.rangeF(0.12, 0.5) * H
		padR := 1.5 + lr.float64()*1.3
		padCol := blendHex(state.Palette.Success, base, 0.3+lr.float64()*0.25)
		b.fillEllipseHexF(px, py, padR, padR*0.42, padCol, 1, opNormal)
		// notch: a small wedge cut out with base-colored stroke
		b.strokeHexF(px, py, px+padR*0.95, py-padR*0.3, 0.22, blendHex(base, padCol, 0.4), 1, opNormal)
		// vein
		b.strokeHexF(px-padR*0.7, py, px+padR*0.6, py, 0.14, blendHex(padCol, bright, 0.3), 0.8, opNormal)
	}
	b.endLayer()

	// --- the koi: two big fish on slow lissajous swim paths
	drawKoi(b, state.Palette, ts, W, H, state.Seed, 0, state.ReducedMotion)
	drawKoi(b, state.Palette, ts, W, H, state.Seed, 1, state.ReducedMotion)

	// --- ripples expanding at beat slots
	b.beginLayer(layerSea, 0, 0, sw(b), sh(b))
	ripSlot := int64(wrapT(t, 24*time.Hour)/time.Millisecond) / 1400
	for k := 0; k < 3; k++ {
		slot := ripSlot - int64(k)
		rr := newRNG(state.Seed, uint64(slot)*521+31)
		frac := (float64(wrapT(t, 24*time.Hour)/time.Millisecond) / 1000.0) - float64(slot)*1.4
		if frac < 0 || frac >= 1.4 {
			continue
		}
		progress := frac / 1.4
		rx := rr.rangeF(0.2, 0.8) * W
		ry := rr.rangeF(0.25, 0.75) * H
		b.strokeCircleHexF(rx, ry, 0.3+progress*3.4, 0.3, state.Palette.Cyan, 0.5*(1-progress), opNormal)
	}
	b.endLayer()

	// --- drifting petals landing on the water
	for i := 0; i < 4; i++ {
		slot := int64(wrapT(t, 24*time.Hour)/time.Millisecond)/2100 + int64(i)
		pfrac := (float64(wrapT(t, 24*time.Hour)/time.Millisecond) / 1000.0) - float64(slot)*2.1
		if pfrac < 0 || pfrac >= 2.1 {
			continue
		}
		fr := newRNG(state.Seed^0xa41, uint64(i)*97+53)
		px := fr.rangeF(0.1, 0.9) * W
		py := pfrac * H * 0.42
		sway := math.Sin(pfrac*4.2+float64(i)*1.9) * 1.5
		col := blendHex(state.Palette.Magenta, state.Palette.BrightText, 0.35)
		b.fillCircleHexF(px+sway, py, 0.3, col, 0.9, opNormal)
	}

	// --- bottom stones: a few dark ellipses at the floor
	b.beginLayer(layerLand, 0, int(H*0.8), sw(b), sh(b))
	for i := 0; i < 6; i++ {
		r := newRNG(state.Seed, uint64(i)*311+7)
		sx := r.float64() * W
		ss := 0.8 + r.float64()*1.6
		b.fillEllipseHexF(sx, H-0.5, ss*1.6, ss*0.6, blendHex(base, state.Palette.SurfaceStrong, 0.28), 1, opNormal)
	}
	b.endLayer()
}

// drawKoi renders one koi: a spine chain of body blobs that undulates
// along its swim path, two-tone with patches, a fanned tail fin, eyes
// and fluttering pectoral fins.
func drawKoi(b *buffer, pal Palette, ts, W, H float64, seed int64, variant int, reduced bool) {
	undAmp := 0.30
	if reduced {
		undAmp = 0.045
	}
	pathF := func(tt float64) (float64, float64) {
		if variant == 0 {
			return (0.5 + 0.33*math.Cos(tt*0.33+1.7)) * W, (0.42 + 0.26*math.Sin(tt*0.47+0.4)) * H
		}
		return (0.5 + 0.35*math.Cos(tt*0.27+4.2)) * W, (0.55 + 0.25*math.Sin(tt*0.39+2.9)) * H
	}
	now := ts
	head := func(tt float64) (float64, float64, float64) {
		x, y := pathF(tt)
		x2, y2 := pathF(tt + 0.18)
		return x, y, math.Atan2(y2-y, x2-x)
	}

	// spine points behind the head at fixed arc-length offsets
	const pts = 11
	segLen := 0.075 * W / float64(pts) * 6 // tuned so the body reads at small sizes
	if segLen > 2.0 {
		segLen = 2.0
	}
	prof := []float64{0.56, 0.82, 1.12, 1.35, 1.42, 1.28, 1.0, 0.72, 0.5, 0.31, 0.17}

	hx, hy, ang := head(now)
	type pt struct{ x, y, a float64 }
	spine := []pt{{hx, hy, ang}}
	for i := 1; i < pts; i++ {
		prev := spine[i-1]
		// undulation: perpendicular sine wave traveling head → tail
		und := math.Sin(now*2.6-float64(i)*0.72+float64(variant)*1.9) * undAmp
		pa := prev.a + und*0.22
		spine = append(spine, pt{prev.x - math.Cos(pa)*segLen, prev.y - math.Sin(pa)*segLen, pa})
	}

	// body colors: koi A = pale orange with white patches; koi B = rich orange, white patches
	var bodyCol, patchCol, bellyCol, finCol string
	if variant == 0 {
		bodyCol = blendHex(pal.BrightText, pal.Orange, 0.55)
		patchCol = blendHex(bodyCol, pal.Base, 0.15)
		bellyCol = blendHex(bodyCol, pal.BrightText, 0.3)
	} else {
		bodyCol = blendHex(pal.Orange, pal.BrightText, 0.22)
		patchCol = blendHex(pal.BrightText, pal.Orange, 0.35)
		bellyCol = blendHex(bodyCol, pal.BrightText, 0.35)
	}
	finCol = blendHex(pal.Orange, pal.Base, 0.35)

	// tail fin: a fan behind the tail tip, undulating
	tail := spine[pts-1]
	perp := tail.a + math.Pi/2
	fan := math.Sin(now*2.6-float64(pts)*0.72+float64(variant)*1.9) * 0.4
	for i, spread := range []float64{-0.55, -0.25, 0.05, 0.35} {
		a := tail.a + fan + spread
		tipX := tail.x + math.Cos(a)*2.6
		tipY := tail.y + math.Sin(a)*2.6
		b.strokeHexF(tail.x, tail.y, tipX, tipY, 0.85, finCol, 0.9, opNormal)
		_ = i
		_ = perp
	}

	// pectoral fins: two small strokes on the sides near the head
	for _, side := range []float64{-1, 1} {
		fl := math.Sin(now*3.4+side*1.1+float64(variant)) * 0.25
		fx := spine[2].x + math.Cos(spine[2].a+side*(0.8+fl))*1.5
		fy := spine[2].y + math.Sin(spine[2].a+side*(0.8+fl))*1.5
		b.strokeHexF(spine[2].x, spine[2].y, fx, fy, 0.5, finCol, 0.85, opNormal)
	}

	// body from tail to head so the head overlaps
	for i := pts - 1; i >= 0; i-- {
		sp := spine[i]
		r := prof[i]
		// belly lighter under, back darker above (light from above)
		b.fillCircleHexF(sp.x, sp.y, r, bodyCol, 1, opNormal)
		b.fillCircleHexF(sp.x, sp.y+r*0.35, r*0.72, bellyCol, 0.9, opNormal)
		// patches: deterministic per segment
		lr := newRNG(seed, uint64(i)*61+11+uint64(variant)*7)
		if lr.float64() < 0.42 && i > 0 && i < pts-2 {
			b.fillCircleHexF(sp.x+(lr.float64()-0.5)*r, sp.y+(lr.float64()-0.5)*r*0.7, r*0.48, patchCol, 0.95, opNormal)
		}
	}

	// eyes: two dark dots on the head sides
	for _, side := range []float64{-1, 1} {
		ea := spine[0].a + side*math.Pi/2
		ex := spine[0].x + math.Cos(ea)*0.30
		ey := spine[0].y + math.Sin(ea)*0.30
		b.fillCircleHexF(ex, ey, 0.16, pal.Base, 1, opNormal)
	}
}
