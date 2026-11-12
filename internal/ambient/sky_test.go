package ambient

import (
	"math"
	"testing"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

// sky_test.go — the night sky stays in the cool band: every row of the
// gradient sits between 170 and 260 degrees of hue (teal → blue →
// indigo) for the default theme, a derived light theme and a derived
// warm theme (their roles, not hand-picked stand-ins).

func ambientFromTheme(p theme.Palette) Palette {
	return Palette{
		Base: p.Base, Surface: p.Surface, SurfaceRaised: p.SurfaceRaised,
		SurfaceStrong: p.SurfaceStrong, Border: p.Border, Text: p.Text,
		BrightText: p.BrightText, MutedSoft: p.MutedSoft, Accent: p.Accent,
		AccentSoft: p.AccentSoft, Success: p.Success, Warning: p.Warning,
		Danger: p.Danger, HeatWarm: p.HeatWarm, HeatHot: p.HeatHot,
		HeatOverdue: p.HeatOverdue, Blue: p.Blue, Cyan: p.Cyan,
		Yellow: p.Yellow, Magenta: p.Magenta, Orange: p.Orange, Brown: p.Brown,
	}
}

func themeWith(mode string, p theme.Palette) Palette {
	p.Mode = mode
	p.Derive()
	return ambientFromTheme(p)
}

func defaultThemePalette() Palette { return DefaultPalette() }

func lightThemePalette() Palette {
	return themeWith("light", theme.Palette{
		Name:       "mtm-light",
		Background: "#e9e6dc", DarkerBg: "#d8d4c8", DarkBg: "#dedacd", LightBg: "#f2efe6",
		Foreground: "#2e2b26", BrightFg: "#14120f", Muted: "#7a766c",
		Accent: "#a86a2a", Selection: "#c7c2b2",
		Red: "#b04a3a", Green: "#5a7a3a", Yellow: "#a67a1e",
		Blue: "#3f6391", Magenta: "#7a4a7a", Cyan: "#2e7a7a",
		Orange: "#b0623f", Brown: "#7a5c3d",
	})
}

func warmThemePalette() Palette {
	return themeWith("dark", theme.Palette{
		Name:       "mtm-warm",
		Background: "#26190f", DarkerBg: "#1a120a", DarkBg: "#2e2115", LightBg: "#322519",
		Foreground: "#f0e4cf", BrightFg: "#fff6e0", Muted: "#b8a488",
		Accent: "#d08040", Selection: "#453322",
		Red: "#d06050", Green: "#9aa060", Yellow: "#d0a040",
		Blue: "#607898", Magenta: "#a87090", Cyan: "#58908a",
		Orange: "#c87838", Brown: "#a07850",
	})
}

func fpixToOKLab(c fpix) oklab {
	// fb stores sRGB-encoded 0..1; undo the encode, then convert
	r := srgbToLinear(uint8(math.Round(float64(c.r) * 255)))
	g := srgbToLinear(uint8(math.Round(float64(c.g) * 255)))
	b := srgbToLinear(uint8(math.Round(float64(c.b) * 255)))
	return linearToOKLab(r, g, b)
}

const coolMin, coolMax = 170.0, 260.0

func TestNightSkyStaysCool(t *testing.T) {
	for _, tc := range []struct {
		name string
		pal  Palette
	}{
		{"default", defaultThemePalette()},
		{"light", lightThemePalette()},
		{"warm", warmThemePalette()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// 1) the stops themselves and every OKLab-lerped point between them
			stops := nightSkyStops(tc.pal)
			if len(stops) != 4 {
				t.Fatalf("nightSkyStops built %d stops, want 4", len(stops))
			}
			for i := 0; i+1 < len(stops); i++ {
				for k := 0; k <= 8; k++ {
					mixed := oklabMix(stops[i].hex, stops[i+1].hex, float64(k)/8)
					h, c := oklabHue(hexToOKLab(mixed))
					if c < 0.045 {
						continue // below the perceptible chroma floor: hue undefined
					}
					if h < coolMin || h > coolMax {
						t.Errorf("stop segment %s→%s point %d: hue %.1f° outside [%s, %s] (chroma %.3f)",
							stops[i].hex, stops[i+1].hex, k, h, "170", "260", c)
					}
				}
			}
			// 2) the rendered sky, row by row (median hue of chromatic
			// pixels), skipping the horizon haze and the terrain
			st := SceneState{
				Progress: 0.45, Phase: PhaseWork,
				Now:  time.Date(2026, 11, 7, 21, 30, 0, 0, time.Local),
				Seed: 42, Palette: tc.pal, FullFrame: true,
			}
			sc := New("night", ModeHalfblock)
			r := sc.FramePixels(3*time.Second, 100, 30, st)
			lay := layoutForState(st, float64(r.W), float64(r.H))
			hazTop := lay.Horizon - float64(r.H)*0.018 - 1
			for y := 0; y < int(hazTop); y++ {
				hues := []float64{}
				for x := 0; x < r.W; x++ {
					px := r.Pix[y*r.W+x]
					o := fpixToOKLab(px)
					_, c := oklabHue(o)
					if c < 0.045 {
						continue // below the perceptible chroma floor: hue undefined
					}
					h, _ := oklabHue(o)
					hues = append(hues, h)
				}
				if len(hues) < r.W/4 {
					continue // mostly achromatic row: no constraint
				}
				sortFloat64s(hues)
				median := hues[len(hues)/2]
				if median < coolMin || median > coolMax {
					t.Errorf("row %d: median sky hue %.1f° outside [%s, %s]", y, median, "170", "260")
				}
			}
		})
	}
}

func sortFloat64s(in []float64) {
	for i := 1; i < len(in); i++ {
		for j := i; j > 0 && in[j] < in[j-1]; j-- {
			in[j], in[j-1] = in[j-1], in[j]
		}
	}
}
