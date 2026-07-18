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

// renderHeader renders the single top row: "unspool" leading, then the tab
// strip pushed to the right with a colorPanel background band spanning the
// full width (mirrors wwlog's headerView).
func renderHeader(active tab, width int) string {
	title := styleHeaderAccent.Render("unspool")

	var tabs string
	for i, label := range tabLabels {
		if tab(i) == active {
			tabs += styleTabActive.Render(label)
		} else {
			tabs += styleTabInactive.Render(label)
		}
	}

	gap := width - lipgloss.Width(title) - lipgloss.Width(tabs)
	if gap < 0 {
		gap = 0
	}
	band := lipgloss.NewStyle().Background(colorPanel)
	return band.Width(width).Render(title + band.Render(spaces(gap)) + tabs)
}

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}
