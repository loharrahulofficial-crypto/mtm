package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

// TestSidebarExamChipWordBoundary pins the pinned exam chip: it always
// fits the sidebar width, keeps the date label, and truncates the title
// at a whole-word boundary with an ellipsis — never mid-word — at sidebar
// widths 20 through 32. Regression for "sidebar pinned exam clipping".
func TestSidebarExamChipWordBoundary(t *testing.T) {
	for w := 20; w <= 32; w++ {
		s := fixtures["exams"](t)
		m := New(dumpConfig(), s, sync.New(s.Root))
		chip := nearestExamChip(m, w)
		_ = s.Close()
		plain := StripANSI(chip)
		if got := lipgloss.Width(plain); got > w {
			t.Errorf("w=%d: chip is %d cells: %q", w, got, plain)
		}
		if !strings.Contains(plain, "Nov 13") {
			t.Errorf("w=%d: chip lost the date label: %q", w, plain)
		}
		// Title portion starts after the " · " separator.
		idx := strings.Index(plain, "· ")
		if idx < 0 {
			t.Fatalf("w=%d: chip has no separator: %q", w, plain)
		}
		title := strings.TrimPrefix(plain[idx:], "· ")
		full := "DBMS midterm — relational algebra + SQL"
		if !strings.HasSuffix(title, "…") {
			// Short enough to fit whole? Only when the budget allows.
			if lipgloss.Width(full) <= w-13 {
				if title != full {
					t.Errorf("w=%d: full title expected to fit but got %q", w, title)
				}
			}
			continue
		}
		prefix := strings.TrimSuffix(title, "…")
		if !strings.HasPrefix(full, prefix) {
			t.Errorf("w=%d: truncated title %q is not a prefix of the full title", w, title)
			continue
		}
		// Word boundary: the prefix must be exactly the first k words.
		words := strings.Fields(full)
		whole := false
		joined := ""
		for k := 1; k <= len(words); k++ {
			joined = strings.Join(words[:k], " ")
			if prefix == joined {
				whole = true
				break
			}
		}
		if !whole {
			t.Errorf("w=%d: title cut mid-word (last-resort char cut used where a word fits): %q", w, title)
		}
	}
}
