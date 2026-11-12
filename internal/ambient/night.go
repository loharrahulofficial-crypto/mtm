package ambient

import (
	"math"
	"time"
)

// nightScene paints a moonlit sky: a vertical gradient (deep zenith to
// warm horizon haze), a filled moon with limb shading, correct lit phase
// and a soft halo, magnitude-varied stars with twinkle and cross-glow on
// the brightest ones, occasional shooting stars, aurora ribbons as
// additive noise curtains with slow vertical shimmer, a still lake below
// the horizon that mirrors the moon, aurora and brightest stars (flipped,
// squashed, dimmed, rippling) with a treeline reflection along the
// shoreline, and a silhouetted horizon in two depths (far haze ridge,
// near pine line).
type nightScene struct {
	mode string
}

func (s *nightScene) Name() string { return "night" }

func (s *nightScene) Frame(t time.Duration, w, h int, state SceneState) []Line {
	return renderFrame(s, s.mode, t, w, h, state, false)
}

func (s *nightScene) Reward(t time.Duration, w, h int, state SceneState) []Line {
	return renderFrame(s, s.mode, t, w, h, state, true)
}

func (s *nightScene) FramePixels(t time.Duration, w, h int, state SceneState) Raster {
	return renderRaster(s, t, w, h, state)
}

// starv is one deterministic star with parallax drift.
type starv struct {
	x, y  float64
	mag   float64
	phase float64
	speed float64
}

func nightStars(seed int64, w, h int) []starv {
	r := newRNG(seed, 17)
	n := int(float64(w*h) / 11)
	if n < 25 {
		n = 25
	}
	out := make([]starv, 0, n)
	for i := 0; i < n; i++ {
		mag := r.float64()
		mag = mag * mag // bias toward dim stars
		out = append(out, starv{
			x:     r.float64() * float64(w),
			y:     r.float64() * float64(h) * 0.72,
			mag:   mag,
			phase: r.rangeF(0, 2*math.Pi),
			speed: r.rangeF(0.6, 1.7),
		})
	}
	return out
}

// glyphFor maps a twinkle value to a legibility glyph (ascii mode).
func glyphFor(tw float64) rune {
	switch {
	case tw > 0.82:
		return '✦'
	case tw > 0.6:
		return '+'
	case tw > 0.38:
		return '·'
	case tw > 0.2:
		return ':'
	default:
		return '.'
	}
}

