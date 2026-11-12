package ambient

import (
	"math"
	"time"
)

// mountainsScene paints a cozy dusk valley in the Firewatch idiom: one
// bold warm gradient, a soft low sun with a breathing halo, simple
// cloud masses that blend into the sky rather than fighting it, and
// four ridge layers dissolving into haze with atmospheric perspective.
// Snow stipple on the near crests and slow gliding birds give the
// fine-grid modes (quadrant/sextant) visible detail. The sun sinks
// gently along the pomodoro arc; break phase holds it high.
type mountainsScene struct {
	mode string
}

func (s *mountainsScene) Name() string { return "mountains" }

func (s *mountainsScene) Frame(t time.Duration, w, h int, state SceneState) []Line {
	return renderFrame(s, s.mode, t, w, h, state, false)
}

func (s *mountainsScene) Reward(t time.Duration, w, h int, state SceneState) []Line {
	return renderFrame(s, s.mode, t, w, h, state, true)
}

func (s *mountainsScene) FramePixels(t time.Duration, w, h int, state SceneState) Raster {
	return renderRaster(s, t, w, h, state)
}

func (s *mountainsScene) paint(b *buffer, p *paletteModel, t time.Duration, w, h int, state SceneState) {
	W := float64(sw(b))
	H := float64(sh(b))
	lay := layoutForState(state, W, H)
	rmp := BuildRamps(state.Palette)
	ts := float64(wrapT(t, 10*time.Minute)) / float64(time.Second)
	base := state.Palette.Base
	bright := state.Palette.BrightText
	warn := state.Palette.Warning

	// --- sky: one bold dusk gradient, zenith to warm horizon
	b.fillGradientHex(0, 0, W, H, rmp.SkyTop, rmp.SkyHorizon, 1, opNormal)

	// --- sun: warm disc with a breathing halo, sinking with the timer
	// arc (work) or holding high (break). Always fully in frame.
	prog := clampf(state.Progress, 0, 1)
	if state.Phase == PhaseBreak {
		prog *= 0.3
	}
	sunR := lay.MinDim * 0.075
	if sunR < 1.2 {
		sunR = 1.2
	}
	sunX := W * (0.62 + 0.10*math.Sin(ts*0.05))
	sunY := lay.Horizon - lay.MinDim*(0.36-0.26*prog)
	sunY = clampf(sunY, sunR+0.5, lay.Horizon-sunR*0.3)
	b.anchor("sun", sunX-sunR, sunY-sunR, sunX+sunR, sunY+sunR)
	b.beginLayer(layerCelestial, maxInt(0, int(sunX-sunR*3)), maxInt(0, int(sunY-sunR*3)), minInt(sw(b), int(sunX+sunR*3)+2), minInt(sh(b), int(sunY+sunR*3)+2))
	haloPulse := 0.5 + reducedAmp(state, 0.14)*math.Sin(ts*0.42)
	sunCol := blendHex(warn, bright, 0.35)
	b.fillCircleHexF(sunX, sunY, sunR*2.4, blendHex(sunCol, rmp.SkyHorizon, 0.55), 0.30*haloPulse+0.15, opAdd)
	b.fillCircleHexF(sunX, sunY, sunR*1.5, blendHex(sunCol, rmp.SkyHorizon, 0.35), 0.35*haloPulse+0.2, opAdd)
	b.fillCircleHexF(sunX, sunY, sunR, sunCol, 1, opNormal)
	b.fillCircleHexF(sunX-sunR*0.2, sunY-sunR*0.22, sunR*0.62, blendHex(sunCol, bright, 0.45), 1, opNormal)
	b.endLayer()

	// --- clouds: bold soft masses drifting slowly, tinted between the
	// haze and the sun so they melt into the gradient (Firewatch skies).
	cloud := cachedPerlin(state.Seed, 211)
	b.beginLayer(layerSky, 0, 0, sw(b), int(lay.Horizon)+1)
	cloudCol := blendHex(rmp.Haze, rmp.Highlight, 0.22)
	for i := 0; i < 4; i++ {
		cr := newRNG(state.Seed, uint64(910+i*7))
		cy := H * cr.rangeF(0.05, 0.16)
		cw := W * cr.rangeF(0.10, 0.24)
		speed := cr.rangeF(1.2, 3.0)
		if cr.float64() < 0.5 {
			speed = -speed
		}
		cx := math.Mod(cr.rangeF(0, 1)*W+ts*speed*(0.4+reducedAmp(state, 0.6)), W+cw*2) - cw
		if cx < -cw {
			cx += W + cw*2
		}
		bumps := 3 + cr.intn(2)
		for k := 0; k < bumps; k++ {
			br := newRNG(state.Seed, uint64(930+i*11+k*3))
			dx := (float64(k)/float64(bumps) - 0.5) * cw
			dy := cloud.At(float64(i)*3.7+float64(k)*1.3, ts*0.03) * lay.MinDim * 0.02
			rx := cw * br.rangeF(0.22, 0.34)
			ry := rx * br.rangeF(0.28, 0.4)
			a := 0.16 + 0.10*br.float64()
			b.fillEllipseHexF(cx+dx, cy+dy, rx, ry, cloudCol, a, opNormal)
		}
	}
	b.endLayer()

	// --- birds: two tiny gliders crossing the sky with slow wing beats
	b.beginLayer(layerSky, 0, 0, sw(b), int(lay.Horizon)+1)
	birdCol := blendHex(base, bright, 0.42)
	for i := 0; i < 2; i++ {
		br := newRNG(state.Seed, uint64(1200+i*13))
		speed := br.rangeF(1.0, 2.2)
		bx := math.Mod(br.rangeF(0, 1)*W+ts*speed*(0.3+reducedAmp(state, 0.7)), W+4) - 2
		if bx < -2 {
			bx += W + 4
		}
		by := H*br.rangeF(0.10, 0.30) + math.Sin(ts*0.4+float64(i)*2.0)*lay.MinDim*0.015
		flap := math.Sin(ts*2.6+float64(i)*1.9) * 0.32
		if s.mode == ModeASCII {
			glyph := '^'
			if flap < 0 {
				glyph = 'v'
			}
			b.mark(int(clampf(bx, 0, W-1)), int(clampf(by, 0, H-1)), glyph, mixIndex(p, cWarning, p.base, 0.55))
			continue
		}
		wing := lay.MinDim * 0.014
		b.strokeHexF(bx-wing, by-flap*wing, bx, by, 0.2, birdCol, 0.8, opNormal)
		b.strokeHexF(bx, by, bx+wing, by-flap*wing, 0.2, birdCol, 0.8, opNormal)
	}
	b.endLayer()

	// --- ridges: four layers, far hazier and lighter, near darker and
	// steeper — atmospheric perspective carries the depth.
	sil := cachedPerlin(state.Seed, 421)
	b.anchor("horizon", 0, lay.Horizon-0.01, W, lay.Horizon+0.01)
	b.beginLayer(layerLand, 0, int(lay.Horizon-H*0.30), sw(b), sh(b))
	for layer := 0; layer < 4; layer++ {
		depth := 0.88 - float64(layer)*0.23 // far → near
		body := blendHex(rmp.Mid, base, float64(layer)*0.13)
		body = rmp.ApplyHaze(body, clampf(depth, 0, 1))
		baseY := lay.Horizon - H*0.055 + float64(layer)*H*0.055
		amp := H * (0.085 - float64(layer)*0.014)
		freq := 0.030 + float64(layer)*0.014
		spAmp := 0.0
		spFreq := 0.05
		if layer >= 2 {
			spAmp = H * (0.015 + float64(layer-2)*0.012)
			spFreq = 0.05 + float64(layer-2)*0.012
		}
		p := ridgeParams{swellAmp: amp, swellFreq: freq, spireAmp: spAmp, spireFreq: spFreq, phase: 3.0 + float64(layer)*2.1}
		paintRidgeLayer(b, sil, W, treelineLayer{baseY: baseY, yBot: H, params: p, crest: body, base: blendHex(body, base, 0.45)})
		// snow stipple on the two nearest crests above the snowline
		if layer >= 2 {
			for x := 0.0; x < W; x += 0.75 {
				fy := ridgeHeightAt(sil, x, baseY, p)
				snowY := baseY - amp*0.55
				if fy < snowY {
					edge := snowY + sil.At(x*0.16+float64(layer)*7.7, 9.1)*amp*0.16
					capBottom := math.Min(edge, fy+(snowY-fy)*0.55+lay.MinDim*0.012)
					if capBottom > fy {
						snow := rmp.ApplyHaze(rmp.Highlight, clampf(depth*0.55, 0, 1))
						b.fillRectHexF(x, fy, x+0.75, capBottom, snow, 0.85, opNormal)
					}
				}
			}
		}
	}
	// valley fog: a soft warm wash that sits between the mid ridges
	fogA := 0.10 + 0.05*math.Sin(ts*0.11)
	b.fillRectHexF(0, lay.Horizon-H*0.02, W, lay.Horizon+H*0.05, rmp.Haze, fogA, opAdd)
	b.endLayer()
}
