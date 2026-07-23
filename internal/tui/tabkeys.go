package tui

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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

// handlePlaylistsKey routes by focusedColumn instead of a drill-down mode:
// column 0 (the playlists list itself) gets playlist-level actions
// (new/delete); columns 1 and 2 (that playlist's videos, and its detail
// pane) both act on playlistItemsList's selection, since the detail column
// only ever mirrors whatever's highlighted in column 1 — only column 1
// forwards keys to a list.Update, since column 2 is read-only (its Up/Down
// is intercepted earlier, in handleGlobalKey, to scroll instead).
func (m Model) handlePlaylistsKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.focusedColumn == 0 {
		switch {
		case key.Matches(msg, m.keys.NewList):
			m.creatingPlaylist = true
			m.newPlaylistInput.SetValue("")
			return m, tea.Batch(clearScreenCmd(), m.newPlaylistInput.Focus())
		case key.Matches(msg, m.keys.Remove):
			return m.confirmDeleteSelectedPlaylist()
		}
		var cmd tea.Cmd
		m.playlistsList, cmd = m.playlistsList.Update(msg)
		next, loadCmd := m.syncOpenPlaylistToSelection()
		return next, tea.Batch(cmd, loadCmd)
	}

	switch {
	case key.Matches(msg, m.keys.Play):
		return m, m.playSelected(false)
	case key.Matches(msg, m.keys.AudioOnly):
		return m, m.playSelected(true)
	case key.Matches(msg, m.keys.Remove):
		return m.removeSelectedFromOpenPlaylist()
	case key.Matches(msg, m.keys.AddToList):
		return m.openMovePickerForSelected()
	}
	if m.focusedColumn == 1 {
		var cmd tea.Cmd
		m.playlistItemsList, cmd = m.playlistItemsList.Update(msg)
		return m, cmd
	}
	return m, nil
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
		if m.focusedColumn == 1 {
			m.playlistItemsList, cmd = m.playlistItemsList.Update(msg)
		} else {
			m.playlistsList, cmd = m.playlistsList.Update(msg)
		}
	case tabLiked:
		m.likedList, cmd = m.likedList.Update(msg)
	}
	return m, cmd
}

// viewActiveTab renders the current single-list tabs (Feed/Queue/Liked).
// Playlists has its own dedicated multi-column renderer — see
// viewPlaylistsColumns — since it isn't a single list.
func (m Model) viewActiveTab() string {
	switch m.activeTab {
	case tabFeed:
		return m.feedList.View()
	case tabQueue:
		return m.queueList.View()
	case tabLiked:
		return m.likedList.View()
	}
	return ""
}

// viewPlaylistsColumns renders the Playlists tab's up-to-three columns
// (playlists, that playlist's videos, selected video detail), each in a
// focus-aware box — see columnBox and playlistsColumnWidths.
func (m Model) viewPlaylistsColumns(h int) string {
	widths := playlistsColumnWidths(m.width)
	cols := []string{columnBox(m.playlistsList.View(), widths[0], h, m.focusedColumn == 0)}
	if len(widths) > 1 {
		cols = append(cols, columnBox(m.playlistItemsList.View(), widths[1], h, m.focusedColumn == 1))
	}
	if len(widths) > 2 {
		detail := m.renderDetailContent(columnContentHeight(h))
		cols = append(cols, columnBox(detail, widths[2], h, m.focusedColumn == 2))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cols...)
}

// playlistsFooterHints returns the Playlists tab's key legend for whichever
// column currently has focus.
func (m Model) playlistsFooterHints() []hint {
	if m.focusedColumn == 0 {
		return []hint{{"n", "new"}, {"d", "delete"}, {"tab", "switch"}, {"r", "sync"}, {"q", "quit"}}
	}
	return []hint{{"↵", "play"}, {"A", "audio"}, {"d", "remove"}, {"p", "move"}, {"tab", "switch"}, {"r", "sync"}, {"q", "quit"}}
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

// syncOpenPlaylistToSelection loads the currently-highlighted playlist's
// videos into the middle column if it isn't already the one shown there —
// called whenever the highlighted row in playlistsList could have changed
// (initial load, navigation, deletion), so the items/detail columns always
// reflect whatever's highlighted without a separate "open" step (there's
// no drill-down anymore: all three Playlists columns are visible at once).
// Deliberately fires on every highlight change rather than debouncing:
// each fetch is a single cheap videos.list batch call (1 quota unit), and
// the response is staleness-guarded against the selection having moved on
// again before it returns — see handlePlaylistItemsLoaded.
func (m Model) syncOpenPlaylistToSelection() (Model, tea.Cmd) {
	sel, ok := m.playlistsList.SelectedItem().(playlistRow)
	if !ok || sel.playlist.PlaylistID == m.openPlaylistID {
		return m, nil
	}
	m.openPlaylistID = sel.playlist.PlaylistID
	m.openPlaylistTitle = sel.playlist.Title
	m.statusMsg = "loading playlist…"
	m.busy = true
	return m, tea.Batch(openPlaylistCmd(m.cfg, sel.playlist.PlaylistID), m.spinner.Tick)
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

		if id == m.openPlaylistID {
			// The deleted playlist was the one shown in the middle column
			// — clear openPlaylistID so syncOpenPlaylistToSelection below
			// doesn't no-op on its "already showing this one" guard and
			// actually reloads for whatever's now highlighted.
			m.openPlaylistID = ""
		}
		next, loadCmd := m.syncOpenPlaylistToSelection()
		return next, tea.Batch(clearScreenCmd(), deletePlaylistCmd(m.cfg, id, title), loadCmd)
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
		m.busy = true
		return m, tea.Batch(loadPlaylistsCmd(m.cfg), m.spinner.Tick)
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
		m.busy = true
		return m, tea.Batch(loadPlaylistsCmd(m.cfg), m.spinner.Tick)
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

// mirrorPlaylistID resolves the Queue auto-mirror's playlist ID, if known:
// the configured override if set, else whatever queue.json has adopted.
// Returns "" if neither is known (e.g. the mirror hasn't been created
// yet) — callers should treat that as "nothing to filter."
func (m Model) mirrorPlaylistID() string {
	if m.cfg.Queue.MirrorPlaylistID != "" {
		return m.cfg.Queue.MirrorPlaylistID
	}
	qf, err := m.store.LoadQueue()
	if err != nil {
		return ""
	}
	return qf.MirrorPlaylist
}
