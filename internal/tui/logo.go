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
// the accent red, top to bottom.
func renderGradientLogo() string {
	lines := strings.Split(strings.TrimLeft(asciiLogo, "\n"), "\n")
	n := len(lines) - 1
	if n < 1 {
		n = 1
	}
	rendered := make([]string, len(lines))
	for i, line := range lines {
		t := float64(i) / float64(n)
		c := lerpColor(colorTeal, colorAccent, t)
		rendered[i] = lipgloss.NewStyle().Foreground(c).Bold(true).Render(line)
	}
	return strings.Join(rendered, "\n")
}
