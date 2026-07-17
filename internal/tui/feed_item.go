package tui

import (
	"fmt"
	"time"

	"github.com/ali5ter/unspool/internal/feed"
)

// feedItem adapts a feed.Item to bubbles/list's DefaultItem interface.
type feedItem struct {
	feed.Item
}

func (i feedItem) FilterValue() string { return i.Video.Title + " " + i.Channel }

func (i feedItem) Title() string {
	title := i.Video.Title
	if !i.State.Seen {
		title = styleNew.Render("● ") + title
	}
	return title
}

func (i feedItem) Description() string {
	age := humanAge(i.Video.PublishedAt)
	dur := humanDuration(i.Video.DurationSeconds)
	badge := ""
	if i.Video.ContainsSyntheticMedia {
		badge = "  ◆ synthetic media"
	}
	if i.State.Seen {
		badge += "  ✓"
	}
	return styleMeta.Render(fmt.Sprintf("%s · %s · %s%s", i.Channel, age, dur, badge))
}

func humanAge(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func humanDuration(seconds int) string {
	if seconds <= 0 {
		return "—"
	}
	m := seconds / 60
	s := seconds % 60
	if m >= 60 {
		return fmt.Sprintf("%d:%02d:%02d", m/60, m%60, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
