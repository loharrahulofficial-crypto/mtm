package tui

import (
	"strings"
	"testing"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

// ambient_sidebar_test.go — the sidebar ambient window (spec 5.1): a framed
// scene panel pinned below the course list on every view, filling only the
// free rows. Minimum 8 rows incl. the borders; collapses to nothing on
// short terminals; ui.ambient_sidebar: false hides it entirely.

func nightDumpConfig() config.Config {
	cfg := dumpConfig()
	cfg.UI.Scene = "night"
	cfg.UI.AmbientSidebar = true
	return cfg
}

// windowSpan finds the ambient window's ╭..╰ row range within the sidebar
// region, by border geometry (any scene title).
func windowSpan(frame string, sideW int) (top, bottom int, ok bool) {
	lines := strings.Split(frame, "\n")
	sideRunes := func(l string) []rune {
		return []rune(StripANSI(ansi.Cut(l, 0, sideW)))
	}
	isTopEdge := func(cells []rune) bool {
		// 1-cell pad, then ╭ ... ╮ at the far cell
		return len(cells) >= 4 && cells[1] == '╭' && cells[len(cells)-2] == '╮'
	}
	for i, l := range lines {
		if !isTopEdge(sideRunes(l)) {
			continue
		}
		for j := i + 1; j < len(lines); j++ {
			cells := sideRunes(lines[j])
			if len(cells) >= 2 && cells[1] == '╰' {
				return i, j, true
			}
		}
		return i, -1, false
	}
	return 0, 0, false
}

func TestSidebarAmbientWindowShowsOnEveryView(t *testing.T) {
	for _, view := range []string{"home", "list", "board", "calendar", "courses"} {
		t.Run(view, func(t *testing.T) {
			s := coursesFixture(t)
			defer func() { _ = s.Close() }()
			m := BuildDumpModel(nightDumpConfig(), s, sync.New(s.Root), view, 140, 40, DumpFixedTime)
			frame := m.View()
			sideW := sideWidth(m.width, m.sidebarHidden)
			top, bottom, ok := windowSpan(frame, sideW)
			if !ok {
				t.Fatalf("view %s: no ambient window in a 140x40 frame", view)
			}
			if bottom-top+1 < ambientSidebarMinRows {
				t.Errorf("view %s: window is %d rows, want at least %d", view, bottom-top+1, ambientSidebarMinRows)
			}
		})
	}
}

func TestSidebarAmbientWindowBelowCourseList(t *testing.T) {
	s := coursesFixture(t)
	defer func() { _ = s.Close() }()
	m := BuildDumpModel(nightDumpConfig(), s, sync.New(s.Root), "home", 160, 45, DumpFixedTime)
	sideW := sideWidth(m.width, m.sidebarHidden)
	innerH := m.height - 1 - 2
	_, rendered, _, winH := sidebarRowPlan(m, sideW-2, innerH)
	if winH < ambientSidebarMinRows {
		t.Fatalf("window plan is %d rows, want >= %d", winH, ambientSidebarMinRows)
	}
	frame := m.View()
	lines := strings.Split(frame, "\n")
	// unfocused panels paint one plain surface row above the body
	winTop := 1 + brandLines + rendered
	if winTop >= len(lines) {
		t.Fatalf("window top row %d beyond the frame", winTop)
	}
	normLine := strings.Join(strings.FieldsFunc(StripANSI(ansi.Cut(lines[winTop], 0, sideW)), func(r rune) bool { return unicode.IsSpace(r) }), " ")
	if !strings.Contains(normLine, "╭─ N I G H T") {
		t.Errorf("row %d is not the window top edge: %q", winTop, StripANSI(lines[winTop]))
	}
	// every content row of the window carries both border cells (1-cell pads
	// on either side of the panel inside the sidebar)
	for r := winTop + 1; r < winTop+winH-1; r++ {
		cells := []rune(StripANSI(ansi.Cut(lines[r], 0, sideW)))
		if len(cells) != sideW {
			t.Fatalf("row %d: sidebar is %d cells, want %d", r, len(cells), sideW)
		}
		if cells[1] != '│' || cells[sideW-2] != '│' {
			t.Errorf("row %d: window borders at cells 1/%d are %q/%q, want │/│",
				r, sideW-2, cells[1], cells[sideW-2])
		}
	}
}

func TestSidebarAmbientWindowCollapsesOnShortTerminals(t *testing.T) {
	for h := 18; h <= 30; h++ {
		s := emptyFixture(t)
		func() {
			defer func() { _ = s.Close() }()
			m := BuildDumpModel(nightDumpConfig(), s, sync.New(s.Root), "home", 140, h, DumpFixedTime)
			sideW := sideWidth(m.width, m.sidebarHidden)
			innerH := h - 1 - 2
			_, _, _, winH := sidebarRowPlan(m, sideW-2, innerH)
			frame := m.View()
			_, _, ok := windowSpan(frame, sideW)
			if winH < ambientSidebarMinRows && ok {
				t.Errorf("h=%d: window shown with winH=%d (plan says it does not fit)", h, winH)
			}
			if winH >= ambientSidebarMinRows && !ok {
				t.Errorf("h=%d: window hidden with winH=%d (plan says it fits)", h, winH)
			}
		}()
	}
}

func TestSidebarAmbientWindowToggle(t *testing.T) {
	// ui.ambient_sidebar: false hides the window entirely.
	s := emptyFixture(t)
	defer func() { _ = s.Close() }()
	m := BuildDumpModel(func() config.Config {
		cfg := nightDumpConfig()
		cfg.UI.AmbientSidebar = false
		return cfg
	}(), s, sync.New(s.Root), "home", 140, 40, DumpFixedTime)
	sideW := sideWidth(m.width, m.sidebarHidden)
	if _, _, ok := windowSpan(m.View(), sideW); ok {
		t.Fatal("window shown with ui.ambient_sidebar: false")
	}
	// ui.ambient: off hides it too.
	m2 := BuildDumpModel(func() config.Config {
		cfg := nightDumpConfig()
		cfg.UI.Ambient = "off"
		return cfg
	}(), s, sync.New(s.Root), "home", 140, 40, DumpFixedTime)
	if _, _, ok := windowSpan(m2.View(), sideW); ok {
		t.Fatal("window shown with ui.ambient: off")
	}
}

func TestSidebarAmbientWindowEveryLineWidth(t *testing.T) {
	s := emptyFixture(t)
	defer func() { _ = s.Close() }()
	m := BuildDumpModel(nightDumpConfig(), s, sync.New(s.Root), "home", 140, 40, DumpFixedTime)
	sideW := sideWidth(m.width, m.sidebarHidden)
	innerH := m.height - 1 - 2
	_, _, _, winH := sidebarRowPlan(m, sideW-2, innerH)
	win := m.ambientSidebarWindow(sideW-2, winH)
	if win == nil {
		t.Fatal("window nil for a 140x40 empty Home")
	}
	for i, l := range win {
		if got := lipgloss.Width(l); got != sideW-2 {
			t.Errorf("line %d width = %d, want %d", i, got, sideW-2)
		}
	}
}

func TestSidebarRowPlanTilesPanel(t *testing.T) {
	// brand + rows + window + footer must tile the panel exactly.
	for _, h := range []int{28, 32, 36, 40, 45, 50} {
		s := coursesFixture(t)
		m := BuildDumpModel(nightDumpConfig(), s, sync.New(s.Root), "home", 140, h, DumpFixedTime)
		sideW := sideWidth(m.width, m.sidebarHidden)
		innerH := h - 1 - 2
		_, rendered, footer, winH := sidebarRowPlan(m, sideW-2, innerH)
		if sum := brandLines + rendered + winH + footer; sum != innerH {
			t.Errorf("h=%d: brand(%d)+rows(%d)+window(%d)+footer(%d) = %d, want %d",
				h, brandLines, rendered, winH, footer, sum, innerH)
		}
		_ = s.Close()
	}
}
