package gauges

import "strings"

// ThinBar renders frac ∈ [0,1] into `cells` cells using a lower-half-block
// glyph for both the fill and the track, so a row of bars never merges into
// a slab the way full-height blocks do. The fill and the track differ only
// in color — the caller applies it (subtle track color, accent/course-color
// fill). Any non-zero fraction shows at least one filled cell.
func ThinBar(frac float64, cells int) (fill, track string) {
	if cells < 1 {
		return "", ""
	}
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	n := int(frac*float64(cells) + 0.5)
	if frac > 0 && n == 0 {
		n = 1
	}
	return strings.Repeat("▄", n), strings.Repeat("▄", cells-n)
}
