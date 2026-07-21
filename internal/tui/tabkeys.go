package tui

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/ali5ter/unspool/internal/feed"
	"github.com/ali5ter/unspool/internal/queue"
	"github.com/ali5ter/unspool/internal/store"
)

// handleTabKey dispatches a keypress to the active tab's handler.
func (m Model) handleTabKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.activeTab {
	case tabFeed:
		return m.handleFeedKey(msg)
	case tabQueue:
		return m.handleQueueKey(msg)
	case tabPlaylists:
		return m.handlePlaylistsKey(msg)
	case tabLiked:
		return m.handleLikedKey(msg)
	}
	return m.updateActiveList(msg)
}

func (m Model) handleFeedKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Play):
		return m, m.playSelected(false)
	case key.Matches(msg, m.keys.AudioOnly):
		return m, m.playSelected(true)
	case key.Matches(msg, m.keys.AddQueue):
		return m.addSelectedToQueue()
	case key.Matches(msg, m.keys.Mute):
		return m.muteSelectedChannel()
	case key.Matches(msg, m.keys.Like):
		return m, m.likeSelected()
	case key.Matches(msg, m.keys.AddToList):
		return m.openPickerForSelected()
	}
	var cmd tea.Cmd
	m.feedList, cmd = m.feedList.Update(msg)
	return m, cmd
}

func (m Model) handleQueueKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Play):
		return m, m.playSelected(false)
	case key.Matches(msg, m.keys.AudioOnly):
		return m, m.playSelected(true)
	case key.Matches(msg, m.keys.Remove):
		return m.removeSelectedFromQueue()
	case key.Matches(msg, m.keys.Like):
		return m, m.likeSelected()
	case key.Matches(msg, m.keys.AddToList):
		return m.openPickerForSelected()
	}
	var cmd tea.Cmd
	m.queueList, cmd = m.queueList.Update(msg)
	return m, cmd
}

func (m Model) handlePlaylistsKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.viewingPlaylist {
		switch {
		case key.Matches(msg, m.keys.Play):
			return m, m.playSelected(false)
		case key.Matches(msg, m.keys.AudioOnly):
			return m, m.playSelected(true)
		case key.Matches(msg, m.keys.Remove):
			return m.removeSelectedFromOpenPlaylist()
		case key.Matches(msg, m.keys.AddToList):
			return m.openMovePickerForSelected()
		case key.Matches(msg, m.keys.Back):
			m.viewingPlaylist = false
			return m, nil
		}
		var cmd tea.Cmd
		m.playlistItemsList, cmd = m.playlistItemsList.Update(msg)
		return m, cmd
	}

	switch {
	case key.Matches(msg, m.keys.Play):
		return m.openSelectedPlaylist()
	case key.Matches(msg, m.keys.NewList):
		m.creatingPlaylist = true
		m.newPlaylistInput.SetValue("")
		return m, tea.Batch(clearScreenCmd(), m.newPlaylistInput.Focus())
	case key.Matches(msg, m.keys.Remove):
		return m.confirmDeleteSelectedPlaylist()
	}
	var cmd tea.Cmd
	m.playlistsList, cmd = m.playlistsList.Update(msg)
	return m, cmd
}

func (m Model) handleLikedKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Play):
		return m, m.playSelected(false)
	case key.Matches(msg, m.keys.AudioOnly):
		return m, m.playSelected(true)
	case key.Matches(msg, m.keys.Like):
		return m, m.likeSelected()
	case key.Matches(msg, m.keys.AddQueue):
		return m.addSelectedToQueue()
	case key.Matches(msg, m.keys.AddToList):
		return m.openPickerForSelected()
	}
	var cmd tea.Cmd
	m.likedList, cmd = m.likedList.Update(msg)
	return m, cmd
}

// updateActiveList forwards a non-key message (e.g. list-internal ticks) to
// whichever list the active tab is currently showing.
func (m Model) updateActiveList(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.activeTab {
	case tabFeed:
		m.feedList, cmd = m.feedList.Update(msg)
	case tabQueue:
		m.queueList, cmd = m.queueList.Update(msg)
	case tabPlaylists:
		if m.viewingPlaylist {
			m.playlistItemsList, cmd = m.playlistItemsList.Update(msg)
		} else {
			m.playlistsList, cmd = m.playlistsList.Update(msg)
		}
	case tabLiked:
		m.likedList, cmd = m.likedList.Update(msg)
	}
	return m, cmd
}

func (m Model) viewActiveTab() string {
	switch m.activeTab {
	case tabFeed:
		return m.feedList.View()
	case tabQueue:
		return m.queueList.View()
	case tabPlaylists:
		if m.viewingPlaylist {
			return m.playlistItemsList.View()
		}
		return m.playlistsList.View()
	case tabLiked:
		return m.likedList.View()
	}
	return ""
}

