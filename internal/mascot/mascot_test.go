package mascot

import (
	"testing"

	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

func TestPoseCount(t *testing.T) {
	if n := NumPoses(); n != 12 {
		t.Fatalf("NumPoses = %d, want 12", n)
	}
}

func TestPoseNames(t *testing.T) {
	want := []string{"wave", "point", "search", "keycap", "pomo", "celebrate", "tasks", "idea", "idle", "think", "alert", "sleep"}
	seen := map[string]bool{}
	for i := range want {
		if got := PoseName(i); got != want[i] {
			t.Errorf("PoseName(%d) = %q, want %q", i, got, want[i])
		}
		seen[want[i]] = true
	}
	if len(seen) != len(want) {
		t.Fatal("duplicate pose names")
	}
}

func TestPoseGridConsistent(t *testing.T) {
	for p := 0; p < NumPoses(); p++ {
		w, h, pix := Grid(p)
		if w != 40 || h != 48 {
			t.Errorf("pose %d: grid %dx%d, want 40x48", p, w, h)
		}
		if len(pix) != w*h {
			t.Errorf("pose %d: pix len %d, want %d", p, len(pix), w*h)
		}
	}
}

func TestPoseNonEmpty(t *testing.T) {
	for p := 0; p < NumPoses(); p++ {
		_, _, pix := Grid(p)
		n := 0
		for _, v := range pix {
			if v != 0 {
				n++
			}
		}
		if n < 100 {
			t.Errorf("pose %d: only %d opaque pixels", p, n)
		}
	}
}

// TestNoStrayPixels flags isolated opaque pixels — the signature of an
// anti-aliased source that the block snap didn't absorb.
func TestNoStrayPixels(t *testing.T) {
	for p := 0; p < NumPoses(); p++ {
		w, h, pix := Grid(p)
		iso, n := 0, 0
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if pix[y*w+x] == 0 {
					continue
				}
				n++
				notIso := false
				for _, d := range [][2]int{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}} {
					nx, ny := x+d[0], y+d[1]
					if nx < 0 || nx >= w || ny < 0 || ny >= h {
						continue // off-grid neighbours: not "stray"
					}
					if pix[ny*w+nx] != 0 {
						notIso = true
					}
				}
				if !notIso {
					iso++
				}
			}
		}
		if n == 0 {
			continue
		}
		if frac := float64(iso) / float64(n); frac > 0.08 {
			t.Errorf("pose %s: %.1f%% isolated pixels, want < 8%%", PoseName(p), frac*100)
		}
	}
}

// TestPoseSignatureRoles confirms each pose still carries its distinctive
// props/colors after extraction.
func TestPoseSignatureRoles(t *testing.T) {
	mustHave := map[string][]string{
		"wave":      {"gold"},
		"point":     {"gold"},
		"search":    {"gold", "blue"},
		"keycap":    {"gold", "cream"},
		"pomo":      {"red"},
		"celebrate": {"gold", "bright"},
		"tasks":     {"sage"},
		"idea":      {"yellow"},
		"idle":      {"cream", "gold"},
		"think":     {"yellow"},
		"alert":     {"gold"},
		"sleep":     {"gold", "muted"},
	}
	for name, roles := range mustHave {
		p := PoseByName(name)
		if p < 0 {
			t.Fatalf("pose %s not baked in", name)
		}
		_, _, pix := Grid(p)
		present := map[string]bool{}
		for _, v := range pix {
			if v != 0 {
				present[PaletteRoles[v]] = true
			}
		}
		for _, r := range roles {
			if !present[r] {
				t.Errorf("pose %s: role %q missing from baked data (has: %v)", name, r, present)
			}
		}
	}
}

func TestRenderDeterministic(t *testing.T) {
	pal := theme.Defaults()
	a := Render(0, pal)
	b := Render(0, pal)
	for i := range a.Pix {
		if a.Pix[i] != b.Pix[i] {
			t.Fatalf("pose 0 render differs at %d", i)
		}
	}
}

func TestRenderDiffersAcrossPoses(t *testing.T) {
	pal := theme.Defaults()
	diff := 0
	a := Render(0, pal) // wave
	for p := 1; p < NumPoses(); p++ {
		b := Render(p, pal)
		for i := range a.Pix {
			if a.Pix[i] != b.Pix[i] {
				diff++
				break
			}
		}
	}
	if diff == 0 {
		t.Fatal("all poses render identical")
	}
}

func TestAnchors(t *testing.T) {
	if a := Anchor(PoseByName("wave")); a.Eyes[0].Empty() {
		t.Errorf("wave: eyes not detected: %v", a.Eyes)
	}
	if a := Anchor(PoseByName("point")); a.Eyes[0].Empty() {
		t.Errorf("point: eyes not detected: %v", a.Eyes)
	}
	if a := Anchor(PoseByName("sleep")); a.Chest.Empty() {
		t.Errorf("sleep: chest LED grid not detected")
	}
	if a := Anchor(PoseByName("tasks")); a.Chest.Empty() {
		t.Errorf("tasks: chest LED grid not detected")
	}
}

func PoseByName(name string) int {
	for p := 0; p < NumPoses(); p++ {
		if PoseName(p) == name {
			return p
		}
	}
	return -1
}
