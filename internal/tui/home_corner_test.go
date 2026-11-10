package tui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

// home_corner_test.go — the Home scene corner (spec 0.1): it renders only
// when the main pane is wide enough and there is free space below the last
// Home band. It never moves, shrinks or overlaps an existing band — the
// IDEAS band at w >= 110 in particular — and never counts toward a panel's
// blank ratio. Otherwise it stays hidden.

func cornerModel(t *testing.T, s *store.Store, w, h int) Model {
	t.Helper()
	return BuildDumpModel(dumpConfig(), s, sync.New(s.Root), "home", w, h, DumpFixedTime)
}

func bandBottom(bands [][]homeRect) int {
	used := 0
	for _, band := range bands {
		for _, r := range band {
			if r.y+r.h > used {
				used = r.y + r.h
			}
		}
	}
	return used
}

func findBandRect(bands [][]homeRect, title string) (homeRect, bool) {
	for _, band := range bands {
		for _, r := range band {
			if r.title == title {
				return r, true
			}
		}
	}
	return homeRect{}, false
}

func TestSceneCornerHiddenWhenNarrow(t *testing.T) {
	s := emptyFixture(t)
	defer func() { _ = s.Close() }()
	m := cornerModel(t, s, 100, 50)
	_, mainW := mainPaneGeometry(m)
	if _, ok := homeSceneCornerRect(m, mainW, m.height-1); ok {
		t.Fatal("scene corner shown in a 100-col terminal")
	}
}

func TestSceneCornerHiddenWhenNoFreeRows(t *testing.T) {
	s := coursesFixture(t) // populated Home: bands absorb the height
	defer func() { _ = s.Close() }()
	for _, h := range []int{24, 30} {
		m := cornerModel(t, s, 120, h)
		_, mainW := mainPaneGeometry(m)
		mainH := m.height - 1
		free, _ := homeFreeRows(m, mainW, mainH)
		_, ok := homeSceneCornerRect(m, mainW, mainH)
		if free < sceneCornerMinRows && ok {
			t.Fatalf("120x%d: corner shown with only %d free rows", h, free)
		}
		if free >= sceneCornerMinRows && !ok {
			t.Fatalf("120x%d: corner hidden despite %d free rows", h, free)
		}
	}
}

func TestSceneCornerFitsFreeSpace(t *testing.T) {
	s := emptyFixture(t) // sparse: fewest bands, most free space
	defer func() { _ = s.Close() }()
	m := cornerModel(t, s, 200, 50)
	_, mainW := mainPaneGeometry(m)
	mainH := m.height - 1
	bands := homeLayout(m, mainW, mainH)
	used := bandBottom(bands)
	free, _ := homeFreeRows(m, mainW, mainH)
	if free < sceneCornerMinRows {
		t.Fatalf("empty 200x50 Home has only %d free rows, want at least %d", free, sceneCornerMinRows)
	}
	rect, ok := homeSceneCornerRect(m, mainW, mainH)
	if !ok {
		t.Fatal("scene corner hidden on a 200x50 empty Home")
	}
	if rect.w != sceneCornerWidth {
		t.Fatalf("corner width = %d, want %d", rect.w, sceneCornerWidth)
	}
	if rect.x+rect.w != mainW {
		t.Fatalf("corner not right-anchored in the main pane: x=%d w=%d mainW=%d", rect.x, rect.w, mainW)
	}
	if rect.h < sceneCornerMinRows || rect.h > sceneCornerMaxRows {
		t.Fatalf("corner height = %d, want %d..%d", rect.h, sceneCornerMinRows, sceneCornerMaxRows)
	}
	if rect.y < used {
		t.Fatalf("corner y=%d overlaps band space (bands reach %d)", rect.y, used)
	}
	if rect.y+rect.h > used+free {
		t.Fatalf("corner y=%d h=%d escapes the free rows (end %d, free ends %d)", rect.y, rect.h, rect.y+rect.h, used+free)
	}
}

