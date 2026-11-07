package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/loharrahulofficial-crypto/mtm/internal/gauges"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

// styles bundles every lipgloss style the TUI uses, built once per theme
// load. No colors are hardcoded anywhere else in the TUI — everything here
// derives from theme semantic roles (internal/theme).
type styles struct {
	p theme.Palette

	// surfaces
	base    lipgloss.Style // screen background
	surface lipgloss.Style // panel fill
	raised  lipgloss.Style // floating: cards, modals, selected rows
	strong  lipgloss.Style // emphasized fill (heatmap cells, etc.)

	// legacy aliases (kept: tests and older call sites use them)
	dim       lipgloss.Style
	accent    lipgloss.Style
	border    lipgloss.Style
	paneTitle lipgloss.Style
	selected  lipgloss.Style
	done      lipgloss.Style
	header    lipgloss.Style
	footer    lipgloss.Style
	footerKey lipgloss.Style

	// new semantic helpers
	muted       lipgloss.Style // muted text on surface
	mutedBase   lipgloss.Style // muted text on base
	mutedRaised lipgloss.Style
	borderIdle  lipgloss.Style // unfocused floating border
	focusEdge   lipgloss.Style // focused pane border (accent)

	priorityUrgent lipgloss.Style
	priorityHigh   lipgloss.Style
	priorityMedium lipgloss.Style
	priorityLow    lipgloss.Style

	statusDone  lipgloss.Style
	statusDoing lipgloss.Style

	overdue  lipgloss.Style
	dueToday lipgloss.Style
	dueSoon  lipgloss.Style
	heatWarm lipgloss.Style
	heatHot  lipgloss.Style
	heatOver lipgloss.Style
	weekend  lipgloss.Style

	badge   func(fg, bg string) lipgloss.Style
	syncOK  lipgloss.Style
	syncBad lipgloss.Style

	sectionLabel lipgloss.Style // small-caps muted section labels
	groupHeader  lipgloss.Style // quiet list group headers

	pillActive   lipgloss.Style // active statusline view pill (accent fill)
	pillInactive lipgloss.Style

	mText   lipgloss.Style // floating modal text on raised
	mMuted  lipgloss.Style
	mAccent lipgloss.Style
}

func newStyles(p theme.Palette) styles {
	base := lipgloss.Color(p.Base)
	surface := lipgloss.Color(p.Surface)
	raised := lipgloss.Color(p.SurfaceRaised)
	strong := lipgloss.Color(p.SurfaceStrong)
	borderC := lipgloss.Color(p.Border)
	fg := lipgloss.Color(p.Text)
	bright := lipgloss.Color(p.BrightText)
	muted := lipgloss.Color(p.MutedSoft)
	mutedRaw := lipgloss.Color(p.Muted)
	accent := lipgloss.Color(p.Accent)

	s := styles{p: p}
	s.base = lipgloss.NewStyle().Foreground(fg).Background(base)
	s.surface = lipgloss.NewStyle().Foreground(fg).Background(surface)
	s.raised = lipgloss.NewStyle().Foreground(fg).Background(raised)
	s.strong = lipgloss.NewStyle().Foreground(fg).Background(strong)

	s.muted = lipgloss.NewStyle().Foreground(muted).Background(surface)
	s.mutedBase = lipgloss.NewStyle().Foreground(mutedRaw).Background(base)
	s.mutedRaised = lipgloss.NewStyle().Foreground(muted).Background(raised)
	s.borderIdle = lipgloss.NewStyle().Foreground(borderC).Background(raised)
	s.focusEdge = lipgloss.NewStyle().Foreground(accent).Background(surface)

	// Legacy aliases rebuilt on semantic roles.
	s.dim = lipgloss.NewStyle().Foreground(mutedRaw).Background(base)
	s.accent = lipgloss.NewStyle().Foreground(accent).Background(surface).Bold(true)
	s.border = lipgloss.NewStyle().BorderForeground(borderC).Background(surface)
	s.paneTitle = lipgloss.NewStyle().Foreground(muted).Background(surface).Bold(true)
	s.selected = lipgloss.NewStyle().Foreground(fg).Background(raised).Bold(true)
	s.done = lipgloss.NewStyle().Foreground(muted).Background(surface).Strikethrough(true)
	s.header = lipgloss.NewStyle().Foreground(bright).Background(surface).Bold(true)
	s.footer = lipgloss.NewStyle().Foreground(mutedRaw).Background(base)
	s.footerKey = lipgloss.NewStyle().Foreground(accent).Background(base).Bold(true)

	s.priorityUrgent = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Danger)).Background(surface).Bold(true)
	s.priorityHigh = lipgloss.NewStyle().Foreground(lipgloss.Color(p.HeatHot)).Background(surface).Bold(true)
	s.priorityMedium = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Warning)).Background(surface)
	s.priorityLow = lipgloss.NewStyle().Foreground(mutedRaw).Background(surface)

	s.statusDone = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Success)).Background(surface)
	s.statusDoing = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Blue)).Background(surface)

	s.overdue = lipgloss.NewStyle().Foreground(lipgloss.Color(p.HeatOverdue)).Background(surface).Bold(true)
	s.heatWarm = lipgloss.NewStyle().Foreground(lipgloss.Color(p.HeatWarm)).Background(surface)
	s.heatHot = lipgloss.NewStyle().Foreground(lipgloss.Color(p.HeatHot)).Background(surface).Bold(true)
	s.heatOver = lipgloss.NewStyle().Foreground(lipgloss.Color(p.HeatOverdue)).Background(surface).Bold(true)
	s.dueToday = lipgloss.NewStyle().Foreground(lipgloss.Color(p.HeatHot)).Background(surface).Bold(true)
	s.dueSoon = lipgloss.NewStyle().Foreground(lipgloss.Color(p.HeatWarm)).Background(surface)
	s.weekend = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Muted)).Background(surface)

	s.badge = func(fgc, bgc string) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(bgc)).Background(lipgloss.Color(fgc)).Bold(true).Padding(0, 1)
	}
	s.syncOK = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Success)).Background(base)
	s.syncBad = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Danger)).Background(base).Bold(true)

	s.sectionLabel = lipgloss.NewStyle().Foreground(muted).Background(surface).Bold(true)
	s.groupHeader = lipgloss.NewStyle().Foreground(mutedRaw).Background(surface)

	s.pillActive = lipgloss.NewStyle().Foreground(base).Background(accent).Bold(true)
	s.pillInactive = lipgloss.NewStyle().Foreground(mutedRaw).Background(base)

	s.mText = lipgloss.NewStyle().Foreground(fg).Background(raised)
	s.mMuted = lipgloss.NewStyle().Foreground(muted).Background(raised)
	s.mAccent = lipgloss.NewStyle().Foreground(accent).Background(raised).Bold(true)
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
		return ""
	}
	label, heat := gauges.DueLabel(due, now)
	style := s.dueSoon
	switch heat {
	case gauges.HeatCalm:
		style = s.dueSoon
	case gauges.HeatWarm:
		style = s.heatWarm
	case gauges.HeatHot:
		style = s.heatHot
	case gauges.HeatOverdue:
		style = s.heatOver
	}
	if done {
		style = s.dim
	}
	return style.Render(label)
}

// heatStyle resolves the due heat to a style for trailing chips.
func (s styles) heatStyle(h gauges.Heat, done bool) lipgloss.Style {
	if done {
		return s.dim
	}
	switch h {
	case gauges.HeatWarm:
		return s.heatWarm
	case gauges.HeatHot:
		return s.heatHot
	case gauges.HeatOverdue:
		return s.heatOver
	default:
		return s.dim
	}
}
