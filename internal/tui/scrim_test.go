package tui

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/loharrahulofficial-crypto/mtm/internal/ambient"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

// scrim_test.go — spec 5.3: the Focus scrim must bring text vs backdrop up
// to WCAG 4.5 automatically, and the overlay must never eat scene cells.

func testScenePalette() ambient.Palette {
	d := theme.Defaults()
	return ambient.Palette{
		Base: d.Base, Surface: d.Surface, SurfaceRaised: d.SurfaceRaised, SurfaceStrong: d.SurfaceStrong,
		Border: d.Border, Text: d.Text, BrightText: d.BrightText, MutedSoft: d.MutedSoft,
		Accent: d.Accent, AccentSoft: d.AccentSoft, Success: d.Success, Warning: d.Warning, Danger: d.Danger,
		HeatWarm: d.HeatWarm, HeatHot: d.HeatHot, HeatOverdue: d.HeatOverdue,
		Blue: d.Blue, Cyan: d.Cyan, Yellow: d.Yellow, Magenta: d.Magenta, Orange: d.Orange, Brown: d.Brown,
	}
}

func TestFocusScrimAlphaMeetsContrast(t *testing.T) {
	def := testScenePalette()
	mono := def
	mono.Accent = def.Base
	mono.Surface = def.Base
	mono.SurfaceRaised = def.Base
	mono.SurfaceStrong = def.Base
	mono.Border = def.Base
	white := def
	white.Base = "#ffffff"
	white.Surface = "#ffffff"
	white.SurfaceRaised = "#ffffff"
	white.SurfaceStrong = "#ffffff"
	white.BrightText = "#101010"
	for _, tc := range []struct {
		name string
		p    ambient.Palette
	}{{"default", def}, {"monochrome", mono}, {"all-white", white}} {
		t.Run(tc.name, func(t *testing.T) {
			a := focusScrimAlpha(tc.p.BrightText, tc.p)
			if a < focusScrimFloor || a > focusScrimCeil {
				t.Fatalf("alpha = %v outside [%v, %v]", a, focusScrimFloor, focusScrimCeil)
			}
			maxHex, _ := paletteMaxRole(tc.p)
			dir := focusScrimDir(tc.p.BrightText)
			backdrop := theme.Blend(maxHex, scrimTarget(dir), a)
			var ratio float64
			if dir == scrimDarken {
				ratio = theme.ContrastRatio(tc.p.BrightText, backdrop)
			} else {
				ratio = theme.ContrastRatio(backdrop, tc.p.BrightText)
			}
			if ratio < focusScrimMinRatio {
				t.Fatalf("BrightText vs scrimmed max role %s = %.2f, want ≥ %.1f", backdrop, ratio, focusScrimMinRatio)
			}
		})
	}
}

func TestScrimPaletteDarkensEveryRole(t *testing.T) {
	def := testScenePalette()
	out := scrimPalette(def, 0.5, scrimDarken)
	for _, name := range []string{"base", "surface", "brightText", "accent", "cyan", "warning", "heatHot"} {
		get := func(p ambient.Palette) string {
			switch name {
			case "base":
				return p.Base
			case "surface":
				return p.Surface
			case "brightText":
				return p.BrightText
			case "accent":
				return p.Accent
			case "cyan":
				return p.Cyan
			case "warning":
				return p.Warning
			case "heatHot":
				return p.HeatHot
			}
			return "#000000"
		}
		in, out := get(def), get(out)
		if in == out {
			t.Errorf("role %s unchanged by a 0.5 scrim", name)
		}
		if hexLuminance(out) >= hexLuminance(in) {
			t.Errorf("role %s got brighter under the scrim (%s → %s)", name, in, out)
		}
	}
}