func TestSceneCornerNeverOverlapsIdeasBand(t *testing.T) {
	s := coursesFixture(t) // seeds open ideas, so the IDEAS band exists at w >= 110
	defer func() { _ = s.Close() }()
	for _, size := range [][2]int{{200, 50}, {240, 55}} {
		w, h := size[0], size[1]
		m := cornerModel(t, s, w, h)
		_, mainW := mainPaneGeometry(m)
		mainH := m.height - 1
		bands := homeLayout(m, mainW, mainH)
		irect, ideasOK := findBandRect(bands, "IDEAS")
		if !ideasOK {
			t.Fatalf("%dx%d: expected the IDEAS band for open ideas", w, h)
		}
		rect, cornerOK := homeSceneCornerRect(m, mainW, mainH)
		if !cornerOK {
			continue // hidden corner trivially overlaps nothing
		}
		if rect.y < irect.y+irect.h && irect.y < rect.y+rect.h {
			t.Fatalf("%dx%d: corner y=%d h=%d overlaps IDEAS y=%d h=%d", w, h, rect.y, rect.h, irect.y, irect.h)
		}
	}
}

// hexTriple parses #rrggbb into its components.
func hexTriple(hex string) (int, int, int, bool) {
	h := strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(h) == 3 {
		h = strings.Repeat(string(h[0]), 2) + strings.Repeat(string(h[1]), 2) + strings.Repeat(string(h[2]), 2)
	}
	if len(h) != 6 {
		return 0, 0, 0, false
	}
	var r, g, b int
	if _, err := fmt.Sscanf(h, "%02x%02x%02x", &r, &g, &b); err != nil {
		return 0, 0, 0, false
	}
	return r, g, b, true
}

// bgSeq returns the SGR sequence lipgloss emits when it renders a space on
// bg (matching what the chrome emits for that color).
func bgSeq(hex string) string {
	probe := lipgloss.NewStyle().Background(lipgloss.Color(hex)).Render(" ")
	start := strings.Index(probe, "48;2;")
	if start < 0 {
		return ""
	}
	end := strings.Index(probe[start:], "m")
	if end < 0 {
		return ""
	}
	return probe[start : start+end+1]
}

var scratchSGR = regexp.MustCompile("\x1b\\[([0-9;]*)m")

// bgAtCells walks the line ANSI-aware and returns the active background
// triple for each cell in [x0, x1).
func bgAtCells(line string, x0, x1 int) []string {
	cur := ""
	out := []string{}
	x := 0
	rest := line
	for len(rest) > 0 {
		var text, seq string
		loc := scratchSGR.FindStringIndex(rest)
		if loc == nil {
			text = rest
			rest = ""
		} else {
			text = rest[:loc[0]]
			seq = rest[loc[0]:loc[1]]
			rest = rest[loc[1]:]
		}
		if seq != "" {
			parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(seq, "\x1b["), "m"), ";")
			for j := 0; j+4 < len(parts); j++ {
				if parts[j] == "48" && parts[j+1] == "2" {
					cur = strings.Join(parts[j+2:j+5], ";")
				}
			}
		}
		w := ansi.StringWidth(text)
		if w > 0 {
			for k := 0; k < w; k++ {
				if x+k >= x0 && x+k < x1 {
					out = append(out, cur)
				}
			}
			x += w
		}
	}
	return out
}

// TestSceneCornerSitsInsideSurfaceBand: the ambient panel paints entirely
// within the Surface slab band — no Base-colored cells in its rows. Both the
// raw scene-renderer Base spelling and the lipgloss chrome Base spelling are
// checked, because the two pipelines were emitting different triples for the
// same hex (the visible seam).
func TestSceneCornerSitsInsideSurfaceBand(t *testing.T) {
	s := emptyFixture(t)
	defer func() { _ = s.Close() }()
	m := cornerModel(t, s, 200, 50)
	_, mainW := mainPaneGeometry(m)
	paneX, _ := mainPaneGeometry(m)
	rect, ok := homeSceneCornerRect(m, mainW, m.height-1)
	if !ok {
		t.Fatal("no scene corner on a 200x50 empty Home")
	}
	p := theme.Defaults()
	r, g, b, ok := hexTriple(p.Base)
	if !ok {
		t.Fatalf("unparsable Base %q", p.Base)
	}
	baseRaw := fmt.Sprintf("%d;%d;%d", r, g, b)
	if _, err := fmt.Sscanf(bgSeq(p.Base), "48;2;%d;%d;%d", &r, &g, &b); err != nil {
		t.Fatalf("unparsable lipgloss Base sequence %q", bgSeq(p.Base))
	}
	baseChrome := fmt.Sprintf("%d;%d;%d", r, g, b)
	frame := m.View()
	rows := strings.Split(frame, "\n")
	for i := 0; i < rect.h; i++ {
		line := rows[3+rect.y+i]
		for _, cellBG := range bgAtCells(line, paneX+rect.x, paneX+rect.x+rect.w) {
			if cellBG == baseRaw || cellBG == baseChrome {
				t.Errorf("corner row %d carries the Base background — panel straddles the slab band:\n%s", 3+rect.y+i, StripANSI(line))
				break
			}
		}
	}
}

