package ambient

import "math"

// forest.go — the shared noise-driven jagged forest silhouette painter,
// in 2–3 depth layers (farther layers lighter and hazier). The ridgeline
// is a continuous serrated profile — a low rolling swell plus sparse,
// sharp spires — never discrete columns and never a flat slab.
//
// The vertical gradient is evaluated on an absolute-y ramp, so every
// column of the fill carries the exact same color at the same height —
// no per-column gradient drift (the "faint vertical striping" of the old
// per-column fills).

// ridgeParams describe one treeline layer: a rolling swell with sparse
// sharp spires. Heights are absolute sub-cell pixels above baseY.
type ridgeParams struct {
	swellAmp, swellFreq float64 // low rolling swell
	spireAmp, spireFreq float64 // sharp, irregular spires
	phase               float64 // noise phase (seed-derived)
}

// treelineLayer bundles one depth layer of the treeline with its geometry
// and colors, so the renderer and the stripe test agree exactly.
type treelineLayer struct {
	baseY  float64     // ridge reference height (sub-cells)
	yBot   float64     // fill bottom (sub-cells)
	params ridgeParams // ridge shape
	crest  string      // color at the ridge crest (top)
	base   string      // color at the fill base (bottom)
}

// ridgeHeightAt returns the absolute y of the ridgeline at x (smaller y
// = higher trees). Shared by the painter and the lake reflection so the
// reflection matches the silhouette exactly.
func ridgeHeightAt(sil *perlin, x, baseY float64, p ridgeParams) float64 {
	swell := sil.At(x*p.swellFreq+p.phase, 2.3) * p.swellAmp
	// the spire component is ridged noise — sharp cusps at the noise's
	// zero crossings, windowed sparse, with a power curve that narrows
	// them into pine teeth
	sp := sil.At(x*p.spireFreq+p.phase*1.7, 9.1)
	sp = 1 - math.Abs(sp)
	sp = clampf((sp-0.30)/0.70, 0, 1)
	spire := math.Pow(sp, 2.2) * p.spireAmp
	spire += math.Abs(sil.At(x*3.7+p.phase*3.1, 5.9)) * p.swellAmp * 0.35
	return baseY + swell - spire
}

// paintRidgeLayer paints one depth layer of the treeline. The fill ends
// at layer.yBot. Colors are absolute (haze applied by the caller); the
// gradient runs along absolute y, so all columns agree on the color of
// every row.
func paintRidgeLayer(b *buffer, sil *perlin, W float64, layer treelineLayer) {
	p := layer.params
	yTop := layer.baseY - p.swellAmp - p.spireAmp - p.swellAmp*0.35 - 1
	for x := 0.0; x < W; x += 0.25 {
		y := ridgeHeightAt(sil, x, layer.baseY, p)
		if y < yTop {
			y = yTop
		}
		t := clampf((y-yTop)/(layer.yBot-yTop+0.001), 0, 1)
		crest := blendHex(layer.crest, layer.base, t)
		b.fillGradientHex(x, y, x+0.25, layer.yBot, crest, layer.base, 1, opNormal)
	}
}

// paintMistBase lays a touch of drifting mist along the treeline base.
func paintMistBase(b *buffer, silMist *perlin, ts, W, H, baseY float64, mistCol string, seedPhase float64) {
	for i := 0; i < 3; i++ {
		drift := math.Mod(ts*0.35+float64(i)*W*0.37+silMist.At(float64(i)*3.1, seedPhase)*W*0.1, W+80) - 40
		b.fillEllipseHexF(drift, baseY-1.2+float64(i)*1.3, W*0.22, H*0.018, mistCol, 0.15, opNormal)
	}
}
