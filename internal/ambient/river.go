package ambient

import (
	"math"
	"time"
)

// riverScene paints a golden-hour river valley: warm sky, a soft low
// sun, one hazy far ridge, and a winding river that widens toward the
// viewer carrying the sky's color. Movement comes from pulsing flow
// dashes drifting downstream, a flickering sun glitter column on the
// water, swaying bank grass, and a few fireflies. Cozy-reference
// techniques: short directional pulses for flow, a shortened shimmer
// reflection, and foreground reeds.
type riverScene struct {
	mode string
}

func (s *riverScene) Name() string { return "river" }

func (s *riverScene) Frame(t time.Duration, w, h int, state SceneState) []Line {
	return renderFrame(s, s.mode, t, w, h, state, false)
}

func (s *riverScene) Reward(t time.Duration, w, h int, state SceneState) []Line {
	return renderFrame(s, s.mode, t, w, h, state, true)
}

func (s *riverScene) FramePixels(t time.Duration, w, h int, state SceneState) Raster {
	return renderRaster(s, t, w, h, state)
}

// riverCenter returns the river centerline x at sub-raster row y and
// the half-width there, for a river that runs from the horizon to the
// viewer, wandering more as it nears.
func riverCenter(riv *perlin, W, H, horizon, y float64) (cx, hw float64) {
	v := clampf((y-horizon)/(H-horizon+0.001), 0, 1)
	cx = W*0.5 + riv.At(v*3.0+5.0, 2.2)*W*0.16*(0.25+v)
	hw = W*0.02 + v*v*W*0.26
	return cx, hw
}