// §5.2: the corner is now an optional larger panel — hidden while the
// sidebar window is on; the corner-only surface (ambient=home) and the
// sidebar-family with the window toggled off still show it.

func ambientModel(t *testing.T, s *store.Store, mutate func(*config.Config), w, h int) Model {
	t.Helper()
	cfg := dumpConfig()
	cfg.UI.Scene = "night"
	mutate(&cfg)
	return BuildDumpModel(cfg, s, sync.New(s.Root), "home", w, h, DumpFixedTime)
}

func normSpaces(s string) string {
	return strings.Join(strings.FieldsFunc(s, unicode.IsSpace), " ")
}

func frameHasCorner(frame string) bool {
	return strings.Contains(normSpaces(StripANSI(frame)), "A M B I E N T")
}

func frameHasSidebarWindow(frame string, sideW int) bool {
	_, _, ok := windowSpan(frame, sideW)
	return ok
}

func TestCornerHiddenWhenSidebarWindowOn(t *testing.T) {
	s := emptyFixture(t)
	defer func() { _ = s.Close() }()
	m := ambientModel(t, s, func(c *config.Config) {}, 200, 50)
	frame := m.View()
	if frameHasCorner(frame) {
		t.Fatal("corner rendered while the sidebar window is on")
	}
	if !frameHasSidebarWindow(frame, sideWidth(m.width, m.sidebarHidden)) {
		t.Fatal("sidebar window missing at default config")
	}
}

func TestCornerShownWhenSidebarWindowToggledOff(t *testing.T) {
	s := emptyFixture(t)
	defer func() { _ = s.Close() }()
	m := ambientModel(t, s, func(c *config.Config) { c.UI.AmbientSidebar = false }, 200, 50)
	frame := m.View()
	if !frameHasCorner(frame) {
		t.Fatal("corner hidden even though the sidebar window is off and ambient=both")
	}
	if frameHasSidebarWindow(frame, sideWidth(m.width, m.sidebarHidden)) {
		t.Fatal("sidebar window rendered while ui.ambient_sidebar is false")
	}
}

func TestCornerShownOnlyForHomeSurface(t *testing.T) {
	for _, tc := range []struct {
		ambient string
		want    bool
	}{
		{"home", true},
		{"both", false},
		{"sidebar", false},
		{"splash", false},
		{"off", false},
	} {
		t.Run(tc.ambient, func(t *testing.T) {
			s := emptyFixture(t)
			defer func() { _ = s.Close() }()
			m := ambientModel(t, s, func(c *config.Config) { c.UI.Ambient = tc.ambient }, 200, 50)
			if got := frameHasCorner(m.View()); got != tc.want {
				t.Errorf("ambient=%s: corner visible=%v, want %v", tc.ambient, got, tc.want)
			}
		})
	}
}

func TestCornerIsTheLargerOptionalPanel(t *testing.T) {
	s := emptyFixture(t)
	defer func() { _ = s.Close() }()
	m := ambientModel(t, s, func(c *config.Config) { c.UI.Ambient = "home" }, 200, 50)
	_, mainW := mainPaneGeometry(m)
	rect, ok := homeSceneCornerRect(m, mainW, m.height-1)
	if !ok {
		t.Fatal("corner rect missing on a 200x50 empty Home")
	}
	if rect.w != sceneCornerWidth {
		t.Errorf("corner width = %d, want %d", rect.w, sceneCornerWidth)
	}
	if sceneCornerWidth <= 46 {
		t.Errorf("sceneCornerWidth = %d, want the larger panel (> 46)", sceneCornerWidth)
	}
	if rect.h > sceneCornerMaxRows {
		t.Errorf("corner height = %d, cap is %d", rect.h, sceneCornerMaxRows)
	}
}
