package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rivo/uniseg"

	"github.com/loharrahulofficial-crypto/mtm/internal/gauges"
	"github.com/loharrahulofficial-crypto/mtm/internal/mascot"
)

// guide.go — Mo's guided tour overlay: opening state, typewriter pacing,
// coach-card placement, and key handling. Mo the mascot (internal/mascot)
// plays in the card frame next to this; the card itself fades in by the
// 200ms guide entrance (Phase 4).

type guideSpotlight int

const (
	spotNone guideSpotlight = iota
	spotSidebar
	spotMain
	spotDetail
	spotFooter
	spotHeader
)

type guideStep struct {
	Title     string
	Body      string
	Pose      mascot.Pose
	Spotlight guideSpotlight
	Keycap    string // primary shortcut pill, e.g. "2" or "ctrl+b"
}

var guideSteps = []guideStep{
	{Title: "Welcome to mtm!", Body: "Hi — I'm Mo, the desktop assistant. This tour walks you through the key spots.", Pose: mascot.Wave, Spotlight: spotSidebar},
	{Title: "The sidebar", Body: "Toggle it with ctrl+b. Collapses on narrow screens.", Pose: mascot.Keycap, Spotlight: spotSidebar, Keycap: "ctrl+b"},
	{Title: "The main panel", Body: "This panel holds your list, calendar and board — everything you do here.", Pose: mascot.Point, Spotlight: spotMain},
	{Title: "Mark tasks done", Body: "Move to a task and press x to mark it done.", Pose: mascot.Tasks, Spotlight: spotMain},
	{Title: "Try the board: 2", Body: "The board gives you a column view — drag cards between Done / Progress / To Do.", Pose: mascot.Pomo, Spotlight: spotMain, Keycap: "2"},
	{Title: "Focus mode: F", Body: "Focus gives you a full-screen Pomodoro for deep work.", Pose: mascot.Idea, Spotlight: spotFooter, Keycap: "F"},
	{Title: "You're all set!", Body: "Press enter or n to move on. esc closes the tour.", Pose: mascot.Idle, Spotlight: spotNone},
}

const guideTypingRate = 55 // chars/sec

func guideStepCount() int { return len(guideSteps) }

// guideState is the mutable guide overlay state.
type guideState struct {
	Step     int
	Pose     mascot.Pose // Mo pose for the current step
	Last     time.Time   // step start
	Revealed bool        // enter/space: reveal the full line, wait for advance

	cardX, cardY       int       // current coach-card position (interpolated)
	cardDstX, cardDstY int       // target card position
	cardMoveStart      time.Time // when interpolation started
	cardMoving         bool      // true while the card interpolates between steps
}

func (m *Model) openGuide() {
	m.mode = modeGuide
	m.guide.Step = 0
	m.guide.Pose = guideSteps[0].Pose
	m.guide.Last = m.clock.Now()
	m.guide.Revealed = false
	dstX, dstY := m.guideDock(m.width, m.height)
	m.guide.cardX, m.guide.cardY = dstX, dstY
	m.guide.cardDstX, m.guide.cardDstY = dstX, dstY
	m.guide.cardMoveStart = m.clock.Now()
	m.guide.cardMoving = false
}

// guideText returns the typable body of the current step, revealed so far.
func (m Model) guideText() string {
	if m.guide.Step >= len(guideSteps) {
		return ""
	}
	full := guideSteps[m.guide.Step].Body
	if m.guide.Revealed {
		return full
	}
	elapsed := m.clock.Now().Sub(m.guide.Last)
	if elapsed <= 0 {
		return ""
	}
	n := int(elapsed.Seconds() * guideTypingRate)
	if n >= len(full) {
		return full
	}
	return graphemePrefix(full, n)
}

func graphemePrefix(s string, n int) string {
	if n <= 0 || n >= len(s) {
		return s
	}
	out := []byte{}
	count := 0
	for len(s) > 0 && count < n {
		cluster, rest, _, _ := uniseg.FirstGraphemeClusterInString(s, -1)
		out = append(out, cluster...)
		s = rest
		count++
	}
	return string(out)
}

// --- keys ---

func (m *Model) updateGuideKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.mode = modeNormal
		m.viewStamp++
		return m, nil
	case "enter", " ":
		if m.guide.Revealed {
			return m.advanceGuide()
		}
		m.guide.Revealed = true
		m.viewStamp++
		return m, m.armGuide()
	case "n", "right", "arrow-right":
		return m.advanceGuide()
	}
	return m, nil
}

func (m *Model) advanceGuide() (tea.Model, tea.Cmd) {
	if m.guide.Step+1 >= guideStepCount() {
		m.mode = modeNormal
		m.viewStamp++
		return m, nil
	}
	m.guide.Step++
	m.guide.Pose = guideSteps[m.guide.Step].Pose
	m.guide.Last = m.clock.Now()
	m.guide.Revealed = false
	dstX, dstY := m.guideDock(m.width, m.height)
	if m.guide.cardMoving || m.guide.cardDstX != dstX || m.guide.cardDstY != dstY {
		m.guide.cardMoving = true
		m.guide.cardDstX, m.guide.cardDstY = dstX, dstY
		m.guide.cardMoveStart = m.clock.Now()
	}
	m.viewStamp++
	return m, m.armGuide()
}

