package ambient

import "testing"

// ramps_test.go — §2.6: ramp derivation, adjacency contrast guarantee,
// atmospheric perspective.

func adjacentPairs(r Ramps) [][2]string {
	return [][2]string{
		{r.SkyTop, r.SkyHorizon},
		{r.SkyHorizon, r.Haze},
		{r.Mid, r.Highlight},
		{r.Highlight, r.Glow},
	}
}

func lightPalette() Palette {
	p := DefaultPalette()
	p.Base = "#f4f1e8"
	p.Surface = "#eae6da"
	p.SurfaceRaised = "#dedacd"
	p.SurfaceStrong = "#cfcaba"
	p.Border = "#b8b3a2"
	p.Text = "#3a382f"
	p.BrightText = "#141410"
	p.MutedSoft = "#6b675a"
	p.Accent = "#8a5a00"
	return p
}

func TestRampsAdjacentContrast(t *testing.T) {
	for _, tc := range []struct {
		name string
		p    Palette
	}{
		{"default", DefaultPalette()},
		{"monochrome", monochromePalette()},
		{"light", lightPalette()},
	} {
		r := BuildRamps(tc.p)
		for i, pair := range adjacentPairs(r) {
			if ratio := contrastRatio(pair[0], pair[1]); ratio < minLayerContrast {
				t.Errorf("%s: pair %d (%s vs %s) contrast %.2f < %.2f", tc.name, i, pair[0], pair[1], ratio, minLayerContrast)
			}
		}
	}
}

func TestRampsDeterministic(t *testing.T) {
	a := BuildRamps(DefaultPalette())
	b := BuildRamps(DefaultPalette())
	if a != b {
		t.Fatal("BuildRamps not deterministic")
	}
}

func TestRampsOrderedByLuminance(t *testing.T) {
	r := BuildRamps(DefaultPalette())
	if relativeLuminance(r.SkyTop) >= relativeLuminance(r.SkyHorizon) {
		t.Fatal("sky top is not darker than the horizon haze")
	}
	if relativeLuminance(r.Highlight) >= relativeLuminance(r.Glow) {
		t.Fatal("highlight is not darker than the glow core")
	}
}

func TestHazeForIsMonotonic(t *testing.T) {
	prev := -1.0
	for d := 0.0; d <= 1.0001; d += 0.05 {
		v := HazeFor(d)
		if v < prev {
			t.Fatalf("HazeFor not monotonic at %.2f: %.3f < %.3f", d, v, prev)
		}
		if v < 0 || v > 1 {
			t.Fatalf("HazeFor(%.2f) = %.3f out of [0,1]", d, v)
		}
		prev = v
	}
}

func TestApplyHazeMovesTowardHaze(t *testing.T) {
	r := BuildRamps(DefaultPalette())
	c := "#d78f79"
	near := r.ApplyHaze(c, 0)
	far := r.ApplyHaze(c, 1)
	if near != c {
		t.Fatalf("depth 0 changed the color: %s → %s", c, near)
	}
	full := blendHex(c, r.Haze, HazeFor(1))
	if far != full {
		t.Fatalf("depth 1 = %s, want %s", far, full)
	}
	// and the distance to Haze shrinks with depth
	rd0 := colorDist(c, r.Haze)
	rd1 := colorDist(far, r.Haze)
	if rd1 >= rd0 {
		t.Fatal("increasing depth did not move the color toward Haze")
	}
	_ = rd0
}

func colorDist(a, b string) int {
	ar, ag, ab := hexToRGB(a)
	br, bg, bb := hexToRGB(b)
	dr, dg, db := int(ar)-int(br), int(ag)-int(bg), int(ab)-int(bb)
	if dr < 0 {
		dr = -dr
	}
	if dg < 0 {
		dg = -dg
	}
	if db < 0 {
		db = -db
	}
	return dr + dg + db
}

func monochromePalette() Palette {
	p := DefaultPalette()
	// hostile input: every role the same value can converge only via
	// Base and BrightText, which is exactly what enforceContrast uses.
	p.Surface = p.Base
	p.Base = "#2a2a2a"
	p.BrightText = "#e0e0e0"
	p.Accent = "#2a2a2a"
	p.SurfaceStrong = "#2a2a2a"
	return p
}
