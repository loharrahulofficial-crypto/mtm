package mascot

// fidelity_test.go — audit the baked poses against the AI source sheet
// (internal/mascot/assets/mo_sheet.png). Reports per-pose diff percentages
// between the raw segmented/downsampled source (sourcePose, factor 16 —
// the same pipeline as cmd/moextract) and the baked grid (Grid).
// Skips when the sheet is missing or undecodable.

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"testing"

	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

func srcRGB(im image.Image, x, y int) (uint8, uint8, uint8) {
	r, g, b, _ := im.At(x, y).RGBA()
	return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
}

func hexBytes(s string) (uint8, uint8, uint8) {
	v := uint32(0)
	if len(s) == 7 && s[0] == '#' {
		v = parseHex(s[1:7])
	}
	return uint8(v >> 16), uint8(v >> 8), uint8(v)
}

func parseHex(s string) uint32 {
	var v uint32
	for i := 0; i < len(s) && i < 6; i++ {
		c := s[i]
		v <<= 4
		switch {
		case c >= '0' && c <= '9':
			v |= uint32(c - '0')
		case c >= 'a' && c <= 'f':
			v |= uint32(c-'a') + 10
		case c >= 'A' && c <= 'F':
			v |= uint32(c-'A') + 10
		}
	}
	return v
}

// sourcePose replicates cmd/moextract's segment→crop→mode-downsample→snap
// pipeline for cell i, factor 16 — used to diff against the baked grid.
func sourcePose(im image.Image, factor, i, tol int) [][]uint8 {
	b := im.Bounds()
	cols, rows := 4, 3
	cw, ch := b.Dx()/cols, b.Dy()/rows
	ox, oy := (i%cols)*cw, (i/cols)*ch
	cell := image.Rect(ox, oy, ox+cw, oy+ch)
	w, h := cell.Dx(), cell.Dy()

	bgR, bgG, bgB := srcRGB(im, b.Min.X+2, b.Min.Y+2)
	bg := struct{ r, g, b uint8 }{bgR, bgG, bgB}
	dist := func(x, y int) float64 {
		r, g, bl := srcRGB(im, cell.Min.X+x, cell.Min.Y+y)
		dr := float64(r) - float64(bg.r)
		dg := float64(g) - float64(bg.g)
		db := float64(bl) - float64(bg.b)
		return math.Sqrt(dr*dr + dg*dg + db*db)
	}

	outside := make([]bool, w*h)
	var stack []int
	try := func(x, y int) {
		if x < 0 || x >= w || y < 0 || y >= h {
			return
		}
		idx := y*w + x
		if outside[idx] {
			return
		}
		if dist(x, y) < float64(tol) {
			outside[idx] = true
			stack = append(stack, idx)
		}
	}
	for x := 0; x < w; x++ {
		try(x, 0)
		try(x, h-1)
	}
	for y := 0; y < h; y++ {
		try(0, y)
		try(w-1, y)
	}
	for len(stack) > 0 {
		idx := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		x, y := idx%w, idx/w
		for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			try(x+d[0], y+d[1])
		}
	}

	minx, miny, maxx, maxy := w, h, -1, -1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !outside[y*w+x] {
				if x < minx {
					minx = x
				}
				if x > maxx {
					maxx = x
				}
				if y < miny {
					miny = y
				}
				if y > maxy {
					maxy = y
				}
			}
		}
	}
	if maxx < 0 {
		return nil
	}
	bw, bh := maxx-minx+1, maxy-miny+1
	gw, gh := bw/factor, bh/factor
	if gw < 1 || gh < 1 {
		gw, gh = 1, 1
	}
	pal := themeRoles(theme.Defaults())
	grid := make([][]uint8, gh)
	for y := range grid {
		grid[y] = make([]uint8, gw)
	}
	for by := 0; by < gh; by++ {
		for bx := 0; bx < gw; bx++ {
			counts := map[uint8]int{}
			for y := 0; y < factor; y++ {
				for x := 0; x < factor; x++ {
					sx := minx + bx*factor + x
					sy := miny + by*factor + y
					if sx >= maxx+1 || sy >= maxy+1 || outside[sy*w+sx] {
						continue
					}
					r, g, bl := srcRGB(im, cell.Min.X+sx, cell.Min.Y+sy)
					cr, cg, cb := float64(r), float64(g), float64(bl)
					best := uint8(1)
					bd := math.MaxFloat64
					for i := 1; i < len(pal); i++ {
						hr, hg, hb := hexBytes(pal[i])
						dr := cr - float64(hr)
						dg := cg - float64(hg)
						db := cb - float64(hb)
						if d := math.Sqrt(dr*dr + dg*dg + db*db); d < bd {
							bd = d
							best = uint8(i)
						}
					}
					counts[best]++
				}
			}
			best, bn := uint8(0), 0
			for i := 1; i < len(pal); i++ {
				if counts[uint8(i)] > bn {
					bn = counts[uint8(i)]
					best = uint8(i)
				}
			}
			grid[by][bx] = best
		}
	}
	return grid
}

func TestFidelity(t *testing.T) {
	f, err := os.Open("/home/rahul/Downloads/1.png")
	if err != nil {
		t.Skipf("source sheet not present: %v", err)
	}
	defer f.Close()
	im, err := png.Decode(f)
	if err != nil {
		t.Skipf("source sheet undecodable: %v", err)
	}
	w, h, _ := Grid(0)
	fmt.Printf("baked grid: %dx%d, %d poses\n", w, h, NumPoses())
	for i := 0; i < NumPoses(); i++ {
		src := sourcePose(im, 16, i, 10)
		if src == nil {
			t.Errorf("pose %s: source extraction found nothing", PoseName(i))
			continue
		}
		_, _, pix := Grid(i)
		sh, sw := len(src), len(src[0])
		bh, bw := h, w
		same, tot := 0, 0
		for y := 0; y < bh; y++ {
			for x := 0; x < bw; x++ {
				sy, sx := y*sh/bh, x*sw/bw // nearest-neighbor sample
				if sy >= sh {
					sy = sh - 1
				}
				if sx >= sw {
					sx = sw - 1
				}
				if pix[y*w+x] == src[sy][sx] {
					same++
				}
				tot++
			}
		}
		fmt.Printf("%-10s source-grid=%dx%d  diff=%.1f%%\n", PoseName(i), sw, sh, 100*(1-float64(same)/float64(tot)))
	}
}
