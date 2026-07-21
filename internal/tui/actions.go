package tui

import (
	"context"
	"fmt"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/ali5ter/unspool/config"
	"github.com/ali5ter/unspool/internal/api"
	"github.com/ali5ter/unspool/internal/queue"
	"github.com/ali5ter/unspool/internal/store"
)

// mirrorQueueCmd reconciles the Queue mirror in the background. It returns
// no message on success (silent) so it doesn't stomp the "added/removed"
// status text already shown optimistically; failures still surface.
func mirrorQueueCmd(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		client, err := newClient(ctx, cfg)
		if err != nil {
			return statusErrMsg{err: fmt.Errorf("queue mirror: %w", err)}
		}
		st := store.New(cfg.StoreDir)
		if err := queue.SyncMirror(ctx, client, st, cfg); err != nil {
			return statusErrMsg{err: fmt.Errorf("queue mirror: %w", err)}
		}
		return nil
	}
}

// likeSelected toggles the like state of the selected video, using the
// locally-cached liked flag to decide which direction to toggle.
func (m Model) likeSelected() tea.Cmd {
	video, _, ok := m.selectedVideo()
	if !ok {
		return nil
	}
	fs, _ := m.store.LoadFeedState()
	newLiked := !fs.State[video.VideoID].Liked
	cfg, videoID := m.cfg, video.VideoID

	return func() tea.Msg {
		ctx := context.Background()
		client, err := newClient(ctx, cfg)
		if err != nil {
			return statusErrMsg{err: err}
		}
		rating := "like"
		if !newLiked {
			rating = "none"
		}
		if err := client.RateVideo(ctx, videoID, rating); err != nil {
			return statusErrMsg{err: err}
		}
		st := store.New(cfg.StoreDir)
		if err := st.SetVideoLiked(videoID, newLiked); err != nil {
			return statusErrMsg{err: err}
		}
		if newLiked {
			return statusErrMsg{text: "liked"}
		}
		return statusErrMsg{text: "unliked"}
	}
}

// playlistsLoadedMsg carries the result of loadPlaylistsCmd.
type playlistsLoadedMsg struct {
	playlists []store.Playlist
	err       error
}

func loadPlaylistsCmd(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		client, err := newClient(ctx, cfg)
		if err != nil {
			return playlistsLoadedMsg{err: err}
		}
		playlists, err := client.ListPlaylists(ctx)
		if err != nil {
			return playlistsLoadedMsg{err: err}
		}
		st := store.New(cfg.StoreDir)
		_ = st.SavePlaylistsCache(store.PlaylistsCacheFile{Playlists: playlists})
		return playlistsLoadedMsg{playlists: playlists}
	}
}

func (m Model) handlePlaylistsLoaded(msg playlistsLoadedMsg) (tea.Model, tea.Cmd) {
	m.playlistsLoaded = true
	if msg.err != nil {
		m.statusMsg = "load playlists failed: " + msg.err.Error()
		m.pickerPending = false
		return m, nil
	}

	// The Queue auto-mirror is a real playlist on the account, so
	// playlists.list legitimately returns it — but it's the exact same
	// content as the dedicated Queue tab, and adding a video to it
	// directly here would desync it from queue.json (the next mirror
	// reconciliation would just remove it again, since it isn't in the
	// local queue). Filtered from both the browse list and the add/move
	// picker.
	mirrorID := m.mirrorPlaylistID()
	items := make([]list.Item, 0, len(msg.playlists))
	for _, p := range msg.playlists {
		if mirrorID != "" && p.PlaylistID == mirrorID {
			continue
		}
		items = append(items, playlistRow{playlist: p})
	}
	m.playlistsList.SetItems(items)
	if m.pickerMoveFromID != "" {
		m.pickerList.SetItems(excludePlaylist(items, m.pickerMoveFromID))
	} else {
		m.pickerList.SetItems(items)
	}
	m.statusMsg = "loaded playlists"

	if m.pickerPending {
		m.pickerPending = false
		m.pickerActive = true
		return m, clearScreenCmd()
	}
	return m, nil
}

// playlistItemsLoadedMsg carries the result of openPlaylistCmd.
type playlistItemsLoadedMsg struct {
	refs    []api.PlaylistItemRef
	details map[string]api.VideoDetail
	err     error
}

// openPlaylistCmd lists a playlist's items and, since a playlist can hold
// any video from any channel (not just subscribed ones — m.videoIndex,
// built from the last feed sync, essentially never has these), batches a
// videos.list call to fetch real channel/duration/publish-date metadata
// for all of them. Confirmed live: without this, every playlist item's
// preview showed nothing but the bare video ID.
func openPlaylistCmd(cfg *config.Config, playlistID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		client, err := newClient(ctx, cfg)
		if err != nil {
			return playlistItemsLoadedMsg{err: err}
		}
		refs, err := client.ListPlaylistItemRefs(ctx, playlistID)
		if err != nil {
			return playlistItemsLoadedMsg{err: err}
		}
		ids := make([]string, len(refs))
		for i, ref := range refs {
			ids[i] = ref.VideoID
		}
		details, err := client.FetchVideoDetails(ctx, ids)
		if err != nil {
			// Non-fatal: fall back to bare video IDs rather than failing
			// the whole playlist view over a metadata lookup.
			details = nil
		}
		return playlistItemsLoadedMsg{refs: refs, details: details}
	}
}

