// Command moextract turns the AI-generated Mo sprite sheet into crisp,
// deterministic pose data for internal/mascot.
//
// The source sheet is an anti-aliased, slightly noisy render. Feeding it
// straight to the terminal emitter would fringe every edge and make the
// procedural blink/lip-sync anchors jitter. This tool reverses that:
//
//  1. flood-fill the page background from each cell border (tolerant to the
//     source's +-2 noise, but far from the robot's dark screen);
//  2. crop each pose to its sprite bbox;
//  3. mode-downsample into a small native pixel grid (one art pixel per
//     block, so no sub-pixel blends survive);
//  4. snap every pixel to the exact mtm theme palette;
//  5. despeckle isolated pixels;
//  6. locate per-pose anchors (head, eyes, mouth, antenna star, chest LEDs).
//
// It writes a review contact sheet plus (with -go) the generated Go data.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

// R is an inclusive-exclusive pixel rectangle in grid coordinates.
type R struct{ X0, Y0, X1, Y1 int }

func (r R) Empty() bool { return r.X0 >= r.X1 || r.Y0 >= r.Y1 }
func (r R) Dx() int     { return r.X1 - r.X0 }
func (r R) Dy() int     { return r.Y1 - r.Y0 }
func (r R) Intersects(o R) bool {
	return r.X0 < o.X1 && r.X1 > o.X0 && r.Y0 < o.Y1 && r.Y1 > o.Y0
}
func (r R) Union(o R) R {
	x0 := minInt(r.X0, o.X0)
	y0 := minInt(r.Y0, o.Y0)
	x1 := maxInt(r.X1, o.X1)
	y1 := maxInt(r.Y1, o.Y1)
	return R{x0, y0, x1, y1}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type rgb struct{ r, g, b uint8 }

// roles are the exact mtm theme colors (internal/theme.Defaults). Role 0 is
// reserved for transparency in the emitted grid.
var roles = []struct {
	name string
	c    rgb
}{
	{"transparent", rgb{0, 0, 0}},
	{"base", rgb{0x14, 0x28, 0x2d}},
	{"darker", rgb{0x0c, 0x19, 0x1e}},
	{"surface", rgb{0x25, 0x3c, 0x41}},
	{"panel", rgb{0x3d, 0x59, 0x60}},
	{"cream", rgb{0xe8, 0xdf, 0xc4}},
	{"bright", rgb{0xff, 0xf2, 0xd5}},
	{"gold", rgb{0xd3, 0xae, 0x78}},
	{"sage", rgb{0xa8, 0xb8, 0x87}},
	{"red", rgb{0xd7, 0x8f, 0x79}},
	{"orange", rgb{0xd7, 0xa0, 0x6c}},
	{"yellow", rgb{0xdf, 0xc1, 0x81}},
	{"blue", rgb{0x91, 0xaf, 0xc5}},
	{"cyan", rgb{0x86, 0xb8, 0xb5}},
	{"muted", rgb{0xa1, 0xaf, 0xa9}},
}

const (
	transparent = 0
)

func isCreamRole(r uint8) bool { return r == 5 || r == 6 } // cream | bright

func (c rgb) dist(o rgb) float64 {
	dr := float64(c.r) - float64(o.r)
	dg := float64(c.g) - float64(o.g)
	db := float64(c.b) - float64(o.b)
	return math.Sqrt(dr*dr + dg*dg + db*db)
}

func snap(c rgb) int {
	best := 1
	bd := math.MaxFloat64
	for i := 1; i < len(roles); i++ {
		if d := c.dist(roles[i].c); d < bd {
			bd = d
			best = i
		}
	}
	return best
}

type anchorSet struct {
	head  R
	eyes  [2]R
	mouth R
	star  R
	chest R
}

type pose struct {
	name string
	grid [][]uint8
	w, h int
	anch anchorSet
}

var poseNames = [12]string{
	"wave", "point", "search", "keycap",
	"pomo", "celebrate", "tasks", "idea",
	"idle", "think", "alert", "sleep",
}

func main() {
	in := flag.String("in", "", "source sprite sheet PNG")
	out := flag.String("out", "", "output directory for generated Go files")
	preview := flag.String("preview", "", "contact-sheet PNG path")
	factor := flag.Int("factor", 12, "source pixels per art pixel")
	bgTol := flag.Float64("bg-tol", 10, "background flood-fill tolerance")
	rows := flag.Int("rows", 3, "sheet rows")
	cols := flag.Int("cols", 4, "sheet columns")
	goGen := flag.Bool("go", false, "write generated Go data")
	flag.Parse()
	if *in == "" {
		fmt.Fprintln(os.Stderr, "moextract: -in is required")
		os.Exit(2)
	}
	if *factor < 1 {
		*factor = 1
	}

	src, err := loadPNG(*in)
	if err != nil {
		fmt.Fprintln(os.Stderr, "moextract:", err)
		os.Exit(1)
	}
	b := src.Bounds()
	cw, ch := b.Dx() / *cols, b.Dy() / *rows
	bg := srcAt(src, b.Min.X+2, b.Min.Y+2)

	poses := make([]*pose, 0, *rows**cols)
	for ry := 0; ry < *rows; ry++ {
		for rx := 0; rx < *cols; rx++ {
			cell := image.Rect(b.Min.X+rx*cw, b.Min.Y+ry*ch, b.Min.X+rx*cw+cw, b.Min.Y+ry*ch+ch)
			poses = append(poses, extractPose(src, cell, poseNames[len(poses)], *factor, bg, *bgTol))
		}
	}

	for _, p := range poses {
		printPose(p)
	}
	if *preview != "" {
		if err := writePreview(*preview, poses, 4); err != nil {
			fmt.Fprintln(os.Stderr, "moextract:", err)
			os.Exit(1)
		}
		fmt.Println("preview:", *preview)
	}
	if *goGen {
		if *out == "" {
			fmt.Fprintln(os.Stderr, "moextract: -go needs -out DIR")
			os.Exit(2)
		}
		if err := writeGo(*out, poses); err != nil {
			fmt.Fprintln(os.Stderr, "moextract:", err)
			os.Exit(1)
		}
		fmt.Println("wrote Go data to", *out)
	}
}

func loadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	im, _, err := image.Decode(f)
	return im, err
}

