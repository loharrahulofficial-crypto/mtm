package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/loharrahulofficial-crypto/mtm/internal/clock"
	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

// Bug report: created/updated/completed stamps, calendar cells, and the
// statusline clock were computed from scattered bare time.Now() calls, so a
// stored timestamp and a displayed clock could disagree. All of them must
// flow from the one injectable Clock (D-011).
func TestStoredStampsMatchFixedClock(t *testing.T) {
	fixed := time.Date(2026, 11, 7, 14, 32, 45, 0, time.Local)
	s := newFixedStore(t, fixed)

	task := &core.Task{Title: "write the thing", Project: "inbox", Status: "todo", Kind: core.KindTask}
	if err := s.Create(task); err != nil {
		t.Fatal(err)
	}
	want := fixed.Format(core.TimestampFormat)
	if task.Created != want {
		t.Errorf("created = %q, want %q", task.Created, want)
	}
	if task.Updated != want {
		t.Errorf("updated = %q, want %q", task.Updated, want)
	}

	if _, err := actions.CompleteTask(s, task); err != nil {
		t.Fatal(err)
	}
	if task.Completed != want {
		t.Errorf("completed = %q, want %q", task.Completed, want)
	}
	if task.Updated != want {
		t.Errorf("updated after done = %q, want %q", task.Updated, want)
	}
}

// The TUI's displayed wall clock must be the same clock that stamps the
// files — a screenshot of the statusline and the on-disk frontmatter can
// never drift apart.
func TestDisplayedClockMatchesStoredStamps(t *testing.T) {
	fixedEnv(t)
	fixed := time.Date(2026, 11, 7, 14, 32, 45, 0, time.Local)
	s := newFixedStore(t, fixed)

	cfg := config.Default()
	cfg.Sync.Auto = false
	m := New(cfg, s, sync.New(s.Root))
	right := m.footerRight(200)
	if !strings.Contains(right, fixed.Format("15:04")) {
		t.Errorf("statusline = %q, want it to contain fixed clock %q", right, fixed.Format("15:04"))
	}
}

// Calendar "today" must come from the injected clock too — with a fixed
// clock set to 2026-11-07, cell 2026-11-07 renders as today even if the
// system date differs.
func TestCalendarTodayMatchesFixedClock(t *testing.T) {
	fixedEnv(t)
	fixed := time.Date(2026, 11, 7, 14, 32, 45, 0, time.Local)
	s := newFixedStore(t, fixed)

	cfg := config.Default()
	cfg.Sync.Auto = false
	m := New(cfg, s, sync.New(s.Root))
	today := m.clock.Today()
	if got := today.Format("2006-01-02"); got != "2026-11-07" {
		t.Fatalf("calendar today = %q, want 2026-11-07", got)
	}
	if sty := calendarCellStyle(m, today); !sameRender(sty, m.styles.dueToday) {
		t.Errorf("today cell lost the today-pill style under fixed clock")
	}
}

func newFixedStore(t *testing.T, fixed time.Time) *store.Store {
	t.Helper()
	s, err := store.Open(t.TempDir(), t.TempDir()+"/index.json")
	if err != nil {
		t.Fatal(err)
	}
	s.Clock = clock.Fixed(fixed)
	if err := s.SaveProject(core.DefaultProject("inbox")); err != nil {
		t.Fatal(err)
	}
	return s
}
