package tui

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/loharrahulofficial-crypto/mtm/internal/ambient"
	"github.com/loharrahulofficial-crypto/mtm/internal/perf"
)

// frameBuilds counts actual viewInner() rebuilds (not cache hits).
// Tests read it to assert that a no-op tick repaints nothing.
var frameBuilds int64

// View renders the whole screen (and counts it for the profiling harness).
// When nothing visible changed since the last call, the cached block is
// returned without touching Render/Join/paint — Bubble Tea's line diff then
// flushes zero bytes.
func (m Model) View() string {
	fc := m.frame
	if fc != nil && fc.view != "" && fc.stamp == m.viewStamp && fc.scene == m.ambientStamp &&
		fc.w == m.width && fc.h == m.height {
		perf.CountView(len(fc.view))
		return fc.view
	}
	buildStart := time.Now()
	s := m.viewInner()
	buildDur := time.Since(buildStart)
	atomic.AddInt64(&frameBuilds, 1)
	if m.ambientActive() {
		// Adaptive quality tiers act on the end-to-end frame cost —
		// scene raster + full View() assembly — not only the raster.
		ambient.ObserveFrameCost(buildDur)
	}
	if fc != nil {
		fc.view, fc.stamp, fc.scene, fc.w, fc.h = s, m.viewStamp, m.ambientStamp, m.width, m.height
	}
	perf.CountView(len(s))
	return s
}

func (m Model) viewInner() string {
	if m.width == 0 || m.height == 0 {
		return "loading…"
	}
	baseColor := lipgloss.Color(m.palette.Base)
	if m.width < minUsableWidth || m.height < minUsableHeight {
		return tooSmallView(m.width, m.height, baseColor, m.keymap.Seq(aQuit))
	}
	if m.mode == modeConflict {
		return m.renderConflictScreen()
	}
	if m.mode == modeSplash {
		return normalizeBlock(m.renderSplash(), m.width, m.height, baseColor)
	}
	if m.mode == modeAmbient {
		return normalizeBlock(m.renderAmbientSettings(), m.width, m.height, baseColor)
	}
	if m.mode == modeFocus {
		fscreen := normalizeBlock(m.renderFocus(), m.width, m.height, baseColor)
		if m.rewardUntil.After(m.clock.Now()) {
			if panel := m.renderRewardPanel(); panel != "" {
				return overlay(m.width, m.height, fscreen, panel, baseColor)
			}
		}
		return fscreen
	}
	if m.projectPage != "" {
		page := m.renderProjectPage(m.width, m.height-1)
		footer := m.renderFooterBar(m.width)
		screen := joinBlocksV([]string{page, footer})
		switch m.mode {
		case modeHelp:
			return overlay(m.width, m.height, screen, renderHelpBox(m.keymap, m.styles, m.width), baseColor)
		case modeConfirm:
			return overlay(m.width, m.height, screen, m.renderConfirmBox(), baseColor)
		case modePicker:
			return overlay(m.width, m.height, screen, m.renderPickerBox(), baseColor)
		case modePalette:
			return overlay(m.width, m.height, screen, m.renderPaletteBox(), baseColor)
		}
		if m.rewardUntil.After(m.clock.Now()) {
			if panel := m.renderRewardPanel(); panel != "" {
				return overlay(m.width, m.height, screen, panel, baseColor)
			}
		}
		return normalizeBlock(screen, m.width, m.height, baseColor)
	}

	contentH := m.height - 1 // footer

	sidebarW := sideWidth(m.width, m.sidebarHidden)
	detailW := detailWidth(m.width, m.detailOn)
	if m.view == viewCourses {
		detailW = 0 // course dashboard is read-only; no detail pane
	}
	if m.view == viewStats {
		detailW = 0 // stats view is read-only; no detail pane
	}
	if m.view == viewIdeas {
		detailW = 0 // ideas are notes; the panel is the room
	}
	mainW, _ := mainPaneSize(m)

	pc := m.panels
	var mainPC, detailPC, footerPC *cachedPanel
	if pc != nil {
		mainPC, detailPC, footerPC = &pc.main, &pc.detail, &pc.footer
	}
	mainKey := panelKey{stamp: m.viewStamp, scene: m.mainSceneStamp(mainW, contentH), w: mainW, h: contentH, theme: m.themeVersion, focus: m.focus == focusMain}
	main := cachedStringPanel(mainPC, mainKey, func() string { return m.renderMain(mainW, contentH) })

	parts := []string{}
	if sidebarW > 0 {
		parts = append(parts, renderSidebar(m, sidebarW, contentH), gutterColumn(contentH, m.styles.base))
	}
	parts = append(parts, main)
	if detailW > 0 {
		detailKey := panelKey{stamp: m.viewStamp, w: detailW, h: contentH, theme: m.themeVersion, focus: m.focus == focusDetail}
		detail := cachedStringPanel(detailPC, detailKey, func() string { return m.renderDetail(detailW, contentH) })
		parts = append(parts, gutterColumn(contentH, m.styles.base), detail)
	}
	middle := joinBlocksH(parts)
	// The panes must tile the frame exactly; anything that overflows (a long
	// style, a wrap) would push the top row off the terminal, so normalize.
	middle = normalizeBlock(middle, m.width, contentH, baseColor)

	footerKey := panelKey{stamp: m.viewStamp, w: m.width, h: 1, theme: m.themeVersion}
	footer := cachedStringPanel(footerPC, footerKey, func() string { return m.renderFooterBar(m.width) })
	screen := joinBlocksV([]string{middle, footer})

	switch m.mode {
	case modeHelp:
		return overlay(m.width, m.height, screen, renderHelpBox(m.keymap, m.styles, m.width), baseColor)
	case modeConfirm:
		return overlay(m.width, m.height, screen, m.renderConfirmBox(), baseColor)
	case modePicker:
		return overlay(m.width, m.height, screen, m.renderPickerBox(), baseColor)
	case modePalette:
		return overlay(m.width, m.height, screen, m.renderPaletteBox(), baseColor)
	}
	if m.rewardUntil.After(m.clock.Now()) {
		if panel := m.renderRewardPanel(); panel != "" {
			return overlay(m.width, m.height, screen, panel, baseColor)
		}
	}
	return normalizeBlock(screen, m.width, m.height, baseColor)
}