func (s *riverScene) paint(b *buffer, p *paletteModel, t time.Duration, w, h int, state SceneState) {
	W := float64(sw(b))
	H := float64(sh(b))
	lay := layoutForState(state, W, H)
	rmp := BuildRamps(state.Palette)
	ts := float64(wrapT(t, 10*time.Minute)) / float64(time.Second)
	base := state.Palette.Base
	bright := state.Palette.BrightText
	warn := state.Palette.Warning
	horizon := lay.Horizon

	// --- sky: warm golden-hour gradient
	b.fillGradientHex(0, 0, W, H, rmp.SkyTop, rmp.SkyHorizon, 1, opNormal)

	// --- sun: low and soft, sinking a little along the work arc
	prog := clampf(state.Progress, 0, 1)
	sunR := lay.MinDim * 0.07
	if sunR < 1.2 {
		sunR = 1.2
	}
	sunX := W * 0.36
	sunY := horizon - lay.MinDim*(0.34-0.20*prog)
	sunY = clampf(sunY, sunR+0.5, horizon-sunR*0.3)
	b.anchor("sun", sunX-sunR, sunY-sunR, sunX+sunR, sunY+sunR)
	b.beginLayer(layerCelestial, maxInt(0, int(sunX-sunR*3)), maxInt(0, int(sunY-sunR*3)), minInt(sw(b), int(sunX+sunR*3)+2), minInt(sh(b), int(sunY+sunR*3)+2))
	haloPulse := 0.5 + reducedAmp(state, 0.14)*math.Sin(ts*0.42)
	sunCol := blendHex(warn, bright, 0.4)
	b.fillCircleHexF(sunX, sunY, sunR*2.4, blendHex(sunCol, rmp.SkyHorizon, 0.55), 0.30*haloPulse+0.15, opAdd)
	b.fillCircleHexF(sunX, sunY, sunR, sunCol, 1, opNormal)
	b.fillCircleHexF(sunX-sunR*0.2, sunY-sunR*0.22, sunR*0.62, blendHex(sunCol, bright, 0.45), 1, opNormal)
	b.endLayer()

	// --- clouds: two soft masses melting into the gradient
	cloud := cachedPerlin(state.Seed, 611)
	b.beginLayer(layerSky, 0, 0, sw(b), int(horizon)+1)
	cloudCol := blendHex(rmp.Haze, rmp.Highlight, 0.22)
	for i := 0; i < 2; i++ {
		cr := newRNG(state.Seed, uint64(1600+i*7))
		cy := H * cr.rangeF(0.06, 0.14)
		cw := W * cr.rangeF(0.12, 0.22)
		speed := cr.rangeF(1.0, 2.2)
		cx := math.Mod(cr.rangeF(0, 1)*W+ts*speed*(0.4+reducedAmp(state, 0.6)), W+cw*2) - cw
		if cx < -cw {
			cx += W + cw*2
		}
		for k := 0; k < 3; k++ {
			br := newRNG(state.Seed, uint64(1630+i*9+k*3))
			dx := (float64(k)/3.0 - 0.5) * cw
			dy := cloud.At(float64(i)*2.9+float64(k)*1.1, ts*0.03) * lay.MinDim * 0.018
			rx := cw * br.rangeF(0.2, 0.32)
			ry := rx * br.rangeF(0.28, 0.4)
			b.fillEllipseHexF(cx+dx, cy+dy, rx, ry, cloudCol, 0.13+0.09*br.float64(), opNormal)
		}
	}
	b.endLayer()

	// --- far ridge: one hazy layer under the sky
	sil := cachedPerlin(state.Seed, 711)
	b.anchor("horizon", 0, horizon-0.01, W, horizon+0.01)
	landCol := rmp.ApplyHaze(blendHex(rmp.Mid, base, 0.28), 0.15)
	b.beginLayer(layerLand, 0, int(horizon-H*0.14), sw(b), sh(b))
	for x := 0.0; x < W; x += 0.75 {
		fy := horizon + sil.At(x*0.012+7.1, 5.5)*H*0.045
		ridge := rmp.ApplyHaze(rmp.Mid, 0.8)
		b.fillGradientHex(x, fy, x+0.75, H, ridge, blendHex(ridge, base, 0.45), 1, opNormal)
	}
	// land base below the ridge line
	b.fillGradientHex(0, horizon+H*0.05, W, H, landCol, blendHex(landCol, base, 0.45), 1, opNormal)

	// --- the river: winding band widening toward the viewer
	riv := cachedPerlin(state.Seed, 811)
	for y := horizon; y < H; y += 0.5 {
		cx, hw := riverCenter(riv, W, H, horizon, y)
		v := clampf((y-horizon)/(H-horizon+0.001), 0, 1)
		water := blendHex(rmp.SkyHorizon, rmp.Mid, 0.30+0.30*v)
		b.fillRectHexF(cx-hw, y, cx+hw, y+0.5, water, 1, opNormal)
		// sun glitter column: a shortened, flickering reflection that
		// follows the river where it passes under the sun
		if math.Abs(cx-sunX) < hw*0.6+0.4 {
			flick := math.Sin(ts*3.1 + y*1.25)
			if flick > -0.2 {
				glint := blendHex(sunCol, bright, 0.3)
				gw := hw * (0.10 + 0.08*float64(flick))
				gx := sunX + riv.At(y*0.10+3.0, ts*0.05)*hw*0.14
				b.fillRectHexF(gx-gw, y, gx+gw, y+0.4, glint, 0.30+0.25*float64(flick), opAdd)
			}
		}
	}

	// --- flow dashes: short bright pulses drifting downstream (the
	// directional-flow read: staggered, seeded, perspective-scaled)
	b.beginLayer(layerSea, 0, int(horizon), sw(b), sh(b))
	for i := 0; i < 10; i++ {
		fr := newRNG(state.Seed, uint64(1700+i*11))
		v0 := fr.float64()
		xOff := fr.rangeF(-0.55, 0.55)
		speed := fr.rangeF(0.025, 0.055)
		v := math.Mod(v0+ts*speed*(0.5+reducedAmp(state, 0.5)), 1.0)
		y := horizon + v*(H-horizon)
		cx, hw := riverCenter(riv, W, H, horizon, y)
		dashLen := hw * (0.16 + 0.22*v)
		dx := cx + xOff*hw*0.9
		pulse := 0.55 + 0.45*math.Sin(ts*2.2+v0*6.28+float64(i)*1.3)
		if pulse <= 0.15 {
			continue
		}
		col := blendHex(rmp.Highlight, bright, 0.25)
		b.fillRectHexF(dx-dashLen, y, dx+dashLen, y+0.35, col, 0.28*pulse, opAdd)
	}
	b.endLayer()

	// --- bank grass: short strokes swaying with a slow breeze
	b.beginLayer(layerFoliage, 0, int(horizon), sw(b), sh(b))
	grassCol := blendHex(landCol, rmp.Highlight, 0.42)
	for i := 0; i < 22; i++ {
		gr := newRNG(state.Seed, uint64(1800+i*13))
		v := gr.rangeF(0.15, 0.95)
		side := 1.0
		if gr.float64() < 0.5 {
			side = -1.0
		}
		xOff := gr.rangeF(1.08, 1.7)
		phase := gr.rangeF(0, 6.28)
		y := horizon + v*(H-horizon)
		cx, hw := riverCenter(riv, W, H, horizon, y)
		x := cx + side*hw*xOff
		if x < 0.5 || x > W-0.5 {
			continue
		}
		sway := math.Sin(ts*0.9+phase) * 0.35
		l := lay.MinDim * (0.012 + v*0.022)
		b.strokeHexF(x, y, x+sway, y-l, 0.2, grassCol, 0.6, opNormal)
	}
	// foreground reeds: taller, slower, in the bottom corners
	for i := 0; i < 5; i++ {
		rr := newRNG(state.Seed, uint64(1900+i*7))
		v := rr.rangeF(0.88, 1.0)
		side := 1.0
		if rr.float64() < 0.5 {
			side = -1.0
		}
		y := horizon + v*(H-horizon)
		cx, hw := riverCenter(riv, W, H, horizon, y)
		x := clampf(cx+side*hw*rr.rangeF(1.05, 1.5), 0.5, W-0.5)
		sway := math.Sin(ts*0.6+float64(i)*1.1) * 0.6
		l := lay.MinDim * rr.rangeF(0.045, 0.075)
		b.strokeHexF(x, y, x+sway, y-l, 0.26, grassCol, 0.75, opNormal)
	}
	// fireflies: a few drifting glow motes along the banks
	fly := cachedPerlin(state.Seed, 911)
	for i := 0; i < 6; i++ {
		fr := newRNG(state.Seed, uint64(2000+i*17))
		v := fr.rangeF(0.3, 0.9)
		side := 1.0
		if fr.float64() < 0.5 {
			side = -1.0
		}
		xOff := fr.rangeF(1.1, 1.8)
		phase := fr.rangeF(0, 6.28)
		y := horizon + v*(H-horizon) + fly.At(float64(i)*1.7, ts*0.06)*H*0.02
		cx, hw := riverCenter(riv, W, H, horizon, y)
		x := cx + side*hw*xOff + fly.At(float64(i)*3.1+7.0, ts*0.05)*W*0.02
		if x < 0.3 || x > W-0.3 || y < horizon || y > H-0.3 {
			continue
		}
		pulse := math.Sin(ts*1.3 + phase)
		if pulse < 0.1 {
			continue
		}
		glow := blendHex(warn, bright, 0.45)
		if s.mode == ModeASCII {
			b.mark(int(x), int(y), '·', mixIndex(p, cWarning, p.base, 0.35))
			continue
		}
		b.fillCircleHexF(x, y, 0.24, glow, 0.55*pulse, opNormal)
		b.glowHexAt(x, y, glow, 0.20*pulse)
	}
	b.endLayer()
}
