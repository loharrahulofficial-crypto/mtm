// Package mascot draws the desktop robot that runs between the user and the
// mtm guide. Poses are stored in generated form (poses_gen.go, baked by
// cmd/moextract from the AI-generated sprite sheet); Render re-colors them
// onto any theme palette at runtime, keeping light themes and live theme
// reload working.
package mascot

import (
	"strconv"
	"strings"

	"github.com/loharrahulofficial-crypto/mtm/internal/ambient"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

// Pose indexes posePixels (use the constants Wave..Sleep below).
type Pose = int

const (
	Wave Pose = iota
	Point
	Search
	Keycap
	Pomo
	Celebrate
	Tasks
	Idea
	Idle
	Think
	Alert
	Sleep
)

func (r R) Dx() int { return r.X1 - r.X0 }
func (r R) Dy() int { return r.Y1 - r.Y0 }
func (r R) Inside(o R) bool {
	return r.X0 >= o.X0 && r.X1 <= o.X1 && r.Y0 >= o.Y0 && r.Y1 <= o.Y1
}

// NumPoses is the number of baked pose sprites.
func NumPoses() int { return len(posePixels) }

// PoseName returns the pose's stable name ("wave", "point", ...).
func PoseName(p int) string { return posePixels[p].Name }

// Grid returns the pose pixel data: width, height and row-major role indices
// (0 = transparent; otherwise an index into PaletteRoles).
func Grid(p int) (int, int, []uint8) {
	d := posePixels[p]
	return d.W, d.H, d.Pix
}

// Anchor returns the pose's animation anchors in pixel coordinates.
func Anchor(p int) AnchorSet { return poseAnchors[p] }

// themeRoles maps role index -> the semantic theme hex, index 0 excluded.
func themeRoles(p theme.Palette) []string {
	return []string{"",
		p.Base, p.DarkerBg, p.Surface, p.SurfaceStrong, p.Foreground, p.BrightFg,
		p.Accent, p.Green, p.Red, p.Orange, p.Yellow, p.Cyan, p.Muted,
	}
}

func hexF(h string) (float32, float32, float32) {
	c := strings.TrimPrefix(strings.TrimSpace(h), "#")
	if len(c) != 6 {
		return 0, 0, 0
	}
	v, err := strconv.ParseUint(c, 16, 32)
	if err != nil {
		return 0, 0, 0
	}
	return float32(v>>16&0xff) / 255, float32(v>>8&0xff) / 255, float32(v&0xff) / 255
}

// Render re-colors pose p onto pal. Alpha is 1 wherever the pose is opaque
// (the generated sprite has no anti-aliased edges after extraction).
func Render(p int, pal theme.Palette) ambient.Raster {
	w, h, pix := Grid(p)
	roles := themeRoles(pal)
	return ambient.RasterFromPixels(w, h, func(x, y int) (r, g, b, a float32) {
		if v := pix[y*w+x]; v != 0 && int(v) < len(roles) {
			return join3(hexF(roles[v]))
		}
		return 0, 0, 0, 0
	})
}

func join3(r, g, b float32) (float32, float32, float32, float32) {
	return r, g, b, 1
}
