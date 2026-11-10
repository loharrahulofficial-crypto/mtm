package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/gauges"
)

// Ideas view (key 6): incubation cards grouped by status.
// Seed → exploring → parked → dropped cycle with `s`; promoted ideas stay
// readable here (they are what "promoted" means) until the user drops them.
var ideaStatusOrder = []string{core.IdeaSeed, core.IdeaExploring, core.IdeaParked, core.IdeaPromoted, core.IdeaDropped}

var ideaStatusLabels = map[string]string{
	core.IdeaSeed:      "S E E D",
	core.IdeaExploring: "E X P L O R I N G",
	core.IdeaParked:    "P A R K E D",
	core.IdeaPromoted:  "P R O M O T E D",
	core.IdeaDropped:   "D R O P P E D",
}

// ideaStatusColor maps a status to its stripe color (semantic palette roles).
func ideaStatusColor(m Model, status string) lipgloss.Color {
	switch strings.ToLower(status) {
	case core.IdeaSeed:
		return lipgloss.Color(m.palette.Warning)
	case core.IdeaExploring:
		return lipgloss.Color(m.palette.Accent)
	case core.IdeaParked:
		return lipgloss.Color(m.palette.MutedSoft)
	case core.IdeaPromoted:
		return lipgloss.Color(m.palette.Success)
	case core.IdeaDropped:
		return lipgloss.Color(m.palette.Danger)
	default:
		return lipgloss.Color(m.palette.AccentSoft)
	}
}

// ideaCards walks the store and returns the visible ideas, grouped in
// lifecycle order; within a group, most recently updated first.
func ideaCards(m Model) []*core.Idea {
	all := m.store.Ideas()
	out := []*core.Idea{}
	for _, status := range ideaStatusOrder {
		group := []*core.Idea{}
		for _, i := range all {
			if strings.EqualFold(i.Status, status) {
				group = append(group, i)
			}
		}
		for i := 1; i < len(group); i++ {
			for j := i; j > 0; j-- {
				if group[j].Updated > group[j-1].Updated {
					group[j], group[j-1] = group[j-1], group[j]
				}
			}
		}
		out = append(out, group...)
	}
	return out
}

// renderIdeasBody renders the panel content (inner width × inner height).
// Two lines per card plus a group header per non-empty group.
func renderIdeasBody(m Model, w, h int) []string {
	surface := lipgloss.Color(m.palette.Surface)
	base := lipgloss.NewStyle().Background(surface)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(surface)

	cards := ideaCards(m)
	if len(cards) == 0 {
		lines := []string{
			"",
			base.Render("   No ideas yet."),
			base.Render("   " + m.keymap.Seq(aAdd) + " captures one · " + m.keymap.Seq(aPalette) + " works from anywhere"),
			"",
		}
		for len(lines) < h {
			lines = append(lines, "")
		}
		return lines
	}

	// Flatten lines, remembering which line belongs to which card index.
	type span struct{ start, card int }
	var lineSpans []int // line index → card index (-1 for non-card lines)
	lines := []string{}

	lastStatus := ""
	cardIdx := -1
	for _, idea := range cards {
		status := strings.ToLower(idea.Status)
		if status != lastStatus {
			lastStatus = status
			count := 0
			for _, c := range cards {
				if strings.EqualFold(c.Status, status) {
					count++
				}
			}
			lines = append(lines,
				" "+muted.Render(ideaStatusLabels[status]+"  ·  "+fmt.Sprintf("%d", count)))
			lineSpans = append(lineSpans, -1)
		}
		cardIdx++
		selected := cardIdx == m.ideaSel
		stripeColor := ideaStatusColor(m, idea.Status)
		fill := surface
		if selected {
			fill = lipgloss.Color(m.palette.SurfaceRaised)
		}

		// Line 1: stripe, title, rating bar (unrated shows "—").
		ratingText := " — "
		if idea.Rating > 0 {
			barFill, barTrack := gauges.Bar(float64(idea.Rating)/5.0, 5)
			ratingText = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(fill).Render(barFill) +
				lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.SurfaceStrong)).Background(fill).Render(barTrack)
		}
		ratingW := 8
		titleW := w - 3 - ratingW - 1
		if titleW < 4 {
			titleW = 4
		}
		title := truncatePlain(idea.Title, titleW)
		stripe := lipgloss.NewStyle().Foreground(stripeColor).Background(fill).Render("▌")
		titleStyle := lipgloss.NewStyle().Background(fill).Foreground(lipgloss.Color(m.palette.Text))
		if selected {
			titleStyle = lipgloss.NewStyle().Background(fill).Foreground(lipgloss.Color(m.palette.BrightText)).Bold(true)
		}
		pad := w - 2 - lipgloss.Width(title) - 1 - ratingW - 1
		if pad < 0 {
			pad = 0
		}
		line1 := " " + stripe + " " + titleStyle.Render(title) + strings.Repeat(" ", pad) + ratingText + " "
		lines = append(lines, line1)
		lineSpans = append(lineSpans, cardIdx)

		// Line 2: age · tags · first body line (preview), muted.
		meta := " " + ideaAge(m, idea)
		if tags := ideaTagLine(idea.Tags, 3); tags != "" {
			meta += " · " + tags
		}
		if preview := ideaBodyPreview(idea.Body, w-lipgloss.Width(meta)-3); preview != "" {
			meta += " · " + preview
		}
		metaLine := "   " + lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(fill).Render(truncatePlain(meta, w-3))
		lines = append(lines, metaLine)
		lineSpans = append(lineSpans, cardIdx)
	}

	// Scroll so the selected card stays visible.
	selLine := -1
	for i, ci := range lineSpans {
		if ci == m.ideaSel {
			selLine = i
			break
		}
	}
	if selLine >= 0 {
		if m.ideaScroll > selLine {
			m.ideaScroll = selLine
		}
		if selLine >= m.ideaScroll+h {
			m.ideaScroll = selLine - h + 1
		}
	}
	if max := len(lines) - h; m.ideaScroll > max {
		m.ideaScroll = max
	}
	if m.ideaScroll < 0 {
		m.ideaScroll = 0
	}

	visible := lines[m.ideaScroll:]
	if len(visible) > h {
		visible = visible[:h]
	}
	for len(visible) < h {
		visible = append(visible, "")
	}
	return visible
}

