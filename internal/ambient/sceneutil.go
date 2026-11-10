package ambient

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
