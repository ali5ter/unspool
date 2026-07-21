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

// renderLogo renders asciiLogo in a single solid colour, no background
// (the splash screen has no background band behind anything else either,
// so this matches its surroundings) — the same teal accent the header uses
// for tab highlights, so the splash and the header read as the same brand
// mark.
func renderLogo() string {
	return renderLogoStyled(lipgloss.NewStyle().Foreground(colorTeal).Bold(true))
}

// renderHeaderLogo renders asciiLogo for use inside the header row, which
// (unlike the splash) has an explicit colorPanel background band behind
// everything else on that row — style must include it too, or the logo's
// own character cells would render against the terminal's default
// background instead and visibly mismatch the band around them.
func renderHeaderLogo() string {
	return renderLogoStyled(lipgloss.NewStyle().Foreground(colorTeal).Bold(true).Background(colorPanel))
}

// renderLogoStyled applies style to asciiLogo. Each line is padded to the
// widest line's width before rendering — cfonts letterforms aren't uniform
// width per row, and relying on trailing whitespace surviving inside the Go
// source is fragile; centering (JoinVertical/JoinHorizontal with Center)
// needs equal-width lines or the logo appears to shift sideways row to row.
func renderLogoStyled(style lipgloss.Style) string {
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

	rendered := make([]string, len(lines))
	for i, line := range lines {
		padded := line + strings.Repeat(" ", maxWidth-lipgloss.Width(line))
		rendered[i] = style.Render(padded)
	}
	return strings.Join(rendered, "\n")
}
