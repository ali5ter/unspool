package tui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// tab identifies one of the top-level TUI sections.
type tab int

const (
	tabFeed tab = iota
	tabQueue
	tabPlaylists
	tabLiked
	tabCount
)

func (t tab) String() string {
	switch t {
	case tabFeed:
		return "feed"
	case tabQueue:
		return "queue"
	case tabPlaylists:
		return "playlists"
	case tabLiked:
		return "liked"
	default:
		return "?"
	}
}

func (t tab) next() tab {
	return (t + 1) % tabCount
}

func (t tab) prev() tab {
	return (t - 1 + tabCount) % tabCount
}

var tabLabels = [...]string{"feed", "queue", "playlists", "liked"}

// logoHeight is asciiLogo's row count.
const logoHeight = 2

// headerHeight is the header's total row count. Previously logoHeight+1,
// the +1 for a bottom rule delineating it from the content below — no
// longer needed now that every column below renders in its own bordered
// box (columnBox): that box's top edge already marks the boundary, so a
// second rule right above it was redundant.
const headerHeight = logoHeight

// renderHeader renders the header: the logo top-left, the tab strip to its
// right on the logo's bottom row.
//
// The tab strip and the gap before it are built as an explicit logoHeight-
// row block (padTopWithBG), not left for lipgloss.JoinHorizontal to pad up
// to the logo's height on its own: JoinHorizontal pads a shorter block
// with plain, unstyled blank rows, not the block's own background — that
// left a stray patch of the terminal's default background (not
// colorPanel) sitting in the row above the tabs, wherever the gap/tabs
// block needed padding to match the logo's height. Confirmed directly by
// inspecting cell background attributes from a cast recording, not just
// visually.
func renderHeader(active tab, width int) string {
	band := lipgloss.NewStyle().Background(colorPanel)
	logo := renderHeaderLogo()

	var tabs string
	for i, label := range tabLabels {
		if tab(i) == active {
			tabs += styleTabActive.Render(label)
		} else {
			tabs += styleTabInactive.Render(label)
		}
	}
	row := band.Render("  ") + tabs
	padded := padTopWithBG(row, logoHeight-1, colorPanel)

	left := lipgloss.JoinHorizontal(lipgloss.Bottom, logo, padded)
	return band.Width(width).Render(left)
}

// padTopWithBG pads a single-line string s with extraRows blank lines
// above it, each filled with bg for its full width, then returns the
// result with s on the last row.
func padTopWithBG(s string, extraRows int, bg color.Color) string {
	if extraRows <= 0 {
		return s
	}
	blank := lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", lipgloss.Width(s)))
	return strings.Repeat(blank+"\n", extraRows) + s
}
