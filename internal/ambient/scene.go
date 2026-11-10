package ambient

import (
	"math"
	"strings"
	"time"
)

// paintable is implemented by every scene: it paints into the sub-raster.
// Frame composes paint + render; Reward adds the celebration overlay.
type paintable interface {
	paint(b *buffer, p *paletteModel, t time.Duration, w, h int, state SceneState)
	Name() string
}

// newFrameBuffer paints one frame and returns the buffer for inspection
// (the golden/invariant tests read the raster, not just the lines).
func newFrameBuffer(s paintable, mode string, t time.Duration, w, h int, state SceneState) *buffer {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	p := paletteFor(state.Palette)
	b := newBuffer(mode, w, h, p)
	b.dither = state.Dither
	s.paint(b, p, t, w, h, state)
	b.palette = p
	return b
}

// renderFrame builds one line-set for the frame (reward adds the sparkle).
func renderFrame(s paintable, mode string, t time.Duration, w, h int, state SceneState, reward bool) []Line {
	p := paletteFor(state.Palette)
	b := newFrameBuffer(s, mode, t, w, h, state)
	b.palette = p
	if reward {
		paintReward(b, t, state, s.Name())
	}
	return b.render(p, w, h)
}

// New returns the scene named name (unknown names fall back to night),
// rendered with the given mode (auto resolves to halfblock).
func New(name, mode string) Scene {
	mode = resolveMode(mode)
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "campfire":
		return &campfireScene{mode: mode}
	case "snow":
		return &snowScene{mode: mode}
	case "koi":
		return &koiScene{mode: mode}
	case "rain":
		return &rainScene{mode: mode}
	case "ocean":
		return &oceanScene{mode: mode}
	case "bonsai":
		return &bonsaiScene{mode: mode}
	case "night":
		return &nightScene{mode: mode}
	default:
		return &nightScene{mode: mode}
	}
}

// Registered lists every scene name in stable order.
func Registered() []string {
	return []string{"night", "bonsai", "ocean", "rain", "koi", "snow", "campfire"}
}

// paintReward draws the scene-specific 1.5s celebration: an eased,
// scene-aware burst (star shower, petal burst, sparkle sweep, breaking
// clouds, fish leap, snow glow, ember flare). Falls back to generic
// sparkles for anything unrecognized.
func paintReward(b *buffer, t time.Duration, state SceneState, sceneName string) {
	switch strings.ToLower(strings.TrimSpace(sceneName)) {
	case "night":
		rewardNight(b, t, state)
	case "bonsai":
		rewardBonsai(b, t, state)
	case "ocean":
		rewardOcean(b, t, state)
	case "rain":
		rewardRain(b, t, state)
	case "koi":
		rewardKoi(b, t, state)
	case "snow":
		rewardSnow(b, t, state)
	case "campfire":
		rewardCampfire(b, t, state)
	default:
		rewardGeneric(b, t, state.Seed)
	}
}

// rewardProgress maps reward t ∈ [0, 1.5s] to 0..1.
func rewardProgress(t time.Duration) float64 {
	return clampf(float64(wrapT(t, 1500*time.Millisecond))/float64(1500*time.Millisecond), 0, 1)
}

// rewardNight: a bright shooting star streaks across, then a star-burst pops.
func rewardNight(b *buffer, t time.Duration, state SceneState) {
	prog := rewardProgress(t)
	W := float64(b.sw)
	H := float64(b.sh)
	rng0 := newRNG(state.Seed, 911)
	// shooting star: sweeps across the sky in the first 0.8 of the envelope
	shoot := clampf(prog/0.62, 0, 1)
	if shoot > 0 && shoot < 1 {
		e := EaseInOutCubic(shoot)
		dir := 1.0
		if rng0.float64() < 0.5 {
			dir = -1.0
		}
		x0 := W * (0.18 + 0.25*rng0.float64())
		y0 := H * (0.10 + 0.22*rng0.float64())
		headX := x0 + dir*e*W*0.5
		headY := y0 + e*H*0.24
		base := state.Palette.BrightText
		trailLen := 6 + int(e*4)
		for k := 0; k < trailLen; k++ {
			fade := 1 - float64(k)/float64(trailLen)
			kx := headX - dir*e*W*0.045*float64(k)
			ky := headY - e*H*0.02*float64(k)
			col := blendHex(base, state.Palette.Base, 1-fade*0.85)
			if k == 0 {
				b.fillCircleHexF(kx, ky, 0.5, col, 1, opNormal)
				continue
			}
			b.fillCircleHexF(kx, ky, 0.34*fade+0.09, col, fade, opNormal)
		}
	}
	// star-burst at the classic spot once the streak passes
	burst := Envelope(prog, 0.38, 0.52, 0.72, 1.0)
	if burst <= 0 {
		return
	}
	bx := W * (0.62 + 0.2*rng0.float64())
	by := H * (0.16 + 0.12*rng0.float64())
	arm := burst * 4.2
	for k := 1; k <= 5; k++ {
		d := arm * float64(k) / 5
		a := burst * (1 - float64(k)/6) * 0.9
		if a <= 0.03 {
			continue
		}
		for _, dir := range [][2]float64{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			b.fillCircleF(bx+dir[0]*d, by+dir[1]*d, 0.34, cBrightText, a, opNormal)
		}
	}
	b.fillCircleF(bx, by, 0.5+0.35*burst, cBrightText, burst, opNormal)
}