func (m Model) renderMain(w, h int) string {
	focused := m.focus == focusMain
	panel := func(title string, body []string) string {
		return renderPanel(m, title, w, h, focused, body)
	}
	if m.mode == modeNotes {
		content := m.renderNotesEditor(w-2, h-2)
		return panel("notes", strings.Split(content, "\n"))
	}
	switch m.view {
	case viewHome:
		// Home gets no outer border; its inner panels carry the single
		// focus ring (renderHomeBody gives it to TODAY when focused).
		return renderPanelBorderless(m, w, h, renderHomeBody(m, w, h))
	case viewCourses:
		// Borderless slab like Home; the dashboard is read-only with scroll.
		return renderCoursesBody(m, w, h)
	case viewStats:
		return renderStatsBody(m, w, h)
	case viewIdeas:
		open := len(m.store.Ideas())
		return panel("IDEAS · "+fmt.Sprintf("%d", open), renderIdeasBody(m, w-2, h-2))
	case viewBoard:
		return panel("board", renderBoardBody(m, w-2, h-2))
	case viewCalendar:
		// The panel title carries the visible month (letterspaced
		// "CALENDAR" + normal-case month); the chevrons hint the J/K
		// month keys.
		return renderPanel(m, "CALENDAR · "+m.calMonth.Format("Jan 2006"), w, h, focused, renderCalendarBody(m, w-2, h-2), m.icon("chev_left")+" "+m.icon("chev_right"))
	default:
		body := renderListBody(m, w-2, h-2)
		chips := filterChipsText(m, (w*45)/100)
		return renderPanel(m, listPanelTitle(m), w, h, focused, body, "", chips)
	}
}

// overlay centers box over base. baseColor is the background the base block
// was painted on; box rows keep their own background.
func overlay(width, height int, base, box string, baseColor lipgloss.Color) string {
	box = strings.TrimRight(box, "\n")
	boxLines := strings.Split(box, "\n")
	boxW := 0
	for _, l := range boxLines {
		if w := fastWidth(l); w > boxW {
			boxW = w
		}
	}
	x := (width - boxW) / 2
	if x < 0 {
		x = 0
	}
	y := (height - len(boxLines)) / 2
	if y < 0 {
		y = 0
	}
	baseLines := strings.Split(base, "\n")
	for i, bl := range boxLines {
		row := y + i
		if row >= len(baseLines) {
			break
		}
		baseLines[row] = spliceLine(baseLines[row], x, width, bl, baseColor)
	}
	return paint(strings.Join(baseLines, "\n"), width, height, baseColor)
}

// spliceLine splices boxLine into base at cell offset left, keeping base's
// prefix and suffix cells. Both sides stay exactly width cells.
func spliceLine(base string, left, width int, boxLine string, fill lipgloss.Color) string {
	prefix := clipStyled(base, left)
	if pad := left - fastWidth(prefix); pad > 0 {
		prefix += lipgloss.NewStyle().Background(fill).Render(strings.Repeat(" ", pad))
	}
	boxW := fastWidth(boxLine)
	suffixW := width - left - boxW
	suffix := ""
	if suffixW > 0 {
		suffix = clipStyled(ansi.Cut(base, left, ansi.StringWidth(base)), suffixW)
		if pad := suffixW - fastWidth(suffix); pad > 0 {
			suffix += lipgloss.NewStyle().Background(fill).Render(strings.Repeat(" ", pad))
		}
		suffix = bgOpen(fill) + suffix
	}
	return prefix + boxLine + suffix
}

func paneBorderLeft(m *Model) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color(m.palette.Muted))
}
