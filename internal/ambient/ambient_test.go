package ambient

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ambient_test.go — golden frames, frame invariants and benchmarks for the
// Phase 4b-B scenes (spec: fixed times, sizes, seeds and states in each
// render mode, plus Reward frames).

var goldenUpdate = os.Getenv("MTM_UPDATE_GOLDEN") == "1"

func defaultPalette() Palette {
	return Palette{
		Base: "#14282d", Surface: "#253c41", SurfaceRaised: "#2f4a50", SurfaceStrong: "#3d5960",
		Border: "#a1afa9", Text: "#e8dfc4", BrightText: "#fff2d5", MutedSoft: "#a1afa9",
		Accent: "#d3ae78", AccentSoft: "#8a744f", Success: "#a8b887", Warning: "#dfc181",
		Danger: "#d78f79", HeatWarm: "#dfc181", HeatHot: "#d3ae78", HeatOverdue: "#d78f79",
		Blue: "#91afc5", Cyan: "#86b8b5", Yellow: "#dfc181", Magenta: "#b5a0ba",
		Orange: "#d7a06c", Brown: "#b79b7f",
	}
}

func stateFor(progress float64, phase Phase) SceneState {
	return SceneState{
		Progress: progress,
		Phase:    phase,
		Now:      time.Date(2026, 11, 7, 21, 30, 0, 0, time.Local),
		Seed:     42,
		Palette:  defaultPalette(),
	}
}

type goldenCase struct {
	name     string
	scene    string
	mode     string
	progress float64
	phase    Phase
	frameT   time.Duration
	size     [2]int
	reward   bool
}

func goldenCases() []goldenCase {
	out := []goldenCase{}
	for _, scene := range Registered() {
		for _, mode := range []string{ModeHalfblock, ModeASCII, ModeBraille} {
			states := []struct {
				name string
				p    float64
				ph   Phase
			}{
				{"idle", 0, PhaseIdle},
				{"work_p0", 0, PhaseWork},
				{"work_p05", 0.5, PhaseWork},
				{"work_p1", 1.0, PhaseWork},
				{"break_p0", 0, PhaseBreak},
				{"break_p05", 0.5, PhaseBreak},
				{"break_p1", 1.0, PhaseBreak},
			}
			for _, st := range states {
				out = append(out, goldenCase{
					name:     fmt.Sprintf("%s-%s-%s", scene, st.name, mode),
					scene:    scene,
					mode:     mode,
					progress: st.p,
					phase:    st.ph,
					frameT:   12 * time.Second,
					size:     [2]int{60, 18},
				})
			}
			out = append(out, goldenCase{
				name:     fmt.Sprintf("%s-reward-%s", scene, mode),
				scene:    scene,
				mode:     mode,
				progress: 1.0,
				phase:    PhaseWork,
				frameT:   800 * time.Millisecond,
				size:     [2]int{60, 18},
				reward:   true,
			})
		}
	}
	return out
}

