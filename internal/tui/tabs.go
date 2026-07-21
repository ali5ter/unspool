package tui

import "charm.land/lipgloss/v2"

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

// headerHeight is renderHeader's row count — asciiLogo is 2 lines. Callers
// sizing the rest of the layout (listHeight) must match this.
const headerHeight = 2

// renderHeader renders the header as headerHeight rows: the logo top-left,
// then the tab strip to its right, vertically centered against the logo's
// height. band.Width(width) pads every row (including the logo's own,
// already-styled rows) out to the full terminal width with the same
// background, so the gap to the right of the tabs reads as one continuous
// band rather than stopping short where the logo/tabs content ends.
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

	left := lipgloss.JoinHorizontal(lipgloss.Center, logo, band.Render("  "), tabs)
	return band.Width(width).Render(left)
}
