package ambient

import (
	"strings"
	"time"
)

// paintable is implemented by every scene: it paints into the sub-raster.
// Frame composes paint + render; Reward adds the sparkle overlay.
type paintable interface {
	paint(b *buffer, p *paletteModel, t time.Duration, w, h int, state SceneState)
}

// newFrameBuffer paints one frame and returns the buffer for inspection
// (the golden/invariant tests read the raster, not just the lines).
func newFrameBuffer(s paintable, mode string, t time.Duration, w, h int, state SceneState) *buffer {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	p := paletteFor(state.Palette)
	b := newBuffer(mode, w, h, p)
	b.dither = state.Dither
	s.paint(b, p, t, w, h, state)
	b.palette = p
	return b
}

// renderFrame builds one line-set for the frame (reward adds the sparkle).
func renderFrame(s paintable, mode string, t time.Duration, w, h int, state SceneState, reward bool) []Line {
	p := paletteFor(state.Palette)
	b := newFrameBuffer(s, mode, t, w, h, state)
	b.palette = p
	if reward {
		paintReward(b, t, state.Seed)
	}
	return b.render(p, w, h)
}

// New returns the scene named name (unknown names fall back to night),
// rendered with the given mode (auto resolves to halfblock).
func New(name, mode string) Scene {
	mode = resolveMode(mode)
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "campfire":
		return &campfireScene{mode: mode}
	case "snow":
		return &snowScene{mode: mode}
	case "koi":
		return &koiScene{mode: mode}
	case "rain":
		return &rainScene{mode: mode}
	case "ocean":
		return &oceanScene{mode: mode}
	case "bonsai":
		return &bonsaiScene{mode: mode}
	case "night":
		return &nightScene{mode: mode}
	default:
		return &nightScene{mode: mode}
	}
}

// Registered lists every scene name in stable order.
func Registered() []string {
	return []string{"night", "bonsai", "ocean", "rain", "koi", "snow", "campfire"}
}

// paintReward draws staggered cross-shaped bursts. Multi-cell bursts read
// even in halfblock mode (single 1-cell sparks drown in dense scenes).
func paintReward(b *buffer, t time.Duration, seed int64) {
	progress := float64(wrapT(t, 1500*time.Millisecond)) / float64(1500*time.Millisecond)
	rng := newRNG(seed^0x5eed5eed, 99)
	p := b.palette
	for i := 0; i < 6; i++ {
		phase := progress - float64(i)*0.055
		if phase <= 0.04 || phase >= 0.92 {
			continue
		}
		cx := int(rng.rangeF(0.16, 0.84) * float64(b.sw-1))
		cy := int(rng.rangeF(0.12, 0.62) * float64(b.sh-1))
		arm := 2 + int(phase*5) // 2..6 cells reach at the peak
		if phase > 0.66 {
			arm-- // collapse as the spark dies
		}
		col := cBrightText
		if phase > 0.66 {
			col = cWarning // embers cooling
		}
		b.mark(cx, cy, '✦', col)
		b.mark(cx+1, cy, '✦', col)
		b.mark(cx, cy+1, '✦', col)
		b.mark(cx+1, cy+1, '✦', col)
		for d := 2; d <= arm; d++ {
			fade := float64(d)/float64(maxInt(arm, 1)+1) + phase*0.25
			ci := mixIndex(p, col, p.base, clampf(fade, 0, 1)*0.75)
			b.mark(cx+d, cy, '·', ci)
			b.mark(cx-d, cy, '·', ci)
			b.mark(cx, cy+d, '·', ci)
			b.mark(cx, cy-d, '·', ci)
		}
	}
}
