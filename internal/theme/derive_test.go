package theme

import (
	"strings"
	"testing"
)

func TestBlendInterpolates(t *testing.T) {
	got := Blend("#000000", "#ffffff", 0.5)
	if got != "#808080" {
		t.Errorf("Blend mid = %q, want #808080", got)
	}
	if got := Blend("#14282d", "#14282d", 0.5); got != "#14282d" {
		t.Errorf("Blend same = %q", got)
	}
	if got := Blend("#14282d", "#e8dfc4", 0); got != "#14282d" {
		t.Errorf("Blend t=0 = %q", got)
	}
	if got := Blend("#14282d", "#e8dfc4", 1); got != "#e8dfc4" {
		t.Errorf("Blend t=1 = %q", got)
	}
	// Non-hex (terminal-fallback) colors pass through unchanged.
	if got := Blend("3", "#ffffff", 0.5); got != "3" {
		t.Errorf("Blend non-hex = %q, want passthrough", got)
	}
}

func TestSurfacesAscend(t *testing.T) {
	p := Defaults()
	frac := func(hex string) float64 {
		c, _ := parseHex(hex)
		c2, _ := parseHex(p.Foreground)
		b, _ := parseHex(p.Background)
		// projection along foreground-background axis
		d := float64(int(c2.r) - int(b.r))
		if d == 0 {
			return -1
		}
		return float64(int(c.r)-int(b.r)) / d
	}
	s1, s2, s3 := frac(p.Surface), frac(p.SurfaceRaised), frac(p.SurfaceStrong)
	if !(s1 < s2 && s2 < s3) {
		t.Errorf("surfaces not ascending: surface %v raised %v strong %v", s1, s2, s3)
	}
	if s1 <= 0 || s3 >= 0.5 {
		t.Errorf("surfaces out of expected range: %v %v %v", s1, s2, s3)
	}
}

func TestDerivedContrast(t *testing.T) {
	p := Defaults()
	if r := ContrastRatio(p.Text, p.Surface); r < minTextOnBase {
		t.Errorf("text on surface contrast = %.2f, want >= %.1f", r, minTextOnBase)
	}
	if r := ContrastRatio(p.Text, p.SurfaceRaised); r < minTextOnBase {
		t.Errorf("text on raised contrast = %.2f, want >= %.1f", r, minTextOnBase)
	}
	if r := ContrastRatio(p.Accent, p.Surface); r < minAccentOnBase {
		t.Errorf("accent on surface contrast = %.2f, want >= %.1f", r, minAccentOnBase)
	}
}

func TestContrastRatioKnownValues(t *testing.T) {
	if r := ContrastRatio("#ffffff", "#000000"); r < 20.9 || r > 21.1 {
		t.Errorf("black/white contrast = %.2f, want ~21", r)
	}
	if r := ContrastRatio("#000000", "#000000"); r < 0.99 || r > 1.01 {
		t.Errorf("same color contrast = %.2f, want 1", r)
	}
}

func TestCoursePaletteStableAndSoftened(t *testing.T) {
	p := Defaults()
	if len(p.CoursePalette) < 8 {
		t.Fatalf("course palette has %d colors, want >= 8", len(p.CoursePalette))
	}
	// Same project always gets the same color; different names spread.
	a := CourseColor(p, "college/dbms", "")
	b := CourseColor(p, "college/dbms", "")
	if a != b {
		t.Errorf("course color unstable: %q vs %q", a, b)
	}
	// Softened: closer to base than the raw ANSI hue.
	for i, c := range p.CoursePalette {
		if c == p.Red || c == p.Green || c == p.Blue {
			t.Errorf("course palette[%d] = %q is not softened", i, c)
		}
	}
	// Override wins.
	got := CourseColor(p, "x", "#ff0000")
	if got != Soften("#ff0000", p.Base) {
		t.Errorf("hex override = %q, want %q", got, Soften("#ff0000", p.Base))
	}
	got = CourseColor(p, "x", "red")
	if got != Soften(p.Red, p.Base) {
		t.Errorf("named override = %q, want %q", got, Soften(p.Red, p.Base))
	}
	got = CourseColor(p, "x", "not-a-color")
	if got == "" {
		t.Errorf("bogus override fell back to nothing")
	}
}

func TestHeatmapRamp(t *testing.T) {
	p := Defaults()
	if len(p.Heatmap) != 5 {
		t.Fatalf("heatmap ramp has %d levels, want 5", len(p.Heatmap))
	}
	for i, c := range p.Heatmap {
		if _, ok := parseHex(c); !ok {
			t.Errorf("heatmap[%d] = %q is not a hex color", i, c)
		}
	}
}

func TestDeriveLightModeFallsBack(t *testing.T) {
	p := Palette{
		Mode:       "light",
		Background: "#fdf6e3",
		Foreground: "#000000",
		BrightFg:   "#000000",
		Muted:      "#777777",
		Accent:     "#b58900",
		DarkerBg:   "#eee8d5",
		DarkBg:     "#eee8d5",
		LightBg:    "#ffffff",
		Selection:  "#eee8d5",
		Red:        "#dc322f", Green: "#859900", Yellow: "#b58900",
		Blue: "#268bd2", Magenta: "#d33682", Cyan: "#2aa198", Orange: "#cb4b16", Brown: "#8f6f5a",
	}
	p.Derive()
	if r := ContrastRatio(p.Text, p.Surface); r < minTextOnBase {
		t.Errorf("light-mode text on surface contrast = %.2f, want >= %.1f", r, minTextOnBase)
	}
	if p.Heatmap[0] == "" || !strings.HasPrefix(p.Heatmap[4], "#") {
		t.Errorf("light heatmap not derived: %v", p.Heatmap)
	}
}
