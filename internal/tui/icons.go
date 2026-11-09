package tui

// Icons are defined in exactly one place — one file, one table — with a
// Nerd Font glyph, a Unicode fallback and an empty rendering for
// ui.icons: none. Modes: "nerd" (default), "unicode", "none".
var iconDefs = []struct {
	name    string
	nerd    string
	unicode string
}{
	{"home", "\uf015", "⌂"},
	{"list", "\uf03a", "☰"},
	{"board", "\uf0db", "▥"},
	{"calendar", "\uf073", "▦"},
	{"focus", "\uf06e", "◎"},
	{"check", "\uf046", "✓"},
	{"circle", "\uf10c", "○"},
	{"warning", "\uf071", "⚠"},
	{"exam", "\uf19d", "✎"},
	{"clock", "\uf017", "◷"},
	{"flame", "\uf06d", "▲"},
	{"sync", "\uf021", "↻"},
	{"search", "\uf002", "⌕"},
	{"tag", "\uf02b", "#"},
	{"project", "\uf07b", "▸"},
	{"help", "\uf059", "?"},
	{"plus", "\uf067", "+"},
	{"chev_left", "\uf053", "‹"},
	{"chev_right", "\uf054", "›"},
	{"dot", "\uf111", "•"},
	{"due", "\uf133", "◑"},
	{"course", "\uf02d", "▮"},
	{"stats", "\uf080", "▤"},
}

// buildIcons resolves the icon table for a ui.icons mode.
func buildIcons(mode string) map[string]string {
	m := make(map[string]string, len(iconDefs))
	for _, d := range iconDefs {
		switch mode {
		case "unicode":
			m[d.name] = d.unicode
		case "none":
			m[d.name] = ""
		default: // nerd
			m[d.name] = d.nerd
		}
	}
	return m
}

// icon returns the glyph for name in the model's configured mode, or ""
// when icons are disabled or the name is unknown. Views should treat the
// empty string as "no icon, no leading space".
func (m Model) icon(name string) string {
	if m.icons == nil {
		return buildIcons("nerd")[name]
	}
	return m.icons[name]
}

// hasIcon reports whether icons are rendered (ui.icons != none and the
// icon exists and is non-empty).
func (m Model) hasIcon(name string) bool { return m.icon(name) != "" }
