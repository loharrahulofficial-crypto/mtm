package tui

import (
	"strings"
	"testing"

	xansi "github.com/charmbracelet/x/ansi"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

// roadmap_test.go — the roadmap timeline bar (spec 0.4): a completed
// milestone's bar renders no track glyphs, and timestamped created dates in
// milestone frontmatter parse (previously they silently zeroed the created
// day, so every bar spanned the whole window).

func barRow(t *testing.T, frame, contains string) string {
	t.Helper()
	for _, line := range strings.Split(frame, "\n") {
		plain := xansi.Strip(line)
		if strings.Contains(plain, contains) && (strings.ContainsRune(plain, '█') || strings.ContainsRune(plain, '░')) {
			return plain
		}
	}
	t.Fatalf("no bar row containing %q in frame:\n%s", contains, frame)
	return ""
}

func TestRoadmapDoneMilestoneBarHasNoStrayTrack(t *testing.T) {
	s := projectToolsFixture(t)
	defer func() { _ = s.Close() }()
	// Mark v0.1 — study MVP (0 of 2 tasks done) as done; its timeline bar
	// must render full, with no stray ░ next to the today marker.
	for _, ms := range s.Milestones("college/dbms") {
		if ms.ID == "ms0001" {
			ms.Status = "done"
			if err := s.UpdateMilestone(ms); err != nil {
				t.Fatal(err)
			}
		}
	}
	m := BuildDumpModel(dumpConfig(), s, sync.New(s.Root), "roadmap", 120, 40, DumpFixedTime)
	row := barRow(t, m.View(), "study MVP")
	if strings.ContainsRune(row, '░') {
		t.Fatalf("done milestone bar contains a stray ░ glyph:\n%s", row)
	}
	if !strings.ContainsRune(row, '┃') {
		t.Fatalf("today marker missing from the completed bar:\n%s", row)
	}
}

func TestRoadmapBarStartsAtCreationDate(t *testing.T) {
	s := projectToolsFixture(t)
	defer func() { _ = s.Close() }()
	// Both fixture milestones carry timestamped created dates
	// ("2026-...T09:00"); before the parse fix their bars started at the
	// window start instead of the creation day.
	m := BuildDumpModel(dumpConfig(), s, sync.New(s.Root), "roadmap", 120, 40, DumpFixedTime)
	row := barRow(t, m.View(), "study MVP")
	barCell := strings.IndexAny(row, "█░")
	if barCell < 0 {
		t.Fatalf("no bar cells in row:\n%s", row)
	}
	// ms0001 was created 18 days before DumpFixedTime, which is deep into
	// the 12-week window: the bar must not reach the label cell edge.
	if barCell <= 24 { // labelW+1 = 23 at w=120 (labelW capped at 22)
		t.Fatalf("bar starts at cell %d — created date was not honored:\n%s", barCell, row)
	}
}
