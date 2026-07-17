package tui

import "charm.land/lipgloss/v2"

// Warm near-black base with a desaturated red accent (PRD §7.4) — never
// YouTube's white/red glare.
var (
	colorBG     = lipgloss.Color("#141210")
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
	styleHeader = lipgloss.NewStyle().Foreground(colorText).Bold(true).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(colorLine).Padding(0, 1)
	styleStatusBar = lipgloss.NewStyle().Foreground(colorMuted).
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(colorLine).Padding(0, 1)
	styleSelected = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(colorAccent).Padding(0, 0, 0, 1)
)
