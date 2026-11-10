package ambient

import (
	"math"
	"time"
)

// rainScene is rainfall over a cabin: braille-fine streaks at varied
// speeds, pine silhouettes in two parallax depths, a cabin with a lit
// flickering window and chimney smoke, drifting fog, and optional lightning
// (off by default). Progress eases the rain — the sky clears over the
// pomodoro. Idle mode sits in the same clearing state.
type rainScene struct {
	mode string
}

func (s *rainScene) Name() string { return "rain" }

func (s *rainScene) Frame(t time.Duration, w, h int, state SceneState) []Line {
	return renderFrame(s, s.mode, t, w, h, state, false)
}

func (s *rainScene) Reward(t time.Duration, w, h int, state SceneState) []Line {
	return renderFrame(s, s.mode, t, w, h, state, true)
}

func (s *rainScene) paint(b *buffer, p *paletteModel, t time.Duration, w, h int, state SceneState) {
	tSec := tSecF(t)
	clearing := clampf(state.Progress, 0, 1)
	if state.Phase == PhaseBreak {
		clearing = clampf(clearing+0.35, 0, 1) // breaks calm the rain
	}
	if state.Phase == PhaseIdle {
		clearing = 1
	}

	// Sky stays the untouched base color (flat fills render as harsh blocks).
	// The clearing is signalled by the fog thinning and the drops easing.

	// Fog wisps: sparse noise dots drifting sideways
	fog := newPerlin(state.Seed, 211)
	for y := sh(b) * 40 / 100; y < sh(b)*72/100; y++ {
		fx := float64(y) / float64(sh(b))
		for x := 0; x < sw(b); x += 2 {
			n := fog.At(tSec*0.07+fx*4.0+float64(x)*0.05, fx*9.0+31.7)
			if n > 0.34-clearing*0.35 {
				b.mark(x, y, '·', mixIndex(p, cText, p.base, 0.55+(n-0.3)*0.5))
			}
		}
	}

	// Rain: seeded streaks with varied speed; braille does fine dots
	dropCount := int(18 + float64(sw(b)*sh(b))/16*(1-clearing*0.75))
	for i := 0; i < dropCount; i++ {
		r := newRNG(state.Seed, uint64(i)*433+3)
		dx := r.float64() * float64(sw(b))
		speed := r.rangeF(0.28, 0.62) * (1 + float64(i%3)*0.18) * (1 - clearing*0.55)
		length := r.rangeF(2, 5)
		yOff := r.rangeF(0, float64(sh(b))*2)
		yPos := (float64(i)*7.31 + tSec*speed*7.31 + yOff)
		head := math.Mod(yPos, float64(sh(b))+length) - length
		for k := 0; k < int(length); k++ {
			y := head - float64(k)*1.35
			x := dx - float64(k)*0.22 // slant
			if y < 0 || int(y) >= sh(b) {
				continue
			}
			fade := 1 - float64(k)/length
			ci := mixIndex(p, cCyan, p.base, 0.25+fade*0.45)
			switch b.mode {
			case ModeASCII:
				ch := '|'
				if k == 0 {
					ch = '.'
				}
				b.mark(int(x), int(y), ch, ci)
			case ModeBraille:
				b.glow(int(x), int(y*1.0), ci)
			default: // halfblock
				b.glow(int(x), int(y), ci)
			}
		}
	}

	// Ripples on the clearing pond at the bottom (beat-slotted)
	ripSlot := int64(wrapT(t, 24*time.Hour)/time.Millisecond) / 650
	for k := 0; k < 4; k++ {
		slot := ripSlot - int64(k)
		rr := newRNG(state.Seed, uint64(slot)*281+17)
		frac := (float64(wrapT(t, 24*time.Hour)/time.Millisecond) / 1000.0) - float64(slot)*0.65
		if frac < 0 || frac >= 0.65 {
			continue
		}
		rx := rr.rangeF(0.12, 0.88) * float64(sw(b))
		ry := sh(b) - 2
		radius := frac / 0.65 * 3.2
		ci := mixIndex(p, cCyan, p.base, 0.55+frac*0.4)
		for dx := -3; dx <= 3; dx++ {
			d := absF(float64(dx))
			if d <= radius && d >= radius-0.9 {
				b.glow(int(rx)+dx, int(ry), ci)
			}
		}
	}

	// Pines: two parallax depths along the ground
	pine := newPerlin(state.Seed, 311)
	ground := sh(b) * 80 / 100
	for x := 0; x < sw(b); x++ {
		gy := ground + int(pine.At(float64(x)*0.16, 4.9)*2)
		treeH := 2 + int((pine.At(float64(x)*0.5, 12.3)+1)*2.2)
		ci := mixIndex(p, cSurfaceStrong, p.base, 0.2)
		for y := gy - treeH; y < minInt(gy+2, sh(b)); y++ {
			if y < gy-treeH+1 && treeH > 2 {
				b.mark(x, y, '▴', ci)
				continue
			}
			b.glow(x, y, ci)
		}
	}

	// Cabin (right third): body, roof, lit window with flicker, chimney smoke
	cx0 := sw(b) * 62 / 100
	cw := sw(b) * 26 / 100
	if cw < 8 {
		cw = 8
	}
	chH := sh(b) - ground + (ground - sh(b)*72/100) // cabin height from roof base down
	_ = chH
	cabinTop := sh(b)*72/100 + 1
	cx1 := cx0 + cw
	if cx1 > sw(b)-1 {
		cx1 = sw(b) - 1
	}
	// body
	for y := cabinTop; y < sh(b); y++ {
		b.glowBlock(cx0, y, cx1, y+1, mixIndex(p, cBrown, p.base, 0.35))
	}
	// roof: stepped trapezoid
	for r := 0; r < 3; r++ {
		y := cabinTop - 1 - r
		off := r
		for x := cx0 - off; x < cx1+off; x++ {
			if x >= 0 && x < sw(b) {
				b.glow(x, y, mixIndex(p, cSurfaceRaised, p.base, 0.15))
			}
		}
	}
	// window: 2×2 lit cells flickering with seeded noise
	flick := newPerlin(state.Seed, 401)
	winW := (cx1 - cx0) / 3
	if winW < 1 {
		winW = 1
	}
	winX := cx0 + (cx1-cx0)*2/5
	winY := cabinTop + 1
	for dy := 0; dy < 2; dy++ {
		for dx := 0; dx < winW; dx++ {
			n := flick.At(tSec*1.7+float64(dx)*0.8, float64(dy)*3.1+7.0)
			ci := mixIndex(p, cWarning, p.base, 0.05)
			if n < -0.25 {
				ci = mixIndex(p, cWarning, p.base, 0.5) // guttering flame
			}
			b.mark(winX+dx, winY+dy, '▪', ci)
		}
	}
	// fire glow spilling from the window onto the porch
	b.glow(winX+winW, winY+2, mixIndex(p, cWarning, p.base, 0.65))
	// door: dark notch
	b.glowBlock(cx0+(cx1-cx0)/3, cabinTop+(sh(b)-cabinTop)/2+1, cx0+(cx1-cx0)/3+maxInt((cx1-cx0)/5, 1), sh(b), mixIndex(p, cBase, p.base, 0))
	// chimney + smoke: three dots wiggling upward, fading with height
	chimneyX := cx1 - maxInt((cx1-cx0)/5, 2)
	b.glowBlock(chimneyX, cabinTop-3, chimneyX+1, cabinTop, mixIndex(p, cSurfaceStrong, p.base, 0.2))
	for k := 0; k < 5; k++ {
		ff := float64(wrapT(t, 24*time.Hour)/time.Second)*1.4 + float64(k)*1.7 + float64(state.Seed%7)
		fp := math.Mod(ff, 6) / 6
		py := float64(cabinTop-3) - fp*float64(sh(b))*0.28
		px := float64(chimneyX) + math.Sin(fp*5.2+float64(k))*1.6*fp
		ci := mixIndex(p, cText, p.base, 0.35+fp*0.55)
		b.mark(int(px), int(py), '·', ci)
	}

	// Lightning: rare, silent, off by default. One jagged bolt + a flash.
	if state.Lightning {
		slot := int64(wrapT(t, 24*time.Hour)/time.Millisecond) / 21000
		lr := newRNG(state.Seed, uint64(slot)*613+41)
		if lr.float64() < 0.28 {
			frac := (float64(wrapT(t, 24*time.Hour)/time.Millisecond) / 1000.0) - float64(slot)*21
			if frac >= 0 && frac < 0.18 {
				// flash: wash the top rows toward bright
				for y := 0; y < sh(b)/3; y++ {
					b.glowBlock(0, y, sw(b), y+1, mixIndex(p, cYellow, p.base, 0.55))
				}
				// bolt: jagged diagonal
				bx := lr.rangeF(0.2, 0.8) * float64(sw(b))
				by := 0.0
				rb := newRNG(state.Seed, uint64(slot)*911+5)
				for k := 0; k < sh(b)*2/3; k++ {
					b.mark(int(bx+0.5), int(by), '│', cBrightText)
					bx += float64(trb(rb, k))
					by += 0.7
					if int(by) >= sh(b) {
						break
					}
				}
			}
		}
	}
}

func trb(r *rng, k int) int {
	switch r.intn(5) {
	case 0:
		return -1
	case 1:
		return 1
	default:
		return 0
	}
}
