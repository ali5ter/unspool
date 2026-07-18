package tui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// asciiLogo is generated with the same tool and font wwlog's splash logo
// uses: `cfonts 'unspool' --font chrome` (visual kinship, PRD §7.4).
// Regenerate with that exact command if the wordmark ever needs to change.
const asciiLogo = `
 ╦ ╦ ╔╗╔ ╔═╗ ╔═╗ ╔═╗ ╔═╗ ╦
 ║ ║ ║║║ ╚═╗ ╠═╝ ║ ║ ║ ║ ║
 ╚═╝ ╝╚╝ ╚═╝ ╩   ╚═╝ ╚═╝ ╩═╝`

// lerpColor linearly interpolates between two colours at position t ∈ [0,1].
func lerpColor(a, b color.Color, t float64) color.Color {
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	lerp := func(x, y uint32) uint8 {
		fx, fy := float64(x>>8), float64(y>>8)
		return uint8(fx + (fy-fx)*t)
	}
	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", lerp(ar, br), lerp(ag, bg), lerp(ab, bb)))
}

// renderGradientLogo renders asciiLogo with a gradient from teal (new) to
// the accent red, top to bottom. Each line is padded to the widest line's
// width before rendering — cfonts letterforms aren't uniform width per row
// (e.g. this font's bottom row runs 2 cells wider than the top two), and
// relying on trailing whitespace surviving inside the Go source is fragile;
// centering (JoinVertical(Center, ...)) needs equal-width lines or the
// logo appears to shift sideways row to row.
func renderGradientLogo() string {
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

	n := len(lines) - 1
	if n < 1 {
		n = 1
	}
	rendered := make([]string, len(lines))
	for i, line := range lines {
		t := float64(i) / float64(n)
		c := lerpColor(colorTeal, colorAccent, t)
		padded := line + strings.Repeat(" ", maxWidth-lipgloss.Width(line))
		rendered[i] = lipgloss.NewStyle().Foreground(c).Bold(true).Render(padded)
	}
	return strings.Join(rendered, "\n")
}
