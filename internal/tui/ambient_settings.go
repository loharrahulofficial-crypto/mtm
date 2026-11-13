package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/loharrahulofficial-crypto/mtm/internal/ambient"
	"github.com/loharrahulofficial-crypto/mtm/internal/config"
)

// ambient_settings.go — the :ambient screen. A live, full-terminal preview
// on the left (the scene re-renders as settings change) and a grouped
// settings list on the right. Changes apply to the session immediately;
// "save settings" writes the ui.* keys this screen manages into
// ~/.config/mtm/config.yaml (atomic write, .bak of the previous file).

// ambientSettingRow{ label, value, adjust } — adjust cycles the value;
// group buckets rows under a caps header in the settings list.
type ambientSettingRow struct {
	group  string
	label  string
	value  func(m Model) string
	adjust func(m *Model, dir int)
}

// ambientGroups preserves row order and emits the list's caps headers.
var ambientGroups = []struct {
	key    string
	header string
}{
	{"scene", "S C E N E"},
	{"places", "P L A C E S"},
	{"feel", "F E E L"},
	{"actions", "A C T I O N S"},
}

func ambientRows() []ambientSettingRow {
	scenes := append([]string{"auto", "random"}, ambient.Registered()...)
	surfaces := []string{"both", "sidebar", "home", "splash", "off"}
	fpsSteps := []int{10, 15, 30, 45, 60}
	fmtSeconds := []float64{1.0, 2.5, 4.0, 6.0}
	renders := []string{"auto", "halfblock", "braille", "ascii", "quadrant", "sextant"}
	cycleStr := func(cur string, dir int, options []string) string {
		i := 0
		for j, o := range options {
			if o == cur {
				i = j
				break
			}
		}
		i = (i + dir + len(options)) % len(options)
		return options[i]
	}
	cycleF := func(cur float64, dir int, options []float64) float64 {
		i := 0
		for j, o := range options {
			if o == cur {
				i = j
				break
			}
		}
		i = (i + dir + len(options)) % len(options)
		return options[i]
	}
	cycleI := func(cur int, dir int, options []int) int {
		i := 0
		for j, o := range options {
			if o == cur {
				i = j
				break
			}
		}
		i = (i + dir + len(options)) % len(options)
		return options[i]
	}
	dirty := func(m *Model) { m.ambientDirty = true }
	return []ambientSettingRow{
		{
			group: "scene", label: "scene (mood)",
			value: func(m Model) string { return m.cfg.UI.Scene },
			adjust: func(m *Model, dir int) {
				m.cfg.UI.Scene = cycleStr(m.cfg.UI.Scene, dir, scenes)
				m.ambientSceneName = m.sceneForLaunch()
				m.sceneName = "" // rebuild
				dirty(m)
			},
		},
		{
			group: "scene", label: "render",
			value: func(m Model) string { return m.cfg.UI.AmbientRender },
			adjust: func(m *Model, dir int) {
				m.cfg.UI.AmbientRender = cycleStr(m.cfg.UI.AmbientRender, dir, renders)
				m.sceneName = "" // rebuild
				dirty(m)
			},
		},
		{
			group: "scene", label: "fps",
			value: func(m Model) string { return fmt.Sprintf("%d", m.cfg.UI.AmbientFPS) },
			adjust: func(m *Model, dir int) {
				m.cfg.UI.AmbientFPS = cycleI(m.cfg.UI.AmbientFPS, dir, fpsSteps)
				dirty(m)
			},
		},
		{
			group: "places", label: "surfaces",
			value: func(m Model) string { return m.cfg.UI.Ambient },
			adjust: func(m *Model, dir int) {
				m.cfg.UI.Ambient = cycleStr(m.cfg.UI.Ambient, dir, surfaces)
				dirty(m)
			},
		},
		{
			group: "places", label: "sidebar window",
			value: func(m Model) string {
				if m.cfg.UI.AmbientSidebar {
					return "on"
				}
				return "off"
			},
			adjust: func(m *Model, dir int) {
				m.cfg.UI.AmbientSidebar = !m.cfg.UI.AmbientSidebar
				dirty(m)
			},
		},
		{
			group: "feel", label: "animations",
			value: func(m Model) string {
				if m.animations {
					return "on"
				}
				return "off"
			},
			adjust: func(m *Model, dir int) {
				m.animations = !m.animations
				dirty(m)
			},
		},
		{
			group: "feel", label: "splash seconds",
			value: func(m Model) string { return fmt.Sprintf("%.1fs", m.cfg.UI.SplashSeconds) },
			adjust: func(m *Model, dir int) {
				m.cfg.UI.SplashSeconds = cycleF(m.cfg.UI.SplashSeconds, dir, fmtSeconds)
				dirty(m)
			},
		},
		{
			group: "feel", label: "reward animation",
			value: func(m Model) string {
				if m.cfg.UI.RewardAnimation {
					return "on"
				}
				return "off"
			},
			adjust: func(m *Model, dir int) {
				m.cfg.UI.RewardAnimation = !m.cfg.UI.RewardAnimation
				dirty(m)
			},
		},
		{
			group: "feel", label: "rain lightning",
			value: func(m Model) string {
				if m.cfg.UI.RainLightning {
					return "on"
				}
				return "off"
			},
			adjust: func(m *Model, dir int) {
				m.cfg.UI.RainLightning = !m.cfg.UI.RainLightning
				dirty(m)
			},
		},
		{
			group: "actions", label: "save settings",
			value: func(m Model) string { return "config.yaml" },
			adjust: func(m *Model, dir int) {
				m.saveAmbientUI()
			},
		},
	}
}

