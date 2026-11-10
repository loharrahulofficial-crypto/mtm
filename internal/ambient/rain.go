package ambient

import (
	"math"
	"time"
)

// rainScene paints layered rainfall over a lit cabin: near drops are
// long, bright and fast, far drops short and dim, all slanted by the
// wind; a warm flickering window that lights the rain near it; chimney
// smoke curling on noise; pines in two depth layers; mist at the base;
// expanding puddle rings; optional lightning (off by default). Progress
// eases the rain — the sky clears over the pomodoro.
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
	W := float64(sw(b))
	H := float64(sh(b))
	base := state.Palette.Base
	text := state.Palette.Text
	rmp := BuildRamps(state.Palette)
	ts := tSecF(t)

	clearing := clampf(state.Progress, 0, 1)
	if state.Phase == PhaseBreak {
		clearing = clampf(clearing+0.35, 0, 1) // breaks calm the rain
	}
	if state.Phase == PhaseIdle {
		clearing = 1
	}

	// --- sky: a moody vertical gradient, brightening as it clears
	topHex := blendHex(rmp.SkyTop, base, 0.25*(1-clearing))
	horizHex := blendHex(rmp.SkyHorizon, base, 0.35*(1-clearing))
	b.fillGradientHex(0, 0, W, H, topHex, horizHex, 1, opNormal)

	groundY := H * 0.80

	// --- rain, in two depth layers (far: short, dim, slow; near: the reverse)
	wind := cachedPerlin(state.Seed, 211)
	windAt := func(y float64) float64 {
		return wind.At(ts*0.10, y*0.05+31.7) * 0.16 * (1 - clearing*0.6)
	}
	dropLayers := []struct {
		count  int
		length [2]float64
		speed  [2]float64
		width  float64
		ci     string
		alpha  float64
	}{
		{int(W * H / 55), [2]float64{0.8, 1.6}, [2]float64{0.55, 0.8}, 0.14, blendHex(state.Palette.Cyan, base, 0.42), 0.5},
		{int(W * H / 22), [2]float64{1.6, 3.4}, [2]float64{1.0, 1.45}, 0.22, blendHex(state.Palette.Cyan, base, 0.12), 0.85},
	}
	for li, layer := range dropLayers {
		n := int(float64(layer.count) * (1 - clearing*0.72))
		for i := 0; i < n; i++ {
			r := newRNG(state.Seed, uint64(i)*433+3+uint64(li)*7)
			x := r.float64() * (W + 8)
			speed := layer.speed[0] + r.float64()*(layer.speed[1]-layer.speed[0])
			length := layer.length[0] + r.float64()*(layer.length[1]-layer.length[0])
			yOff := r.rangeF(0, H*2)
			yPos := math.Mod(yOff+ts*speed*22, H+length+6) - length
			// cabin window warmth lights raindrops near it
			dropCol := layer.ci
			if li == 1 {
				wx := W * 0.72
				wy := H * 0.66
				dist := math.Hypot(x-wx, yPos-wy)
				if dist < 9 {
					dropCol = blendHex(layer.ci, state.Palette.Warning, (9-dist)/9*0.75)
				}
			}
			slant := windAt(yPos)
			b.strokeHexF(x+slant*length, yPos, x, yPos+length, layer.width, dropCol, layer.alpha, opNormal)
		}
	}

	// --- far pine line (hazy, atmospheric perspective)
	sil := cachedPerlin(state.Seed, 311)
	b.beginLayer(layerLand, 0, int(H*0.55), sw(b), sh(b))
	for x := 0.0; x < W; x += 0.7 {
		gy := groundY + sil.At(x*0.024+3.1, 4.9)*2.2
		treeH := H * 0.09 * (sil.At(x*0.09+9.7, 2.3) + 1.4)
		farCol := rmp.ApplyHaze(state.Palette.SurfaceStrong, 0.68)
		b.fillRectHexF(x, gy-treeH, x+0.7, gy+1.5, farCol, 1, opNormal)
	}
	// --- near pine line (dark silhouette)
	for x := 0.0; x < W; x += 1.1 {
		gy := groundY + sil.At(x*0.03+17.3, 8.1)*2.4 + 3
		treeH := H * 0.13 * (sil.At(x*0.07+27.9, 6.7) + 1.5)
		nearCol := blendHex(base, state.Palette.SurfaceStrong, 0.14)
		for i := 0; i < 3; i++ {
			tb := float64(i) / 3
			width := treeH * 0.36 * (tb*0.85 + 0.3)
			yC := gy - treeH*(1-tb) + treeH*0.18
			b.fillRectHexF(x+0.55-width, yC, x+0.55+width, yC+treeH*0.26, nearCol, 1, opNormal)
		}
	}
	// --- mist at the base: drifting soft light
	b.beginLayer(layerFog, 0, int(H*0.62), sw(b), sh(b))
	for i := 0; i < 5; i++ {
		mx := math.Mod(ts*1.7+float64(i)*W*0.27, W+50) - 25
		my := H*0.78 + float64(i%3)*H*0.05
		b.fillEllipseHexF(mx, my, W*0.24, H*0.045, blendHex(text, base, 0.55), 0.16+0.06*math.Sin(ts*0.5+float64(i)*1.7), opNormal)
	}
	b.endLayer()

	// --- cabin (right third): body, roof, flickering window, chimney smoke
	cabinW := W * 0.19
	if cabinW < 10 {
		cabinW = 10
	}
	cx0 := W * 0.66
	cx1 := cx0 + cabinW
	cabinTop := H*0.615 + 1
	b.beginLayer(layerCabin, int(cx0-5), int(cabinTop-4), int(cx1+5), sh(b))
	// body
	b.fillRectHexF(cx0, cabinTop, cx1, H+1, blendHex(base, state.Palette.Brown, 0.45), 1, opNormal)
	// roof: stepped trapezoid
	for r := 0; r < 4; r++ {
		off := float64(r) * 1.35
		b.fillRectHexF(cx0-1.5-off, cabinTop-1.6-float64(r)*1.15, cx1+1.5+off, cabinTop-0.6-float64(r)*1.15, blendHex(base, state.Palette.SurfaceRaised, 0.32), 1, opNormal)
	}
	// door: dark notch
	dw := cabinW * 0.18
	b.fillRectHexF(cx0+cabinW*0.16, cabinTop+H*0.115, cx0+cabinW*0.16+dw, H+1, blendHex(base, base, 0.5), 1, opNormal)
	// window: 2x2 cells, warm flicker
	flick := cachedPerlin(state.Seed, 401)
	winW := cabinW * 0.30
	if winW < 2.2 {
		winW = 2.2
	}
	winX := cx0 + cabinW*0.44
	winY := cabinTop + H*0.055
	fmsg := flick.At(ts*1.55, 7.0)
	windowWarm := 0.68 + 0.32*(0.5+0.5*fmsg)
	if fmsg < -0.55 {
		windowWarm = 0.38 // guttering flame
	}
	b.fillRectHexF(winX, winY, winX+winW, winY+H*0.075, blendHex(state.Palette.Warning, base, 1-windowWarm), 1, opNormal)
	b.fillRectHexF(winX+winW*0.42, winY, winX+winW*0.58, winY+H*0.075, blendHex(base, state.Palette.Warning, 0.3), 1, opNormal) // mullion
	// porch glow spilling onto the ground
	b.fillEllipseHexF(winX+winW*0.5, cabinTop+H*0.115, cabinW*0.42, H*0.055, state.Palette.Warning, 0.10+0.08*(0.5+0.5*fmsg), opAdd)
	// chimney + curling smoke on noise
	chimX := cx1 - cabinW*0.22
	b.fillRectHexF(chimX, cabinTop-4.6, chimX+1.1, cabinTop-1.2, blendHex(base, state.Palette.SurfaceStrong, 0.35), 1, opNormal)
	for k := 0; k < 6; k++ {
		fp := math.Mod(ts*0.75+float64(k)*0.32+float64(state.Seed%7)*0.13, 2.1) / 2.1
		py := cabinTop - 4.6 - fp*H*0.24
		px := chimX + 0.55 + math.Sin(fp*5.1+float64(k)*1.35)*1.7*fp + fp*1.15
		r := 0.32 + fp*0.5
		col := blendHex(text, base, 0.35+fp*0.5)
		b.fillCircleHexF(px, py, r, col, 0.34+0.25*(1-fp), opNormal)
	}
	b.endLayer()

	// --- puddle rings on the clearing at the base
	b.beginLayer(layerSea, 0, int(H*0.82), sw(b), sh(b))
	ripSlot := int64(wrapT(t, 24*time.Hour)/time.Millisecond) / 750
	for k := 0; k < 4; k++ {
		slot := ripSlot - int64(k)
		rr := newRNG(state.Seed, uint64(slot)*281+17)
		frac := (float64(wrapT(t, 24*time.Hour)/time.Millisecond) / 1000.0) - float64(slot)*0.75
		if frac < 0 || frac >= 0.75 {
			continue
		}
		progress := frac / 0.75
		rx := rr.rangeF(0.15, 0.9) * W
		ry := H*0.90 + rr.rangeF(-1, 1)
		b.strokeCircleHexF(rx, ry, 0.4+progress*2.6, 0.26, state.Palette.Cyan, 0.75*(1-progress), opNormal)
	}
	b.endLayer()

	// --- lightning (off by default): brief global brightening + branching bolt
	if state.Lightning {
		slot := int64(wrapT(t, 24*time.Hour)/time.Millisecond) / 21000
		lr := newRNG(state.Seed, uint64(slot)*613+41)
		if lr.float64() < 0.30 {
			frac := (float64(wrapT(t, 24*time.Hour)/time.Millisecond) / 1000.0) - float64(slot)*21
			if frac >= 0 && frac < 0.22 {
				flash := 1 - frac/0.22
				flash = flash * flash
				b.fillRectHexF(0, 0, W, H*0.5, state.Palette.Yellow, 0.16*flash, opAdd)
				// bolt: jagged polyline with two branches
				bx := lr.rangeF(0.25, 0.75) * W
				by := 0.0
				rb := newRNG(state.Seed, uint64(slot)*911+5)
				for k := 0; k < 34; k++ {
					nx := bx + float64(trb(rb, k))
					ny := by + 0.75
					b.strokeHexF(bx, by, nx, ny, 0.34, state.Palette.BrightText, 0.9*flash, opAdd)
					// one branch every few segments
					if k == 8 || k == 17 {
						side := 1.0
						if rb.float64() < 0.5 {
							side = -1.0
						}
						bbx := bx
						for j := 0; j < 8; j++ {
							bbx += side * (0.55 + rb.float64()*0.5)
							b.strokeHexF(bbx-side*0.6, by+float64(j)*0.75, bbx, by+float64(j+1)*0.75, 0.22, state.Palette.BrightText, 0.55*flash, opAdd)
						}
					}
					bx, by = nx, ny
					if by >= H*0.75 {
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