// homeIdeas returns up to n open ideas (seed/exploring/parked), most recently
// updated first. Promoted and dropped ideas are noise for the Home glance.
func homeIdeas(m Model, n int) []*core.Idea {
	open := []*core.Idea{}
	for _, i := range m.store.Ideas() {
		switch strings.ToLower(i.Status) {
		case core.IdeaSeed, core.IdeaExploring, core.IdeaParked:
			open = append(open, i)
		}
	}
	for i := 1; i < len(open); i++ {
		for j := i; j > 0; j-- {
			if open[j].Updated > open[j-1].Updated {
				open[j], open[j-1] = open[j-1], open[j]
			}
		}
	}
	if len(open) > n {
		open = open[:n]
	}
	return open
}

// homeIdeasBody renders the compact Home band: one line per idea, stripe +
// title + rating bar. The panel clips/pads to its width.
func homeIdeasBody(m Model, w, h int) []string {
	surface := lipgloss.Color(m.palette.Surface)
	lines := []string{}
	for _, i := range homeIdeas(m, h) {
		fill := surface
		ratingText := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(fill).Render("  —  ")
		if i.Rating > 0 {
			barFill, barTrack := gauges.Bar(float64(i.Rating)/5.0, 5)
			ratingText = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(fill).Render(barFill) +
				lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.SurfaceStrong)).Background(fill).Render(barTrack)
		}
		ratingW := 7
		titleW := w - 3 - ratingW - 1
		if titleW < 4 {
			titleW = 4
		}
		title := truncatePlain(i.Title, titleW)
		stripe := lipgloss.NewStyle().Foreground(ideaStatusColor(m, i.Status)).Background(fill).Render("▌")
		titleStyle := lipgloss.NewStyle().Background(fill).Foreground(lipgloss.Color(m.palette.Text))
		pad := w - 2 - lipgloss.Width(title) - 1 - ratingW - 1
		if pad < 0 {
			pad = 0
		}
		lines = append(lines, " "+stripe+" "+titleStyle.Render(title)+strings.Repeat(" ", pad)+ratingText+" ")
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return lines
}

// selectedIdea returns the idea at the flattened selection index, or nil.
func (m Model) selectedIdea() *core.Idea {
	cards := ideaCards(m)
	if m.ideaSel < 0 || m.ideaSel >= len(cards) {
		return nil
	}
	return cards[m.ideaSel]
}

// findIdeaByID locates an idea in the current index by id.
func (m Model) findIdeaByID(id string) *core.Idea {
	for _, i := range m.store.Ideas() {
		if i.ID == id {
			return i
		}
	}
	return nil
}

// cycleIdeaStatus advances the visible lifecycle: seed → exploring →
// parked → dropped → seed. promoted_to is preserved either way.
func cycleIdeaStatus(cur string) string {
	order := []string{core.IdeaSeed, core.IdeaExploring, core.IdeaParked, core.IdeaDropped}
	for i, v := range order {
		if strings.EqualFold(v, cur) {
			return order[(i+1)%len(order)]
		}
	}
	return core.IdeaSeed
}

