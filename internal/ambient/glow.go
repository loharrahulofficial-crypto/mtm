package ambient

import "math"

// glow.go — the per-pixel radial falloff behind sun/moon glow and bloom.
// One exponential, no stacked discs: alpha(d) = peak * exp(-d²/2σ²) where
// d is the aspect-corrected distance from the center. Every glow in this
// package goes through fillGlowF so halos never show concentric rings or
// flat rim edges.

// fillGlowF adds a Gaussian glow at (cx, cy) in sub-cell coordinates.
func (b *buffer) fillGlowF(cx, cy, sigma, peak float64, r, g, bl float32, op blendOp) {
	ss := float64(fSSV())
	asp := b.fb.aspectCorr()
	k := 3.4 // exp(-5.8) tail below
	rOut := sigma * k
	x0 := maxInt(0, int((cx-rOut)*ss))
	x1 := minInt(b.fb.pw, int((cx+rOut)*ss)+1)
	y0 := maxInt(0, int((cy-rOut/asp)*ss))
	y1 := minInt(b.fb.ph, int((cy+rOut/asp)*ss)+1)
	for py := y0; py < y1; py++ {
		pyf := float64(py)/ss + 0.5/ss
		dy := (pyf - cy) * asp
		dy2 := dy * dy
		for px := x0; px < x1; px++ {
			pxf := float64(px)/ss + 0.5/ss
			dx := pxf - cx
			d2 := dx*dx + dy2
			a := peak * math.Exp(-d2/(2*sigma*sigma))
			if a <= 0.004 {
				continue
			}
			b.fb.blendPx(px, py, fpix{r, g, bl, float32(a)}, op)
		}
	}
}

// fillGlowHexF paints a Gaussian glow from a hex color.
func (b *buffer) fillGlowHexF(cx, cy, sigma, peak float64, hex string, op blendOp) {
	r, g, bl, ok := hexF(hex)
	if !ok {
		return
	}
	b.fillGlowF(cx, cy, sigma, peak, r, g, bl, op)
}

// limbDarken multiplies a sphere's radiance by a cos(gamma) falloff
// toward the limb, so a disc reads as a ball, not a sticker.
// u = 0.45: mild for both moon and sun.
func limbDarken(cosGamma float64) float64 {
	const u = 0.45
	cosGamma = clampf(cosGamma, 0, 1)
	return 1 - u*(1-cosGamma)
}

// paintMoonHalo paints the Gaussian halo for any sphere in this package.
func paintMoonHalo(b *buffer, mX, mY, sigma, amp float64, col [3]float32) {
	asp := b.fb.aspectCorr()
	rOut := sigma * 2.7
	ss := float64(fSSV())
	x0 := maxInt(0, int((mX-rOut)*ss))
	x1 := minInt(b.fb.pw, int((mX+rOut)*ss)+1)
	y0 := maxInt(0, int((mY-rOut/asp)*ss))
	y1 := minInt(b.fb.ph, int((mY+rOut/asp)*ss)+1)
	for py := y0; py < y1; py++ {
		pyf := float64(py)/ss + 0.5/ss
		dy := (pyf - mY) * asp
		dy2 := dy * dy
		if dy2 >= rOut*rOut {
			continue
		}
		for px := x0; px < x1; px++ {
			pxf := float64(px)/ss + 0.5/ss
			d := math.Hypot(pxf-mX, dy)
			a := amp * math.Exp(-(d*d)/(2*sigma*sigma))
			if a <= 0.004 {
				continue
			}
			b.fb.blendPx(px, py, fpix{col[0], col[1], col[2], float32(a)}, opAdd)
		}
	}
}

// paintSunSphere paints the sun/moon disc for the water line: a sphere
// with limb darkening and a Gaussian halo ("bloom"). lit/hot are the
// limb and core colors; haloCol is the warm glow tone.
func paintSunSphere(b *buffer, mX, mY, mR, ts float64, limbHex, hotHex string, haloCol [3]float32, haloPeak float64) {
	ss := float64(fSSV())
	asp := b.fb.aspectCorr()
	// halo under the disc
	sigma := mR * 0.95
	paintMoonHalo(b, mX, mY, sigma, haloPeak*(0.72+0.28*math.Sin(ts*0.42)), haloCol)
	// sphere
	lr, lg, lb, _ := hexF(limbHex)
	tr, tg, tb, _ := hexF(hotHex)
	x0 := maxInt(0, int((mX-mR-1)*ss))
	x1 := minInt(b.fb.pw, int((mX+mR+1)*ss)+1)
	y0 := maxInt(0, int((mY-(mR+1)/asp)*ss))
	y1 := minInt(b.fb.ph, int((mY+(mR+1)/asp)*ss)+1)
	for py := y0; py < y1; py++ {
		pyf := float64(py)/ss + 0.5/ss
		dy := (pyf - mY) * asp
		for px := x0; px < x1; px++ {
			pxf := float64(px)/ss + 0.5/ss
			d := math.Hypot(pxf-mX, dy)
			z := math.Sqrt(math.Max(0, 1-(d/mR)*(d/mR)))
			cov := clampf(mR-d+0.6, 0, 1)
			if cov <= 0.004 {
				continue
			}
			limb := limbDarken(z)
			// core mix: center is hot, limb is the field color, with limb
			// darkening bending the whole profile toward the rim
			t := math.Pow(z, 1.6)
			r := lr + (tr-lr)*float32(t)
			g := lg + (tg-lg)*float32(t)
			bl := lb + (tb-lb)*float32(t)
			r *= float32(limb)
			g *= float32(limb)
			bl *= float32(limb)
			b.fb.blendPx(px, py, fpix{r, g, bl, float32(cov)}, opNormal)
			b.fb.inkAt(px, py, float32(cov))
		}
	}
}