func srcAt(im image.Image, x, y int) rgb {
	r, g, bl, _ := im.At(x, y).RGBA()
	return rgb{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8)}
}

// segment floods the background from the cell border and returns the sprite
// bbox. mask is indexed [y*cellW+x] over the cell.
func segment(im image.Image, cell image.Rectangle, bg rgb, tol float64) ([]bool, R) {
	w, h := cell.Dx(), cell.Dy()
	outside := make([]bool, w*h)
	var stack []int
	try := func(x, y int) {
		if x < 0 || x >= w || y < 0 || y >= h {
			return
		}
		i := y*w + x
		if outside[i] {
			return
		}
		if srcAt(im, cell.Min.X+x, cell.Min.Y+y).dist(bg) < tol {
			outside[i] = true
			stack = append(stack, i)
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
		i := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		x, y := i%w, i/w
		for _, d := range [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
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
		return outside, R{}
	}
	return outside, R{minx, miny, maxx + 1, maxy + 1}
}

func extractPose(im image.Image, cell image.Rectangle, name string, factor int, bg rgb, tol float64) *pose {
	mask, bbox := segment(im, cell, bg, tol)
	if bbox.Empty() {
		return &pose{name: name}
	}
	w, h := bbox.Dx()/factor, bbox.Dy()/factor
	if w < 1 || h < 1 {
		w, h = 1, 1
	}
	cellW := cell.Dx()
	grid := make([][]uint8, h)
	for y := range grid {
		grid[y] = make([]uint8, w)
	}
	for by := 0; by < h; by++ {
		for bx := 0; bx < w; bx++ {
			counts := make([]int, len(roles))
			for y := 0; y < factor; y++ {
				for x := 0; x < factor; x++ {
					sx := bbox.X0 + bx*factor + x
					sy := bbox.Y0 + by*factor + y
					if sx >= bbox.X1 || sy >= bbox.Y1 {
						continue
					}
					if mask[sy*cellW+sx] {
						continue
					}
					counts[snap(srcAt(im, cell.Min.X+sx, cell.Min.Y+sy))]++
				}
			}
			best, bn := 0, 0
			for i := 1; i < len(counts); i++ {
				if counts[i] > bn {
					bn = counts[i]
					best = i
				}
			}
			grid[by][bx] = uint8(best)
		}
	}
	despeckle(grid)
	p := &pose{name: name, grid: grid, w: w, h: h}
	findAnchors(p)
	if p.anch.head.Empty() {
		// Derive a head from any cream component so a positional anchor
		// still exists even when detection is imperfect.
		if cs := components(p, isCreamRole); len(cs) > 0 {
			p.anch.head = cs[0]
		}
	}
	return normalize(p)
}

// Canvas every pose is placed onto so the mascot is the same size and head
// height regardless of pose — the pop transition then reads as motion, not
// growth. The head lands at a fixed top-left across poses.
const canvasW, canvasH = 40, 48

var headAnchor = R{5, 8, 35, 24} // where the pose head goes in the canvas

func normalize(p *pose) *pose {
	if p.w == 0 || p.h == 0 {
		return p
	}
	out := &pose{name: p.name, grid: make([][]uint8, canvasH)}
	for y := range out.grid {
		out.grid[y] = make([]uint8, canvasW)
	}
	ox := headAnchor.X0 - p.anch.head.X0
	oy := headAnchor.Y0 - p.anch.head.Y0
	for y := 0; y < p.h; y++ {
		for x := 0; x < p.w; x++ {
			nx, ny := x+ox, y+oy
			if nx < 0 || nx >= canvasW || ny < 0 || ny >= canvasH {
				continue
			}
			out.grid[ny][nx] = p.grid[y][x]
		}
	}
	out.w, out.h = canvasW, canvasH
	sh := func(r R) R {
		if r.Empty() {
			return R{}
		}
		r.X0 += ox
		r.X1 += ox
		r.Y0 += oy
		r.Y1 += oy
		return r
	}
	out.anch = anchorSet{
		head:  sh(p.anch.head),
		eyes:  [2]R{sh(p.anch.eyes[0]), sh(p.anch.eyes[1])},
		mouth: sh(p.anch.mouth),
		star:  sh(p.anch.star),
		chest: sh(p.anch.chest),
	}
	return out
}

func despeckle(g [][]uint8) {
	h := len(g)
	if h < 3 {
		return
	}
	w := len(g[0])
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			c := g[y][x]
			a, b, d, e := g[y-1][x], g[y+1][x], g[y][x-1], g[y][x+1]
			if a == b && b == d && d == e && a != c {
				g[y][x] = a
			}
		}
	}
}

