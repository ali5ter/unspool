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

// renderHeader renders the single top row: "unspool", a " · " separator,
// then the tab strip — all grouped together on the left, exactly like
// wwlog's headerView (title + " · " + tabs, left-aligned; wwlog then right-
// aligns a date range in the remaining space, which unspool has no
// equivalent of, so that space is just left as background band).
func renderHeader(active tab, width int) string {
	band := lipgloss.NewStyle().Background(colorPanel)
	title := styleHeaderAccent.Render("unspool")

	var tabs string
	for i, label := range tabLabels {
		if tab(i) == active {
			tabs += styleTabActive.Render(label)
		} else {
			tabs += styleTabInactive.Render(label)
		}
	}

	left := lipgloss.JoinHorizontal(lipgloss.Center, title, band.Render(" · "), tabs)
	gap := width - lipgloss.Width(left)
	if gap < 0 {
		gap = 0
	}
	return band.Width(width).Render(left + band.Render(spaces(gap)))
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