func (s *nightScene) paint(b *buffer, p *paletteModel, t time.Duration, w, h int, state SceneState) {
	W := float64(sw(b))
	H := float64(sh(b))
	lay := layoutForState(state, float64(sw(b)), float64(sh(b)))
	rmp := BuildRamps(state.Palette)
	ts := float64(wrapT(t, 10*time.Minute)) / float64(time.Second)
	base := state.Palette.Base
	surfaceStrong := state.Palette.SurfaceStrong
	bright := state.Palette.BrightText

	// --- sky: deep zenith to a lifted cool teal at the horizon,
	// interpolated in OKLab (Base → Blue → Cyan), never through the old
	// brown. The far ridge rides on top at the horizon line.
	fillSkyOKLab(b, 0, 0, W, H, nightSkyStops(state.Palette), opNormal)
	// a whisper of warm haze right at the horizon — the one warm note allowed
	b.fillRectHexF(0, lay.Horizon-H*0.018, W, lay.Horizon+0.4, state.Palette.AccentSoft, 0.10, opNormal)

	// --- aurora: three curtains (bright foot, fading upward), plus a
	// whisper of Milky Way arcing behind them
	b.beginLayer(layerSky, 0, 0, sw(b), minInt(sh(b), sh(b)/2+2))
	paintMilkyWay(b, state, ts)
	paintAuroraCurtains(b, state, ts, 1.0, 1.0, nil)
	b.endLayer()

	// --- stars: varied magnitudes, twinkle, and cross-glow on the brightest
	b.beginLayer(layerStars, 0, 0, sw(b), int(H*0.74)+1)
	for _, st := range nightStars(state.Seed, sw(b), sh(b)) {
		tw := 0.55 + 0.45*math.Sin(ts*st.speed+st.phase)
		brightness := st.mag * tw
		if brightness < 0.05 {
			continue
		}
		fade := 1 - brightness*0.82
		if s.mode == ModeASCII {
			b.mark(int(st.x), int(st.y), glyphFor(brightness), mixIndex(p, cBrightText, p.base, fade))
			continue
		}
		r := 0.18 + brightness*0.5
		col := blendHex(bright, base, fade)
		switch {
		case brightness > 0.78: // brighter star with a soft cross glow
			b.fillCircleHexF(st.x, st.y, r, col, 1, opNormal)
			b.strokeHexF(st.x-1.5, st.y, st.x+1.5, st.y, 0.22, col, 0.35, opAdd)
			b.strokeHexF(st.x, st.y-1.5, st.x, st.y+1.5, 0.22, col, 0.35, opAdd)
		case brightness > 0.4:
			b.fillCircleHexF(st.x, st.y, r, col, 1, opNormal)
		default:
			b.fillCircleHexF(st.x, st.y, 0.26, col, brightness, opNormal)
		}
	}
	b.endLayer()

	// --- moon: phase from the date; progress walks it across the sky
	prog := clampf(state.Progress, 0, 1)
	phase := moonPhase(state.Now)
	mR := lay.MinDim * 0.115
	if mR < 1.6 {
		mR = 1.6
	}
	mX := W * (0.10 + 0.80*prog)
	mX = clampf(mX, mR+0.5, W-mR-0.5)
	mY := lay.Horizon*0.88 - math.Sin(math.Pi*prog)*lay.Horizon*0.62
	mY = clampf(mY, mR+0.5, lay.Horizon-mR*0.4)
	b.anchor("moon", mX-mR, mY-mR, mX+mR, mY+mR)
	skyAt := blendHex(rmp.SkyTop, rmp.SkyHorizon, clampf(mY/H, 0, 1))
	cols := sphereCols(bright, base, skyAt)
	mCols := cols // the lake reflection mirrors the same tones
	b.beginLayer(layerCelestial, int(mX-mR-2), int(mY-mR-2), int(mX+mR+3), int(mY+mR+3))
	paintMoonSphere(b, mX, mY, mR, phase, ts, cols, haloCol(bright, state.Palette.Accent))
	b.endLayer()

	// --- shooting stars: slotted, fading tails
	b.beginLayer(layerStars, 0, 0, sw(b), sh(b)/2)
	slotSec := int64(5)
	slot := int64(wrapT(t, 24*time.Hour)/time.Second) / slotSec
	slotR := newRNG(state.Seed, uint64(slot)*977+13)
	if slotR.float64() < 0.55 {
		frac := (float64(wrapT(t, 24*time.Hour)/time.Millisecond)/1000.0 - float64(slot*slotSec)) / 0.8
		if frac >= 0 && frac < 1 {
			dir := 1.0
			if slotR.float64() < 0.5 {
				dir = -1.0
			}
			x0 := slotR.rangeF(0.15, 0.65) * W
			y0 := slotR.rangeF(0.03, 0.24) * H
			headX := x0 + dir*frac*W*0.22
			headY := y0 + frac*H*0.16
			for k := 0; k < 8; k++ {
				fade := 1 - float64(k)/8
				kx := headX - dir*frac*W*0.22*(float64(k)/7)
				ky := headY - frac*H*0.16*(float64(k)/7)
				col := blendHex(bright, base, 1-fade*0.85)
				if k == 0 {
					b.fillCircleHexF(kx, ky, 0.5, col, 1, opNormal)
					b.glowHexAt(kx, ky, bright, 0.5)
					continue
				}
				b.fillCircleHexF(kx, ky, 0.42*fade+0.1, col, fade, opNormal)
			}
		}
	}
	b.endLayer()

	// --- still lake: the water below the horizon mirrors the moon, the
	// aurora and the brightest stars — flipped, squashed (grazing-angle
	// reflection), dimmed, and displaced by horizontal ripple. A slow
	// shimmer and a glint path run under the moon; the treeline shows
	// as a flipped, dim reflection along the shoreline.
	layH := lay.Horizon
	squash := 0.45
	sil := cachedPerlin(state.Seed, 101)
	rippleAt := func(y float64) float64 {
		depth := (y - layH) / H
		if depth < 0 {
			depth = 0
		}
		return (sil.At(y*0.30, ts*0.20)*0.9 + math.Sin(y*0.55-ts*0.6)*0.35) * (0.25 + depth*1.7)
	}
	shimmerAt := func(y float64) float64 {
		return 0.70 + 0.30*math.Sin(y*1.6-ts*0.85+sil.At(y*0.5, 11.3)*1.4)
	}
	// treeline depth layers: the far haze ridge, a mid line, and the
	// near pines (farther = lighter and hazier). The near layer also
	// drives the lake's treeline reflection.
	pine := blendHex(base, surfaceStrong, 0.08)
	layers := nightTreelineLayers(H, layH, surfaceStrong, base, rmp)
	lakeTop := blendHex(rmp.SkyHorizon, base, 0.38)
	lakeBot := blendHex(base, state.Palette.Blue, 0.12)
	b.beginLayer(layerSea, 0, int(layH), sw(b), sh(b))
	b.fillGradientHex(0, layH, W, H, lakeTop, lakeBot, 1, opNormal)
	// mirrored aurora curtains (dimmer, compressed, rippling)
	paintAuroraCurtains(b, state, ts, squash, 0.55, rippleAt)
	// mirrored bright stars
	for _, st := range nightStars(state.Seed, sw(b), sh(b)) {
		if st.mag < 0.62 {
			continue
		}
		tw := 0.55 + 0.45*math.Sin(ts*st.speed+st.phase)
		yDest := layH + (layH-st.y)*squash
		if yDest <= layH+0.3 || yDest >= H {
			continue
		}
		fx := 0.16 + tw*st.mag*0.28
		col := blendHex(bright, base, 1-fx)
		b.fillCircleHexF(st.x+rippleAt(yDest), yDest, 0.22+st.mag*0.14, col, fx, opNormal)
	}
	// mirrored moon: squashed, dimmed, rippling per row (the same tones
	// as the direct disc, dimmed by the water)
	yTop := layH + (layH-(mY+mR))*squash
	yBot := layH + (layH-(mY-mR))*squash
	ssF := float64(fSSV())
	fb := b.fb
	for py := int(yTop * ssF); py < int(yBot*ssF); py++ {
		pyf := float64(py)/ssF + 0.5/ssF
		if pyf <= layH || py >= fb.ph {
			continue
		}
		dx := rippleAt(pyf)
		srcY := layH - (pyf-layH)/squash
		shim := shimmerAt(pyf)
		for px := int((mX - mR - 1 + dx) * ssF); px < int((mX+mR+1+dx)*ssF); px++ {
			if px < 0 || px >= fb.pw {
				continue
			}
			pxf := float64(px)/ssF + 0.5/ssF
			col, cov := moonShade(mX, mY, mR, phase, mCols, pxf-dx, srcY)
			a := cov * 0.42 * shim
			if a <= 0.02 {
				continue
			}
			fb.blendPx(px, py, fpix{col[0], col[1], col[2], float32(a)}, opNormal)
		}
	}
	// glint path: slow dashes drifting under the moon + a soft pool glow
	b.fillEllipseHexF(mX, yBot+(yBot-layH)*0.5+1.0, mR*1.15, (yBot-layH)*0.45+1.2, blendHex(bright, base, 0.72), 0.16, opAdd)
	glint := cachedPerlin(state.Seed, 921)
	for y := layH + 1; y < H; y += 0.5 {
		depth := (y - layH) / H
		n := glint.At(ts*0.55, y*0.42)
		if n < 0.42-0.25*depth {
			continue
		}
		off := sil.At(y*0.8, ts*0.4) * (1.2 + depth*2.4)
		a := 0.30 + 0.35*math.Sin(ts*1.9+y*1.35)
		if a <= 0.10 {
			continue
		}
		b.fillEllipseHexF(mX+off+rippleAt(y), y, 0.5+0.5*a, 0.16, blendHex(bright, base, 0.45+depth*0.3), a*0.75, opAdd)
	}
	// treeline reflection: flipped, dim, jagged — anchored to the shoreline
	nearYBot := layers[2].yBot
	for i := 0; float64(i)*0.25 < W; i++ {
		x := float64(i) * 0.25
		relH := nearYBot - ridgeHeightAt(sil, x, layers[2].baseY, layers[2].params)
		top := layH + 1 + relH*0.5
		b.fillRectHexF(x, layH+0.8, x+0.25, top, pine, 0.35, opNormal)
	}
	b.endLayer()

	// --- horizon: the treeline in three depth layers — a far haze ridge,
	// a hazier mid line, and the near pines — continuous serrated
	// ridgelines, each filled with a vertical absolute-y gradient (no
	// striping). The bases reach a hair below the horizon row; the lake
	// begins again beneath them.
	b.anchor("horizon", 0, lay.Horizon-0.01, W, lay.Horizon+0.01)
	b.beginLayer(layerLand, 0, int(lay.Horizon-H*0.22), sw(b), sh(b))
	for i, layer := range layers {
		paintRidgeLayer(b, sil, W, layer)
		if i == 1 {
			paintMistBase(b, cachedPerlin(state.Seed, 311), ts, W, H, layH+H*0.012, blendHex(rmp.Haze, base, 0.3), 3.3)
		}
	}
	b.endLayer()
}