// ---- anchors ----

func components(p *pose, pred func(uint8) bool) []R {
	seen := make([]bool, p.w*p.h)
	var out []R
	var sizes []int
	for y := 0; y < p.h; y++ {
		for x := 0; x < p.w; x++ {
			i := y*p.w + x
			if seen[i] || !pred(p.grid[y][x]) {
				continue
			}
			minx, miny, maxx, maxy, n := x, y, x, y, 0
			stack := []int{i}
			seen[i] = true
			for len(stack) > 0 {
				ci := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				cx, cy := ci%p.w, ci/p.w
				n++
				if cx < minx {
					minx = cx
				}
				if cx > maxx {
					maxx = cx
				}
				if cy < miny {
					miny = cy
				}
				if cy > maxy {
					maxy = cy
				}
				for _, d := range [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
					nx, ny := cx+d[0], cy+d[1]
					if nx < 0 || nx >= p.w || ny < 0 || ny >= p.h || seen[ny*p.w+nx] {
						continue
					}
					ni := ny*p.w + nx
					if pred(p.grid[ny][nx]) {
						seen[ni] = true
						stack = append(stack, ni)
					}
				}
			}
			out = append(out, R{minx, miny, maxx + 1, maxy + 1})
			sizes = append(sizes, n)
		}
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && sizes[j] > sizes[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
			sizes[j], sizes[j-1] = sizes[j-1], sizes[j]
		}
	}
	return out
}

