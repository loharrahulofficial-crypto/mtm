package tui

import (
	"testing"

	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

// Spec 0c.2: at a tall terminal the UP NEXT band grows to show up to 8
// tasks (8 rows + panel chrome); at short terminals it stays bounded by
// the blank-ratio cap.
func TestUpNextGrowsAtTallSizes(t *testing.T) {
	for _, tc := range []struct {
		name string
		w, h int
	}{
		{"200x60", 200, 60},
		{"120x60", 120, 60},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := manyTasksFixture(t)
			defer func() { _ = s.Close() }()
			m := BuildDumpModel(dumpConfig(), s, sync.New(s.Root), "home", tc.w, tc.h, DumpFixedTime)
			_, mainW := mainPaneGeometry(m)
			for _, band := range homeLayout(m, mainW, tc.h-1) {
				for _, r := range band {
					if r.title == "UP NEXT" {
						if r.h < 10 {
							t.Errorf("%dx%d: UP NEXT band h = %d, want >= 10 (8 tasks + chrome)", tc.w, tc.h, r.h)
						}
						return
					}
				}
			}
			t.Errorf("%dx%d: no UP NEXT band found", tc.w, tc.h)
		})
	}
}