func (m Model) handlePlaylistItemsLoaded(msg playlistItemsLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.statusMsg = "load playlist failed: " + msg.err.Error()
		return m, nil
	}
	items := make([]list.Item, 0, len(msg.refs))
	for _, ref := range msg.refs {
		row := playlistItemRow{ref: ref}
		if d, ok := msg.details[ref.VideoID]; ok {
			row.video = store.Video{
				VideoID:                ref.VideoID,
				Title:                  ref.Title,
				DurationSeconds:        d.DurationSeconds,
				PublishedAt:            d.PublishedAt,
				ContainsSyntheticMedia: d.ContainsSyntheticMedia,
				Description:            d.Description,
			}
			row.channel = d.ChannelTitle
		} else if it, ok := m.videoIndex[ref.VideoID]; ok {
			row.video = it.Video
			row.channel = it.Channel
		}
		items = append(items, row)
	}
	m.playlistItemsList.SetItems(items)
	m.viewingPlaylist = true
	m.statusMsg = "loaded " + m.openPlaylistTitle
	return m, nil
}

// playlistCreatedMsg carries the result of createPlaylistCmd.
type playlistCreatedMsg struct {
	id    string
	title string
	err   error
}

func createPlaylistCmd(cfg *config.Config, title string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		client, err := newClient(ctx, cfg)
		if err != nil {
			return playlistCreatedMsg{err: err}
		}
		id, err := client.CreatePlaylist(ctx, title)
		if err != nil {
			return playlistCreatedMsg{err: err}
		}
		return playlistCreatedMsg{id: id, title: title}
	}
}

func (m Model) handlePlaylistCreated(msg playlistCreatedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.statusMsg = "create playlist failed: " + msg.err.Error()
		return m, nil
	}
	m.statusMsg = "created " + msg.title
	row := playlistRow{playlist: store.Playlist{PlaylistID: msg.id, Title: msg.title}}
	m.playlistsList.SetItems(append(m.playlistsList.Items(), row))
	m.pickerList.SetItems(append(m.pickerList.Items(), row))
	return m, nil
}

// playlistDeletedMsg carries the result of deletePlaylistCmd.
type playlistDeletedMsg struct {
	title string
	err   error
}

func deletePlaylistCmd(cfg *config.Config, playlistID, title string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		client, err := newClient(ctx, cfg)
		if err != nil {
			return playlistDeletedMsg{title: title, err: err}
		}
		if err := client.DeletePlaylist(ctx, playlistID); err != nil {
			return playlistDeletedMsg{title: title, err: err}
		}
		return playlistDeletedMsg{title: title}
	}
}

// handlePlaylistDeleted reports the result of a deletion already applied
// optimistically to playlistsList/pickerList (see updateDeletingPlaylist) —
// consistent with every other destructive action in this app (dequeue,
// remove-item, mute): on failure this only surfaces the error, it doesn't
// restore the row. The row comes back on the next playlists reload.
func (m Model) handlePlaylistDeleted(msg playlistDeletedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.statusMsg = "delete playlist failed: " + msg.err.Error()
		return m, nil
	}
	m.statusMsg = "deleted " + msg.title
	return m, nil
}

// addToPlaylistCmd adds a video to a playlist, used by the picker overlay.
func addToPlaylistCmd(cfg *config.Config, playlistID, playlistTitle, videoID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		client, err := newClient(ctx, cfg)
		if err != nil {
			return statusErrMsg{err: err}
		}
		if _, err := client.AddPlaylistItem(ctx, playlistID, videoID); err != nil {
			return statusErrMsg{err: err}
		}
		return statusErrMsg{text: "added to " + playlistTitle}
	}
}

// removePlaylistItemCmd removes an item from whatever playlist is currently
// open, used after the Queue tab's optimistic local removal.
func removePlaylistItemCmd(cfg *config.Config, playlistItemID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		client, err := newClient(ctx, cfg)
		if err != nil {
			return statusErrMsg{err: err}
		}
		if err := client.RemovePlaylistItem(ctx, playlistItemID); err != nil {
			return statusErrMsg{err: err}
		}
		return statusErrMsg{text: "removed from playlist"}
	}
}

// likedLoadedMsg carries the result of loadLikedCmd.
type likedLoadedMsg struct {
	videos []store.Video
	err    error
}

func loadLikedCmd(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		client, err := newClient(ctx, cfg)
		if err != nil {
			return likedLoadedMsg{err: err}
		}
		videos, err := client.ListLikedVideos(ctx)
		if err != nil {
			return likedLoadedMsg{err: err}
		}
		return likedLoadedMsg{videos: videos}
	}
}

func (m Model) handleLikedLoaded(msg likedLoadedMsg) (tea.Model, tea.Cmd) {
	m.likedLoaded = true
	if msg.err != nil {
		m.statusMsg = "load liked videos failed: " + msg.err.Error()
		return m, nil
	}
	items := make([]list.Item, 0, len(msg.videos))
	for _, v := range msg.videos {
		items = append(items, likedRow{video: v})
	}
	m.likedList.SetItems(items)
	m.statusMsg = "loaded liked videos"
	return m, nil
}
