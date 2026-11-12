package ambient

import "math"

// wordmark.go — the splash wordmark paints through the raster (via
// SceneState.Wordmark) instead of being spliced in as text cells, so
// the aspect-corrected geometry applies to the letters exactly as it
// does to every other scene shape: a square bitmap pixel measures
// square on screen in every render mode. It also lets the dissolve
// tail wipe the letters with the scene, and lets the painter pick a
// letter color that contrasts the scene it sits on.

// WordmarkRect is one lit bitmap pixel in terminal-cell units. Y is
// float (letters rise in sub-row steps); W and H are the on-screen
// square pixel (1 cell × cellW/cellH). Alpha is the per-letter appear
// factor (pulse included).
type WordmarkRect struct {
	X, Y  float64
	W, H  float64
	Alpha float64
}

// WordmarkPaint carries the wordmark for one frame (nil for every
// scene that is not the splash).
type WordmarkPaint struct {
	Rects []WordmarkRect
	// BrightText/Base are the theme's text extremes — the painter picks
	// whichever contrasts the sampled scene at the wordmark's center
	// (blending toward white or black if neither is enough).
	BrightText, Base string
	// ShadowAlpha darkens the scene one subpixel under every lit pixel
	// (a drop shadow); 0 disables.
	ShadowAlpha float64
	// MinContrast is the required WCAG ratio between the letter color and
	// the sampled scene (3 for the wordmark).
	MinContrast float64
}

// applyWordmark paints the shadow then the letters. The letter color is
// chosen against the scene as it exists right now (after the scene paint
// and the scrims, before the dissolve), at the wordmark's center.
func applyWordmark(b *buffer, wm *WordmarkPaint, sxr, syr int) {
	if wm == nil || len(wm.Rects) == 0 {
		return
	}
	ss := float64(fSSV())
	fx := float64(sxr) * ss // scene x unit (1 cell) in raster pixels
	fy := float64(syr) * ss // scene y unit (1 cell) in raster pixels

	// sample the scene at the wordmark's center
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, r := range wm.Rects {
		if r.Alpha <= 0 {
			continue
		}
		minX = math.Min(minX, r.X*fx)
		minY = math.Min(minY, r.Y*fy)
		maxX = math.Max(maxX, (r.X+r.W)*fx)
		maxY = math.Max(maxY, (r.Y+r.H)*fy)
	}
	var bg [3]float32 = hexF32(wm.Base)
	if maxX > minX {
		cx := int((minX + maxX) / 2)
		cy := int((minY + maxY) / 2)
		bg = sampleWindow(b.fb, cx, cy, 1)
	}
	lr, lg, lb := letterColor(hexF32(wm.BrightText), hexF32(wm.Base), bg, wm.MinContrast)

	// shadow pass: every lit pixel darkens the scene one subpixel lower
	dy := 1.0 / float64(syr) // one y-subcell, in cell units
	base := hexF32(wm.Base)
	for _, r := range wm.Rects {
		if r.Alpha <= 0 {
			continue
		}
		a := wm.ShadowAlpha * r.Alpha
		if a <= 0 {
			continue
		}
		b.fb.fillRectF(fsubcells(r.X, sxr), fsubcells(r.Y+dy, syr), fsubcells(r.X+r.W, sxr), fsubcells(r.Y+r.H+dy, syr), base[0], base[1], base[2], a, opNormal)
	}
	// letters
	for _, r := range wm.Rects {
		if r.Alpha <= 0 {
			continue
		}
		b.fb.fillRectF(fsubcells(r.X, sxr), fsubcells(r.Y, syr), fsubcells(r.X+r.W, sxr), fsubcells(r.Y+r.H, syr), lr, lg, lb, r.Alpha, opNormal)
		// feed the ink raster so ASCII/braille show the letters
		inkRect(b.fb, fsubcells(r.X, sxr), fsubcells(r.Y, syr), fsubcells(r.X+r.W, sxr), fsubcells(r.Y+r.H, syr), float32(r.Alpha))
	}
}

// fsubcells converts cell units to sub-cell units for the given density.
func fsubcells(v float64, sub int) float64 { return v * float64(sub) }

// hexF32 parses #rrggbb into 0..1 floats (neutral gray on malformed input).
func hexF32(hex string) [3]float32 {
	r, g, b, ok := hexF(hex)
	if !ok {
		return [3]float32{0.8, 0.8, 0.8}
	}
	return [3]float32{r, g, b}
}

// sampleWindow averages a (2n+1)² box of raster pixels (clamped to the raster).
func sampleWindow(fb *fbuf, cx, cy, n int) [3]float32 {
	var acc [3]float64
	var count float64
	for y := cy - n; y <= cy+n; y++ {
		if y < 0 || y >= fb.ph {
			continue
		}
		for x := cx - n; x <= cx+n; x++ {
			if x < 0 || x >= fb.pw {
				continue
			}
			p := fb.px[y*fb.pw+x]
			acc[0] += float64(p.r)
			acc[1] += float64(p.g)
			acc[2] += float64(p.b)
			count++
		}
	}
	if count == 0 {
		return [3]float32{0, 0, 0}
	}
	return [3]float32{float32(acc[0] / count), float32(acc[1] / count), float32(acc[2] / count)}
}

func relLumF(c [3]float32) float64 {
	ch := func(v float32) float64 {
		f := float64(v)
		if f <= 0.03928 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(c[0]) + 0.7152*ch(c[1]) + 0.0722*ch(c[2])
}

func contrastF(a, b [3]float32) float64 {
	la, lb := relLumF(a), relLumF(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func mixF(a, b [3]float32, t float32) [3]float32 {
	return [3]float32{a[0] + (b[0]-a[0])*t, a[1] + (b[1]-a[1])*t, a[2] + (b[2]-a[2])*t}
}

// letterColor picks a letter color that contrasts the sampled scene:
// bright wins if it qualifies, then the base color, otherwise the
// nearer extreme blended toward white or black until it reaches min
// (bounded; returns the best found for pathological palettes).
func letterColor(bright, base, bg [3]float32, min float64) (float32, float32, float32) {
	if contrastF(bright, bg) >= min {
		return bright[0], bright[1], bright[2]
	}
	if contrastF(base, bg) >= min {
		return base[0], base[1], base[2]
	}
	// push toward the extreme farther from the scene's luminance
	var goal [3]float32
	if relLumF(bg) < 0.5 {
		goal = [3]float32{1, 1, 1}
	} else {
		goal = [3]float32{0, 0, 0}
	}
	start := mixF(bright, goal, 0.5)
	best, bestC := start, contrastF(start, bg)
	for i := 1; i <= 32; i++ {
		c := mixF(start, goal, float32(i)/32)
		cr := contrastF(c, bg)
		if cr > bestC {
			best, bestC = c, cr
		}
		if bestC >= min {
			best = c
			break
		}
	}
	return best[0], best[1], best[2]
}

// inkRect marks ink coverage for ASCII/braille glyph selection.
func inkRect(fb *fbuf, x0, y0, x1, y1 float64, alpha float32) {
	fss := float64(fSSV())
	ix0 := int(math.Floor(x0 * fss))
	ix1 := int(math.Ceil(x1 * fss))
	iy0 := int(math.Floor(y0 * fss))
	iy1 := int(math.Ceil(y1 * fss))
	for py := iy0; py < iy1; py++ {
		if py < 0 || py >= fb.ph {
			continue
		}
		for px := ix0; px < ix1; px++ {
			if px < 0 || px >= fb.pw {
				continue
			}
			fb.inkAt(px, py, alpha)
		}
	}
}
