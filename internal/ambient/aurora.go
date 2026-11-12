package ambient

import "math"

// aurora.go — the aurora as curtains: three additive noise ribbons in
// theme hues (Cyan, Success, soft Magenta), each a vertical gradient
// with a bright foot at its lower edge fading upward, modulated by
// noise, with slow vertical ray shimmer. The lake mirror reuses the same
// painter dimmed and squashed.

// auroraRibbon is one curtain: a hue, a vertical anchor (the foot's
// height as a share of the raster height), and the noise that shapes it.
type auroraRibbon struct {
	col   string  // ribbon hue
	baseY float64 // foot height, share of H
	thick float64 // curtain thickness, share of H
	sway  float64 // horizontal sway amplitude, share of H
	freq  float64 // sway frequency along the width
	phase float64 // noise phase offset
	alpha float64 // brightness scale
}

// auroraRibbons returns the three curtains, far to near, theme hues.
func auroraRibbons(state SceneState) []auroraRibbon {
	return []auroraRibbon{
		{col: state.Palette.Cyan, baseY: 0.145, thick: 0.062, sway: 0.105, freq: 5.2, phase: 0, alpha: 1.0},
		{col: state.Palette.Success, baseY: 0.252, thick: 0.055, sway: 0.095, freq: 4.3, phase: 7.3, alpha: 0.82},
		{col: blendHex(state.Palette.Magenta, state.Palette.Base, 0.30), baseY: 0.358, thick: 0.050, sway: 0.085, freq: 5.8, phase: 15.9, alpha: 0.66},
	}
}

// paintAuroraCurtains paints the curtains. squash < 1 mirrors them into
// the lake reflection band (laid out against the shoreline, dimmed);
// ripple displaces each dab horizontally (nil for the direct pass).
func paintAuroraCurtains(b *buffer, state SceneState, ts float64, squash, dimmer float64, ripple func(float64) float64) {
	W := float64(sw(b))
	H := float64(sh(b))
	layH := layoutForState(state, W, H).Horizon
	aura := cachedPerlin(state.Seed, 71)
	for i, r := range auroraRibbons(state) {
		height := r.thick * H
		for x := 0.0; x < W; x += 0.5 {
			fx := x / W
			foot := r.baseY*H + aura.At(fx*r.freq+r.phase, ts*0.055+r.phase*0.7)*r.sway*H
			for y := foot - height; y <= foot; y += 0.5 {
				if y < 0 {
					continue
				}
				d := clampf((foot-y)/height, 0, 1) // 0 at the foot, 1 at the top
				// bright foot, fading upward (the curtain profile)
				up := math.Pow(math.Max(0, 1-d), 1.7)
				// noise-modulated brightness along the ribbon
				n := 0.5 + 0.5*aura.At(fx*5.0+r.phase*1.7, ts*0.09+float64(i)*3.1)
				intensity := (0.09 + 0.18*n) * r.alpha * dimmer
				// slow vertical ray shimmer — bands drifting upward
				ray := 0.58 + 0.42*math.Sin(ts*0.55+y*0.85+x*0.055+aura.At(fx*2.0, ts*0.04+float64(i))*2.1+float64(i)*2.4)
				cov := up * intensity * ray
				if cov <= 0.02 {
					continue
				}
				yDest := y
				xOff := 0.0
				if squash < 1 {
					yDest = layH + (layH-y)*squash
					if ripple != nil {
						xOff = ripple(yDest)
					}
				}
				b.glowHexAt(x+xOff, yDest, r.col, clampf(cov, 0, 0.75))
			}
		}
	}
}

// paintMilkyWay paints a faint diagonal band of galactic glow — a wide
// soft arc across the sky, modulated by noise. Deliberately very faint;
// remove it by passing enabled=false.
func paintMilkyWay(b *buffer, state SceneState, ts float64) {
	W := float64(sw(b))
	H := float64(sh(b))
	mw := cachedPerlin(state.Seed, 314)
	// a wide diagonal band from lower-left to upper-right
	fx0, fy0 := -0.25*W, H*0.52
	fx1, fy1 := 1.25*W, H*0.06
	band := 0.10 * H
	for x := 0.0; x < W; x += 0.75 {
		// parametric position along the arc
		t := clampf((x-fx0)/(fx1-fx0), 0, 1)
		cy := fy0 + (fy1-fy0)*t + mw.At(t*6.0, ts*0.02)*H*0.05
		for y := cy - band; y <= cy+band; y += 0.5 {
			d := math.Abs(y-cy) / band
			up := 1 - d*d
			n := 0.5 + 0.5*mw.At(t*9.0+31.7, y*0.10+ts*0.015)
			cov := up * up * 0.045 * (0.35 + 0.65*n)
			if cov <= 0.008 {
				continue
			}
			b.fillCircleHexF(x, y, 0.75, blendHex(state.Palette.Cyan, state.Palette.Base, 0.55), cov, opAdd)
		}
	}
}
