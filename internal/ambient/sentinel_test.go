package ambient

import (
	"strings"
	"testing"
	"time"
)

// sentinel_test.go — §6.1: no debug sentinels in output. Malformed theme
// roles must fall back to the default palette value for that role (or the
// theme Base), record the role name in the diagnostics list, and never
// emit magenta. Test builds panic to keep the failure loud.

func withFallbackAllowed(t *testing.T) {
	t.Helper()
	debugAllowFallback = true
	t.Cleanup(func() { debugAllowFallback = false })
}

func TestBuildPaletteMalformedRoleFallsBackAndRecords(t *testing.T) {
	withFallbackAllowed(t)
	ResetDiagnostics()
	p := DefaultPalette()
	p.Warning = "not-a-color"
	p.Cyan = "#qqqqqq"
	m := buildPalette(p)
	for _, c := range m.table {
		if c == (rgb{255, 0, 255}) {
			t.Fatal("sentinel 255,0,255 leaked into the palette table")
		}
	}
	got := strings.Join(Diagnostics(), "\n")
	for _, want := range []string{"Warning", "Cyan"} {
		if !strings.Contains(got, want) {
			t.Fatalf("diagnostics %q missing role %q", got, want)
		}
	}
	// the malformed roles fell back to the default palette values
	if m.table[cWarning] != mustRGB(DefaultPalette().Warning) {
		t.Fatalf("Warning fell back to %v, want default %v", m.table[cWarning], mustRGB(DefaultPalette().Warning))
	}
	if m.table[cCyan] != mustRGB(DefaultPalette().Cyan) {
		t.Fatalf("Cyan fell back to %v, want default %v", m.table[cCyan], mustRGB(DefaultPalette().Cyan))
	}
}

func mustRGB(hex string) rgb {
	c, ok := parseHex(hex)
	if !ok {
		panic(hex)
	}
	return c
}

func TestBuildPaletteMalformedRolePanicsInTestBuilds(t *testing.T) {
	ResetDiagnostics()
	p := DefaultPalette()
	p.Accent = "oops"
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected a loud panic on malformed role in test builds")
		}
	}()
	_ = buildPalette(p)
}

func TestHexToRGBMalformedFallsBackToBaseAndRecords(t *testing.T) {
	withFallbackAllowed(t)
	ResetDiagnostics()
	r, g, b := hexToRGB("definitely-not-a-color")
	base := mustRGB("#14282d")
	if r != base.r || g != base.g || b != base.b {
		t.Fatalf("hexToRGB fallback = (%d,%d,%d), want (%d,%d,%d)", r, g, b, base.r, base.g, base.b)
	}
	got := strings.Join(Diagnostics(), "\n")
	if !strings.Contains(got, "definitely-not-a-color") {
		t.Fatalf("diagnostics %q missing the offending value", got)
	}
}

func TestHexToRGBMalformedPanicsInTestBuilds(t *testing.T) {
	ResetDiagnostics()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected a loud panic on malformed hex in test builds")
		}
	}()
	_, _, _ = hexToRGB("not-a-color")
}

func TestFrameWithMalformedPaletteEmitsNoSentinel(t *testing.T) {
	withFallbackAllowed(t)
	ResetDiagnostics()
	for _, name := range Registered() {
		st := stateFor(0.5, PhaseWork)
		st.Palette.Danger = "bogus"
		st.Palette.Cyan = "#qqqqqq"
		st.Palette.Yellow = ""
		s := New(name, ModeHalfblock)
		frame := strings.Join(s.Frame(12*time.Second, 60, 18, st), "\n")
		if strings.Contains(frame, "255;0;255") || strings.Contains(frame, "ff00ff") || strings.Contains(frame, "FF00FF") {
			t.Fatalf("scene %s emitted the 255,0,255 sentinel", name)
		}
	}
}
