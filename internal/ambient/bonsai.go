package ambient

import (
	"math"
	"time"
)

// bonsaiScene grows a seeded procedural bonsai: a curved, tapering trunk
// with two-tone bark, branches bowing on curved spines, foliage as
// clustered soft blobs with lit and shadow sides, blossoms and falling
// petals, a mossy pot on a soft ground shadow, under a faint sky.
// Progress drives growth; idle shows the full swaying tree.
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

func (s *bonsaiScene) FramePixels(t time.Duration, w, h int, state SceneState) Raster {
	return renderRaster(s, t, w, h, state)
}

// branchSeg is one tapered piece of a branch polyline.
type branchSeg struct {
	x0, y0, x1, y1 float64
	w0, w1         float64 // width at each end (subcell units)
	depth          int
}

// buildBonsai recursively grows the tree from the root. Branches carry
// their own curvature (a per-branch bow), so the silhouette reads as
// grown, not straightedge.
func buildBonsai(seed int64, swI, shI int, growth float64) []branchSeg {
	r := newRNG(seed, 41)
	segs := []branchSeg{}
	life := 0.75 + float64(r.intn(1000))/1000.0 // seeded vigor 0.75..1.75
	maxDepth := int(3.2*growth*life) + 2
	trunkLen := float64(shI) * 0.34

	var grow func(x, y, angle, length, width float64, depth int)
	grow = func(x, y, angle, length, width float64, depth int) {
		if depth > maxDepth || length < 0.9 || width < 0.28 {
			return
		}
		// curved spine: bow the branch perpendicular to its heading
		bend := (r.float64() - 0.5) * 0.5 * (1 + float64(depth)*0.4)
		ex := x + math.Cos(angle)*length
		ey := y + math.Sin(angle)*length
		// split each branch into two tapered pieces with a mid kink
		midA := angle + bend*0.55
		midX := x + math.Cos(midA)*length*0.55
		midY := y + math.Sin(midA)*length*0.55
		segs = append(segs,
			branchSeg{x, y, midX, midY, width, width * 0.82, depth},
			branchSeg{midX, midY, ex, ey, width * 0.82, width * 0.62, depth},
		)
		// children: 2 side shoots, alternating sides
		nChild := 2
		if depth == 0 {
			nChild = 3 // trunk splits into three primary limbs
		}
		for c := 0; c < nChild; c++ {
			side := 1.0
			if c%2 == 1 {
				side = -1.0
			}
			spread := 0.42 + float64(depth)*0.10
			childAngle := angle + side*spread*(0.8+r.float64()*0.5)*life*0.55
			childLen := length * (0.66 + r.float64()*0.14)
			childWidth := width * 0.60
			grow(ex, ey, childAngle, childLen, childWidth, depth+1)
		}
	}
	lean := r.rangeF(-0.16, 0.16)
	jitter := (newRNG(seed, 43).float64() - 0.5) * 1.3
	rootY := float64(shI) - 1.6
	grow(float64(swI)/2+jitter, rootY, -math.Pi/2+lean, trunkLen, math.Min(float64(swI)*0.055, 4.6), 0)
	return segs
}

// revealed reports how many leading segments are drawn for growth ∈ [0,1].
func revealed(segs []branchSeg, growth float64) int {
	if growth >= 1 {
		return len(segs)
	}
	total := 0.0
	for _, s := range segs {
		total += math.Hypot(s.x1-s.x0, s.y1-s.y0)
	}
	want := clampf(growth, 0, 1) * total
	acc := 0.0
	n := 0
	for i, s := range segs {
		acc += math.Hypot(s.x1-s.x0, s.y1-s.y0)
		n = i + 1
		if acc >= want {
			break
		}
	}
	return n
}