// rewardBonsai: petals burst outward from the canopy and a bloom pulse
// brightens the tree briefly.
func rewardBonsai(b *buffer, t time.Duration, state SceneState) {
	prog := rewardProgress(t)
	W := float64(b.sw)
	H := float64(b.sh)
	rng0 := newRNG(state.Seed, 733)
	cx := W * 0.5
	cy := H * 0.34
	// bloom pulse: a soft warm wash over the canopy that swells then fades
	pulse := Envelope(prog, 0.1, 0.35, 0.55, 1.0)
	if pulse > 0 {
		b.fillCircleHexF(cx, cy, H*0.20, blendHex(state.Palette.Warning, state.Palette.Base, 0.4), 0.16*pulse, opAdd)
		b.fillCircleHexF(cx, cy, H*0.11, blendHex(state.Palette.Magenta, state.Palette.Warning, 0.5), 0.18*pulse, opAdd)
	}
	// petal burst: petals fly out along seeded directions with pop easing
	rng1 := newRNG(state.Seed, 739)
	for i := 0; i < 14; i++ {
		dir := rng1.rangeF(0, 2*math.Pi)
		speed := 5.5 + rng1.float64()*5.5
		delay := rng1.float64() * 0.22
		pLocal := clampf((prog-delay)/(1-delay), 0, 1)
		if pLocal <= 0 {
			continue
		}
		e := EaseOutCubic(pLocal)
		d := e * speed * 0.095 * H
		px := cx + math.Cos(dir)*d
		py := cy + math.Sin(dir)*d*0.6 + pLocal*pLocal*H*0.14 // arc downward as they fall
		fade := 1 - pLocal*0.55
		col := blendHex(state.Palette.Magenta, state.Palette.BrightText, 0.35+pLocal*0.2)
		b.fillCircleHexF(px, py, 0.4, col, fade, opNormal)
		_ = rng0
	}
}

// rewardOcean: a sparkle sweep runs across the water then a final shimmer pulse.
func rewardOcean(b *buffer, t time.Duration, state SceneState) {
	prog := rewardProgress(t)
	W := float64(b.sw)
	H := float64(b.sh)
	seaY := H * 0.56
	// sweep: from seeded side, a bright band crosses the water over 0.85 of the envelope
	sweep := clampf(prog/0.72, 0, 1)
	dir := 1.0
	sweepRng := newRNG(state.Seed, 911)
	if sweepRng.float64() < 0.5 {
		dir = -1.0
	}
	x := -6.0 + (W+12)*EaseInOutCubic(sweep)
	if dir < 0 {
		x = W - x
	}
	if sweep > 0 && sweep < 1 {
		for i := 0; i < 12; i++ {
			r := newRNG(state.Seed, uint64(i)*421+17)
			y := seaY + r.float64()*(H-seaY)
			halfW := 1.5 + r.float64()*3.0
			dx := x + (r.float64()-0.5)*halfW*2
			alpha := 0.4 + 0.5*math.Sin(prog*9+float64(i)*1.7)
			if alpha <= 0.1 {
				continue
			}
			col := blendHex(state.Palette.Warning, state.Palette.BrightText, 0.3)
			b.fillEllipseHexF(dx, y, 0.8+0.8*alpha, 0.3, col, alpha, opAdd)
		}
		// band glint highlight
		b.fillRectHexF(x-2.2, seaY, x+2.2, H, blendHex(state.Palette.Warning, state.Palette.BrightText, 0.5), 0.10*(1-sweep), opAdd)
	}
	// final whole-water shimmer pulse
	pulse := Envelope(prog, 0.55, 0.75, 0.85, 1.0)
	if pulse > 0 {
		b.fillRectHexF(0, seaY, W, H, blendHex(state.Palette.Cyan, state.Palette.BrightText, 0.3), 0.14*pulse, opAdd)
	}
}