// nightTreelineLayers defines the three shoreline layers of the night
// treeline: the far haze ridge, a mid depth line, and the near pines —
// farther layers lighter and hazier. Single source for the painter and
// the banding test.
func nightTreelineLayers(H, layH float64, surfStrong, base string, rmp Ramps) []treelineLayer {
	farCrest := rmp.ApplyHaze(surfStrong, 0.72)
	midCrest := blendHex(blendHex(base, surfStrong, 0.22), farCrest, 0.35)
	pine := blendHex(base, surfStrong, 0.08)
	return []treelineLayer{
		{
			baseY:  layH + H*0.008,
			yBot:   layH + H*0.012,
			params: ridgeParams{swellAmp: H * 0.050, swellFreq: 0.016, spireAmp: H * 0.075, spireFreq: 0.150, phase: 7.3},
			crest:  farCrest,
			base:   blendHex(farCrest, base, 0.42),
		},
		{
			baseY:  layH + H*0.010,
			yBot:   layH + H*0.020,
			params: ridgeParams{swellAmp: H * 0.032, swellFreq: 0.023, spireAmp: H * 0.105, spireFreq: 0.260, phase: 21.7},
			crest:  midCrest,
			base:   blendHex(midCrest, base, 0.42),
		},
		{
			baseY:  layH + H*0.016,
			yBot:   layH + H*0.028,
			params: ridgeParams{swellAmp: H * 0.020, swellFreq: 0.031, spireAmp: H * 0.125, spireFreq: 0.340, phase: 40.2},
			crest:  pine,
			base:   blendHex(pine, base, 0.45),
		},
	}
}

// glowHexAt adds a small additive glow dab at float position.
func (b *buffer) glowHexAt(cx, cy float64, hex string, a float64) {
	r, g, bl, ok := hexF(hex)
	if !ok {
		return
	}
	b.fb.fillCircleF(cx, cy, 0.62, r, g, bl, a, opAdd)
}
