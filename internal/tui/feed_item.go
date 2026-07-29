package tui

import (
	"fmt"
	"time"

	"github.com/ali5ter/unspool/internal/feed"
)

// feedItem adapts a feed.Item to bubbles/list's DefaultItem interface.
type feedItem struct {
	feed.Item
	// aiBadge is precomputed at item-build time (handleSyncDone) via
	// aiBadgeFor — combines the heuristic score and any cached inspect
	// verdict, so Description() doesn't need its own reference to the
	// config or verdict cache.
	aiBadge string
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
	badge += i.aiBadge
	if i.State.Seen {
		badge += "  ✓"
	}
	return styleMeta.Render(fmt.Sprintf("%s · %s · %s%s", i.Channel, age, dur, badge))
}

// humanAge formats t as a compact age token: minutes/hours/days/weeks/
// months while recent, then an absolute "Jan 2006" month-year once past a
// year. Users don't reason about "1284d" (issue #4), so anything older than
// ~a year reads as a date instead of an ever-growing day count.
func humanAge(t time.Time) string {
	s, _ := ageParts(t)
	return s
}

// ageParts returns humanAge's text plus whether it is a relative span (true)
// or an absolute date (false). Callers that phrase it as "<age> ago" (the
// preview meta line) use the bool to skip the suffix on the absolute form —
// "Jan 2023 ago" would read wrong.
func ageParts(t time.Time) (text string, relative bool) {
	if t.IsZero() {
		return "—", false
	}
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes())), true
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours())), true
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24)), true
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dw", int(d.Hours()/24/7)), true
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo", int(d.Hours()/24/30)), true
	default:
		return t.Format("Jan 2006"), false
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
