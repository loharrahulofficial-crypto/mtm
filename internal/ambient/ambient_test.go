package ambient

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
			allowed := map[string]bool{}
			pm := buildPalette(defaultPalette())
			for _, h := range pm.hex {
				allowed[h] = true
			}
			for color := range found {
				if !allowed[color] {
					t.Fatalf("%s %s: color %s is not a theme role or a blend toward Base", scene, mode, color)
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