func TestGoldenSceneFrames(t *testing.T) {
	for _, tc := range goldenCases() {
		t.Run(tc.name, func(t *testing.T) {
			s := New(tc.scene, tc.mode)
			st := stateFor(tc.progress, tc.phase)
			var lines []Line
			if tc.reward {
				lines = s.Reward(tc.frameT, tc.size[0], tc.size[1], st)
			} else {
				lines = s.Frame(tc.frameT, tc.size[0], tc.size[1], st)
			}
			got := strings.Join(lines, "\n") + "\n"
			golden := filepath.Join("testdata", tc.name+".golden")
			if goldenUpdate {
				if err := os.MkdirAll("testdata", 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("missing golden %s (run MTM_UPDATE_GOLDEN=1): %v", golden, err)
			}
			if string(want) != got {
				t.Fatalf("frame mismatch — got:\n%s", got)
			}
		})
	}
}

// Invariants: exact dimensions, no panics at tiny sizes, determinism,
// and theme-role colors only.

// TestLayerRegionsRespected: every cell painted by a tagged layer must
// lie inside that layer's declared bounding union. This is the regression
// net for elements drifting out of their intended region (a stray cell
// outside the horizon band, a cabin cell off the cabin rect, ...).
func TestLayerRegionsRespected(t *testing.T) {
	times := []time.Duration{0, 800 * time.Millisecond, 5*time.Second + 300*time.Millisecond, 34 * time.Second}
	sizes := [][2]int{{40, 14}, {100, 30}}
	for _, sceneName := range Registered() {
		for _, mode := range []string{ModeHalfblock, ModeBraille, ModeASCII} {
			for _, size := range sizes {
				for _, ft := range times {
					s := New(sceneName, mode)
					p, ok := s.(paintable)
					if !ok {
						t.Fatalf("%s: scene is not paintable", sceneName)
					}
					st := stateFor(0.35, PhaseWork)
					st.Lightning = true // exercise the lightning bolt layer
					b := newFrameBuffer(p, mode, ft, size[0], size[1], st)
					type rect struct{ x0, y0, x1, y1 int }
					union := map[uint8]rect{}
					for _, r := range b.regions {
						u, ok := union[r.tag]
						if !ok {
							union[r.tag] = rect{r.x0, r.y0, r.x1, r.y1}
							continue
						}
						u.x0 = minInt(u.x0, r.x0)
						u.y0 = minInt(u.y0, r.y0)
						u.x1 = maxInt(u.x1, r.x1)
						u.y1 = maxInt(u.y1, r.y1)
						union[r.tag] = u
					}
					for i := 0; i < b.sw*b.sh; i++ {
						tag := b.layer[i]
						if tag == 0 {
							continue
						}
						u, ok := union[tag]
						if !ok {
							t.Fatalf("%s %s %dx%d t=%v: cell tagged layer %d with no recorded region", sceneName, mode, size[0], size[1], ft, tag)
						}
						x, y := i%b.sw, i/b.sw
						if x < u.x0 || y < u.y0 || x >= u.x1 || y >= u.y1 {
							t.Fatalf("%s %s %dx%d t=%v: cell (%d,%d) painted by layer %d outside its region [(%d,%d),(%d,%d))", sceneName, mode, size[0], size[1], ft, x, y, tag, u.x0, u.y0, u.x1, u.y1)
						}
					}
				}
			}
		}
	}
}

func TestFrameExactDimensions(t *testing.T) {
	sizes := [][2]int{{1, 1}, {2, 2}, {5, 3}, {10, 6}, {40, 12}, {60, 18}, {100, 30}}
	for _, scene := range Registered() {
		for _, mode := range []string{ModeHalfblock, ModeASCII, ModeBraille} {
			for _, size := range sizes {
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Fatalf("%s %s %dx%d panicked: %v", scene, mode, size[0], size[1], r)
						}
					}()
					s := New(scene, mode)
					frame := s.Frame(12*time.Second, size[0], size[1], stateFor(0.5, PhaseWork))
					if len(frame) != size[1] {
						t.Fatalf("%s %s %dx%d: %d lines", scene, mode, size[0], size[1], len(frame))
					}
					for i, line := range frame {
						if w := cellWidth(line); w != size[0] {
							t.Fatalf("%s %s %dx%d line %d: %d cells", scene, mode, size[0], size[1], i, w)
						}
					}
				}()
			}
		}
	}
}

func TestDeterministic(t *testing.T) {
	for _, scene := range Registered() {
		s := New(scene, ModeHalfblock)
		st := stateFor(0.6, PhaseWork)
		a := s.Frame(34*time.Second, 40, 12, st)
		b := s.Frame(34*time.Second, 40, 12, st)
		if strings.Join(a, "\n") != strings.Join(b, "\n") {
			t.Fatalf("%s: same seed and t rendered differently", scene)
		}
		st2 := st
		st2.Seed = 999
		c := s.Frame(34*time.Second, 40, 12, st2)
		if strings.Join(a, "\n") == strings.Join(c, "\n") {
			t.Fatalf("%s: changing the seed changed nothing", scene)
		}
	}
}