// saveAmbientUI writes the ui.* keys this screen manages to config.yaml.
// On success the dirty flag clears and the footer note reflects the path;
// on failure the file is left untouched (previous content is kept as .bak).
func (m *Model) saveAmbientUI() {
	kv := map[string]interface{}{
		"scene":            m.cfg.UI.Scene,
		"ambient":          m.cfg.UI.Ambient,
		"ambient_sidebar":  m.cfg.UI.AmbientSidebar,
		"animations":       m.animations,
		"splash_seconds":   m.cfg.UI.SplashSeconds,
		"reward_animation": m.cfg.UI.RewardAnimation,
		"ambient_fps":      m.cfg.UI.AmbientFPS,
		"ambient_render":   m.cfg.UI.AmbientRender,
		"rain_lightning":   m.cfg.UI.RainLightning,
	}
	if err := config.SaveUIKeys("", kv); err != nil {
		m.statusMsg = "save failed: " + err.Error()
		return
	}
	m.ambientDirty = false
	m.ambientSaved = true
	m.statusMsg = "ambient settings saved to " + config.Path()
}

func (m Model) commandScene(arg string) (Model, tea.Cmd) {
	names := ambient.Registered()
	arg = strings.ToLower(strings.TrimSpace(arg))
	switch arg {
	case "", "list":
		m.statusMsg = "scenes: " + strings.Join(names, "  ")
		return m, nil
	case "auto", "random":
		prev := m.ambientSceneName
		m.cfg.UI.Scene = arg
		m.ambientSceneName = m.sceneForLaunch()
		m.sceneName = ""
		if prev != "" && prev != m.ambientSceneName {
			m.beginCrossfade()
		}
		m.statusMsg = "scene: " + arg
		return m, m.armAmbient()
	}
	for _, n := range names {
		if n == arg {
			prev := m.ambientSceneName
			m.cfg.UI.Scene = n
			m.ambientSceneName = n
			m.sceneName = ""
			if prev != "" && prev != n {
				m.beginCrossfade()
			}
			m.statusMsg = "scene: " + n
			return m, m.armAmbient()
		}
	}
	m.statusMsg = fmt.Sprintf("unknown scene %q — scenes: %s (or auto|random)", arg, strings.Join(names, "|"))
	return m, nil
}