func findAnchors(p *pose) {
	if p.h == 0 {
		return
	}
	var head R
	headSize := 0
	for _, c := range components(p, isCreamRole) {
		if c.Y0 > p.h*2/3 {
			continue
		}
		if a := c.Dx() * c.Dy(); a > headSize {
			headSize, head = a, c
		}
	}
	p.anch.head = head
	if head.Empty() {
		return
	}
	inner := R{head.X0 + 1, head.Y0 + 1, head.X1 - 1, head.Y1 - 1}
	if inner.X1 <= inner.X0 || inner.Y1 <= inner.Y0 {
		inner = head
	}
	var eyes [2]R
	var eyeSize [2]int
	var anyEyes []R
	for _, c := range components(p, isCreamRole) {
		if !c.Intersects(inner) {
			continue
		}
		a := c.Dx() * c.Dy()
		if a < 1 {
			continue
		}
		anyEyes = append(anyEyes, c)
		if c.Y1 > inner.Y0+inner.Dy()*2/3 {
			continue
		}
		switch {
		case a > eyeSize[0]:
			eyes[1], eyeSize[1] = eyes[0], eyeSize[0]
			eyes[0], eyeSize[0] = c, a
		case a > eyeSize[1]:
			eyes[1], eyeSize[1] = c, a
		}
	}
	if eyes[0].Empty() && len(anyEyes) > 0 {
		eyes[0] = anyEyes[0]
	}
	if eyes[1].Empty() && len(anyEyes) > 1 {
		eyes[1] = anyEyes[1]
	}
	if eyes[0].X0 > eyes[1].X0 && !eyes[1].Empty() {
		eyes[0], eyes[1] = eyes[1], eyes[0]
	}
	p.anch.eyes = eyes
	for _, c := range components(p, isCreamRole) {
		if !c.Intersects(inner) || c.Y0 < inner.Y0+inner.Dy()/2 {
			continue
		}
		if p.anch.mouth.Empty() || c.Dx()*c.Dy() > p.anch.mouth.Dx()*p.anch.mouth.Dy() {
			p.anch.mouth = c
		}
	}
	for _, c := range components(p, func(r uint8) bool { return r == 7 }) { // gold
		if c.Intersects(head) {
			continue
		}
		if p.anch.star.Empty() || c.Y0 < p.anch.star.Y0 {
			p.anch.star = c
		}
	}
	var chest R
	nLED := 0
	for _, c := range components(p, func(r uint8) bool { return r == 8 || r == 12 || r == 13 }) { // sage | yellow | orange
		if c.Y0 < head.Y1 || c.X0 < 0 || c.Y0 >= p.h {
			continue
		}
		nLED += c.Dx() * c.Dy()
		if chest.Empty() {
			chest = c
		} else {
			chest = chest.Union(c)
		}
	}
	if nLED >= 4 {
		p.anch.chest = chest
	}
}

// ---- reporting ----

func printPose(p *pose) {
	if p.w*p.h == 0 {
		fmt.Printf("%-10s empty\n", p.name)
		return
	}
	counts := map[uint8]int{}
	for _, row := range p.grid {
		for _, v := range row {
			counts[v]++
		}
	}
	tr := float64(counts[0]) / float64(p.w*p.h) * 100
	unif := uniformity(p)
	fmt.Printf("%-10s %2dx%-2d tr=%4.1f%% unif=%.3f head=%v eyes=%v mouth=%v star=%v chest=%v\n",
		p.name, p.w, p.h, tr, unif, p.anch.head, p.anch.eyes, p.anch.mouth, p.anch.star, p.anch.chest)
	fmt.Printf("           roles: %s\n", roleCounts(counts))
}

func uniformity(p *pose) float64 {
	same, n := 0, 0
	for y := 0; y < p.h; y++ {
		for x := 0; x < p.w; x++ {
			c := p.grid[y][x]
			ok := true
			for _, d := range [][2]int{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}} {
				nx, ny := x+d[0], y+d[1]
				if nx < 0 || nx >= p.w || ny < 0 || ny >= p.h || p.grid[ny][nx] != c {
					ok = false
					break
				}
			}
			if ok {
				same++
			}
			n++
		}
	}
	if n == 0 {
		return 1
	}
	return float64(same) / float64(n)
}