var sgrColor = regexp.MustCompile(`[38;]2;(\d+);(\d+);(\d+)`)
var hexRe = regexp.MustCompile(`#[0-9a-fA-F]{6}`)

func TestThemeRolesOnly(t *testing.T) {
	for _, scene := range Registered() {
		for _, mode := range []string{ModeHalfblock, ModeASCII, ModeBraille} {
			s := New(scene, mode)
			frame := s.Frame(12*time.Second, 60, 18, stateFor(0.5, PhaseWork))
			joined := strings.Join(frame, "\n")
			// collect every 24-bit SGR color in the frame
			m := paletteModel{}
			_ = m
			found := map[string]bool{}
			for _, match := range sgrColor.FindAllStringSubmatch(joined, -1) {
				rgb := fmt.Sprintf("#%02x%02x%02x", mustAtoi(match[1]), mustAtoi(match[2]), mustAtoi(match[3]))
				found[rgb] = true
			}
			if len(found) == 0 {
				t.Fatalf("%s %s: no colors in frame — renderer broken?", scene, mode)
			}
			pm := buildPalette(defaultPalette())
			// convex hull of the raw theme roles (blends and gradients live inside it)
			hull := func(ch func(rgb) int) [2]int {
				lo, hi := 255, 0
				for i := 0; i < cRoleCount; i++ {
					v := ch(pm.table[i])
					if v < lo {
						lo = v
					}
					if v > hi {
						hi = v
					}
				}
				return [2]int{lo, hi}
			}
			hullR := hull(func(c rgb) int { return int(c.r) })
			hullG := hull(func(c rgb) int { return int(c.g) })
			hullB := hull(func(c rgb) int { return int(c.b) })
			for color := range found {
				// Blends, gradients and additive glow may produce any point
				// inside the theme's convex hull; an invented hue outside the
				// hull violates the theme-only rule.
				rv, err1 := strconv.ParseUint(color[1:3], 16, 8)
				gv, err2 := strconv.ParseUint(color[3:5], 16, 8)
				bv, err3 := strconv.ParseUint(color[5:7], 16, 8)
				if err1 != nil || err2 != nil || err3 != nil {
					t.Fatalf("%s %s: malformed color %q", scene, mode, color)
				}
				r, g, b := int(rv), int(gv), int(bv)
				if r < hullR[0] || r > hullR[1] || g < hullG[0] || g > hullG[1] || b < hullB[0] || b > hullB[1] {
					t.Fatalf("%s %s: color %s is outside the theme convex hull", scene, mode, color)
				}
			}
		}
	}
}

// cellWidth counts grapheme cells in a rendered line; ambient lines carry
// only single-width runes or the box/block/braille glyph sets — uniseg's
// East-Asian-ambiguous width is 1 for all of them in this build.
func cellWidth(line string) int {
	w := 0
	rest := stripANSI(line)
	for _, r := range rest {
		_ = r
		w++
	}
	return w
}

var ansiStrip = regexp.MustCompile("\x1b\\[[0-9;?]*[a-zA-Z]")

func stripANSI(s string) string { return ansiStrip.ReplaceAllString(s, "") }

func mustAtoi(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		n = n*10 + int(s[i]-'0')
	}
	return n
}

func BenchmarkScenes(b *testing.B) {
	for _, scene := range Registered() {
		for _, mode := range []string{ModeHalfblock, ModeBraille, ModeASCII} {
			b.Run(scene+"/"+mode, func(b *testing.B) {
				s := New(scene, mode)
				st := stateFor(0.5, PhaseWork)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_ = s.Frame(time.Duration(i)*time.Second/10, 100, 30, st)
				}
			})
		}
	}
}