// refreshQueueList rebuilds the Queue tab's rows from queue.json, resolving
// each video ID against the last feed sync's metadata where available.
// Local order (queue.json's order) is preserved as-is.
func (m *Model) refreshQueueList() {
	qf, err := m.store.LoadQueue()
	if err != nil {
		return
	}
	items := make([]list.Item, 0, len(qf.VideoIDs))
	for _, id := range qf.VideoIDs {
		if it, ok := m.videoIndex[id]; ok {
			items = append(items, queueRow{videoID: id, video: it.Video, channel: it.Channel})
		} else {
			items = append(items, queueRow{videoID: id})
		}
	}
	m.queueList.SetItems(items)
}

func (m Model) addSelectedToQueue() (tea.Model, tea.Cmd) {
	video, channel, ok := m.selectedVideo()
	if !ok {
		return m, nil
	}
	if err := queue.Add(m.store, video.VideoID); err != nil {
		return m, func() tea.Msg { return statusErrMsg{err: err} }
	}
	m.videoIndex[video.VideoID] = feed.Item{Video: video, Channel: channel}
	m.refreshQueueList()
	m.statusMsg = "added to queue"
	return m, mirrorQueueCmd(m.cfg)
}

func (m Model) removeSelectedFromQueue() (tea.Model, tea.Cmd) {
	sel, ok := m.queueList.SelectedItem().(queueRow)
	if !ok {
		return m, nil
	}
	if err := queue.Remove(m.store, sel.videoID); err != nil {
		return m, func() tea.Msg { return statusErrMsg{err: err} }
	}
	m.refreshQueueList()
	m.statusMsg = "removed from queue"
	return m, mirrorQueueCmd(m.cfg)
}

func (m Model) muteSelectedChannel() (tea.Model, tea.Cmd) {
	sel, ok := m.feedList.SelectedItem().(feedItem)
	if !ok {
		return m, nil
	}
	channelID := sel.Video.ChannelID
	if err := m.store.MuteChannel(channelID); err != nil {
		return m, func() tea.Msg { return statusErrMsg{err: err} }
	}

	// Filter the muted channel out of the currently rendered feed immediately
	// — don't make the user wait for the next full sync to see it disappear.
	items := m.feedList.Items()
	kept := make([]list.Item, 0, len(items))
	for _, it := range items {
		if fi, ok := it.(feedItem); ok && fi.Video.ChannelID == channelID {
			continue
		}
		kept = append(kept, it)
	}
	m.feedList.SetItems(kept)
	m.statusMsg = "muted " + sel.Channel
	return m, nil
}

func (m Model) openSelectedPlaylist() (tea.Model, tea.Cmd) {
	sel, ok := m.playlistsList.SelectedItem().(playlistRow)
	if !ok {
		return m, nil
	}
	m.openPlaylistID = sel.playlist.PlaylistID
	m.openPlaylistTitle = sel.playlist.Title
	m.playlistItemsList.Title = "▸ " + sel.playlist.Title
	m.statusMsg = "loading playlist…"
	return m, openPlaylistCmd(m.cfg, sel.playlist.PlaylistID)
}

// confirmDeleteSelectedPlaylist opens a confirm overlay for deleting the
// selected playlist — deletion is irreversible on YouTube's side, unlike
// every other destructive action in this app (mute, dequeue, remove-item),
// which is why this one gets a confirm step and those don't.
func (m Model) confirmDeleteSelectedPlaylist() (tea.Model, tea.Cmd) {
	sel, ok := m.playlistsList.SelectedItem().(playlistRow)
	if !ok {
		return m, nil
	}
	m.deletingPlaylist = true
	m.deletePlaylistID = sel.playlist.PlaylistID
	m.deletePlaylistTitle = sel.playlist.Title
	return m, clearScreenCmd()
}

func (m Model) updateDeletingPlaylist(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		m.deletingPlaylist = false
		return m, clearScreenCmd()
	case key.Matches(msg, m.keys.Confirm):
		id, title := m.deletePlaylistID, m.deletePlaylistTitle
		m.deletingPlaylist = false

		items := m.playlistsList.Items()
		kept := make([]list.Item, 0, len(items))
		for _, it := range items {
			if p, ok := it.(playlistRow); ok && p.playlist.PlaylistID == id {
				continue
			}
			kept = append(kept, it)
		}
		m.playlistsList.SetItems(kept)
		m.pickerList.SetItems(kept)

		return m, tea.Batch(clearScreenCmd(), deletePlaylistCmd(m.cfg, id, title))
	}
	return m, nil
}

func (m Model) renderDeletePlaylist() string {
	return renderDialog("Delete playlist?", styleMeta.Render("\""+m.deletePlaylistTitle+"\" — this can't be undone."), "↵ delete   esc cancel")
}