// rewardRain: the clouds break — a clear circle grows over the sky and a
// rainbow tint appears along its base.
func rewardRain(b *buffer, t time.Duration, state SceneState) {
	prog := rewardProgress(t)
	W := float64(b.sw)
	H := float64(b.sh)
	rng0 := newRNG(state.Seed, 613)
	cx := W * (0.4 + 0.25*rng0.float64())
	cy := H * (0.3 + 0.15*rng0.float64())
	// clearing circle grows from 0 to cover the sky over the first 0.7
	clear := EaseOutCubic(clampf(prog/0.7, 0, 1))
	maxR := H * 0.34
	R := clear * maxR
	if R > 0.5 {
		// brighten the sky inside the circle: wash toward the horizon hue
		b.fillCircleHexF(cx, cy, R, blendHex(state.Palette.SurfaceStrong, state.Palette.Warning, 0.55), 0.22*clear, opAdd)
	}
	// rainbow tint: three arcs along the bottom of the circle, fading in later
	rb := Envelope(prog, 0.35, 0.6, 0.8, 1.0)
	if rb > 0 {
		bands := []string{state.Palette.Magenta, state.Palette.Warning, state.Palette.Cyan}
		for i, col := range bands {
			y := cy + R*0.62 + float64(i)*0.9
			b.strokeHexF(cx-R*0.75, y, cx+R*0.75, y, 0.32, col, 0.4*rb, opAdd)
		}
	}
	_ = rng0
}

// rewardKoi: two koi leap in big arcs and splash down.
func rewardKoi(b *buffer, t time.Duration, state SceneState) {
	prog := rewardProgress(t)
	W := float64(b.sw)
	H := float64(b.sh)
	seaY := H * 0.5
	rng0 := newRNG(state.Seed, 521)
	leap := clampf(prog/0.75, 0, 1)
	e := EaseOutCubic(leap)
	for i := 0; i < 2; i++ {
		baseX := W * (0.3 + 0.4*float64(i))
		peak := H * (0.3 + 0.12*rng0.float64())
		// parabolic arc: up then down
		fy := seaY - math.Sin(math.Pi*e)*peak
		fx := baseX + math.Sin(math.Pi*prog*1.6+float64(i)*2.0)*1.2
		// fish body: bright elongated blob
		col := blendHex(state.Palette.Orange, state.Palette.BrightText, 0.25)
		b.fillEllipseHexF(fx, fy, 1.3, 0.5, col, 1, opNormal)
		b.fillEllipseHexF(fx+0.62, fy, 0.48, 0.32, col, 1, opNormal)
		// droplets trailing off the arc
		for k := 0; k < 4; k++ {
			dx := (float64(k)-1.5)*0.5 + math.Sin(prog*20+float64(k)*2+float64(i))*0.2
			dy := float64(k)*0.22 + prog*prog*0.8
			b.fillCircleHexF(fx+dx, fy+dy, 0.18, blendHex(state.Palette.Cyan, state.Palette.BrightText, 0.3), (1-leap)*0.8, opNormal)
		}
	}
	// splash rings at both landing points when the leap descends
	splash := clampf((leap-0.62)/0.38, 0, 1)
	if splash > 0 {
		for i := 0; i < 2; i++ {
			baseX := W * (0.3 + 0.4*float64(i))
			b.strokeCircleHexF(baseX, seaY, 0.4+splash*2.4, 0.26, state.Palette.Cyan, 0.55*(1-splash), opNormal)
		}
	}
}

