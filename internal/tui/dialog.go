package tui

import "charm.land/lipgloss/v2"

// renderDialog wraps content in a bordered, panel-coloured dialog box with
// an accent title at the top and a muted hint line at the bottom. Used for
// the startup splash and in-TUI overlays (add-to-playlist picker,
// new-playlist prompt) — mirrors wwlog's dialog.go.
func renderDialog(title, body, hint string) string {
	titleR := styleDialogTitle.Render(title)
	hintR := styleDialogHint.Render(hint)

	width := lipgloss.Width(body)
	if w := lipgloss.Width(titleR); w > width {
		width = w
	}
	if w := lipgloss.Width(hintR); w > width {
		width = w
	}
	pad := lipgloss.NewStyle().Background(colorPanel).Width(width)

	inner := lipgloss.JoinVertical(lipgloss.Left,
		pad.Render(titleR),
		pad.Render(""),
		pad.Render(body),
		pad.Render(""),
		pad.Render(hintR),
	)
	return styleDialogBox.Render(inner)
}
