package ambient

import "testing"

// warmsky_test.go — the warm dusk-horizon skies stay clear of mud:
// lightness is monotonically non-decreasing from zenith to horizon, and
// no stop or stop-to-stop point sits in the "mud zone" — hue 30–90°
// with OKLCH lightness below 0.45 and chroma below 0.06.

func TestWarmSkyMonotonicLightnessAndNoMudZone(t *testing.T) {
	for _, tc := range []struct {
		name string
		pal  Palette
	}{
		{"default", defaultThemePalette()},
		{"light", lightThemePalette()},
		{"warm", warmThemePalette()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stops := warmSkyStops(tc.pal)
			if len(stops) < 3 {
				t.Fatalf("warmSkyStops built %d stops, want >= 3", len(stops))
			}
			prevL := -1.0
			for i := 0; i+1 < len(stops); i++ {
				for k := 0; k <= 16; k++ {
					mixed := oklchMix(stops[i].hex, stops[i+1].hex, float64(k)/16)
					o := hexToOKLab(mixed)
					h, c := oklabHue(o)
					l := o.l
					if l < prevL {
						t.Errorf("stop %d point %d: lightness %.4f < previous %.4f (not monotonic)", i, k, l, prevL)
					}
					prevL = l
					if h >= 30 && h <= 90 && l < 0.45 && c < 0.06 {
						t.Errorf("stop %d point %d: %s in the mud zone (hue %.1f°, L %.3f, C %.3f)", i, k, mixed, h, l, c)
					}
				}
			}
			// the horizon stop itself: warm and lifted
			horizon := hexToOKLab(stops[len(stops)-1].hex)
			hh, hc := oklabHue(horizon)
			if hh < 20 || hh > 110 {
				t.Errorf("horizon hue %.1f° not in the warm band", hh)
			}
			if horizon.l < 0.45 {
				t.Errorf("horizon lightness %.3f below 0.45 — the haze went muddy", horizon.l)
			}
			if hc < 0.05 {
				t.Errorf("horizon chroma %.3f too low — the haze went grey", hc)
			}
			// the horizon hue derives from AccentSoft/Warning
		})
	}
}

// TestWarmHorizonDerivation: the horizon derives from the warmer of
// AccentSoft/Warning, lifted in lightness.
func TestWarmHorizonDerivation(t *testing.T) {
	for _, tc := range []struct {
		name string
		pal  Palette
	}{
		{"default", defaultThemePalette()},
		{"light", lightThemePalette()},
		{"warm", warmThemePalette()},
	} {
		horizon := warmHorizon(tc.pal)
		o := hexToOKLab(horizon)
		h, c := oklabHue(o)
		soft := hexToOKLab(tc.pal.AccentSoft)
		softH, softC := oklabHue(soft)
		warn := hexToOKLab(tc.pal.Warning)
		warnH, warnC := oklabHue(warn)
		// the hue is that of the more chromatic of the two warm roles
		wantH := softH
		if warnC > softC {
			wantH = warnH
		}
		dh := h - wantH
		if dh > 180 {
			dh -= 360
		} else if dh < -180 {
			dh += 360
		}
		if dh < 0 {
			dh = -dh
		}
		if dh > 25 {
			t.Errorf("%s: horizon hue %.1f° not derived from the warmer AccentSoft/Warning role (want near %.1f°)", tc.name, h, wantH)
		}
		if o.l < 0.40 {
			t.Errorf("%s: horizon L %.3f not lifted (want the lifted AccentSoft/Warning)", tc.name, o.l)
		}
		if c < 0.04 {
			t.Errorf("%s: horizon chroma %.3f too low — grey", tc.name, c)
		}
		_ = softC
	}
}
