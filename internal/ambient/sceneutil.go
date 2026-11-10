package ambient

import "math"

// sceneutil.go — shared low-level helpers for the scenes (kept small;
// each scene inlines its own composition).

func sw(b *buffer) int { return b.sw }
func sh(b *buffer) int { return b.sh }

// star is one deterministic star, shared by scenes that want a starfield.
type star struct {
	x, y  int
	mag   float64
	phase float64
	speed float64
	drift float64 // parallax drift per minute
	layer int
}

// sceneStars builds a deterministic starfield (kept for the scenes that
// reuse it until they are rewritten).
func sceneStars(seed int64, w, h int) []star {
	r := newRNG(seed, 17)
	n := clampf(float64(w*h)/9, 18, 110)
	out := make([]star, 0, int(n))
	for i := 0; i < int(n); i++ {
		layer := r.intn(3) // 0 far, 1 mid, 2 near
		mag := r.float64()
		mag = mag * mag // bias toward dim stars
		out = append(out, star{
			x:     r.intn(maxInt(w, 1)),
			y:     r.intn(maxInt(h*7/10, 1)),
			mag:   mag,
			phase: r.rangeF(0, 2*3.141592653589793),
			speed: r.rangeF(0.5, 1.6),
			drift: []float64{0.004, 0.011, 0.021}[layer],
			layer: layer,
		})
	}
	return out
}

// paintDisc fills a disc with a color; ringOnly draws only the outline.
func paintDisc(b *buffer, cx, cy, r int, ci int, filled bool) {
	for dy := -r - 1; dy <= r+1; dy++ {
		for dx := -r - 1; dx <= r+1; dx++ {
			d2 := float64(dx*dx+dy*dy) / float64(r*r)
			if filled {
				if d2 <= 1 {
					b.glow(cx+dx, cy+dy, ci)
				}
			} else {
				if d2 > 0.72 && d2 <= 1.42 {
					b.glow(cx+dx, cy+dy, ci)
				}
			}
		}
	}
}

// paintMoon draws the moon in its true phase (bright limb, dark disc).
func paintMoon(b *buffer, cx, cy, r int, phase float64, p *paletteModel) {
	term := math.Cos(2*math.Pi*phase) * float64(r)
	waxing := phase < 0.5
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			d2 := float64(dx*dx+dy*dy) / float64(r*r)
			if d2 > 1 {
				continue
			}
			lit := false
			if waxing {
				lit = float64(dx) > -term
			} else {
				lit = float64(dx) < term
			}
			shade := math.Sqrt(d2)
			var ci int
			if lit {
				ci = mixIndex(p, cBrightText, p.base, 0.15+0.3*shade)
			} else {
				ci = mixIndex(p, cSurface, p.base, 0.35+0.4*shade)
			}
			b.glow(cx+dx, cy+dy, ci)
		}
	}
}

// reducedAmp damps an oscillation amplitude hard under reduced motion
// (still visible, but a quiet still rather than a lively scene).
func reducedAmp(state SceneState, full float64) float64 {
	if state.ReducedMotion {
		return full * 0.15
	}
	return full
}
