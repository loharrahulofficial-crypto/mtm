package mascot

import (
	"math"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/ambient"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

// anim.go — the living, time-driven parts of Mo. Every motion is a pure
// function of state (pose, elapsed) evaluated at a frame: no frame counters,
// no randomness beyond tiny deterministic hashes, so the same state renders
// identically on every terminal and unit tests are stable.

type State struct {
	Pose     Pose
	Since    time.Duration // time since the pose became current
	Prev     Pose          // pose before the current one; -1 = none
	Speaking bool          // while the dialogue typewriter is revealing
	Frozen   bool          // reduced motion: static settled frame
}

const (
	popDur        = 220 * time.Millisecond // elastic pose pop
	popCrossDur   = 90 * time.Millisecond  // old/new crossfade
	twinkleDur    = 2400 * time.Millisecond
	chestCycleDur = 1100 * time.Millisecond
)

func fract(x float64) float64 {
	f := math.Mod(x, 1)
	if f < 0 {
		f += 1
	}
	return f
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func periodPhase(t time.Duration, period time.Duration) float64 {
	return fract(float64(t) / float64(period))
}

// popShift yields the vertical spring offset of the pop: starts 2px lower,
// overshoots ~1px up, settles to 0 at k = 1.
func popShift(k float64) int {
	return int(math.Round(2 * (1 - ambient.EaseOutBack(clampF(k, 0, 1)))))
}

// Animate returns the raster of pose s: pop transition, blink, lip-sync,
// antenna twinkle, chest cycle, pose loops — all driven by s.Since.
func Animate(s State, pal theme.Palette) ambient.Raster {
	if s.Frozen {
		return Render(s.Pose, pal)
	}
	w, h, pix := Grid(s.Pose)
	g := copyGrid(w, h, pix)
	poseEffects(g, s, s.Since)
	r := gridToRaster(w, h, g, pal)
	if s.Prev >= 0 {
		tt := float64(s.Since) / float64(popDur)
		if tt > 1 {
			tt = 1
		}
		w2, h2, pix2 := Grid(s.Prev)
		old := gridToRaster(w2, h2, copyGrid(w2, h2, pix2), pal)
		shift := popShift(float64(s.Since) / float64(popDur))
		r = ambient.BlendRasters(translateRaster(old, 0, shift), translateRaster(r, 0, shift), tt)
	}
	return r
}

func gridToRaster(w, h int, g [][]uint8, pal theme.Palette) ambient.Raster {
	roles := themeRoles(pal)
	return ambient.RasterFromPixels(w, h, func(x, y int) (fr, fg, fb, fa float32) {
		if v := g[y][x]; v != 0 && int(v) < len(roles) {
			return join3(hexF(roles[v]))
		}
		return 0, 0, 0, 0
	})
}

func copyGrid(w, h int, pix []uint8) [][]uint8 {
	g := make([][]uint8, h)
	for y := 0; y < h; y++ {
		g[y] = append([]uint8(nil), pix[y*w:(y+1)*w]...)
	}
	return g
}

func translateRaster(r ambient.Raster, dx, dy int) ambient.Raster {
	return ambient.RasterFromPixels(r.W, r.H, func(x, y int) (float32, float32, float32, float32) {
		sx, sy := x-dx, y-dy
		if sx < 0 || sx >= r.W || sy < 0 || sy >= r.H {
			return 0, 0, 0, 0
		}
		r0, g0, b0 := r.Pixel(sx, sy)
		return float32(r0) / 255, float32(g0) / 255, float32(b0) / 255, 1
	})
}

// translateGridV shifts the whole grid up by dy pixels (positive = up);
// a new row's source is old row y+dy.
func translateGridV(g [][]uint8, dy int) {
	if dy == 0 || len(g) == 0 {
		return
	}
	w := len(g[0])
	h := len(g)
	tmp := make([][]uint8, h)
	for y := 0; y < h; y++ {
		src := y + dy
		if src >= 0 && src < h {
			tmp[y] = g[src]
		} else {
			tmp[y] = make([]uint8, w)
		}
	}
	copy(g, tmp)
}

func clearRow(row []uint8) {
	for i := range row {
		row[i] = 0
	}
}

func setRect(g [][]uint8, r R, v uint8) {
	for y := r.Y0; y < r.Y1 && y < len(g); y++ {
		for x := r.X0; x < r.X1 && x < len(g[y]); x++ {
			g[y][x] = v
		}
	}
}

func setRectAt(g [][]uint8, r R, v uint8) { setRect(g, r, v) }

func translateRectUp(g [][]uint8, r R, up int) {
	for y := r.Y0; y < r.Y1; y++ {
		for x := r.X0; x < r.X1; x++ {
			if g[y][x] != 0 && y-up >= 0 {
				g[y-up][x] = g[y][x]
				g[y][x] = 0
			}
		}
	}
}

func translateRectH(g [][]uint8, r R, d int) {
	if d == 0 {
		return
	}
	for y := r.Y0; y < r.Y1; y++ {
		for x := r.X0; x < r.X1; x++ {
			if g[y][x] != 0 {
				if g[y][x+d] == 0 {
					g[y][x+d] = g[y][x]
				}
				g[y][x] = 0
			}
		}
	}
}

// ---------------------------------------------------------------- effects

func poseEffects(g [][]uint8, s State, t time.Duration) {
	p := s.Pose
	a := Anchor(p)

	switch p {
	case Wave: // gentle breath + hand wave on the raised arm
		ph := periodPhase(t, 2400*time.Millisecond)
		translateGridV(g, int(math.Round(ambient.EaseInOutSine(ph)*1.4)))
		if arm := findArm(g, a.Head); !arm.Empty() {
			up := 0
			if periodPhase(t, 160*time.Millisecond) < 0.5 {
				up = 1
			}
			translateRectUp(g, arm, up)
		}
	case Point: // 1px horizontal nudge toward the highlighted pane
		if arm := findArm(g, a.Head); !arm.Empty() {
			d := 0
			if periodPhase(t, 1400*time.Millisecond) < 0.5 {
				d = 1
			}
			translateRectH(g, arm, d)
		}
	case Keycap, Tasks, Idea, Idle, Think, Alert: // gentle breathing + pose specifics
		ph := periodPhase(t, 1600*time.Millisecond)
		translateGridV(g, int(math.Round(ambient.EaseInOutSine(ph)*1)))
		switch p {
		case Alert: // quick horizontal micro-shake
			if t < 300*time.Millisecond {
				frac := float64(t) / float64(30*time.Millisecond)
				dx := int(math.Round(0.7 * (1 - periodPhase(t, 300*time.Millisecond)) * math.Sin(frac*10)))
				translateRectH(g, a.Head, dx)
			}
		case Idea: // warm pulsing lightbulb
			if bulb := findBulb(g); !bulb.Empty() {
				setRect(g, bulb, 6)
			}
		}
	case Pomo: // steady 1s pulse around the tomato rim
		if tm := findTomato(g); !tm.Empty() {
			if clampF(periodPhase(t, 1*time.Second), 0.4, 0.6) > 0.5 {
				setRect(g, tm, 9)
			} else {
				setRect(g, tm, 10)
			}
		}
	case Celebrate: // joyous hop + orbiting sparkles
		k := clampF(float64(t)/float64(850*time.Millisecond), 0, 1)
		d := int(math.Round(2 * (1 - ambient.EaseOutCubic(k))))
		translateGridV(g, d)
		if arm := findArm(g, a.Head); !arm.Empty() {
			const n = 6
			for i := 0; i < n; i++ {
				theta := float64(i)*2*math.Pi/n + fract(float64(s.Since)/1300*1.8)
				x := a.Head.X0 + a.Head.Dx()/2 + int(4*math.Cos(theta))
				y := a.Head.Y0 + a.Head.Dy()/2 - int(4*math.Sin(theta))
				if x >= 0 && x < len(g[0]) && y >= 0 && y < len(g) {
					g[y][x] = 7 // gold
				}
			}
		}
	case Sleep: // slow breath + drifting z glyphs
		translateGridV(g, int(math.Round(ambient.EaseInOutSine(periodPhase(t, 2800*time.Millisecond))*1.2)))
		for _, z := range zGlyphs(g) {
			translateRectUp(g, z, int(3*fract(float64(t)/float64(1400*time.Millisecond))))
		}
	}

	// --- heart-beats (all poses) ---
	if p != Celebrate && p != Sleep { // never blink on Celebrate/Sleep
		for _, eye := range a.Eyes {
			if !eye.Empty() && blink(t, p) {
				setRect(g, eye, 6) // bright row — blink collapsed the bars
			}
		}
	}
	if !a.Mouth.Empty() && s.Speaking {
		lipSync(g, a.Mouth)
	}
	if !a.Star.Empty() {
		k := 0.5 + 0.5*ambient.EaseInOutSine(periodPhase(t, twinkleDur))
		setRectAt(g, a.Star, uint8(blendRole(7, 6, k)))
	}
	chestCycle(g, a.Chest, t)
}

// -------------------------------------------------------------- heart-beats

func blink(t time.Duration, p Pose) bool {
	if p == Celebrate || p == Sleep { // never blink on Celebrate/Sleep
		return false
	}
	lo, hi := 3400*time.Millisecond, 4600*time.Millisecond
	period := lo + time.Duration(600*fract(float64(t)/float64(hi)))
	return fract(float64(t)/float64(period)) > 0.92
}

func lipSync(g [][]uint8, m R) {
	setRectAt(g, m, 6)
}

func chestCycle(g [][]uint8, chest R, t time.Duration) {
	if chest.Empty() {
		return
	}
	var dots []R
	for y := chest.Y0; y < chest.Y1; y++ {
		for x := chest.X0; x < chest.X1; x++ {
			if g[y][x] == 8 {
				dots = append(dots, R{x, y, x + 1, y + 1})
			}
		}
	}
	if len(dots) < 3 {
		return
	}
	// Cycle which dot is shaded brighter.
	idx := int(fract(float64(t)/float64(chestCycleDur)) * float64(len(dots)))
	for i, d := range dots {
		if i == idx {
			setRectAt(g, d, 6)
		}
	}
}

// ---------------------------------------------------------------- component scan

func components(g [][]uint8, role uint8) []R {
	var out []R
	seen := make([][]bool, len(g))
	for i := range seen {
		seen[i] = make([]bool, len(g[i]))
	}
	h, w := len(g), len(g[0])
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if seen[y][x] || g[y][x] != role {
				continue
			}
			r := R{x, y, x + 1, y + 1}
			stack := []int{y*w + x}
			seen[y][x] = true
			for len(stack) > 0 {
				i := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				cy, cx := i/w, i%w
				if cx > r.X1 {
					r.X1 = cx + 1
				}
				if cx < r.X0 {
					r.X0 = cx
				}
				if cy > r.Y1 {
					r.Y1 = cy + 1
				}
				if cy < r.Y0 {
					r.Y0 = cy
				}
				for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					nx, ny := cx+d[0], cy+d[1]
					if nx < 0 || ny < 0 || nx >= w || ny >= h || seen[ny][nx] || g[ny][nx] != role {
						continue
					}
					seen[ny][nx] = true
					stack = append(stack, ny*w+nx)
				}
			}
			out = append(out, r)
		}
	}
	return out
}

func findArm(g [][]uint8, head R) R {
	for _, c := range components(g, 5) { // cream
		if c.Y0 < head.Y0 && c.X0 >= head.X0-1 && c.X1 <= head.X1+2 {
			if !c.Inside(head) && head.Dy()-c.Dy() <= 4 {
				return c
			}
		}
	}
	return R{}
}

func findBulb(g [][]uint8) R {
	for _, c := range components(g, 11) {
		if c.Y0 > len(g)/2-2 && c.Y1 < len(g) {
			return c
		}
	}
	return R{}
}

func findTomato(g [][]uint8) R {
	for _, c := range components(g, 9) {
		return c
	}
	return R{}
}

func zGlyphs(g [][]uint8) []R {
	var out []R
	for _, c := range components(g, 14) { // muted
		if c.Y0 >= len(g)/2 {
			out = append(out, c)
		}
	}
	for _, c := range components(g, 11) { // yellow
		if c.Y0 >= len(g)*3/4 && c.Dy() <= 2 {
			out = append(out, c)
		}
	}
	return out
}

func blendRole(a, b int, k float64) int {
	return int(math.Round(float64(a)*(1-k) + float64(b)*k))
}
