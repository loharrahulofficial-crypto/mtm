package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/loharrahulofficial-crypto/mtm/internal/ambient"
)

// ambient_settings.go — the :ambient screen. A live, full-terminal preview
// on the left (the scene re-renders as settings change) and a mood/settings
// picker on the right. Changes apply to the session; persistence stays in
// ~/.config/mtm/config.yaml (ui.* keys).

const ambientSettingRows = 9

// ambientSettingRow{ label, value, adjust } — adjust cycles the value.
type ambientSettingRow struct {
	label  string
	value  func(m Model) string
	adjust func(m *Model, dir int)
}

func ambientRows() []ambientSettingRow {
	scenes := append([]string{"auto", "random"}, ambient.Registered()...)
	surfaces := []string{"both", "splash", "home", "off"}
	fpsSteps := []int{10, 15, 20, 30}
	fmtSeconds := []float64{1.0, 2.5, 4.0, 6.0}
	renders := []string{"auto", "halfblock", "braille", "ascii"}
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
	return []ambientSettingRow{
		{
			label: "scene (mood)",
			value: func(m Model) string { return m.cfg.UI.Scene },
			adjust: func(m *Model, dir int) {
				m.cfg.UI.Scene = cycleStr(m.cfg.UI.Scene, dir, scenes)
				m.ambientSceneName = m.sceneForLaunch()
				m.sceneName = "" // rebuild
			},
		},
		{
			label: "surfaces",
			value: func(m Model) string { return m.cfg.UI.Ambient },
			adjust: func(m *Model, dir int) {
				m.cfg.UI.Ambient = cycleStr(m.cfg.UI.Ambient, dir, surfaces)
			},
		},
		{
			label: "animations",
			value: func(m Model) string {
				if m.animations {
					return "on"
				}
				return "off"
			},
			adjust: func(m *Model, dir int) {
				m.animations = !m.animations
			},
		},
		{
			label: "splash seconds",
			value: func(m Model) string { return fmt.Sprintf("%.1fs", m.cfg.UI.SplashSeconds) },
			adjust: func(m *Model, dir int) {
				m.cfg.UI.SplashSeconds = cycleF(m.cfg.UI.SplashSeconds, dir, fmtSeconds)
			},
		},
		{
			label: "reward animation",
			value: func(m Model) string {
				if m.cfg.UI.RewardAnimation {
					return "on"
				}
				return "off"
			},
			adjust: func(m *Model, dir int) {
				m.cfg.UI.RewardAnimation = !m.cfg.UI.RewardAnimation
			},
		},
		{
			label: "fps",
			value: func(m Model) string { return fmt.Sprintf("%d", m.cfg.UI.AmbientFPS) },
			adjust: func(m *Model, dir int) {
				m.cfg.UI.AmbientFPS = cycleI(m.cfg.UI.AmbientFPS, dir, fpsSteps)
			},
		},
		{
			label: "render",
			value: func(m Model) string { return m.cfg.UI.AmbientRender },
			adjust: func(m *Model, dir int) {
				m.cfg.UI.AmbientRender = cycleStr(m.cfg.UI.AmbientRender, dir, renders)
				m.sceneName = "" // rebuild
			},
		},
		{
			label: "rain lightning",
			value: func(m Model) string {
				if m.cfg.UI.RainLightning {
					return "on"
				}
				return "off"
			},
			adjust: func(m *Model, dir int) {
				m.cfg.UI.RainLightning = !m.cfg.UI.RainLightning
			},
		},
		{
			label: "persist",
			value: func(m Model) string { return "config.yaml → ui.*" },
			adjust: func(m *Model, dir int) {
				m.statusMsg = "kept for this session — copy ui.* into ~/.config/mtm/config.yaml to persist"
			},
		},
	}
}

func (m Model) commandScene(arg string) (Model, tea.Cmd) {
	names := ambient.Registered()
	arg = strings.ToLower(strings.TrimSpace(arg))
	switch arg {
	case "", "list":
		m.statusMsg = "scenes: " + strings.Join(names, "  ")
		return m, nil
	case "auto", "random":
		m.cfg.UI.Scene = arg
		m.ambientSceneName = m.sceneForLaunch()
		m.sceneName = ""
		m.statusMsg = "scene: " + arg
		return m, m.armAmbient()
	}
	for _, n := range names {
		if n == arg {
			m.cfg.UI.Scene = n
			m.ambientSceneName = n
			m.sceneName = ""
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
	base := lipgloss.NewStyle().Background(lipgloss.Color(m.palette.Base))
	edge := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.AccentSoft)).Background(lipgloss.Color(m.palette.Base))
	fill := base
	bright := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.BrightText)).Background(lipgloss.Color(m.palette.Base)).Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(lipgloss.Color(m.palette.Base))
	mutedBase := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(lipgloss.Color(m.palette.Base))
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(lipgloss.Color(m.palette.Base)).Bold(true)

	// Panel dims: full terminal minus a margin.
	pw := w - 4
	ph := h - 2
	top := titledBorder(m, "A M B I E N T", pw, true, edge)
	bot := titledBorder(m, "", pw, false, edge)

	// Right column: settings list. Preview on the left.
	listW := 34
	prevW := pw - listW - 3
	prevH := ph - 4
	preview := []string{}
	if prevW >= 12 && prevH >= 4 {
		pw2 := prevW
		ph2 := prevH
		preview = m.sceneFrame(m.ambientT, pw2, ph2, false)
	}

	rows := ambientRows()
	sel := m.ambientSettingSel
	if sel >= len(rows) {
		sel = len(rows) - 1
	}
	if sel < 0 {
		sel = 0
	}
	lines := []string{}
	for i, row := range rows {
		marker := "  "
		style := muted
		if i == sel {
			marker = "▸ "
			style = bright
		}
		line := marker + style.Render(row.label) + "\n"
		_ = line
		val := row.value(m)
		vstyle := mutedBase
		if i == sel {
			vstyle = accent
		}
		text := marker + style.Render(row.label) + "  " + vstyle.Render(val)
		lines = append(lines, text)
	}

	// Compose columns
	body := []string{}
	for i := 0; i < prevH; i++ {
		leftRow := ""
		if i < len(preview) {
			leftRow = preview[i]
		}
		left := padLeftExact(leftRow, prevW, fill)
		var right string
		if i < len(lines) {
			right = lines[i]
		} else {
			right = ""
		}
		right = padLeftExact(right, listW, fill)
		body = append(body, left+fill.Render(" ")+right)
	}

	// header row inside panel: title + mood line
	moodLine := muted.Render("  mood · ") + bright.Render(strings.ToUpper(m.cfg.UI.AmbientRender)) + muted.Render("  · live preview")
	header := moodLine
	header = padLeftExact(header, pw-2, fill)

	out := []string{top}
	out = append(out, body...)
	_ = header
	out = append(out, edge.Render("│")+padLeftExact(bright.Render("  esc closes · j/k pick · h/l adjust · enter toggles"), pw-2, fill)+edge.Render("│"))
	out = append(out, bot)
	return strings.Join(out, "\n")
}

// padLeftExact pads/clips a (possibly styled) line to exactly w cells.
func padLeftExact(line string, w int, fill lipgloss.Style) string {
	if lipgloss.Width(line) > w {
		return clipStyled(line, w)
	}
	return line + fill.Render(strings.Repeat(" ", w-lipgloss.Width(line)))
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
	}
	return m, m.armAmbient()
}
