package ambient

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// crossfade.go — raster-space scene transitions. Both frames become
// pixel grids first (w × 2h for halfblock); every pixel blends toward
// the target by the eased progress; the blended raster is quantized and
// emitted once. Text overlays are the caller's job: composited after the
// transition so they stay crisp. Endpoints return the exact source
// lines.

// Dissolve wipes pixels to the base color on a seeded noise sweep — a
// pixel turns to base once the sweep passes it. t ∈ [0,1]; 0 = intact,
// 1 = fully dissolved.
func Dissolve(lines []Line, t float64) []Line {
	if t <= 0 {
		return lines
	}
	if t >= 1 {
		return BlackFrame(lines)
	}
	base := baseHexFromLines(lines)
	r := rasterFromLines(lines, base)
	return DissolveRaster(r, t, base).emitHalfblock(base, emitOpts{pairOnly: true})
}

// baseHexFromLines returns the dominant background hex of the frame
// (default: the mtm default). Scene frames are opaque, so any interior
// cell's bg works; ties break by hex value so the result is deterministic
// across runs (map iteration order is randomized).
func baseHexFromLines(lines []Line) string {
	counts := map[string]int{}
	for _, line := range lines {
		for _, mm := range sgrBg.FindAllStringSubmatch(line, -1) {
			parts := strings.Split(mm[1], ";")
			if len(parts) != 3 {
				continue
			}
			r, ok1 := atoiSmall(parts[0])
			g, ok2 := atoiSmall(parts[1])
			b, ok3 := atoiSmall(parts[2])
			if !ok1 || !ok2 || !ok3 {
				continue
			}
			counts[fmt.Sprintf("#%02x%02x%02x", r, g, b)]++
		}
	}
	best := "#14282d"
	bestN := 0
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if counts[k] > bestN {
			best, bestN = k, counts[k]
		}
	}
	return best
}

var sgrBg = regexp.MustCompile(`48;2;([0-9]+;[0-9]+;[0-9]+)`)

// BlackFrame renders an all-base frame with the same cell grid as lines.
func BlackFrame(lines []Line) []Line {
	base := baseHexFromLines(lines)
	h := len(lines)
	w := 0
	for _, l := range lines {
		if n := len([]rune(stripEscapes(l))); n > w {
			w = n
		}
	}
	return BaseRaster(base, w, h*2).emitHalfblock(base, emitOpts{})
}

// stripEscapes removes ANSI sequences (for width counting).
func stripEscapes(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			for j := i; j < len(s); j++ {
				if s[j] >= 'a' && s[j] <= 'z' || s[j] >= 'A' && s[j] <= 'Z' {
					i = j
					break
				}
			}
			continue
		}
		sb.WriteByte(s[i])
	}
	return sb.String()
}

// Crossfade blends from→to by eased t ∈ [0,1]. Endpoints return the exact
// source lines. Both frames are normalized to (top, bottom) pixel pairs
// first, blended per pixel, then quantized and emitted once.
func Crossfade(from, to []Line, t float64) []Line {
	if t <= 0 {
		return from
	}
	if t >= 1 {
		return to
	}
	e := EaseInOutCubic(t)
	base := baseHexFromLines(from)
	ra := rasterFromLines(from, base)
	rb := rasterFromLines(to, baseHexFromLines(to))
	return BlendRasters(ra, rb, e).emitHalfblock(base, emitOpts{pairOnly: true})
}
