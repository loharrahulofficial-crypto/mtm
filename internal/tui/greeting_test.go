package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/loharrahulofficial-crypto/mtm/internal/store"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

// greeting_test.go — zero counts never render in the warning/danger heat
// colors on the Home greeting line. A non-zero count does heat-color.

func sgrSeq38(hex string) string {
	h := strings.TrimPrefix(strings.TrimSpace(hex), "#")
	var r, g, b int
	if n, _ := fmt.Sscanf(h, "%02x%02x%02x", &r, &g, &b); n != 3 {
		var v uint64
		_, _ = fmt.Sscanf(h, "%x", &v)
		r = int((v >> 16) & 0xff)
		g = int((v >> 8) & 0xff)
		b = int(v & 0xff)
	}
	return fmt.Sprintf("38;2;%d;%d;%d", r, g, b)
}

func homeGreetingFrame(t *testing.T, s *store.Store) string {
	t.Helper()
	m := BuildDumpModel(dumpConfig(), s, sync.New(s.Root), "home", 140, 45, DumpFixedTime)
	frame := m.View()
	for _, l := range strings.Split(frame, "\n") {
		if strings.Contains(l, "Good afternoon") || strings.Contains(l, "Good morning") || strings.Contains(l, "Good evening") {
			return l
		}
	}
	t.Fatal("greeting line not found in the Home frame")
	return ""
}

func TestGreetingZeroCountsAreMuted(t *testing.T) {
	p := theme.Defaults()
	hot := sgrSeq38(p.HeatHot)
	over := sgrSeq38(p.HeatOverdue)
	s := emptyFixture(t)
	defer func() { _ = s.Close() }()
	line := homeGreetingFrame(t, s)
	if strings.Contains(line, hot) {
		t.Errorf("zero due-today count rendered in the hot heat color: %q", StripANSI(line))
	}
	if strings.Contains(line, over) {
		t.Errorf("zero overdue count rendered in the danger heat color: %q", StripANSI(line))
	}
}

func TestGreetingNonZeroCountsHeatColored(t *testing.T) {
	p := theme.Defaults()
	hot := sgrSeq38(p.HeatHot)
	over := sgrSeq38(p.HeatOverdue)
	s := twoTasksFixture(t) // one task due today, due already past 09:00 → timed-overdue
	defer func() { _ = s.Close() }()
	line := homeGreetingFrame(t, s)
	if !strings.Contains(line, hot) {
		t.Errorf("hot heat color missing for a non-zero due-today count: %q", StripANSI(line))
	}
	if !strings.Contains(line, over) {
		t.Errorf("danger heat color missing for a non-zero overdue count: %q", StripANSI(line))
	}
}