func (m Model) removeSelectedFromOpenPlaylist() (tea.Model, tea.Cmd) {
	sel, ok := m.playlistItemsList.SelectedItem().(playlistItemRow)
	if !ok {
		return m, nil
	}
	itemID := sel.ref.PlaylistItemID

	items := m.playlistItemsList.Items()
	kept := make([]list.Item, 0, len(items))
	for _, it := range items {
		if pi, ok := it.(playlistItemRow); ok && pi.ref.PlaylistItemID == itemID {
			continue
		}
		kept = append(kept, it)
	}
	m.playlistItemsList.SetItems(kept)

	return m, removePlaylistItemCmd(m.cfg, itemID)
}

func (m Model) openPickerForSelected() (Model, tea.Cmd) {
	video, channel, ok := m.selectedVideo()
	if !ok {
		return m, nil
	}
	m.pickerVideo = video
	m.pickerChannel = channel
	m.pickerMoveItemID = ""
	m.pickerMoveFromID = ""
	if !m.playlistsLoaded {
		m.pickerPending = true
		m.statusMsg = "loading playlists…"
		return m, loadPlaylistsCmd(m.cfg)
	}
	m.pickerActive = true
	m.pickerList.SetItems(m.playlistsList.Items())
	return m, clearScreenCmd()
}

// openMovePickerForSelected opens the same picker overlay as
// openPickerForSelected, but in "move" mode: used from inside an opened
// playlist's own item list, where there's a source to remove from and a
// specific playlist-item ID (not just a video ID) needed to do that.
func (m Model) openMovePickerForSelected() (Model, tea.Cmd) {
	sel, ok := m.playlistItemsList.SelectedItem().(playlistItemRow)
	if !ok {
		return m, nil
	}
	m.pickerVideo = store.Video{VideoID: sel.ref.VideoID, Title: sel.ref.Title}
	m.pickerChannel = sel.channel
	m.pickerMoveItemID = sel.ref.PlaylistItemID
	m.pickerMoveFromID = m.openPlaylistID
	if !m.playlistsLoaded {
		m.pickerPending = true
		m.statusMsg = "loading playlists…"
		return m, loadPlaylistsCmd(m.cfg)
	}
	m.pickerActive = true
	m.pickerList.SetItems(excludePlaylist(m.playlistsList.Items(), m.openPlaylistID))
	return m, clearScreenCmd()
}

// excludePlaylist returns items without the playlistRow matching
// playlistID — used to keep a video's current playlist out of the "move
// to…" picker's choices.
func excludePlaylist(items []list.Item, playlistID string) []list.Item {
	kept := make([]list.Item, 0, len(items))
	for _, it := range items {
		if p, ok := it.(playlistRow); ok && p.playlist.PlaylistID == playlistID {
			continue
		}
		kept = append(kept, it)
	}
	return kept
}

func (m Model) updatePicker(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		m.pickerActive = false
		return m, clearScreenCmd()
	case key.Matches(msg, m.keys.Confirm):
		sel, ok := m.pickerList.SelectedItem().(playlistRow)
		m.pickerActive = false
		if !ok {
			return m, clearScreenCmd()
		}
		add := addToPlaylistCmd(m.cfg, sel.playlist.PlaylistID, sel.playlist.Title, m.pickerVideo.VideoID)
		if m.pickerMoveItemID == "" {
			return m, tea.Batch(clearScreenCmd(), add)
		}

		// Move: also remove from the source playlist, optimistically
		// dropping the row from the currently-open list — same pattern as
		// removeSelectedFromOpenPlaylist.
		itemID := m.pickerMoveItemID
		items := m.playlistItemsList.Items()
		kept := make([]list.Item, 0, len(items))
		for _, it := range items {
			if pi, ok := it.(playlistItemRow); ok && pi.ref.PlaylistItemID == itemID {
				continue
			}
			kept = append(kept, it)
		}
		m.playlistItemsList.SetItems(kept)
		remove := removePlaylistItemCmd(m.cfg, itemID)
		return m, tea.Batch(clearScreenCmd(), add, remove)
	}
	var cmd tea.Cmd
	m.pickerList, cmd = m.pickerList.Update(msg)
	return m, cmd
}

func (m Model) renderPicker() string {
	verb := "Add"
	if m.pickerMoveItemID != "" {
		verb = "Move"
	}
	return renderDialog(verb+" \""+m.pickerVideo.Title+"\" to playlist", m.pickerList.View(), "↵ select   esc cancel")
}

func (m Model) updateCreatingPlaylist(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		m.creatingPlaylist = false
		return m, clearScreenCmd()
	case key.Matches(msg, m.keys.Confirm):
		title := m.newPlaylistInput.Value()
		m.creatingPlaylist = false
		if title == "" {
			return m, clearScreenCmd()
		}
		return m, tea.Batch(clearScreenCmd(), createPlaylistCmd(m.cfg, title))
	}
	var cmd tea.Cmd
	m.newPlaylistInput, cmd = m.newPlaylistInput.Update(msg)
	return m, cmd
}

func (m Model) renderCreatePlaylist() string {
	return renderDialog("New playlist", m.newPlaylistInput.View(), "↵ create   esc cancel")
}
