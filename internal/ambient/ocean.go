package ambient

import (
	"math"
	"time"
)

// oceanScene paints layered, seeded seascape: parallax wave bands, drifting
// clouds, the real sun (alt by the hour) or moon, gulls on the wing,
// occasional fish silhouettes, and a specular path that walks the water.
// Progress advances the sun across the sky and the tide height.
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

func (s *oceanScene) paint(b *buffer, p *paletteModel, t time.Duration, w, h int, state SceneState) {
	seaY := sh(b) * 56 / 100
	// Tide: the sea creeps upward as the phase matures.
	seaY -= int(clampf(state.Progress, 0, 1) * float64(sh(b)) * 0.06)

	// Sky stays the base color; the horizon line and the celestial body
	// carry the time of day (no flat fills — they render as harsh blocks).
	alt := sunAltitude(state.Now)
	dayness := clampf(alt, 0, 1)
	nightness := clampf(-alt, 0, 1)
	if dayness < 0.25 && nightness < 0.25 {
		// Dusk and dawn: a single warm band hugging the horizon
		for y := seaY - 2; y < seaY; y++ {
			if y >= 0 {
				b.glowBlock(0, y, sw(b), y+1, mixIndex(p, cOrange, p.base, 0.7))
			}
		}
	}
	// Horizon line
	b.glowBlock(0, seaY, sw(b), seaY+1, mixIndex(p, cSurfaceStrong, p.base, 0.5))

	// A few stars at night
	if nightness >= 0.4 {
		for _, st := range sceneStars(state.Seed, sw(b), sh(b)) {
			if st.layer == 0 && st.mag > 0.55 {
				b.mark(st.x, st.y, '·', mixIndex(p, cBrightText, p.base, 0.45))
			}
		}
	}

	// Sun or moon: an arc from rise to set across the sky.
	celestialX, celestialY, celestialR, celestialIsSun := 0, 0, 0, true
	if dayness > 0 {
		rise, set := sunTimes(state.Now)
		hour := float64(state.Now.Hour()) + float64(state.Now.Minute())/60
		frac := clampf((hour-rise)/maxF(set-rise, 0.1), 0, 1)
		celestialX = int(w2x(b, 0.08+0.84*frac))
		celestialY = int(float64(seaY) * (0.75 - 0.62*math.Sin(math.Pi*frac)))
		celestialR = maxInt(minInt(sw(b), sh(b))/13, 2)
		paintDisc(b, celestialX, celestialY, celestialR, mixIndex(p, cYellow, p.base, 0.15), true)
		// subtle warm halo
		paintDisc(b, celestialX, celestialY, celestialR+1, mixIndex(p, cWarning, p.base, 0.75), false)
	} else {
		// moon arc: roughly the opposite of the sun's timing
		hour := float64(state.Now.Hour()) + float64(state.Now.Minute())/60
		mfrac := 0.0
		if hour >= 18 {
			mfrac = (hour - 18) / 12
		} else {
			mfrac = (hour + 6) / 12
		}
		mfrac = clampf(mfrac, 0, 1)
		celestialX = int(w2x(b, 0.08+0.84*mfrac))
		celestialY = int(float64(seaY) * (0.72 - 0.58*math.Sin(math.Pi*mfrac)))
		celestialR = maxInt(minInt(sw(b), sh(b))/14, 2)
		phase := moonPhase(state.Now)
		paintMoon(b, celestialX, celestialY, celestialR, phase, p)
		celestialIsSun = false
	}
	_ = celestialIsSun

	// Clouds: three seeded puffs drifting slowly with parallax.
	tMin := float64(wrapT(t, 20*time.Minute)) / float64(time.Minute)
	for i := 0; i < 3; i++ {
		cr := newRNG(state.Seed, uint64(i)*977+11)
		cloudW := int(cr.rangeF(6, 12))
		cw := sw(b) + cloudW + 8
		cx := int((cr.rangeF(0, float64(cw)))+tMin*(1.3+float64(i)*0.5))%cw - cloudW
		cy := int(cr.rangeF(0.08, 0.38) * float64(seaY))
		cake := mixIndex(p, cText, p.base, 0.35+0.25*float64(i))
		if nightness >= 0.5 {
			cake = mixIndex(p, cText, p.base, 0.7)
		}
		for dy := 0; dy <= 2; dy++ {
			for dx := 0; dx < cloudW; dx++ {
				dxC := float64(dx) - float64(cloudW)/2
				dyC := float64(dy) - 1.0
				if (dxC*dxC)/(float64(cloudW)*float64(cloudW)/4)+(dyC*dyC)/1.6 > 1 {
					continue
				}
				b.glow(cx+dx, cy+dy, cake)
			}
		}
	}

	// Sea: layered sine wave crests, drawn as single brighter rows on the
	// bare base color — clean crest lines instead of flat fills.
	waveCols := []int{cSurface, cCyan, cBlue, cSurfaceStrong}
	for layer := 0; layer < 4; layer++ {
		freq := 0.10 + float64(layer)*0.055
		amp := 0.9 + float64(3-layer)*0.35
		speed := 0.45 + float64(layer)*0.2
		lineOff := float64(seaY) + 1 + float64(layer)*maxF(float64(sh(b)-seaY-1)/4.2, 1)
		xi := newPerlin(state.Seed, uint64(layer)*331+5)
		for x := 0; x < sw(b); x++ {
			wy := lineOff + math.Sin(float64(x)*freq+tSecF(t)*speed+float64(layer)*9.1)*amp + xi.At(float64(x)*0.07, float64(layer)*13.7)*0.7
			ci := mixIndex(p, waveCols[layer], p.base, 0.35+float64(3-layer)*0.12)
			b.glow(x, int(wy), ci)
		}
	}

	// Specular path under the celestial body: flickering glints on water.
	if celestialX > 0 {
		glint := newPerlin(state.Seed, 919)
		for x := maxInt(celestialX-4, 0); x < minInt(celestialX+5, sw(b)); x++ {
			for y := seaY; y < sh(b); y++ {
				n := glint.At(float64(x)*0.5, float64(y)*0.33+tSecF(t)*0.5)
				d := absF(float64(x)-float64(celestialX)) / 4.0
				if n+(1-d) > 0.9-(0.45*float64(y-seaY)/float64(maxInt(sh(b)-seaY, 1))) {
					b.mark(x, y, '·', mixIndex(p, cYellow, p.base, 0.25+d*0.3))
				}
			}
		}
	}

	// Gulls: seeded "v" glyphs bobbing above the water.
	gullR := newRNG(state.Seed, 553)
	for i := 0; i < 3; i++ {
		gx := int(gullR.rangeF(0.15, 0.85) * float64(sw(b)))
		gy := int(gullR.rangeF(0.15, 0.4) * float64(seaY))
		flap := math.Sin(tSecF(t)*2.3+float64(i)*2.1) > 0
		ch := 'v'
		if flap {
			ch = '^'
		}
		b.mark(gx-1, gy, ch, cText)
		b.mark(gx+1, gy, ch, cText)
		_ = gullR
	}

	// Fish event: occasional silhouette leaps in seeded slots.
	slot := int64(wrapT(t, 24*time.Hour)/time.Millisecond) / 9000
	fzR := newRNG(state.Seed, uint64(slot)*733+29)
	if fzR.float64() < 0.6 {
		fz := (float64(wrapT(t, 24*time.Hour)/time.Millisecond) / 1000.0) - float64(slot)*9
		frac := fz / 1.1 // 1.1s leap
		if frac >= 0 && frac < 1 {
			fx := fzR.rangeF(0.2, 0.8) * float64(sw(b))
			fy := float64(seaY) - math.Sin(math.Pi*frac)*float64(sh(b))*0.16
			ci := mixIndex(p, cBlue, p.base, 0.7)
			b.mark(int(fx), int(fy), '⌒', ci)
			b.mark(int(fx)-1, int(fy)+1, '⟍', ci)
			b.mark(int(fx)+1, int(fy)+1, '⟋', ci)
		}
		// bubbles below seeded position
		for k := 0; k < 3; k++ {
			r2 := newRNG(state.Seed, uint64(slot)*911+uint64(k))
			bx := int(r2.rangeF(0.15, 0.85) * float64(sw(b)))
			by := sh(b) - 1 - int((tSecF(t)*1.4+r2.float64()*3))%maxInt(sh(b)-seaY+1, 1)
			b.mark(bx, by, '·', mixIndex(p, cCyan, p.base, 0.4))
		}
	}
}

