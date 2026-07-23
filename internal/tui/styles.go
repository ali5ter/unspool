package tui

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"charm.land/lipgloss/v2"
)

// Warm near-black base with a desaturated red accent — never YouTube's
// white/red glare.
var (
	colorBG     = lipgloss.Color("#141210")
	colorPanel  = lipgloss.Color("#211d19") // header/status band, dialog background
	colorAccent = lipgloss.Color("#b3564a") // desaturated YouTube-red nod
	colorTeal   = lipgloss.Color("#4fae9b") // new
	colorAmber  = lipgloss.Color("#c9a24b") // queued
	colorText   = lipgloss.Color("#ece6e1")
	colorMuted  = lipgloss.Color("#8a8078") // metadata, dimmed/watched rows
	colorLine   = lipgloss.Color("#3a332e")
)

var (
	styleTitle  = lipgloss.NewStyle().Foreground(colorText).Bold(true)
	styleMeta   = lipgloss.NewStyle().Foreground(colorMuted)
	styleNew    = lipgloss.NewStyle().Foreground(colorTeal)
	styleQueued = lipgloss.NewStyle().Foreground(colorAmber)

	styleHeaderAccent = lipgloss.NewStyle().
				Background(colorPanel).
				Foreground(colorTeal).
				Bold(true).
				Padding(0, 1)

	styleTabActive = lipgloss.NewStyle().
			Background(colorLine).
			Foreground(colorAccent).
			Bold(true).
			Padding(0, 1)

	styleTabInactive = lipgloss.NewStyle().
				Background(colorPanel).
				Foreground(colorMuted).
				Padding(0, 1)

	// No top rule: every column above renders in its own bordered box
	// (columnBox) now, so that box's bottom edge already marks the
	// boundary with the footer below — a separate rule right below it
	// would be redundant.
	styleStatusBar = lipgloss.NewStyle().
			Background(colorPanel).
			Foreground(colorMuted).
			Padding(0, 1)

	styleSelected = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(colorAccent).Padding(0, 0, 0, 1)

	// Dialog styles — used for both the startup splash and in-TUI overlays
	// (add-to-playlist picker, new-playlist prompt), for visual consistency.
	styleDialogBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Background(colorPanel).
			Padding(1, 2)

	styleDialogTitle = lipgloss.NewStyle().
				Foreground(colorTeal).
				Background(colorPanel).
				Bold(true)

	styleDialogHint = lipgloss.NewStyle().
			Foreground(colorMuted).
			Background(colorPanel)
)

// pulsePauseTicks is both the delay before a sweep first starts and the
// rest between one sweep ending and the next beginning — MiniDot ticks at
// ~83ms, so 6 ticks is ~500ms. Same value for both, so a busy notice
// always reads as "pause, then sweep, then pause, then sweep…", never
// jumping straight into motion the instant it appears.
const pulsePauseTicks = 6

// pulseTintHalfWidth is half the width (in characters) of the moving
// tinted band in sweepText — a ~2.5 half-width gives a ~5-character-wide
// band, per the intended "a handful of characters, not the whole line"
// look, as opposed to a smooth gradient spanning the entire string.
const pulseTintHalfWidth = 2.5

// colorSweepNeutral/colorSweepTint are sweepText's two colors: the resting
// (and off-band) text color, and the single tint color used both for the
// moving band and — since the spinner glyph is composed into the same
// string before sweepText ever sees it (see statusLine/viewSplash) — the
// spinner glyph itself, so glyph and text always share one "this is
// active" color rather than two different color languages.
var (
	colorSweepNeutral = colorMuted
	colorSweepTint    = colorTeal
)

// sweepText renders s with a ~5-character-wide band of colorSweepTint
// travelling once across the full length of the text (entering and
// exiting past either edge, not just from character 0 to the last), then
// pausing at colorSweepNeutral for pulsePauseTicks before the next pass —
// matching the sweeping color-change effect this was modeled on, rather
// than the previous smooth gradient spanning the whole string or the
// whole string flashing one color in unison. The sweep's own speed scales
// with the string's length (roughly 1 tick per character of travel) so a
// short message like "playing…" and a long one like "Syncing your
// subscriptions…" both sweep at a similar visual pace rather than a fixed
// tick count making short strings crawl and long ones jump between
// characters. No Bold — a plain-weight glyph/text reads as "informational
// status", not as something demanding attention the way bold does.
func sweepText(s string, tick int, bg color.Color) string {
	runes := []rune(s)
	n := len(runes)
	if n == 0 {
		return s
	}
	style := lipgloss.NewStyle().Background(bg)

	travel := float64(n-1) + 2*pulseTintHalfWidth
	sweepTicks := int(math.Round(travel))
	if sweepTicks < 1 {
		sweepTicks = 1
	}
	cycleTicks := pulsePauseTicks + sweepTicks
	cycleTick := tick % cycleTicks

	var b strings.Builder
	if cycleTick < pulsePauseTicks {
		rest := style.Foreground(colorSweepNeutral)
		for _, r := range runes {
			b.WriteString(rest.Render(string(r)))
		}
		return b.String()
	}

	sweepTick := cycleTick - pulsePauseTicks
	center := -pulseTintHalfWidth + float64(sweepTick)/float64(sweepTicks)*travel
	for i, r := range runes {
		dist := math.Abs(float64(i) - center)
		t := 1 - dist/pulseTintHalfWidth
		if t < 0 {
			t = 0
		}
		c := lerpColor(colorSweepNeutral, colorSweepTint, t)
		b.WriteString(style.Foreground(c).Render(string(r)))
	}
	return b.String()
}

// lerpColor blends two colors at t (0=a, 1=b), via the standard
// color.Color.RGBA() method rather than re-parsing hex strings — lipgloss.
// Color's concrete return type is unexported, so RGBA() is the only
// portable way to read channel values back out of it.
func lerpColor(a, b color.Color, t float64) color.Color {
	ar, ag, ab := rgb8(a)
	br, bg, bb := rgb8(b)
	r := lerpByte(ar, br, t)
	g := lerpByte(ag, bg, t)
	bl := lerpByte(ab, bb, t)
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, bl))
}

func lerpByte(a, b uint8, t float64) uint8 {
	return uint8(float64(a) + (float64(b)-float64(a))*t)
}

func rgb8(c color.Color) (r, g, b uint8) {
	rv, gv, bv, _ := c.RGBA()
	return uint8(rv >> 8), uint8(gv >> 8), uint8(bv >> 8)
}
