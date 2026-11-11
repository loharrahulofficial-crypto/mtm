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
	// Dissolve re-emits as halfblock (2 subrows per cell); a sextant
	// frame decodes to 3 subrows — bail to the intact frame instead of
	// mangling geometry.
	if r.H != 2*len(lines) {
		return lines
	}
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
			counts[rgbHex(uint8(r), uint8(g), uint8(b))]++
		}
		for _, mm := range sgrBg256.FindAllStringSubmatch(line, -1) {
			idx, ok := atoiSmall(mm[1])
			if !ok || idx < 0 || idx > 255 {
				continue
			}
			counts[xterm256Hex(uint8(idx))]++
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
var sgrBg256 = regexp.MustCompile(`48;5;([0-9]+)`)

// rgbHex formats channel bytes as "#%02x%02x%02x".
func rgbHex(r, g, b uint8) string {
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// xterm256Hex resolves a 256-palette index to "#rrggbb".
func xterm256Hex(idx uint8) string {
	r, g, b := xterm256RGB(idx)
	return rgbHex(r, g, b)
}

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

// Crossfade blends from→to by eased t ∈ [0,1] using per-cell color interpolation.
// Endpoints return the exact source lines. Instead of pixel-level blending,
// this approach interpolates colors at the cell level, preserving glyph/layout
// structure while smoothly transitioning colors.
func Crossfade(from, to []Line, t float64) []Line {
	if t <= 0 {
		return from
	}
	if t >= 1 {
		return to
	}

	// Eased progress (same as original)
	e := EaseInOutCubic(t)

	// Get base color from 'from' frame (consistent with original)
	base := baseHexFromLines(from)

	// Create rasters from both frames with the same base color
	ra := rasterFromLines(from, base)
	rb := rasterFromLines(to, baseHexFromLines(to))
	// The blend re-emits as halfblock, so it only works for halfblock or
	// quadrant geometry (2 subrows per cell). A sextant frame decodes to
	// 3 subrows — emit would produce nonsense line counts, so bail to the
	// exact target. Callers that need blending for sextant frames use the
	// TUI's per-cell color lerp instead.
	if ra.H != 2*len(from) || rb.H != 2*len(to) {
		return to
	}

	// Ensure both rasters have the same dimensions (fit to 'from' raster)
	if ra.W != rb.W || ra.H != rb.H {
		rb = fitRaster(rb, ra.W, ra.H)
	}

	// Create new raster for result
	out := Raster{W: ra.W, H: ra.H, Pix: make([]fpix, len(ra.Pix))}

	// For each cell position (ra.H/2 cell rows), interpolate colors between frames
	cellH := ra.H / 2
	for ty := 0; ty < cellH; ty++ {
		for tx := 0; tx < ra.W; tx++ {
			// Get top and bottom cell colors from 'from' frame
			top := ra.Pix[(ty*2)*ra.W+tx]
			bot := ra.Pix[(ty*2+1)*ra.W+tx]

			// Get top and bottom cell colors from 'to' frame
			topTo := rb.Pix[(ty*2)*ra.W+tx]
			botTo := rb.Pix[(ty*2+1)*ra.W+tx]

			// Interpolate colors for top cell
			newTopR := float64(top.r)*float64(1-e) + float64(topTo.r)*float64(e)
			newTopG := float64(top.g)*float64(1-e) + float64(topTo.g)*float64(e)
			newTopB := float64(top.b)*float64(1-e) + float64(topTo.b)*float64(e)

			// Interpolate colors for bottom cell
			newBotR := float64(bot.r)*float64(1-e) + float64(botTo.r)*float64(e)
			newBotG := float64(bot.g)*float64(1-e) + float64(botTo.g)*float64(e)
			newBotB := float64(bot.b)*float64(1-e) + float64(botTo.b)*float64(e)

			// Create new fpix with interpolated colors (fpix uses float32 channels)
			out.Pix[(ty*2)*ra.W+tx] = fpix{
				r: float32(newTopR),
				g: float32(newTopG),
				b: float32(newTopB),
				a: 1,
			}
			out.Pix[(ty*2+1)*ra.W+tx] = fpix{
				r: float32(newBotR),
				g: float32(newBotG),
				b: float32(newBotB),
				a: 1,
			}
		}
	}

	// Emit the result using the same logic as original
	return out.emitHalfblock(base, emitOpts{pairOnly: true})
}