// paintDisc fills a disc with a color; ringOnly draws only the outline.
func paintDisc(b *buffer, cx, cy, r int, ci int, filled bool) {
	for dy := -r - 1; dy <= r+1; dy++ {
		for dx := -r - 1; dx <= r+1; dx++ {
			d2 := float64(dx*dx+dy*dy) / float64(r*r)
			if filled {
				if d2 <= 1 {
					b.glow(cx+dx, cy+dy, ci)
				}
			} else {
				if d2 > 0.72 && d2 <= 1.42 {
					b.glow(cx+dx, cy+dy, ci)
				}
			}
		}
	}
}

// paintMoon draws the moon in its true phase (bright limb, dark disc).
func paintMoon(b *buffer, cx, cy, r int, phase float64, p *paletteModel) {
	term := math.Cos(2*math.Pi*phase) * float64(r)
	waxing := phase < 0.5
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			d2 := float64(dx*dx+dy*dy) / float64(r*r)
			if d2 > 1 {
				continue
			}
			lit := false
			if waxing {
				lit = float64(dx) > -term
			} else {
				lit = float64(dx) < term
			}
			shade := math.Sqrt(d2)
			var ci int
			if lit {
				ci = mixIndex(p, cBrightText, p.base, 0.15+0.3*shade)
			} else {
				ci = mixIndex(p, cSurface, p.base, 0.35+0.4*shade)
			}
			b.glow(cx+dx, cy+dy, ci)
		}
	}
}

func w2x(b *buffer, frac float64) float64 { return frac * float64(sw(b)-1) }
func tSecF(t time.Duration) float64       { return float64(wrapT(t, 10*time.Minute)) / float64(time.Second) }
func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
