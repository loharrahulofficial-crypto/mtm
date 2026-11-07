package tui

// Spacing grid for the Study Desk design language — one place for every
// inset and gap. Panels sit on `surface` against the `base` background;
// text never touches an edge or another panel.
const (
	PanelPadX = 1 // horizontal padding inside a panel
	PanelPadY = 1 // vertical padding inside a panel
	PanelGap  = 1 // gap between side-by-side panels
	Gutter    = 1 // sidebar ↔ main gutter
	// ContentInset is the left inset of body text inside a panel
	// (beyond PanelPadX) — track of the rounded border column.
	ContentInset = 0
)
