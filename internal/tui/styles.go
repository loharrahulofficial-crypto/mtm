package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

// styles bundles every lipgloss style the TUI uses, built once per theme
// load. No colors are hardcoded anywhere else in the TUI.
type styles struct {
	p theme.Palette

	base      lipgloss.Style
	dim       lipgloss.Style
	accent    lipgloss.Style
	border    lipgloss.Style
	paneTitle lipgloss.Style
	selected  lipgloss.Style
	done      lipgloss.Style
	header    lipgloss.Style
	footer    lipgloss.Style
	footerKey lipgloss.Style

	priorityUrgent lipgloss.Style
	priorityHigh   lipgloss.Style
	priorityMedium lipgloss.Style
	priorityLow    lipgloss.Style

	statusDone  lipgloss.Style
	statusDoing lipgloss.Style

	overdue  lipgloss.Style
	dueToday lipgloss.Style
	dueSoon  lipgloss.Style

	badge   func(fg, bg string) lipgloss.Style
	syncOK  lipgloss.Style
	syncBad lipgloss.Style
}

func newStyles(p theme.Palette) styles {
	sel := lipgloss.Color(p.Selection)
	bg := lipgloss.Color(p.Background)
	bgDark := lipgloss.Color(p.DarkBg)
	bgDarker := lipgloss.Color(p.DarkerBg)
	bgLight := lipgloss.Color(p.LightBg)
	fg := lipgloss.Color(p.Foreground)
	muted := lipgloss.Color(p.Muted)
	accent := lipgloss.Color(p.Accent)

	s := styles{p: p}
	s.base = lipgloss.NewStyle().Foreground(fg).Background(bg)
	s.dim = lipgloss.NewStyle().Foreground(muted).Background(bg)
	s.accent = lipgloss.NewStyle().Foreground(accent).Background(bg).Bold(true)
	s.border = lipgloss.NewStyle().BorderForeground(muted).Background(bg)
	s.paneTitle = lipgloss.NewStyle().Foreground(accent).Background(bg).Bold(true).Padding(0, 1)
	s.selected = lipgloss.NewStyle().Foreground(fg).Background(sel).Bold(true)
	s.done = lipgloss.NewStyle().Foreground(muted).Background(bg).Strikethrough(true)
	s.header = lipgloss.NewStyle().Foreground(fg).Background(bgDark).Bold(true).Padding(0, 1)
	s.footer = lipgloss.NewStyle().Foreground(muted).Background(bgDarker).Padding(0, 1)
	s.footerKey = lipgloss.NewStyle().Foreground(accent).Background(bgDarker).Bold(true).Padding(0, 1)

	mk := func(c string) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Background(bg).Bold(true)
	}
	s.priorityUrgent = mk(p.Red)
	s.priorityHigh = mk(p.Orange)
	s.priorityMedium = mk(p.Yellow)
	s.priorityLow = lipgloss.NewStyle().Foreground(muted).Background(bg)

	s.statusDone = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Green)).Background(bg)
	s.statusDoing = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Blue)).Background(bg)

	s.overdue = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Red)).Background(bg).Bold(true)
	s.dueToday = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Yellow)).Background(bg).Bold(true)
	s.dueSoon = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Cyan)).Background(bg)

	s.badge = func(fgc, bgc string) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(bgc)).Background(lipgloss.Color(fgc)).Bold(true).Padding(0, 1)
	}
	s.syncOK = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Green)).Background(bgDarker)
	s.syncBad = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Red)).Background(bgDarker).Bold(true)
	_ = bgLight
	return s
}

func (s styles) priority(p string) lipgloss.Style {
	switch strings.ToLower(p) {
	case "urgent":
		return s.priorityUrgent
	case "high":
		return s.priorityHigh
	case "medium":
		return s.priorityMedium
	default:
		return s.priorityLow
	}
}

func (s styles) priorityBadge(p string) string {
	label := strings.ToUpper(p)
	switch strings.ToLower(p) {
	case "urgent":
		label = "URG"
	case "high":
		label = "HIGH"
	case "medium":
		label = "MED"
	case "low":
		label = "LOW"
	}
	return s.priority(p).Render("[" + label + "]")
}

func (s styles) dueBadge(due string, done bool, now time.Time) string {
	if due == "" {
		return s.dim.Render("—")
	}
	style := s.dueSoon
	label := due
	if d, err := core.ParseDue(due, now.Location()); err == nil {
		day := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		switch {
		case done:
			style = s.dim
		case day.Before(today), len(due) > len("2006-01-02") && d.Before(now):
			style = s.overdue
		case day.Equal(today):
			style = s.dueToday
		}
		label = day.Format("2006-01-02")
		if len(due) > len("2006-01-02") {
			label = d.Format("01-02 15:04")
		}
	}
	return style.Render(label)
}
