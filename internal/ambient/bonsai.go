package ambient

import (
	"math"
	"time"
)

// bonsaiScene is a seeded procedural tree, reimplemented from scratch:
// recursive branching with a "life" parameter (seeded vigor) and a growth
// multiplier that scales how far the ramification runs before it stops.
// Growth is driven by Progress — the seedling firms into a full tree over
// a pomodoro. Idle mode shows the swaying full tree.
type bonsaiScene struct {
	mode string
}

func (s *bonsaiScene) Name() string { return "bonsai" }

func (s *bonsaiScene) Frame(t time.Duration, w, h int, state SceneState) []Line {
	return renderFrame(s, s.mode, t, w, h, state, false)
}

func (s *bonsaiScene) Reward(t time.Duration, w, h int, state SceneState) []Line {
	return renderFrame(s, s.mode, t, w, h, state, true)
}

// branch is one generated segment of the tree.
type branch struct {
	x0, y0, x1, y1 float64
	thick          int
	depth          int
}

func buildTree(seed int64, sw, sh int, mult float64) []branch {
	r := newRNG(seed, 41)
	life := 0.65 + float64(r.intn(1000))/1000.0 // seeded vigor 0.65..1.65
	segs := []branch{}
	// seeded ramification: recursive from the root; multiplier scales
	// generation depth and branch count.
	maxDepth := int(3.5 * mult * life)
	if maxDepth < 2 {
		maxDepth = 2
	}
	var grow func(x, y, angle, length float64, thick, depth int)
	grow = func(x, y, angle, length float64, thick, depth int) {
		if depth > maxDepth || length < 1.1 {
			return
		}
		ex := x + math.Cos(angle)*length
		ey := y + math.Sin(angle)*length
		segs = append(segs, branch{x0: x, y0: y, x1: ex, y1: ey, thick: thick, depth: depth})
		// side shoots: 2..4 branches, seeded angle and length
		nChild := 2 + r.intn(2)
		for c := 0; c < nChild; c++ {
			side := 1.0
			if c%2 == 1 {
				side = -1
			}
			childAngle := angle + side*r.rangeF(0.35, 0.85)*mult*life
			childLen := length * r.rangeF(0.62, 0.78)
			childThick := thick - 1
			if childThick < 1 {
				childThick = 1
			}
			grow(ex, ey, childAngle, childLen, childThick, depth+1)
		}
	}
	// trunk: slight deterministic lean, rooted at the base center
	lean := r.rangeF(-0.22, 0.22)
	jitter := (newRNG(seed, 43).float64() - 0.5) * 1.2
	rootY := float64(sh - 1)
	grow(float64(sw)/2+jitter, rootY, -math.Pi/2+lean, float64(sh)*0.38, maxInt(sw/9, 3), 0)
	return segs
}

func (s *bonsaiScene) paint(b *buffer, p *paletteModel, t time.Duration, w, h int, state SceneState) {
	// ground: quiet dark soil line
	ground := sh(b) - 1
	b.beginLayer(layerLand, 0, ground, sw(b), sh(b))
	for x := 0; x < sw(b); x++ {
		b.mark(x, ground, '_', mixIndex(p, cSurfaceStrong, p.base, 0.35))
	}

	b.endLayer()

	grow := state.Progress
	if state.Phase == PhaseIdle {
		grow = 1 // the full tree sways at rest
	}
	grow = clampf(grow, 0, 1)

	b.beginLayer(layerFoliage, 0, 0, sw(b), ground+1)
	segs := buildTree(state.Seed, sw(b), sh(b), 0.85+grow*0.75)
	total := len(segs)
	reveal := grow * float64(total)

	tSec := float64(wrapT(t, 8*time.Minute)) / float64(time.Second)
	sway := math.Sin(tSec*0.8) * 1.0

	for i, seg := range segs {
		if float64(i) >= reveal {
			break
		}
		frac := 1.0
		if float64(i+1) > reveal {
			frac = reveal - float64(i)
		}
		// Sway grows with height (trunk anchored, tips move)
		heightFrac := 1 - (seg.y0 / float64(maxInt(sh(b)-1, 1)))
		off := sway * heightFrac * heightFrac * 1.6
		x0 := seg.x0 + off
		x1 := seg.x1 + off
		// trunk tones: bark brown, tips blending toward base
		ci := mixIndex(p, cBrown, p.base, float64(seg.depth)*0.13)
		drawBonsaiLine(b, x0, seg.y0, x0+(x1-x0)*frac, seg.y0+(seg.y1-seg.y0)*frac, seg.thick, ci)
	}

	// Canopy: leaves cluster on the terminal segments once they exist.
	for i, seg := range segs {
		if float64(i) >= reveal {
			break
		}
		isTip := true
		for j := range segs {
			if j != i && segs[j].depth == seg.depth+1 {
				isTip = false
				break
			}
		}
		if !isTip {
			continue
		}
		heightFrac := 1 - (seg.y1 / float64(maxInt(sh(b)-1, 1)))
		off := sway * heightFrac * heightFrac * 1.6
		lr := newRNG(state.Seed, uint64(i)*61+7)
		size := 2 + lr.intn(2)
		for k := 0; k < size; k++ {
			dx := lr.rangeF(-1.5, 1.5)
			dy := lr.rangeF(-1.0, 1.0)
			ci := mixIndex(p, cSuccess, p.base, 0.1+lr.float64()*0.5)
			if grow > 0.72 && lr.float64() < 0.16 {
				ci = mixIndex(p, cMagenta, p.base, lr.float64()*0.45) // blossom dots
			}
			b.mark(int(seg.x1+off+dx), int(seg.y1+dy), 'o', ci)
		}
		_ = heightFrac
	}

	b.endLayer()

	// Falling petals once the tree bears blossom
	if grow > 0.85 {
		petals := 6
		for i := 0; i < petals; i++ {
			slot := int64(wrapT(t, 24*time.Hour)/time.Second)/3 + int64(i)
			frac := float64(wrapT(t, 24*time.Hour)/time.Second)/3.0 - float64(slot)
			if frac < 0 || frac >= 1 {
				continue
			}
			pr := newRNG(state.Seed^0xbe17, uint64(i)*131+77)
			px := pr.rangeF(0.2, 0.8) * float64(sw(b))
			py := frac*float64(sh(b)) + pr.rangeF(-2, 2)
			ci := mixIndex(p, cOrange, p.base, 0.25+frac*0.4)
			b.mark(int(px+math.Sin(frac*7+float64(i))*1.4), int(py), ',', ci)
		}
	}
}

func drawBonsaiLine(b *buffer, x0, y0, x1, y1 float64, thick int, ci int) {
	dx := x1 - x0
	dy := y1 - y0
	length := math.Hypot(dx, dy)
	if length == 0 {
		length = 1
	}
	steps := int(math.Max(math.Abs(dx), math.Abs(dy)))
	if steps == 0 {
		steps = 1
	}
	nx, ny := -dy/length, dx/length // unit normal, perpendicular to the segment
	half := thick / 2
	for i := 0; i <= steps; i++ {
		f := float64(i) / float64(steps)
		x := x0 + dx*f
		y := y0 + dy*f
		for d := -half; d <= half; d++ {
			b.glow(int(x+nx*float64(d)+0.5), int(y+ny*float64(d)+0.5), ci)
		}
		b.mark(int(x+0.5), int(y+0.5), '█', ci)
	}
}