func (m Model) updateIdeasKey(act string) (tea.Model, tea.Cmd) {
	cards := ideaCards(m)
	clampSel := func() {
		if m.ideaSel < 0 {
			m.ideaSel = 0
		}
		if len(cards) > 0 && m.ideaSel >= len(cards) {
			m.ideaSel = len(cards) - 1
		}
	}
	switch act {
	case aUp:
		m.ideaSel--
	case aDown:
		m.ideaSel++
	case aTop:
		m.ideaSel = 0
	case aBottom:
		if len(cards) > 0 {
			m.ideaSel = len(cards) - 1
		}
	case aPageDown:
		m.ideaSel += 4
	case aPageUp:
		m.ideaSel -= 4
	case aAdd:
		m.openInput(inputIdea, "idea ▸ ", "")
		return m, nil
	case aRate:
		best := m.selectedIdea()
		if best == nil {
			m.statusMsg = "no idea selected"
			return m, nil
		}
		m.mode = modePicker
		m.picker = pickerRating
		m.pickerItems = []string{"1", "2", "3", "4", "5"}
		cur := 0
		if best.Rating > 0 {
			cur = best.Rating - 1
		}
		m.pickerSel = cur
		m.pickerTitle = "rate idea " + best.ID
		return m, nil
	case aStatus:
		i := m.selectedIdea()
		if i == nil {
			m.statusMsg = "no idea selected"
			return m, nil
		}
		next := cycleIdeaStatus(i.Status)
		canonical, err := actions.SetIdeaStatus(m.store, i, next)
		if err != nil {
			m.statusMsg = err.Error()
			return m, nil
		}
		m.statusMsg = "idea → " + canonical
		return m, m.afterChange("idea status: " + i.Title)
	case aPriority: // p — promote the selected idea to a project
		i := m.selectedIdea()
		if i == nil {
			m.statusMsg = "no idea selected"
			return m, nil
		}
		if strings.EqualFold(i.Status, core.IdeaPromoted) {
			m.statusMsg = "already promoted to " + i.PromotedTo
			return m, nil
		}
		m.mode = modeConfirm
		m.confirmTitle = "promote idea?"
		m.confirmBody = fmt.Sprintf("%s  %s — creates a project with the standard statuses, an initial milestone and links the idea to it", i.ID, i.Title)
		id := i.ID
		m.confirmYes = func(m *Model) tea.Cmd {
			now := m.clock.Now()
			res, err := actions.PromoteIdea(m.store, id, "", "", now)
			if err != nil {
				m.statusMsg = err.Error()
				return nil
			}
			m.statusMsg = fmt.Sprintf("promoted → %s (milestone %s, %d starter tasks)", res.Project, res.MilestoneID, len(res.TaskIDs))
			return m.afterChange("idea promote: " + res.Project)
		}
		return m, nil
	case aNotes:
		if i := m.selectedIdea(); i != nil {
			return m.openIdeaNotesEditor()
		}
		return m, nil
	case aNotesEditor:
		if i := m.selectedIdea(); i != nil {
			return m.openIdeaNotesInEditor()
		}
		return m, nil
	case aEnter:
		if i := m.selectedIdea(); i != nil {
			return m.openIdeaNotesEditor()
		}
		return m, nil
	}
	clampSel()
	return m, nil
}

// openIdeaNotesEditor opens the built-in notes editor on the selected idea.
func (m Model) openIdeaNotesEditor() (tea.Model, tea.Cmd) {
	i := m.selectedIdea()
	if i == nil {
		return m, nil
	}
	m.mode = modeNotes
	m.notesKind = "idea"
	m.notesID = i.ID
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.Placeholder = "Markdown notes…"
	ta.SetValue(i.Body)
	ta.CursorEnd()
	m.notes = ta
	m.notes.SetWidth(m.width - 4)
	m.notes.SetHeight(m.height - 4)
	m.notes.Focus()
	return m, nil
}

// openIdeaNotesInEditor opens $EDITOR on a temp file holding the idea's notes.
func (m Model) openIdeaNotesInEditor() (tea.Model, tea.Cmd) {
	i := m.selectedIdea()
	if i == nil {
		return m, nil
	}
	temp, err := os.CreateTemp("", "mtm-idea-notes-*.md")
	if err != nil {
		m.statusMsg = err.Error()
		return m, nil
	}
	if _, err := temp.WriteString(i.Body); err != nil {
		temp.Close()
		os.Remove(temp.Name())
		m.statusMsg = err.Error()
		return m, nil
	}
	temp.Close()
	m.notesKind = "idea"
	m.notesID = i.ID
	cmd := editorCommand(m.cfg.EditorCmd(), temp.Name())
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorDoneMsg{kind: "notes", path: temp.Name(), err: err}
	})
}
func ideaAge(m Model, i *core.Idea) string {
	ts, err := core.ParseDue(i.Updated, m.clock.Now().Location())
	if err != nil {
		return "new"
	}
	now := m.clock.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	day := time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, now.Location())
	days := int(today.Sub(day).Hours() / 24)
	switch {
	case days <= 0:
		return "today"
	case days == 1:
		return "1d ago"
	default:
		return fmt.Sprintf("%dd ago", days)
	}
}

// ideaTagLine renders "#a #b #c" (at most max tags).
func ideaTagLine(tags []string, max int) string {
	if len(tags) == 0 {
		return ""
	}
	out := []string{}
	for i, t := range tags {
		if i >= max {
			out = append(out, fmt.Sprintf("+%d", len(tags)-max))
			break
		}
		out = append(out, "#"+t)
	}
	return strings.Join(out, " ")
}

// ideaBodyPreview returns the first non-empty body line, or "".
func ideaBodyPreview(body string, maxWidth int) string {
	if maxWidth < 8 {
		return ""
	}
	for _, line := range strings.Split(body, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		return truncatePlain(trim, maxWidth)
	}
	return ""
}
