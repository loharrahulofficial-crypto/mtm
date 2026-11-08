package gauges

import (
	"strings"
	"testing"
)

func TestThinBarBasic(t *testing.T) {
	fill, track := ThinBar(0.5, 8)
	if fill != "▄▄▄▄" || track != "▄▄▄▄" {
		t.Fatalf("0.5 of 8: fill=%q track=%q", fill, track)
	}
	fill, track = ThinBar(1, 8)
	if fill != "▄▄▄▄▄▄▄▄" || track != "" {
		t.Fatalf("1.0 of 8: fill=%q track=%q", fill, track)
	}
	fill, track = ThinBar(0, 8)
	if fill != "" || track != "▄▄▄▄▄▄▄▄" {
		t.Fatalf("0 of 8: fill=%q track=%q", fill, track)
	}
	// Any non-zero fraction leaves a visible sliver.
	fill, _ = ThinBar(0.01, 8)
	if fill != "▄" {
		t.Fatalf("0.01 of 8: fill=%q, want one filled cell", fill)
	}
	// Clamps out-of-range input instead of corrupting the grid.
	fill, track = ThinBar(1.7, 4)
	if fill != "▄▄▄▄" || track != "" {
		t.Fatalf("over-range: fill=%q track=%q", fill, track)
	}
}

func TestThinBarCellCount(t *testing.T) {
	for _, tc := range []struct {
		frac  float64
		cells int
	}{{0, 5}, {0.13, 9}, {0.5, 8}, {0.99, 10}, {1, 6}} {
		fill, track := ThinBar(tc.frac, tc.cells)
		if got := len([]rune(fill + track)); got != tc.cells {
			t.Errorf("frac=%v cells=%d: rendered %d cells", tc.frac, tc.cells, got)
		}
	}
}

func TestThinBarNeverFullHeight(t *testing.T) {
	// The whole point of the thin glyph: nothing renders a full block.
	fill, _ := ThinBar(1, 12)
	if strings.Contains(fill, "█") {
		t.Fatalf("thin bar rendered a full block: %q", fill)
	}
}
