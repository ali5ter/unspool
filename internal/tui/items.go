package tui

import (
	"fmt"

	"github.com/ali5ter/unspool/internal/api"
	"github.com/ali5ter/unspool/internal/store"
)

// queueRow is a Queue-tab row: a queued video ID resolved against the last
// feed sync's metadata, when available.
type queueRow struct {
	videoID string
	video   store.Video // zero value if not found in the last sync's index
	channel string
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
		return styleMeta.Render("queued")
	}
	return styleMeta.Render(fmt.Sprintf("%s · %s", r.channel, humanDuration(r.video.DurationSeconds)))
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

// playlistItemRow is a row within an opened playlist.
type playlistItemRow struct {
	ref api.PlaylistItemRef
}

func (r playlistItemRow) FilterValue() string { return r.ref.Title }
func (r playlistItemRow) Title() string       { return r.ref.Title }
func (r playlistItemRow) Description() string { return styleMeta.Render(r.ref.VideoID) }

// likedRow is a Liked-tab row: a video from videos.list(myRating=like).
type likedRow struct {
	video store.Video
}

func (r likedRow) FilterValue() string { return r.video.Title }
func (r likedRow) Title() string       { return r.video.Title }
func (r likedRow) Description() string {
	return styleMeta.Render(fmt.Sprintf("%s · %s · %s", r.video.ChannelTitle, humanAge(r.video.PublishedAt), humanDuration(r.video.DurationSeconds)))
}
