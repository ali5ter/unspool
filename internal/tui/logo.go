package tui

import (
	"math"
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
	lines, width := logoLines()
	rendered := make([]string, len(lines))
	for i, line := range lines {
		padded := line + strings.Repeat(" ", width-lipgloss.Width(line))
		rendered[i] = style.Render(padded)
	}
	return strings.Join(rendered, "\n")
}

// logoLines returns asciiLogo's trimmed lines and the widest line's width —
// factored out of renderLogoStyled so renderHeaderLogoSweep can address the
// same logo by column without duplicating the trim/measure logic.
func logoLines() ([]string, int) {
	lines := strings.Split(strings.TrimRight(strings.TrimLeft(asciiLogo, "\n"), "\n"), "\n")
	width := 0
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
		if w := lipgloss.Width(lines[i]); w > width {
			width = w
		}
	}
	return lines, width
}

// logoSweepTintHalfWidth mirrors sweepText's pulseTintHalfWidth (styles.go)
// for the same ~10-character band width, so the logo's occasional pulse
// and the status notice's busy sweep read as the same visual language.
const logoSweepTintHalfWidth = pulseTintHalfWidth

// logoSweepTotalTicks returns how many ticks one full sweep pass across a
// logo of the given width takes — same "~1 tick per character of travel"
// pacing sweepText uses. The caller (updateInner's logoSweepTickMsg case)
// stops advancing once it reaches this many ticks.
func logoSweepTotalTicks(width int) int {
	travel := float64(width-1) + 2*logoSweepTintHalfWidth
	ticks := int(math.Round(travel))
	if ticks < 1 {
		ticks = 1
	}
	return ticks
}

// renderHeaderLogoSweep renders the header logo with a single bright band
// sweeping left-to-right across every row simultaneously — the same column
// is tinted in each row, so it reads as one straight bar passing over the
// wordmark, not per-line noise. Unlike sweepText's neutral/tint pair (an
// idle/busy distinction), the logo is always "on": its base color stays
// colorTeal throughout, and the sweep is a brief brighter highlight over
// it — closer to light glinting across the wordmark than a status change.
// tick runs 0..logoSweepTotalTicks(width); the caller owns pacing/looping.
func renderHeaderLogoSweep(tick int) string {
	lines, width := logoLines()
	travel := float64(width-1) + 2*logoSweepTintHalfWidth
	totalTicks := logoSweepTotalTicks(width)
	center := -logoSweepTintHalfWidth + float64(tick)/float64(totalTicks)*travel

	base := lipgloss.NewStyle().Bold(true).Background(colorPanel)
	rendered := make([]string, len(lines))
	for li, line := range lines {
		runes := []rune(line)
		var b strings.Builder
		for i := 0; i < width; i++ {
			r := ' '
			if i < len(runes) {
				r = runes[i]
			}
			dist := math.Abs(float64(i) - center)
			t := 1 - dist/logoSweepTintHalfWidth
			if t < 0 {
				t = 0
			}
			c := lerpColor(colorTeal, colorLogoGleam, t)
			b.WriteString(base.Foreground(c).Render(string(r)))
		}
		rendered[li] = b.String()
	}
	return strings.Join(rendered, "\n")
}
