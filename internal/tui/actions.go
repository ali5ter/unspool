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

	items := make([]list.Item, 0, len(msg.playlists))
	for _, p := range msg.playlists {
		items = append(items, playlistRow{playlist: p})
	}
	m.playlistsList.SetItems(items)
	m.pickerList.SetItems(items)
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
	refs []api.PlaylistItemRef
	err  error
}

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
		return playlistItemsLoadedMsg{refs: refs}
	}
}

func (m Model) handlePlaylistItemsLoaded(msg playlistItemsLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.statusMsg = "load playlist failed: " + msg.err.Error()
		return m, nil
	}
	items := make([]list.Item, 0, len(msg.refs))
	for _, ref := range msg.refs {
		items = append(items, playlistItemRow{ref: ref})
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