// renderAmbientSettings renders the :ambient screen full-screen.
func (m Model) renderAmbientSettings() string {
	w, h := m.width, m.height
	fill := lipgloss.NewStyle().Background(lipgloss.Color(m.palette.Base))
	edge := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.AccentSoft)).Background(lipgloss.Color(m.palette.Base))
	bright := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.BrightText)).Background(lipgloss.Color(m.palette.Base)).Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(lipgloss.Color(m.palette.Base))
	mutedBase := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(lipgloss.Color(m.palette.Base))
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(lipgloss.Color(m.palette.Base)).Bold(true)

	pw := w - 4
	ph := h - 2
	top := titledBorder(m, "A M B I E N T", pw, true, edge)
	bot := titledBorder(m, "", pw, false, edge)

	const listW = 36
	const listLabelW = 20
	innerW := pw - 2
	prevW := innerW - listW - 1
	contentRows := ph - 3 // top border + hint row + bottom border
	previewH := contentRows

	// Left column: live preview (or blank fill when too small).
	preview := []string{}
	if prevW >= 12 && previewH >= 4 {
		preview = m.sceneFrame(m.ambientT, prevW, previewH, false)
	}

	// Right column: grouped settings list.
	listLines := m.ambientListLines(listW, listLabelW, previewH, fill, bright, muted, mutedBase, accent)

	out := make([]string, 0, ph)
	out = append(out, top)
	for i := 0; i < contentRows; i++ {
		leftRow := fill.Render(strings.Repeat(" ", prevW))
		if i < len(preview) {
			leftRow = padLeftExact(preview[i], prevW, fill)
		}
		right := fill.Render(strings.Repeat(" ", listW))
		if i < len(listLines) {
			right = listLines[i]
		}
		body := leftRow + fill.Render(" ") + right
		body = padLeftExact(body, innerW, fill)
		out = append(out, edge.Render("│")+body+edge.Render("│"))
	}

	// Hint row, with the unsaved indicator right-aligned.
	hint := muted.Render("  j/k move · h/l adjust · s save · esc exit")
	hintRight := ""
	switch {
	case m.ambientDirty:
		hintRight = accent.Render("● unsaved  ")
	case m.ambientSaved:
		hintRight = mutedBase.Render("saved ✓  ")
	}
	hint = padLeftExact(hint, innerW-fastWidth(hintRight), fill) + hintRight
	hint = padLeftExact(hint, innerW, fill)
	out = append(out, edge.Render("│")+hint+edge.Render("│"))
	out = append(out, bot)
	return strings.Join(out, "\n")
}

// ambientListLines lays the grouped settings rows out, one line each,
// tracking the selection marker and value alignment. When vertical space
// is tight the group headers/spacers are dropped (compact list).
func (m Model) ambientListLines(listW, labelW, rows int, fill lipgloss.Style, bright, muted, mutedBase, accent lipgloss.Style) []string {
	rowsDef := ambientRows()
	sel := m.ambientSettingSel
	if sel >= len(rowsDef) {
		sel = len(rowsDef) - 1
	}
	if sel < 0 {
		sel = 0
	}
	compact := rows < len(rowsDef)+8
	out := []string{}
	lastGroup := ""
	for i, row := range rowsDef {
		if !compact && row.group != lastGroup {
			if lastGroup != "" {
				out = append(out, fill.Render(strings.Repeat(" ", listW)))
			}
			header := " "
			for _, g := range ambientGroups {
				if g.key == row.group {
					header = " " + g.header
				}
			}
			out = append(out, muted.Render(header)+fill.Render(strings.Repeat(" ", listW-fastWidth(header))))
			lastGroup = row.group
		}
		marker := "  "
		style := mutedBase
		vstyle := mutedBase
		if i == sel {
			marker = "▸ "
			style = bright
			vstyle = accent
		}
		label := style.Render(truncatePlain(row.label, labelW))
		seg := padLeftExact(marker+label, 2+labelW, fill)
		val := row.value(m)
		valW := listW - (2 + labelW) - 1 // one trailing cell before the padding
		if fastWidth(val) > valW {
			val = truncatePlain(val, valW)
		}
		line := seg + fill.Render(strings.Repeat(" ", valW-fastWidth(val)+1)) + vstyle.Render(val)
		out = append(out, line)
	}
	return out
}

// padLeftExact pads/clips a (possibly styled) line to exactly w cells.
func padLeftExact(line string, w int, fill lipgloss.Style) string {
	if fastWidth(line) > w {
		return clipStyled(line, w)
	}
	return line + fill.Render(strings.Repeat(" ", w-fastWidth(line)))
}

// updateAmbientKey handles keys in the :ambient screen.
func (m Model) updateAmbientKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	rows := ambientRows()
	switch s {
	case "esc", "q":
		m.mode = modeNormal
		return m, nil
	case "j", "down":
		if m.ambientSettingSel < len(rows)-1 {
			m.ambientSettingSel++
		}
	case "k", "up":
		if m.ambientSettingSel > 0 {
			m.ambientSettingSel--
		}
	case "h", "left":
		rows[m.ambientSettingSel].adjust(&m, -1)
	case "l", "right", "enter", " ":
		rows[m.ambientSettingSel].adjust(&m, 1)
	case "s":
		m.saveAmbientUI()
	}
	return m, m.armAmbient()
}
