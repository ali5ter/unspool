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

func renderTabBar(active tab, width int) string {
	parts := make([]string, len(tabLabels))
	for i, label := range tabLabels {
		if tab(i) == active {
			parts[i] = styleTabActive.Render(" " + label + " ")
		} else {
			parts[i] = styleTabInactive.Render(" " + label + " ")
		}
	}
	return lipgloss.NewStyle().Width(width).Render(lipgloss.JoinHorizontal(lipgloss.Top, parts...))
}