func TestOverlayLineKeepsSceneOutsideInk(t *testing.T) {
	scene := "\x1b[48;2;10;20;30m\x1b[38;2;200;210;220m████████████████\x1b[0m"
	body := "  \x1b[1;38;2;255;255;255mXX\x1b[0m"
	out := overlayLine(scene, body)
	if !strings.Contains(out, "XX") {
		t.Fatal("ink run missing from overlay output")
	}
	if !strings.Contains(out, "10;20;30") {
		t.Fatal("scene background lost outside the ink run")
	}
	if !strings.Contains(out, "200;210;220") {
		t.Fatal("scene foreground lost outside the ink run")
	}
}

func TestOverlayLineAllTransparentKeepsSceneCells(t *testing.T) {
	scene := "\x1b[48;2;10;20;30m\x1b[38;2;200;210;220m████████\x1b[0m"
	out := overlayLine(scene, "        ")
	if got := lipgloss.Width(out); got != 8 {
		t.Fatalf("overlay width = %d, want 8", got)
	}
	if !strings.Contains(out, "10;20;30") {
		t.Fatal("transparent body dropped the scene background")
	}
}

func TestOverlayLineBoldPrefixEscapeNotDropped(t *testing.T) {
	scene := "\x1b[48;2;1;2;3m\x1b[38;2;4;5;6maaaaaaaaaaa\x1b[0m"
	body := "\x1b[1m  X\x1b[0m"
	out := overlayLine(scene, body)
	b := strings.Index(out, "X")
	if b < 0 {
		t.Fatal("X missing from overlay output")
	}
	if !strings.Contains(out[:b], "\x1b[1m") {
		t.Fatal("pending bold escape before the first ink cell was dropped")
	}
}

func baseBGSeq(t *testing.T, hex string) string {
	t.Helper()
	v, err := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32)
	if err != nil {
		t.Fatalf("bad hex %q: %v", hex, err)
	}
	_ = v
	nums := []int{}
	for i := 16; i >= 0; i -= 8 {
		nums = append(nums, int(v>>uint(i)&0xff))
	}
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", nums[0], nums[1], nums[2])
}

func TestFocusBodyTransparentHasNoBaseBG(t *testing.T) {
	s := coursesFixture(t)
	defer func() { _ = s.Close() }()
	m := BuildDumpModel(dumpConfig(), s, sync.New(s.Root), "focus", 120, 40, DumpFixedTime)
	baseSeq := baseBGSeq(t, theme.Defaults().Base)
	for i, l := range m.renderFocusBody(true) {
		if strings.Contains(l, baseSeq) {
			t.Errorf("line %d of the transparent body still carries the Base background", i)
		}
	}
}

func TestFocusFrameHasNoSolidBaseRow(t *testing.T) {
	s := coursesFixture(t)
	defer func() { _ = s.Close() }()
	m := BuildDumpModel(dumpConfig(), s, sync.New(s.Root), "focus", 140, 40, DumpFixedTime)
	base := renderedBGProbe(theme.Defaults().Base)
	if base == "" {
		t.Fatal("could not probe the base background")
	}
	for i, l := range strings.Split(m.View(), "\n") {
		grid := parseFrame(l)
		if len(grid.bg) == 0 {
			t.Fatalf("row %d did not parse", i)
		}
		nonBase := 0
		for _, bg := range grid.bg[0] {
			if bg != base {
				nonBase++
			}
		}
		if nonBase == 0 {
			t.Errorf("row %d is a solid Base plate — the scene must show through", i)
		}
	}
}

func TestFocusTimerAndHintVisibleOnTop(t *testing.T) {
	s := coursesFixture(t)
	defer func() { _ = s.Close() }()
	m := BuildDumpModel(dumpConfig(), s, sync.New(s.Root), "focus", 140, 40, DumpFixedTime)
	plain := StripANSI(m.View())
	for _, want := range []string{"00:00", "space starts a pomodoro", "FOCUS"} {
		if !strings.Contains(plain, want) {
			t.Errorf("focus frame missing %q", want)
		}
	}
}
