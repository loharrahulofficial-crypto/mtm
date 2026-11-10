package ambient

import (
	"math"
	"time"
)

// snowScene is layered snowfall over a quiet treeline: three parallax
// depths of flakes, wind shear, and a rising accumulation on the ground.
type snowScene struct {
	mode string
}

func (s *snowScene) Name() string { return "snow" }

func (s *snowScene) Frame(t time.Duration, w, h int, state SceneState) []Line {
	return renderFrame(s, s.mode, t, w, h, state, false)
}

func (s *snowScene) Reward(t time.Duration, w, h int, state SceneState) []Line {
	return renderFrame(s, s.mode, t, w, h, state, true)
}

func (s *snowScene) paint(b *buffer, p *paletteModel, t time.Duration, w, h int, state SceneState) {
	// Sky stays the base color; the moon, treeline and flakes carry it
	// Dim moon
	moonY := sh(b) / 5
	moonX := sw(b) * 3 / 4
	phase := moonPhase(state.Now)
	paintMoon(b, moonX, moonY, maxInt(minInt(sw(b), sh(b))/8, 2), phase, p)

	// Treeline silhouette
	pine := cachedPerlin(state.Seed, 641)
	ground := sh(b) * 74 / 100
	for x := 0; x < sw(b); x++ {
		gy := ground + int(pine.At(float64(x)*0.17, 9.1)*2)
		ci := mixIndex(p, cSurfaceStrong, p.base, 0.25)
		for y := gy; y < minInt(gy+3, sh(b)); y++ {
			b.glow(x, y, ci)
		}
		// sparse pines reaching above the line
		th := 3 + int((pine.At(float64(x)*0.55, 21.4)+1)*3.2)
		for y := gy - th; y < gy; y++ {
			ci2 := mixIndex(p, cSurfaceStrong, p.base, 0.12)
			if y < gy-th+2 {
				b.mark(x, y, '▴', ci2)
				continue
			}
			b.glow(x, y, ci2)
		}
	}

	// Snow accumulation — creeps up with progress (the pomodoro settles in)
	accum := int(state.Progress * float64(sh(b)) * 0.10)
	for y := sh(b) - 1 - accum; y < sh(b); y++ {
		ci := mixIndex(p, cBrightText, p.base, 0.45+float64(sh(b)-1-y)/float64(maxInt(accum+2, 1))*0.2)
		for x := 0; x < sw(b); x++ {
			b.mark(x, y, '▁', ci)
		}
	}

	// Flakes: three parallax layers
	tSec := tSecF(t)
	wind := cachedPerlin(state.Seed, 661).At(tSec*0.05, 0)
	layers := []struct {
		count  int
		speed  float64
		length int
		ci     int
		drift  float64
	}{
		{int(float64(sw(b)) * 0.55), 0.55, 1, mixIndex(p, cBrightText, p.base, 0.55), 0.4},
		{int(float64(sw(b)) * 0.3), 0.95, 1, mixIndex(p, cBrightText, p.base, 0.3), 0.8},
		{int(float64(sw(b)) * 0.14), 1.5, 2, mixIndex(p, cBrightText, p.base, 0.12), 1.3},
	}
	for li, L := range layers {
		for i := 0; i < L.count; i++ {
			r := newRNG(state.Seed, uint64(li)*1000+uint64(i)*37+7)
			xBase := r.float64() * float64(sw(b))
			yOff := r.rangeF(0, float64(sh(b))*2)
			for k := 0; k < L.length; k++ {
				y := float64(i)*1 + tSec*L.speed*(1+float64(li)*0.25) + yOff - float64(k)*0.9
				yy := math.Mod(y, float64(sh(b))+2) - 1
				xx := xBase + wind*L.drift*3 + math.Sin(tSec*0.8+float64(i))*L.drift
				b.mark(int(xx), int(yy), '·', L.ci)
			}
		}
	}
}
