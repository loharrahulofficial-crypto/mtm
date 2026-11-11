package ambient

import (
	"math"
)

// layout.go — size-independent composition anchors. No scene positions
// or sizes itself in absolute cells: every anchor is a share of the
// raster (min(w, h*2) for size, aspect ratio for the horizon line).
//
// Tall windows get more sky (horizon lower); wide windows get less.
// The horizon always lands in [40%, 70%] of the height.
type sceneLayout struct {
	W, H    float64 // sub-cell raster dims (halfblock: w × 2h)
	MinDim  float64 // min(W, H)
	Aspect  float64 // W / H
	Horizon float64 // absolute sub-cell y of the horizon/ground line
}

// layoutFor builds the layout for a halfblock frame of w cells by h rows.
func layoutFor(w, h int) sceneLayout {
	return layoutForSub(float64(w), float64(h)*2)
}

// layoutForSub builds the layout from the actual sub-cell raster dims so
// every mode (halfblock | braille | ascii) composes correctly.
func layoutForSub(W, H float64) sceneLayout {
	m := math.Min(W, H)
	ar := W / H
	frac := 0.60 - 0.10*(ar-1.0)
	frac = clampf(frac, 0.40, 0.70)
	return sceneLayout{W: W, H: H, MinDim: m, Aspect: ar, Horizon: H * frac}
}
