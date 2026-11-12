package ambient

import (
	"math"
	"time"
)

// oceanScene paints a living seascape: a sun or moon disc with a glow and
// a shimmering reflection column that widens toward the viewer, at least
// five depth-graded water bands with sine crests and foam highlights,
// soft noise clouds at two drift speeds, distant birds, and a lighthouse
// with a blinking light. The sky tracks the real time of day.
type oceanScene struct {
	mode string
}

func (s *oceanScene) Name() string { return "ocean" }

func (s *oceanScene) Frame(t time.Duration, w, h int, state SceneState) []Line {
	return renderFrame(s, s.mode, t, w, h, state, false)
}

func (s *oceanScene) Reward(t time.Duration, w, h int, state SceneState) []Line {
	return renderFrame(s, s.mode, t, w, h, state, true)
}

func (s *oceanScene) FramePixels(t time.Duration, w, h int, state SceneState) Raster {
	return renderRaster(s, t, w, h, state)
}

func (s *oceanScene) paint(b *buffer, p *paletteModel, t time.Duration, w, h int, state SceneState) {
	W := float64(sw(b))
	H := float64(sh(b))
	W_ := int(W)
	lay := layoutForState(state, W, H)
	base := state.Palette.Base
	cyan := state.Palette.Cyan
	blue := state.Palette.Blue
	yellow := state.Palette.Yellow
	bright := state.Palette.BrightText
	text := state.Palette.Text
	rmp := BuildRamps(state.Palette)
	ts := tSecF(t)

	// --- vertical layout: horizon at ~58%, nudged up by the tide
	seaYF := lay.Horizon
	seaYF -= clampf(state.Progress, 0, 1) * H * 0.05
	seaY := int(seaYF)
	b.anchor("horizon", 0, lay.Horizon-0.01, W, lay.Horizon+0.01)

	// --- time of day drives the sky
	alt := sunAltitude(state.Now)
	dayness := clampf(alt, 0, 1)
	nightness := clampf(-alt, 0, 1)
	// dusk is strongest right at the horizon when the sun is low
	dusk := clampf(1-math.Abs(alt)/0.28, 0, 1)
	dayZenith := blendHex(blendHex(base, cyan, 0.55), base, 0.25)
	dayHorizon := blendHex(blendHex(base, yellow, 0.45), base, 0.35)
	topCol := blendHex(rmp.SkyTop, dayZenith, dayness)
	horizCol := blendHex(rmp.SkyHorizon, dayHorizon, dayness)
	horizCol = blendHex(horizCol, state.Palette.Accent, dusk*0.30)

	// sky: deep zenith → horizon haze
	b.fillGradientHex(0, 0, W, seaYF, topCol, horizCol, 1, opNormal)

	// --- stars fading in with the night
	if nightness > 0.35 {
		seed := sceneStars(state.Seed, W_, sh(b))
		for _, st := range seed {
			if st.layer != 0 || st.mag < 0.62 {
				continue
			}
			tw := 0.6 + 0.4*math.Sin(ts*st.speed+st.phase)
			col := blendHex(bright, base, 1-tw*0.8)
			b.fillCircleHexF(float64(st.x)+0.5, float64(st.y)+0.5, 0.34, col, tw, opNormal)
		}
	}

	// --- celestial body: sun by day, moon by night; an arc across the sky
	celestialX := 0.0
	celestialY := 0.0
	celestialR := lay.MinDim * 0.145
	isSun := dayness > 0
	var mCols moonCols
	if isSun {
		rise, set := sunTimes(state.Now)
		hour := float64(state.Now.Hour()) + float64(state.Now.Minute())/60
		frac := clampf((hour-rise)/math.Max(set-rise, 0.1), 0, 1)
		celestialX = clampf(W*(0.10+0.80*frac), celestialR+0.5, W-celestialR-0.5)
		celestialY = clampf(seaYF*(0.78-0.66*math.Sin(math.Pi*frac)), celestialR+0.5, seaYF)
		celestialR = lay.MinDim * 0.155
		// warm body with a hot core
		b.fillCircleHexF(celestialX, celestialY, celestialR, blendHex(yellow, base, 0.45), 1, opNormal)
		b.fillCircleHexF(celestialX-celestialR*0.18, celestialY-celestialR*0.18, celestialR*0.62, yellow, 1, opNormal)
		b.fillCircleHexF(celestialX-celestialR*0.3, celestialY-celestialR*0.3, celestialR*0.3, bright, 1, opNormal)
		// halo
		b.fillCircleHexF(celestialX, celestialY, celestialR*1.9, blendHex(yellow, base, 0.55), 0.55, opAdd)
	} else {
		hour := float64(state.Now.Hour()) + float64(state.Now.Minute())/60
		mfrac := 0.0
		if hour >= 18 {
			mfrac = (hour - 18) / 12
		} else {
			mfrac = (hour + 6) / 12
		}
		mfrac = clampf(mfrac, 0, 1)
		celestialX = clampf(W*(0.10+0.80*mfrac), celestialR+0.5, W-celestialR-0.5)
		celestialY = clampf(seaYF*(0.74-0.6*math.Sin(math.Pi*mfrac)), celestialR+0.5, seaYF)
		celestialR = lay.MinDim * 0.135
		phase := moonPhase(state.Now)
		skyAt := blendHex(rmp.SkyTop, rmp.SkyHorizon, clampf(celestialY/seaYF, 0, 1))
		mCols = sphereCols(bright, base, skyAt)
		b.beginLayer(layerCelestial, int(celestialX-celestialR-2), int(celestialY-celestialR-2), int(celestialX+celestialR+3), int(celestialY+celestialR+3))
		paintMoonSphere(b, celestialX, celestialY, celestialR, phase, ts, mCols, haloCol(bright, state.Palette.Accent))
		b.endLayer()
	}

	// --- soft noise clouds at two parallax speeds
	if isSun {
		b.anchor("sun", celestialX-celestialR, celestialY-celestialR, celestialX+celestialR, celestialY+celestialR)
	} else {
		b.anchor("moon", celestialX-celestialR, celestialY-celestialR, celestialX+celestialR, celestialY+celestialR)
	}
	b.beginLayer(layerFog, 0, 0, sw(b), seaY)
	cloudTint := blendHex(text, base, 0.25)
	if nightness > 0.45 {
		cloudTint = blendHex(text, base, 0.62)
	}
	cloudSeeds := []struct {
		x, y, w float64
		speed   float64
		alpha   float64
	}{
		{0.16, 0.20, 0.16, 0.010, 0.55},
		{0.52, 0.09, 0.22, 0.016, 0.7},
		{0.78, 0.30, 0.13, 0.008, 0.45},
	}
	for i, c := range cloudSeeds {
		cx := math.Mod(c.x*W+ts*c.speed*W*3.5+float64(i)*7, W+60) - 30
		cw := c.w * W
		cy := c.y * seaYF
		puffs := 5
		for j := 0; j < puffs; j++ {
			off := (float64(j) - float64(puffs-1)/2) * cw * 0.32
			pr := cw * (0.24 + 0.13*math.Sin(float64(j)*2.1+float64(i)*1.3))
			b.fillEllipseHexF(cx+off, cy+math.Sin(float64(j)*1.7+float64(i))*1.2, pr, pr*0.52, cloudTint, c.alpha, opNormal)
		}
	}
	b.endLayer()

	// --- water: five depth-graded bands, sine crests, foam highlights
	b.beginLayer(layerSea, 0, seaY, sw(b), sh(b))
	waterRows := sh(b) - seaY
	bands := []struct {
		depth  float64
		hex    string
		height float64
	}{
		{0.00, blendHex(horizCol, cyan, 0.30), 0.13},
		{0.18, blendHex(cyan, blue, 0.35), 0.17},
		{0.38, blendHex(blue, cyan, 0.30), 0.20},
		{0.60, blendHex(blue, base, 0.42), 0.18},
		{0.78, blendHex(blue, base, 0.62), 0.16},
		{0.90, blendHex(blue, base, 0.80), 0.16},
	}
	bandY := float64(seaY)
	for _, band := range bands {
		bh := float64(waterRows) * band.height
		// vertical gradient inside the band (top lighter → bottom deeper)
		topHex := band.hex
		botHex := blendHex(band.hex, base, 0.35)
		b.fillGradientHex(0, bandY, W, bandY+bh, topHex, botHex, 1, opNormal)
		bandY += bh
	}
	// sine wave crests with foam at the peaks
	crestLayers := []struct {
		yOff, amp, freq, speed float64
		crestCol               string
		foam                   bool
	}{
		{0.10, 0.9, 0.145, 0.5, blendHex(cyan, base, 0.25), false},
		{0.28, 1.3, 0.11, 0.38, blendHex(blue, base, 0.15), true},
		{0.46, 1.7, 0.085, 0.3, blendHex(blue, base, 0.05), true},
		{0.64, 2.2, 0.065, 0.22, blendHex(blue, base, 0.28), false},
		{0.84, 2.8, 0.05, 0.17, blendHex(base, blue, 0.28), false},
	}
	sil := cachedPerlin(state.Seed, 311)
	// the celestial disc's mirror image sinks into the water (squashed at
	// grazing angle, clipped by the horizon band) — drawn before the crest
	// strokes so the waves break it up.
	{
		rwink := cachedPerlin(state.Seed, 977)
		squash := 0.5
		refCY := float64(seaY) + (float64(seaY)-celestialY)*squash
		rr := celestialR * squash
		phaseV := moonPhase(state.Now)
		fb := b.fb
		ssF := float64(fSSV())
		aspF := fb.aspectCorr()
		yFrom := float64(seaY) + 1
		yTo := math.Min(float64(sh(b)), refCY+rr+1)
		for py := int(yFrom * ssF); py < int(yTo*ssF); py++ {
			pyf := float64(py)/ssF + 0.5/ssF
			depth := (pyf - float64(seaY)) / float64(waterRows)
			if depth <= 0 {
				continue
			}
			srcY := float64(seaY) - (pyf-float64(seaY))/squash
			dx := rwink.At(pyf*0.35, ts*0.22) * (0.3 + depth*1.6)
			for px := int((celestialX - rr - 1 + dx) * ssF); px < int((celestialX+rr+1+dx)*ssF); px++ {
				if px < 0 || px >= fb.pw {
					continue
				}
				pxf := float64(px)/ssF + 0.5/ssF
				srcX := pxf - dx
				var cov float64
				var cr, cg, cb float32
				if isSun {
					// hot core → warm edge falloff
					d := math.Hypot(srcX-celestialX, (srcY-celestialY)*aspF) / celestialR
					if d >= 1 {
						continue
					}
					cov = clampf(1-d, 0, 1)
					cov *= cov
					r0, g0, b0, _ := hexF(blendHex(bright, yellow, clampf(d*1.4, 0, 1)))
					cr, cg, cb = r0, g0, b0
				} else {
					col, a := moonShade(celestialX, celestialY, celestialR, phaseV, mCols, srcX, srcY, aspF)
					if a <= 0.02 {
						continue
					}
					cov = a
					cr, cg, cb = col[0], col[1], col[2]
				}
				alpha := cov * 0.5 * (1 - depth*0.45)
				if alpha <= 0.02 {
					continue
				}
				fb.blendPx(px, py, fpix{cr, cg, cb, float32(alpha)}, opNormal)
			}
		}
	}
	for li, cl := range crestLayers {
		crestY := float64(seaY) + cl.yOff*float64(waterRows)
		for x := 0.0; x < W; x += 0.6 {
			wave := math.Sin(x*cl.freq+ts*cl.speed+float64(li)*9.1)*cl.amp + sil.At(x*0.055, float64(li)*13.7)*0.9
			crest := crestY + wave
			b.strokeHexF(x, crest, x+0.6, crest, 0.42, cl.crestCol, 0.9, opNormal)
			// foam: at the sharpest peaks, a bright dash
			if cl.foam && wave > cl.amp*0.82 {
				b.fillEllipseHexF(x, crest, 0.65, 0.22, blendHex(bright, base, 0.25), 0.85, opNormal)
			}
		}
	}
	// the working reflection column under the celestial body
	if celestialX > 0 {
		glint := cachedPerlin(state.Seed, 919)
		for y := seaY + 1; y < sh(b); y++ {
			depth := float64(y-seaY) / float64(waterRows)
			halfW := 2.5 + 8.0*depth
			for k := 0; k < 2; k++ {
				n := glint.At(float64(k)*7.7+ts*1.4, float64(y)*0.55+depth*2.2)
				if n <= 0.18 {
					continue
				}
				off := (n - 0.5) * 2 * halfW
				alpha := 0.4 + 0.5*math.Sin(ts*2.1+float64(y)*1.7+float64(k)*2.4)
				if alpha <= 0.12 {
					continue
				}
				col := blendHex(yellow, base, 0.3+depth*0.35)
				if !isSun {
					col = blendHex(bright, base, 0.3+depth*0.35)
				}
				b.fillEllipseHexF(celestialX+off, float64(y)+0.5, 0.9+0.9*alpha, 0.3, col, alpha, opAdd)
			}
		}
	}
	b.endLayer()

	// --- distant birds on the wing
	birds := []struct {
		x, y float64
	}{
		{0.22, 0.30}, {0.31, 0.36}, {0.68, 0.22},
	}
	for i, bd := range birds {
		bx := math.Mod(bd.x*W+ts*1.1+float64(i)*13.3, W)
		by := bd.y * seaYF
		flap := math.Sin(ts*2.4+float64(i)*2.1) > 0
		col := blendHex(text, base, 0.25)
		if flap {
			b.strokeHexF(bx-0.7, by-0.25, bx, by, 0.22, col, 0.9, opNormal)
			b.strokeHexF(bx, by, bx+0.7, by-0.25, 0.22, col, 0.9, opNormal)
		} else {
			b.strokeHexF(bx-0.7, by+0.2, bx, by, 0.22, col, 0.9, opNormal)
			b.strokeHexF(bx, by, bx+0.7, by+0.2, 0.22, col, 0.9, opNormal)
		}
	}

	// --- lighthouse on a rock at the right horizon, with a blinking light
	lx := W * 0.875
	ly := float64(seaY)
	// rock silhouette
	lh := lay.MinDim
	b.fillEllipseHexF(lx, ly+0.5, lh*0.075, lh*0.028, blendHex(base, state.Palette.SurfaceStrong, 0.25), 1, opNormal)
	// tower
	b.fillRectHexF(lx-lh*0.013, ly-lh*0.105, lx+lh*0.013, ly+0.4, blendHex(base, state.Palette.SurfaceRaised, 0.35), 1, opNormal)
	b.fillRectHexF(lx-lh*0.02, ly-lh*0.125, lx+lh*0.02, ly-lh*0.10, blendHex(base, state.Palette.SurfaceRaised, 0.2), 1, opNormal)
	// lamp: blinks on a 4.5s cycle
	blink := math.Mod(ts, 4.5)
	if blink < 1.1 {
		lump := 1 - blink/1.1
		b.fillCircleHexF(lx, ly-lh*0.115, lh*0.008, yellow, 1, opNormal)
		b.fillCircleHexF(lx, ly-lh*0.115, lh*0.026+lh*0.024*(1-lump), blendHex(yellow, base, 0.35), 0.55*lump, opAdd)
		b.strokeHexF(lx-lh*0.075, ly-lh*0.108, lx-lh*0.017, ly-lh*0.112, 0.35, yellow, 0.35*lump, opAdd)
		b.strokeHexF(lx+lh*0.017, ly-lh*0.112, lx+lh*0.075, ly-lh*0.108, 0.35, yellow, 0.35*lump, opAdd)
	}
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func tSecF(t time.Duration) float64 {
	if t < 0 {
		return 0
	}
	sec := float64(t/time.Second) + float64(t%time.Second)/float64(time.Second)
	return sec
}
