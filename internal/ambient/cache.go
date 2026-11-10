package ambient

import (
	"strings"
	"sync"
)

// perlinCache memoizes seeded perlin tables. Scenes construct several
// tables per frame; they are pure functions of (seed, stream) and are
// immutable once built, so they are safe to share across renders and
// goroutines.
var perlinCache sync.Map

func cachedPerlin(seed int64, stream uint64) *perlin {
	key := [2]uint64{uint64(seed), stream}
	if v, ok := perlinCache.Load(key); ok {
		return v.(*perlin)
	}
	p := newPerlin(seed, stream)
	actual, _ := perlinCache.LoadOrStore(key, p)
	return actual.(*perlin)
}

// paletteCache memoizes the palette LUT (roles + blend ladder). It is a
// pure function of the role hexes and is immutable after construction.
var paletteCache sync.Map

func paletteFor(p Palette) *paletteModel {
	key := strings.Join([]string{p.Base, p.Surface, p.SurfaceRaised, p.SurfaceStrong, p.Border, p.Text, p.BrightText, p.MutedSoft,
		p.Accent, p.AccentSoft, p.Success, p.Warning, p.Danger, p.HeatWarm, p.HeatHot, p.HeatOverdue,
		p.Blue, p.Cyan, p.Yellow, p.Magenta, p.Orange, p.Brown}, "|")
	if v, ok := paletteCache.Load(key); ok {
		return v.(*paletteModel)
	}
	m := buildPalette(p)
	actual, _ := paletteCache.LoadOrStore(key, m)
	return actual.(*paletteModel)
}
