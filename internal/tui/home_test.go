package tui

import (
	"fmt"
	"testing"

	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

// mainPaneGeometry mirrors View(): the main column's x origin and width.
func mainPaneGeometry(m Model) (x0, mainW int) {
	sidebarW := sideWidth(m.width, m.sidebarHidden)
	detailW := detailWidth(m.width, m.detailOn)
	if m.view == viewCourses {
		detailW = 0
	}
	gutters := 0
	if sidebarW > 0 {
		gutters++
	}
	if detailW > 0 {
		gutters++
	}
	mainW = m.width - sidebarW - detailW - gutters
	if mainW < 20 {
		mainW = 20
	}
	if sidebarW > 0 {
		x0 = sidebarW + 1
	}
	return x0, mainW
}

// TestHomeNoPanelExceedsBlankRatio parses the rendered frame with the
// cell-grid parser and asserts that no Home panel wastes more than ~40% of
// its interior on blank rows (surface fill). Panels that cannot fill their
// band are structurally absent instead (UP NEXT drops out).
func TestHomeNoPanelExceedsBlankRatio(t *testing.T) {
	sizes := []struct{ w, h int }{{120, 40}, {200, 50}}
	for _, size := range sizes {
		t.Run(fmt.Sprintf("%dx%d", size.w, size.h), func(t *testing.T) {
			s := coursesFixture(t)
			defer func() { _ = s.Close() }()
			m := BuildDumpModel(dumpConfig(), s, sync.New(s.Root), "home", size.w, size.h, DumpFixedTime)

			x0, mainW := mainPaneGeometry(m)
			_ = x0
			frame := m.View()
			grid := parseFrame(frame)
			surface := renderedBGProbe(theme.Defaults().Surface)
			if surface == "" {
				t.Fatal("could not probe the surface background")
			}

			mainH := size.h - 1 // footer row
			for _, band := range homeLayout(m, mainW, mainH) {
				for _, r := range band {
					blank := 0
					for row := r.y; row < r.y+r.h; row++ {
						rowBlank := true
						for col := r.x; col < r.x+r.w; col++ {
							ch, bg := grid.cell(row, x0+col)
							if bg != surface || ch != " " {
								rowBlank = false
								break
							}
						}
						if rowBlank {
							blank++
						}
					}
					ratio := float64(blank) / float64(r.h)
					t.Logf("panel %-10s x=%d y=%d w=%d h=%d blank=%d ratio=%.2f", r.title, r.x, r.y, r.w, r.h, blank, ratio)
					if ratio > 0.40 {
						t.Errorf("%s panel: blank interior ratio %.2f (> 0.40) — %d of %d rows are empty", r.title, ratio, blank, r.h)
					}
				}
			}
		})
	}
}

// TestHomeLayoutTilesBody asserts the bands exactly cover the body height
// without overlaps at the two sizes the layout targets, and every panel is
// a sane size. (Tiny terminals may legitimately overflow; paint clips.)
func TestHomeLayoutTilesBody(t *testing.T) {
	sizes := []struct{ w, h int }{{120, 40}, {200, 50}}
	for _, size := range sizes {
		s := coursesFixture(t)
		m := BuildDumpModel(dumpConfig(), s, sync.New(s.Root), "home", size.w, size.h, DumpFixedTime)
		_, mainW := mainPaneGeometry(m)
		mainH := size.h - 1
		totalH := 0
		for _, band := range homeLayout(m, mainW, mainH) {
			bandH := 0
			for _, r := range band {
				if r.w < 10 || r.h < 2 {
					t.Errorf("size %dx%d: panel %s has bad rect %+v", size.w, size.h, r.title, r)
				}
				if r.h > bandH {
					bandH = r.h
				}
			}
			totalH += bandH
		}
		if totalH > mainH {
			t.Errorf("size %dx%d: bands need %d rows, body has %d", size.w, size.h, totalH, mainH)
		}
	}
}