func roleCounts(m map[uint8]int) string {
	out := ""
	for i := 1; i < len(roles); i++ {
		if m[uint8(i)] > 0 {
			out += fmt.Sprintf("%s:%d ", roles[i].name, m[uint8(i)])
		}
	}
	return out
}

func writePreview(path string, poses []*pose, scale int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	maxW, maxH := 0, 0
	for _, p := range poses {
		if p.w > maxW {
			maxW = p.w
		}
		if p.h > maxH {
			maxH = p.h
		}
	}
	sheet := image.NewRGBA(image.Rect(0, 0, 4*maxW*scale, 3*maxH*scale))
	for i, p := range poses {
		if p.w == 0 {
			continue
		}
		x0 := (i % 4) * maxW * scale
		y0 := (i / 4) * maxH * scale
		for y := 0; y < p.h; y++ {
			for x := 0; x < p.w; x++ {
				c := color.RGBA{0x14, 0x28, 0x2d, 0xff}
				if r := p.grid[y][x]; r != 0 {
					c = color.RGBA{roles[r].c.r, roles[r].c.g, roles[r].c.b, 0xff}
				}
				for dy := 0; dy < scale; dy++ {
					for dx := 0; dx < scale; dx++ {
						sheet.Set(x0+x*scale+dx, y0+y*scale+dy, c)
					}
				}
			}
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, sheet)
}

// ---- generated data ----

func writeGo(dir string, poses []*pose) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var b string
	b += "// Code generated by cmd/moextract. DO NOT EDIT.\n\n"
	b += "package mascot\n\n"
	b += "// Pose pixel data: one role index per art pixel, row-major. Role 0 is\n"
	b += "// transparent; the rest index PaletteRoles below.\n"
	b += "type poseData struct {\n\tName string\n\tW, H int\n\tPix  []uint8\n}\n\n"
	b += "// PaletteRoles maps role indexes to semantic theme roles.\n"
	b += "var PaletteRoles = []string{\n"
	for _, r := range roles {
		b += fmt.Sprintf("\t%q,\n", r.name)
	}
	b += "}\n\n"
	b += "var posePixels = []poseData{\n"
	for _, p := range poses {
		b += fmt.Sprintf("\t{Name: %q, W: %d, H: %d, Pix: []uint8{\n", p.name, p.w, p.h)
		for _, row := range p.grid {
			for _, v := range row {
				b += fmt.Sprintf("\t\t%d,\n", v)
			}
		}
		b += "\t}},\n"
	}
	b += "}\n\n"
	b += "// R is a pixel-space rectangle (inclusive-exclusive). Zero value is empty.\n"
	b += "type R struct{ X0, Y0, X1, Y1 int }\n\n"
	b += "func (r R) Empty() bool { return r.X0 >= r.X1 || r.Y0 >= r.Y1 }\n"
	b += "// AnchorSet carries per-pose animation anchors in pixel coordinates.\n"
	b += "type AnchorSet struct {\n"
	b += "\tHead  R\n\tEyes  [2]R\n\tMouth R\n\tStar  R\n\tChest R\n"
	b += "}\n\n"
	b += "var poseAnchors = []AnchorSet{\n"
	for _, p := range poses {
		a := p.anch
		b += fmt.Sprintf("\t{Head: R{%d, %d, %d, %d}, Eyes: [2]R{R{%d, %d, %d, %d}, R{%d, %d, %d, %d}}, Mouth: R{%d, %d, %d, %d}, Star: R{%d, %d, %d, %d}, Chest: R{%d, %d, %d, %d}},\n",
			a.head.X0, a.head.Y0, a.head.X1, a.head.Y1,
			a.eyes[0].X0, a.eyes[0].Y0, a.eyes[0].X1, a.eyes[0].Y1,
			a.eyes[1].X0, a.eyes[1].Y0, a.eyes[1].X1, a.eyes[1].Y1,
			a.mouth.X0, a.mouth.Y0, a.mouth.X1, a.mouth.Y1,
			a.star.X0, a.star.Y0, a.star.X1, a.star.Y1,
			a.chest.X0, a.chest.Y0, a.chest.X1, a.chest.Y1)
	}
	b += "}\n"
	return os.WriteFile(filepath.Join(dir, "poses_gen.go"), []byte(b), 0o644)
}
