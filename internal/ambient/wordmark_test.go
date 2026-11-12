package ambient

import (
	"math"
	"testing"
)

// wordmark_test.go — the splash wordmark's contrast picker: whatever
// the theme and the sampled scene background, the returned letter color
// clears the minimum WCAG ratio (or is the theme's own extreme when
// nothing else is available).

func TestLetterColorMeetsMinContrast(t *testing.T) {
	themes := []struct {
		name         string
		bright, base string
	}{
		{"default", DefaultPalette().BrightText, DefaultPalette().Base},
		{"light", "#14120f", "#e9e6dc"},
		{"warm", "#fff6e0", "#26190f"},
	}
	bgs := []string{
		"#050b0e", "#14282d", "#253c41", "#33535b", "#4b6a70", "#0d2b33", "#5a5142", "#3a454b",
	}
	for _, th := range themes {
		for _, bg := range bgs {
			lr, lg, lb := letterColor(hexF32(th.bright), hexF32(th.base), hexF32(bg), 3.0)
			got := contrastF([3]float32{lr, lg, lb}, hexF32(bg))
			if got < 3.0 {
				t.Errorf("%s on bg %s: letter color contrast %.2f, want >= 3.0", th.name, bg, got)
			}
		}
	}
}

// TestLetterColorPrefersBrightWhenItFits: the raw BrightText is returned
// unchanged when it already clears the bar.
func TestLetterColorPrefersBrightWhenItFits(t *testing.T) {
	bright := hexF32("#fff2d5")
	base := hexF32("#14282d")
	lr, lg, lb := letterColor(bright, base, hexF32("#0b1a1e"), 3.0)
	if math.Abs(float64(lr-bright[0])) > 1e-6 || math.Abs(float64(lg-bright[1])) > 1e-6 || math.Abs(float64(lb-bright[2])) > 1e-6 {
		t.Errorf("bright was rewritten: got (%.3f,%.3f,%.3f)", lr, lg, lb)
	}
}

// TestApplyWordmarkPaintsLetters: the painter draws the letter pixels
// (against the scene, through the same buffer).
func TestApplyWordmarkPaintsLetters(t *testing.T) {
	p := paletteFor(DefaultPalette())
	b := newBuffer(ModeHalfblock, 20, 6, p, SubpixelAspect(ModeHalfblock, DefaultCellPixels()))
	wm := &WordmarkPaint{
		Rects:       []WordmarkRect{{X: 8, Y: 2, W: 1, H: 1, Alpha: 1}},
		BrightText:  "#fff2d5",
		Base:        "#14282d",
		ShadowAlpha: 0.45,
		MinContrast: 3.0,
	}
	before := b.fb.downsample()
	beforeC := before[5*20+8] // bottom subcell of the letter's cell row
	applyWordmark(b, wm, 1, 2)
	after := b.fb.downsample()
	afterC := after[5*20+8]
	if afterC.r <= beforeC.r {
		t.Errorf("letter pixel did not change: before=(%.3f,%.3f,%.3f) after=(%.3f,%.3f,%.3f)",
			beforeC.r, beforeC.g, beforeC.b, afterC.r, afterC.g, afterC.b)
	}
}
