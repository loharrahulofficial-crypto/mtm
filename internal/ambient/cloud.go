package ambient

// cloud.go — noise-modulated soft-edged cloud masses (mountains, river,
// ocean). Each mass is a per-pixel FBM-threshold coverage mask — no
// stacked discs — so the silhouette is irregular, the edges feather to
// nothing, the tops are lighter than the bottoms, and overlapping masses
// never draw outlines.

// paintCloudMass paints one cloud: an FBM-shaped irregular mass with
// lighter tops and darker bottoms.
//
//	cx, cy     — center in sub-cell coordinates
//	halfW      — horizontal half-width in sub-cells (the mass reaches ~1.15x)
//	halfH      — vertical half-height in sub-cells
//	phase      — noise phase offset (vary per cloud)
//	seed       — the cached scene perlin
//	lightHex   — top color
//	darkHex    — bottom color
//	peak       — max coverage alpha at the mask's core
func paintCloudMass(b *buffer, cx, cy, halfW, halfH, phase float64, seed *perlin, lightHex, darkHex string, peak float64) {
	ss := float64(fSSV())
	asp := b.fb.aspectCorr()
	pf := seed
	lr, lg, lb, _ := hexF(lightHex)
	dr, dg, db, _ := hexF(darkHex)
	x0 := maxInt(0, int((cx-halfW*1.2)*ss))
	x1 := minInt(b.fb.pw, int((cx+halfW*1.2)*ss)+1)
	y0 := maxInt(0, int((cy-halfH*1.2/asp)*ss))
	y1 := minInt(b.fb.ph, int((cy+halfH*1.2/asp)*ss)+1)
	fX := 0.038 + phase*0.0008
	maskLo, maskHi := 0.50, 0.78
	for py := y0; py < y1; py++ {
		pyf := float64(py)/ss + 0.5/ss
		ndy := (pyf - cy) * asp / halfH
		if ndy*ndy >= 1.44 {
			continue
		}
		for px := x0; px < x1; px++ {
			pxf := float64(px)/ss + 0.5/ss
			ndx := (pxf - cx) / halfW
			de := ndx*ndx + ndy*ndy
			if de >= 1.42 {
				continue
			}
			// FBM-shaped mask: irregular but soft silhouette
			n := pf.FBM(pxf*fX+phase*13.7, pyf*asp*fX-phase*7.1, 4)*0.5 + 0.5
			mask := Smoothstep(clampf((n-maskLo)/(maskHi-maskLo), 0, 1)) * (1 - Smoothstep(clampf((de-0.72)/0.55, 0, 1)))
			if mask <= 0.004 {
				continue
			}
			// fake shading: lighter top, darker bottom
			l := clampf(0.5-(pyf-cy)/(2*halfH), 0, 1)
			l = Smoothstep(l)
			r := dr + (lr-dr)*float32(l)
			g := dg + (lg-dg)*float32(l)
			bl := db + (lb-db)*float32(l)
			b.fb.blendPx(px, py, fpix{r, g, bl, float32(peak * mask)}, opNormal)
		}
	}
}
