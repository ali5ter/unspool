package tui

import (
	"fmt"

	"github.com/ali5ter/unspool/internal/api"
	"github.com/ali5ter/unspool/internal/recommend"
	"github.com/ali5ter/unspool/internal/search"
	"github.com/ali5ter/unspool/internal/store"
)

// queueRow is a Queue-tab row: a queued video ID resolved against the last
// feed sync's metadata, when available.
type queueRow struct {
	videoID string
	video   store.Video // zero value if not found in the last sync's index
	channel string
	aiBadge string // set from a cached inspect verdict — see aiBadgeFor
}

func (r queueRow) FilterValue() string { return r.video.Title + " " + r.channel }

func (r queueRow) Title() string {
	if r.video.Title == "" {
		return r.videoID
	}
	return r.video.Title
}

func (r queueRow) Description() string {
	if r.channel == "" {
		return styleMeta.Render("queued" + r.aiBadge)
	}
	return styleMeta.Render(fmt.Sprintf("%s · %s%s", r.channel, humanDuration(r.video.DurationSeconds), r.aiBadge))
}

// playlistRow is a Playlists-tab row: one of the user's playlists.
type playlistRow struct {
	playlist store.Playlist
}

func (r playlistRow) FilterValue() string { return r.playlist.Title }
func (r playlistRow) Title() string       { return r.playlist.Title }
func (r playlistRow) Description() string {
	return styleMeta.Render(fmt.Sprintf("%d video(s)", r.playlist.ItemCount))
}

// playlistItemRow is a row within an opened playlist, resolved against the
// last feed sync's metadata when available — same fallback shape as
// queueRow, and for the same reason: ListPlaylistItemRefs only returns a
// title and video ID (no channel/duration), and fetching full video
// details for every playlist item would cost extra quota for what's
// already sitting in memory for anything that's also in the synced feed.
// Videos outside that window (e.g. from a muted channel, or older than the
// feed's sync horizon) fall back to just the video ID, same as Queue rows.
type playlistItemRow struct {
	ref     api.PlaylistItemRef
	video   store.Video
	channel string
	aiBadge string // set from a cached inspect verdict — see aiBadgeFor
}

func (r playlistItemRow) FilterValue() string { return r.ref.Title }
func (r playlistItemRow) Title() string       { return r.ref.Title }
func (r playlistItemRow) Description() string {
	if r.channel == "" {
		return styleMeta.Render(r.ref.VideoID + r.aiBadge)
	}
	return styleMeta.Render(fmt.Sprintf("%s · %s · %s%s", r.channel, humanAge(r.video.PublishedAt), humanDuration(r.video.DurationSeconds), r.aiBadge))
}

// likedRow is a Liked-tab row: a video from videos.list(myRating=like).
type likedRow struct {
	video   store.Video
	aiBadge string // set from a cached inspect verdict — see aiBadgeFor
}

func (r likedRow) FilterValue() string { return r.video.Title }
func (r likedRow) Title() string       { return r.video.Title }
func (r likedRow) Description() string {
	return styleMeta.Render(fmt.Sprintf("%s · %s · %s%s", r.video.ChannelTitle, humanAge(r.video.PublishedAt), humanDuration(r.video.DurationSeconds), r.aiBadge))
}

// recommendedRow is a Recommended-tab row: a synthesised suggestion with a
// plain-language reason (PRD §5.8 — "no black box").
type recommendedRow struct {
	item    recommend.Item
	aiBadge string // set from a cached inspect verdict — see aiBadgeFor
}

func (r recommendedRow) FilterValue() string { return r.item.Video.Title }
func (r recommendedRow) Title() string       { return r.item.Video.Title }
func (r recommendedRow) Description() string {
	return styleMeta.Render(fmt.Sprintf("%s · %s%s", r.item.Channel, r.item.Reason, r.aiBadge))
}

// noticeRow is a single static, non-actionable informational row — used by
// the Recommended tab when [recommendations].enabled is false in config.
type noticeRow struct {
	text string
}

func (r noticeRow) FilterValue() string { return "" }
func (r noticeRow) Title() string       { return r.text }
func (r noticeRow) Description() string { return "" }

// searchResultRow is a row in the search-results dialog (PRD §5.5) — either
// a local-cache match or a live search.list hit, tagged by source.
type searchResultRow struct {
	result  search.Result
	source  string // "local" | "youtube"
	aiBadge string // set from a cached inspect verdict — see aiBadgeFor
}

func (r searchResultRow) FilterValue() string { return r.result.Video.Title }

func (r searchResultRow) Title() string {
	if r.result.Kind == "playlist" {
		return "▸ " + r.result.Video.Title
	}
	return r.result.Video.Title
}

func (r searchResultRow) Description() string {
	if r.result.Kind == "playlist" {
		return styleMeta.Render("playlist · enter to jump")
	}
	age := humanAge(r.result.Video.PublishedAt)
	return styleMeta.Render(fmt.Sprintf("%s · %s · %s%s", r.result.Channel, age, r.source, r.aiBadge))
}
