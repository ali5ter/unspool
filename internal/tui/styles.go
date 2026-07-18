package tui

import "charm.land/lipgloss/v2"

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

	styleSplashSub = lipgloss.NewStyle().Foreground(colorMuted)
)