func (s *bonsaiScene) paint(b *buffer, p *paletteModel, t time.Duration, w, h int, state SceneState) {
	W := float64(sw(b))
	H := float64(sh(b))
	base := state.Palette.Base
	bright := state.Palette.BrightText
	rmp := BuildRamps(state.Palette)
	ts := tSecF(t)

	// faint sky behind the tree
	b.fillGradientHex(0, 0, W, H, rmp.SkyTop, rmp.SkyHorizon, 1, opNormal)

	// growth: idle shows the full tree; pomodoro grows it
	grow := state.Progress
	if state.Phase == PhaseIdle {
		grow = 1
	}
	grow = clampf(grow, 0, 1)
	breathe := 1.0
	if state.Phase == PhaseIdle {
		breathe = 0.55 + 0.45*math.Sin(ts*0.9) // slow breathing sway at rest
	}

	// ground shadow under the pot
	potW := math.Min(W*0.24, 16)
	potCX := W / 2
	potCY := H - 2.6
	b.fillEllipseHexF(potCX, potCY+1.2, potW*0.78, H*0.035, blendHex(base, "#000000", 0.35), 0.55, opNormal)

	// --- the tree
	segs := buildBonsai(state.Seed, sw(b), sh(b), 0.75+grow*0.9)
	n := revealed(segs, grow)
	swayS := math.Sin(ts*0.75) * reducedAmp(state, 0.55) * breathe

	b.beginLayer(layerLand, 0, 0, sw(b), sh(b))
	for i, seg := range segs {
		if i >= n {
			break
		}
		// sway: tips travel, base barely moves
		heightFrac := 1 - seg.y0/H
		off := swayS * heightFrac * heightFrac * 2.6
		dark := blendHex(base, state.Palette.Brown, 0.62)
		light := blendHex(state.Palette.Brown, bright, 0.28)
		// bark: dark body stroke, then a thin lit edge above it
		b.strokeHexF(seg.x0+off, seg.y0, seg.x1+off, seg.y1, seg.w0, dark, 1, opNormal)
		if seg.depth <= 2 {
			b.strokeHexF(seg.x0+off-seg.w0*0.18, seg.y0-0.15, seg.x1+off-seg.w1*0.18, seg.y1-0.15, seg.w0*0.34, light, 0.8, opNormal)
		}
	}
	b.endLayer()

	// --- foliage: clustered blobs on the branch tips, light from upper left
	b.beginLayer(layerFoliage, 0, 0, sw(b), sh(b))
	folAlpha := clampf((grow-0.52)/0.34, 0, 1)
	if state.Phase == PhaseIdle {
		folAlpha = 1
	}
	for i, seg := range segs {
		if i >= n {
			break
		}
		// a tip is a segment whose children were not generated (depth beyond cap)
		isTip := true
		for j := range segs {
			if j != i && segs[j].depth == seg.depth+1 {
				isTip = false
				break
			}
		}
		if !isTip || seg.depth < 2 {
			continue
		}
		heightFrac := 1 - seg.y1/H
		off := swayS * heightFrac * heightFrac * 2.6
		lr := newRNG(state.Seed, uint64(i)*61+7)
		cx, cy := seg.x1+off, seg.y1
		size := 1.5 + lr.float64()*1.6 + float64(seg.depth-2)*0.4
		// shadow blob (lower right), lit blob (upper left), highlight dot
		b.fillCircleHexF(cx+size*0.22, cy+size*0.22, size, blendHex(state.Palette.Success, base, 0.55), folAlpha, opNormal)
		b.fillCircleHexF(cx-size*0.18, cy-size*0.18, size*0.78, blendHex(state.Palette.Success, bright, 0.22), folAlpha, opNormal)
		b.fillCircleHexF(cx-size*0.32, cy-size*0.32, size*0.34, blendHex(state.Palette.Success, bright, 0.48), folAlpha*0.9, opNormal)
		// occasional blossom dot once the tree is mature
		if grow > 0.78 && lr.float64() < 0.38 {
			b.fillCircleHexF(cx+size*0.1, cy+size*0.05, size*0.24, blendHex(state.Palette.Magenta, bright, 0.3), folAlpha, opNormal)
		}
	}
	b.endLayer()

	// --- falling petals once the tree bears blossom
	if grow > 0.85 || state.Phase == PhaseIdle {
		for i := 0; i < 6; i++ {
			slot := int64(wrapT(t, 24*time.Hour)/time.Second)/3 + int64(i)
			frac := float64(wrapT(t, 24*time.Hour)/time.Second)/3.0 - float64(slot)
			if frac < 0 || frac >= 1 {
				continue
			}
			pr := newRNG(state.Seed^0xbe17, uint64(i)*131+77)
			px := pr.rangeF(0.2, 0.8) * W
			py := frac * (H * 0.94)
			sway := math.Sin(frac*7+float64(i)*1.3) * 1.6
			col := blendHex(state.Palette.Magenta, bright, 0.25+frac*0.2)
			b.fillCircleHexF(px+sway, py, 0.32, col, 0.9*(1-frac*0.5), opNormal)
		}
	}

	// --- the pot: mossy, two tapered tiers on a stone foot
	b.beginLayer(layerLand, int(potCX-potW), int(potCY), int(potCX+potW+1), sh(b))
	stone := blendHex(base, state.Palette.SurfaceStrong, 0.32)
	potC := blendHex(base, state.Palette.Brown, 0.42)
	b.fillEllipseHexF(potCX, potCY, potW, potW*0.30, stone, 1, opNormal) // stone foot
	b.fillRectHexF(potCX-potW*0.78, potCY-potW*0.34, potCX+potW*0.78, potCY-potW*0.05, potC, 1, opNormal)
	b.fillRectHexF(potCX-potW*0.94, potCY-potW*0.46, potCX+potW*0.94, potCY-potW*0.30, blendHex(potC, bright, 0.14), 1, opNormal)
	// moss: dark dots on the rim
	moss := cachedPerlin(state.Seed, 733)
	for x := potCX - potW*0.9; x < potCX+potW*0.9; x += 0.55 {
		if moss.At(x*0.4, 3.7) > 0.1 {
			b.fillCircleHexF(x, potCY-potW*0.36, 0.28, blendHex(state.Palette.Success, base, 0.55), 1, opNormal)
		}
	}
	b.endLayer()
}