// rewardSnow: the ground glow flares and a shimmer of bright flakes bursts.
func rewardSnow(b *buffer, t time.Duration, state SceneState) {
	prog := rewardProgress(t)
	W := float64(b.sw)
	H := float64(b.sh)
	// ground glow flare: a big additive ellipse pulsing once
	flare := Envelope(prog, 0.08, 0.3, 0.55, 1.0)
	if flare > 0 {
		b.fillEllipseHexF(W/2, H*0.94, W*0.62, H*0.16, blendHex(state.Palette.BrightText, state.Palette.Cyan, 0.4), 0.38*flare, opAdd)
	}
	// bright flake burst from the center, spreading outward and falling
	rng0 := newRNG(state.Seed, 733)
	for i := 0; i < 16; i++ {
		dir := rng0.rangeF(-1.1, 1.1) // mostly upward cone
		pseed := rng0.float64()
		delay := pseed * 0.28
		pLocal := clampf((prog-delay)/(1-delay), 0, 1)
		if pLocal <= 0 {
			continue
		}
		e := EaseOutCubic(pLocal)
		d := e * (H * 0.5) * (0.55 + pseed*0.9)
		px := W/2 + math.Sin(dir)*d
		py := H*0.62 - d*0.8 + pLocal*pLocal*H*0.5 // rise then fall
		fade := 1 - pLocal*0.6
		b.fillCircleHexF(px, py, 0.3, blendHex(state.Palette.BrightText, state.Palette.Cyan, 0.2), fade, opNormal)
	}
}

// rewardCampfire: an ember flare — a bright fountain leaps up and the light blooms.
func rewardCampfire(b *buffer, t time.Duration, state SceneState) {
	prog := rewardProgress(t)
	W := float64(b.sw)
	H := float64(b.sh)
	fireX := W * 0.5
	fireY := H * 0.7
	// ember fountain: a tall spray of rising sparks
	flare := Envelope(prog, 0.06, 0.28, 0.5, 1.0)
	if flare > 0 {
		rng0 := newRNG(state.Seed, 881)
		for i := 0; i < 22; i++ {
			delay := rng0.float64() * 0.3
			pLocal := clampf((prog-delay)/(1-delay*0.6), 0, 1)
			if pLocal <= 0 {
				continue
			}
			e := EaseOutCubic(pLocal)
			spx := fireX + math.Sin(rng0.float64()*6.28+float64(i))*1.6*(1-pLocal*0.5)
			spy := fireY - e*H*(0.42+rng0.float64()*0.2)
			fade := (1 - pLocal) * (0.6 + 0.4*math.Sin(prog*18+float64(i)*1.9))
			if fade <= 0.05 {
				continue
			}
			col := state.Palette.Warning
			if pLocal > 0.55 {
				col = state.Palette.Accent
			}
			b.fillCircleHexF(spx, spy, 0.26+0.2*(1-pLocal), col, fade, opAdd)
		}
	}
	// the light blooms wide
	bloom := Envelope(prog, 0.05, 0.3, 0.6, 1.0)
	if bloom > 0 {
		b.fillEllipseHexF(fireX, fireY-H*0.08, W*0.36, H*0.3, state.Palette.Warning, 0.16*bloom, opAdd)
		b.fillEllipseHexF(fireX, fireY+2, W*0.42, H*0.13, state.Palette.Accent, 0.12*bloom, opAdd)
	}
}

// rewardGeneric: the original staggered cross-burst (fallback).
func rewardGeneric(b *buffer, t time.Duration, seed int64) {
	progress := rewardProgress(t)
	rng := newRNG(seed^0x5eed5eed, 99)
	p := b.palette
	for i := 0; i < 6; i++ {
		phase := progress - float64(i)*0.055
		if phase <= 0.04 || phase >= 0.92 {
			continue
		}
		cx := int(rng.rangeF(0.16, 0.84) * float64(b.sw-1))
		cy := int(rng.rangeF(0.12, 0.62) * float64(b.sh-1))
		arm := 2 + int(phase*5)
		if phase > 0.66 {
			arm--
		}
		col := cBrightText
		if phase > 0.66 {
			col = cWarning
		}
		b.mark(cx, cy, '✦', col)
		b.mark(cx+1, cy, '✦', col)
		b.mark(cx, cy+1, '✦', col)
		b.mark(cx+1, cy+1, '✦', col)
		for d := 2; d <= arm; d++ {
			fade := float64(d)/float64(maxInt(arm, 1)+1) + phase*0.25
			b.mark(cx+d, cy, '·', mixIndex(p, col, p.base, clampf(fade, 0, 1)*0.75))
			b.mark(cx-d, cy, '·', mixIndex(p, col, p.base, clampf(fade, 0, 1)*0.75))
			b.mark(cx, cy+d, '·', mixIndex(p, col, p.base, clampf(fade, 0, 1)*0.75))
			b.mark(cx, cy-d, '·', mixIndex(p, col, p.base, clampf(fade, 0, 1)*0.75))
		}
	}
}