// --- tick + arming ---

type guideTickMsg struct{}

func guideTickCmd() tea.Cmd {
	return tea.Tick(42*time.Millisecond, func(time.Time) tea.Msg { return guideTickMsg{} })
}

func (m *Model) armGuide() tea.Cmd {
	if m.mode != modeGuide || !m.ambientFocused {
		m.guideArmed = false
		return nil
	}
	if m.reducedMotionActive() {
		m.guideArmed = false
		return nil
	}
	if m.guideArmed {
		return nil
	}
	m.guideArmed = true
	return guideTickCmd()
}

func (m *Model) handleGuideTick() (tea.Model, tea.Cmd) {
	if m.mode != modeGuide {
		m.guideArmed = false
		return m, nil
	}
	if !m.ambientFocused || m.reducedMotionActive() {
		m.guideArmed = false
		return m, nil
	}
	m.handleGuideCardMove()
	return m, guideTickCmd()
}

// --- render ---

// guideCardText draws the coach card text (title, keycap pill, body, progress).
func (m Model) guideCardText(w int) string {
	s := m.styles
	var b strings.Builder
	// Text block: title, body, keycap pill, progress.
	var t strings.Builder
	t.WriteString(s.paneTitle.Render("  " + guideSteps[m.guide.Step].Title + "  "))
	t.WriteString("\n\n")
	body := wrapLines(strings.TrimRight(m.guideText(), "\n"), w-2, 6)
	t.WriteString(s.surface.Render(strings.Join(body, "\n")))
	if k := guideSteps[m.guide.Step].Keycap; k != "" {
		t.WriteString("\n\n")
		t.WriteString(s.badge(m.palette.BrightText, m.palette.SurfaceStrong).Render("[" + k + "]"))
	}
	t.WriteString("\n\n")
	fill, track := gauges.ThinBar(m.guideProgress(), minInt(w-8, 12))
	t.WriteString(s.accent.Render(fill) + s.muted.Render(track))
	b.WriteString(t.String())
	// Mo avatar below the text.
	for _, ln := range m.guideMoLines() {
		b.WriteString("\n")
		b.WriteString(ln)
	}
	return b.String()
}

// guideMoLines renders Mo as halfblock terminal lines for the card avatar.
func (m Model) guideMoLines() []string {
	r := mascot.Render(m.guide.Pose, m.palette)
	return r.EmitHalfblock(m.palette.Base, false, true, true)
}

func (m Model) guideProgress() float64 {
	if len(guideSteps) == 0 {
		return 0
	}
	return float64(m.guide.Step) / float64(len(guideSteps)-1)
}

func sidebarW(w int, hidden bool) int {
	return sideWidth(w, hidden)
}

func (m Model) guideDock(w, h int) (x, y int) {
	cardW := m.guideCardWidth(w)
	cardH := m.guideCardHeight(w, h)
	spot := guideSteps[m.guide.Step].Spotlight
	sw := m.sidebarW()
	if spot == spotSidebar && sw > 0 {
		x, y = sw+1, h/4
		if x+cardW > w {
			x = w - cardW
		}
	} else {
		x, y = (w-cardW)/2, (h-cardH)/2
	}
	return maxInt(x, 0), maxInt(y, 0)
}

func (m Model) guideCardRect(w, h int) (x, y int) {
	if m.guide.cardMoving {
		return m.guide.cardX, m.guide.cardY
	}
	return m.guideDock(w, h)
}

func (m Model) sidebarW() int { return sideWidth(m.width, m.sidebarHidden) }

func (m Model) guideCardWidth(w int) int { return minInt(w-4, 60) }
func (m Model) guideCardHeight(w, h int) int {
	mo := 0
	if w >= 60 {
		mo = len(m.guideMoLines())
	}
	if mo > 0 {
		cardH := minInt(h-4, 32)
		if cardH < mo {
			cardH = mo
		}
		return cardH
	}
	return 16
}

func (m Model) renderGuide(w, h int) string {
	base := normalizeBlock(m.viewInnerNoGuide(), w, h, lipgloss.Color(m.palette.Base))
	x, y := m.guideCardRect(w, h)
	cardW := m.guideCardWidth(w)
	cardH := m.guideCardHeight(w, h)
	card := normalizeBlock(m.guideCardText(cardW), cardW, cardH, lipgloss.Color(m.palette.Base))
	cardLines := strings.Split(card, "\n")
	lines := strings.Split(base, "\n")
	lines = dimLinesOutsideRegion(lines, m.guideSpotRegion(w, h, m.guide.Step), hexFadeSGR(m.palette.Base), m.guideDimAmount())
	for i := 0; i < cardH && y+i < len(lines); i++ {
		var cl string
		if i < len(cardLines) {
			cl = cardLines[i]
		}
		lines[y+i] = spliceLine(lines[y+i], x, w, cl, lipgloss.Color(m.palette.Base))
	}
	drawGuideBeacon(m, lines, m.guideSpotRegion(w, h, m.guide.Step), m.clock.Now(), w)
	return strings.Join(lines, "\n")
}
