package ambient

import "math"

// perlin is a small seeded Perlin noise (2D + quintic fade, 8 lattice
// gradient directions). Enough for aurora ribbons and fog.
type perlin struct{ perm [512]uint8 }

func newPerlin(seed int64, stream uint64) *perlin {
	r := newRNG(seed, stream)
	p := &perlin{}
	for i := 0; i < 256; i++ {
		p.perm[i] = uint8(i)
	}
	for i := 255; i > 0; i-- {
		j := r.intn(i + 1)
		p.perm[i], p.perm[j] = p.perm[j], p.perm[i]
	}
	for i := 0; i < 256; i++ {
		p.perm[256+i] = p.perm[i]
	}
	return p
}

func fade(t float64) float64       { return t * t * t * (t*(t*6-15) + 10) }
func lerp(a, b, t float64) float64 { return a + (b-a)*t }

func gradAt(hash uint8, x, y float64) float64 {
	switch hash & 7 {
	case 0:
		return x + y
	case 1:
		return x - y
	case 2:
		return -x + y
	case 3:
		return -x - y
	case 4:
		return x
	case 5:
		return -x
	case 6:
		return y
	default:
		return -y
	}
}

// At samples 2D noise at (x, y); the result is roughly in [-1, 1].
func (p *perlin) At(x, y float64) float64 {
	xf := math.Floor(x)
	yf := math.Floor(y)
	x0 := int(xf) & 255
	y0 := int(yf) & 255
	x1 := (x0 + 1) & 255
	y1 := (y0 + 1) & 255
	dx := x - xf
	dy := y - yf
	u := fade(dx)
	v := fade(dy)
	h00 := p.perm[(p.perm[x0]+uint8(y0))&255]
	h10 := p.perm[(p.perm[x1]+uint8(y0))&255]
	h01 := p.perm[(p.perm[x0]+uint8(y1))&255]
	h11 := p.perm[(p.perm[x1]+uint8(y1))&255]
	dot := func(h uint8, dx, dy float64) float64 { return gradAt(h, dx, dy) }
	n00 := dot(h00, dx, dy)
	n10 := dot(h10, dx-1, dy)
	n01 := dot(h01, dx, dy-1)
	n11 := dot(h11, dx-1, dy-1)
	return lerp(lerp(n00, n10, u), lerp(n01, n11, u), v)
}

// FBM sums octaves of noise; the result is smooth, fractal, and stable.
func (p *perlin) FBM(x, y float64, octaves int) float64 {
	sum := 0.0
	amp := 1.0
	freq := 1.0
	total := 0.0
	for o := 0; o < octaves; o++ {
		sum += p.At(x*freq, y*freq) * amp
		total += amp
		amp *= 0.5
		freq *= 2.1
	}
	return sum / total
}

// smoothed applies a parabolic temporal filter around t so noise-driven
// values change smoothly instead of flickering frame to frame.
func smoothed(noise func(t float64) float64, t, step float64) float64 {
	return (2*noise(t) + noise(t+step) + noise(t-step)) / 4
}
