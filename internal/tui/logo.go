package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// asciiLogo is generated with `cfonts 'unspool' --font tiny`. Regenerate
// with that exact command if the wordmark ever needs to change.
const asciiLogo = `
 █ █ █▄ █ █▀▀ █▀█ █▀█ █▀█ █
 █▄█ █ ▀█ ▄▄█ █▀▀ █▄█ █▄█ █▄▄`

// renderLogo renders asciiLogo in a single solid colour — the same
// teal accent the header uses for the "unspool" wordmark (styleHeaderAccent),
// so the splash and the header read as the same brand mark. Each line is
// padded to the widest line's width before rendering — cfonts letterforms
// aren't uniform width per row, and relying on trailing whitespace surviving
// inside the Go source is fragile; centering (JoinVertical(Center, ...))
// needs equal-width lines or the logo appears to shift sideways row to row.
func renderLogo() string {
	lines := strings.Split(strings.TrimRight(strings.TrimLeft(asciiLogo, "\n"), "\n"), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	maxWidth := 0
	for _, line := range lines {
		if w := lipgloss.Width(line); w > maxWidth {
			maxWidth = w
		}
	}

	style := lipgloss.NewStyle().Foreground(colorTeal).Bold(true)
	rendered := make([]string, len(lines))
	for i, line := range lines {
		padded := line + strings.Repeat(" ", maxWidth-lipgloss.Width(line))
		rendered[i] = style.Render(padded)
	}
	return strings.Join(rendered, "\n")
}
