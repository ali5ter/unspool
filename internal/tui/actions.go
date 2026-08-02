package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"golang.org/x/sync/errgroup"

	"github.com/ali5ter/unspool/config"
	"github.com/ali5ter/unspool/internal/api"
	"github.com/ali5ter/unspool/internal/queue"
	"github.com/ali5ter/unspool/internal/recommend"
	"github.com/ali5ter/unspool/internal/store"
)

// sortPlaylistRows sorts playlistRow items alphabetically by title
// (case-insensitive), in place. A real sort/search mode is future work
// (M3+) — this just gives a stable, predictable default order instead of
// whatever playlists.list happens to return.
func sortPlaylistRows(items []list.Item) {
	sort.SliceStable(items, func(i, j int) bool {
		a, _ := items[i].(playlistRow)
		b, _ := items[j].(playlistRow)
		return strings.ToLower(a.playlist.Title) < strings.ToLower(b.playlist.Title)
	})
}

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

// removeSelectedFromLiked unlikes the selected video and removes its row
// from the Liked tab's list, optimistically — same "update the list now,
// persist async" pattern as removeSelectedFromQueue/
// removeSelectedFromOpenPlaylist. Bound to the Remove key ("d") so there's
// always an explicit, unambiguous way to remove a Liked-tab item, distinct
// from the Like key's toggle (which depended on the local liked-state
// cache staying correct — see loadLikedCmd/SyncLikedVideos) — see issue #7.
func (m Model) removeSelectedFromLiked() (tea.Model, tea.Cmd) {
	sel, ok := m.likedList.SelectedItem().(likedRow)
	if !ok {
		return m, nil
	}
	videoID := sel.video.VideoID

	items := m.likedList.Items()
	kept := make([]list.Item, 0, len(items))
	for _, it := range items {
		if lr, ok := it.(likedRow); ok && lr.video.VideoID == videoID {
			continue
		}
		kept = append(kept, it)
	}
	m.likedList.SetItems(kept)

	return m, unlikeCmd(m.cfg, videoID)
}

// unlikeCmd clears a video's like rating on YouTube and in the local
// cache — the async half of removeSelectedFromLiked.
func unlikeCmd(cfg *config.Config, videoID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		client, err := newClient(ctx, cfg)
		if err != nil {
			return statusErrMsg{err: err}
		}
		if err := client.RateVideo(ctx, videoID, "none"); err != nil {
			return statusErrMsg{err: err}
		}
		st := store.New(cfg.StoreDir)
		if err := st.SetVideoLiked(videoID, false); err != nil {
			return statusErrMsg{err: err}
		}
		return statusErrMsg{text: "removed from liked"}
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
	m.busy = false
	if msg.err != nil {
		m.statusMsg = "load playlists failed: " + msg.err.Error()
		m.pickerPending = false
		m.pendingPlaylistJump = ""
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
	sortPlaylistRows(items)
	m.playlistsList.SetItems(items)
	if m.pickerMoveFromID != "" {
		m.pickerList.SetItems(excludePlaylist(items, m.pickerMoveFromID))
	} else {
		m.pickerList.SetItems(items)
	}
	m.statusMsg = "loaded playlists"

	if m.pendingPlaylistJump != "" {
		m.selectPlaylistByID(m.pendingPlaylistJump)
		m.pendingPlaylistJump = ""
	}

	var cmds []tea.Cmd
	if m.activeTab == tabPlaylists {
		// Show the first (now alphabetically first) playlist's videos
		// immediately — there's no drill-down step to wait for anymore.
		var loadCmd tea.Cmd
		m, loadCmd = m.syncOpenPlaylistToSelection()
		cmds = append(cmds, loadCmd)
	}
	if m.pickerPending {
		m.pickerPending = false
		m.pickerActive = true
		cmds = append(cmds, clearScreenCmd())
	}
	return m, tea.Batch(cmds...)
}

// playlistItemsLoadedMsg carries the result of openPlaylistCmd.
type playlistItemsLoadedMsg struct {
	playlistID string
	refs       []api.PlaylistItemRef
	details    map[string]api.VideoDetail
	err        error
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
			return playlistItemsLoadedMsg{playlistID: playlistID, err: err}
		}
		refs, err := client.ListPlaylistItemRefs(ctx, playlistID)
		if err != nil {
			return playlistItemsLoadedMsg{playlistID: playlistID, err: err}
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
		return playlistItemsLoadedMsg{playlistID: playlistID, refs: refs, details: details}
	}
}

// handlePlaylistItemsLoaded applies a fetch's result — unless the
// highlighted playlist has since moved on to a different one (possibly
// through several more) before this response arrived, in which case it's
// stale and discarded: whatever's shown or in flight now is current, not
// this. Same staleness-guard pattern as playbackExitedMsg's PID check.
func (m Model) handlePlaylistItemsLoaded(msg playlistItemsLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.playlistID != m.openPlaylistID {
		return m, nil
	}
	m.busy = false
	if msg.err != nil {
		m.statusMsg = "load playlist failed: " + msg.err.Error()
		return m, nil
	}
	// feed_state.json's Liked flag is already kept in sync with the real
	// account on every Liked-tab load (Store.SyncLikedVideos, issue #7) —
	// reusing it here for the "already liked" badge is free, no extra API
	// call needed (unlike playlistMembership, the reverse direction).
	fs, _ := m.store.LoadFeedState()
	items := make([]list.Item, 0, len(msg.refs))
	for _, ref := range msg.refs {
		row := playlistItemRow{ref: ref, liked: fs.State[ref.VideoID].Liked}
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
		row.aiBadge = m.aiBadgeFor(ref.VideoID, 0)
		items = append(items, row)
	}
	m.playlistItemsList.SetItems(items)
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

	plItems := append(m.playlistsList.Items(), row)
	sortPlaylistRows(plItems)
	m.playlistsList.SetItems(plItems)

	pickerItems := append(m.pickerList.Items(), row)
	sortPlaylistRows(pickerItems)
	m.pickerList.SetItems(pickerItems)
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
		ids := make([]string, 0, len(videos))
		for _, v := range videos {
			ids = append(ids, v.VideoID)
		}
		st := store.New(cfg.StoreDir)
		if err := st.SyncLikedVideos(ids); err != nil {
			return likedLoadedMsg{err: fmt.Errorf("sync local liked state: %w", err)}
		}
		return likedLoadedMsg{videos: videos}
	}
}

func (m Model) handleLikedLoaded(msg likedLoadedMsg) (tea.Model, tea.Cmd) {
	m.likedLoaded = true
	m.busy = false
	if msg.err != nil {
		m.statusMsg = "load liked videos failed: " + msg.err.Error()
		return m, nil
	}
	items := make([]list.Item, 0, len(msg.videos))
	for _, v := range msg.videos {
		items = append(items, likedRow{
			video:       v,
			aiBadge:     m.aiBadgeFor(v.VideoID, 0),
			inPlaylists: m.playlistMembership[v.VideoID], // nil until loadPlaylistMembershipCmd resolves — fine, see likedRow's field doc
		})
	}
	m.likedList.SetItems(items)
	m.statusMsg = "loaded liked videos"
	return m, nil
}

// playlistMembershipLoadedMsg carries the result of
// loadPlaylistMembershipCmd: a video ID -> playlist-titles reverse index.
type playlistMembershipLoadedMsg struct {
	membership map[string][]string
	err        error
}

// playlistMembershipConcurrency bounds how many playlists' contents are
// fetched at once — politeness toward the API at this scale (a handful of
// playlists), the same idea as internal/feed.Sync's channelSyncConcurrency
// rather than a real quota concern (playlistItems.list is 1 unit/page).
const playlistMembershipConcurrency = 5

// loadPlaylistMembershipCmd fetches every (non-mirror) playlist's contents
// and inverts them into video ID -> playlist titles, so the Liked tab can
// show which playlist(s) a video is already in (issue #13). Best-effort:
// a single playlist failing to list doesn't blank the whole index, and
// the command itself only reports a total failure (e.g. auth) via err.
func loadPlaylistMembershipCmd(cfg *config.Config, mirrorID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		client, err := newClient(ctx, cfg)
		if err != nil {
			return playlistMembershipLoadedMsg{err: err}
		}
		playlists, err := client.ListPlaylists(ctx)
		if err != nil {
			return playlistMembershipLoadedMsg{err: err}
		}

		var mu sync.Mutex
		membership := map[string][]string{}
		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(playlistMembershipConcurrency)
		for _, pl := range playlists {
			if mirrorID != "" && pl.PlaylistID == mirrorID {
				continue
			}
			pl := pl
			g.Go(func() error {
				refs, err := client.ListPlaylistItemRefs(gctx, pl.PlaylistID)
				if err != nil {
					return nil // best-effort — skip this one playlist, keep the rest
				}
				mu.Lock()
				for _, ref := range refs {
					membership[ref.VideoID] = append(membership[ref.VideoID], pl.Title)
				}
				mu.Unlock()
				return nil
			})
		}
		_ = g.Wait()
		return playlistMembershipLoadedMsg{membership: membership}
	}
}

// handlePlaylistMembershipLoaded stores the freshly built index and, if
// the Liked tab's rows are already on screen (loadLikedCmd resolved
// first — the two run concurrently, either can win), patches them in
// place rather than waiting for some unrelated reload to pick it up —
// same idea as handleThumbnailLoaded.
func (m Model) handlePlaylistMembershipLoaded(msg playlistMembershipLoadedMsg) (tea.Model, tea.Cmd) {
	m.playlistMembershipLoaded = true
	if msg.err != nil {
		return m, nil // best-effort — badges just don't appear, no error surfaced
	}
	m.playlistMembership = msg.membership

	items := m.likedList.Items()
	patched := make([]list.Item, 0, len(items))
	for _, it := range items {
		row, ok := it.(likedRow)
		if !ok {
			patched = append(patched, it)
			continue
		}
		row.inPlaylists = m.playlistMembership[row.video.VideoID]
		patched = append(patched, row)
	}
	m.likedList.SetItems(patched)
	return m, nil
}

// recommendedLoadedMsg carries the result of loadRecommendedCmd. Unlike
// every other lazy-load command, this one never touches the network — it's
// entirely a local computation (PRD §5.8) — but it still runs through the
// same async+spinner path as Liked/Playlists for a consistent lazy-load UX
// rather than special-casing "this one happens to be free."
type recommendedLoadedMsg struct {
	items []recommend.Item
	err   error
}

func loadRecommendedCmd(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		items, err := recommend.Build(store.New(cfg.StoreDir))
		return recommendedLoadedMsg{items: items, err: err}
	}
}

func (m Model) handleRecommendedLoaded(msg recommendedLoadedMsg) (tea.Model, tea.Cmd) {
	m.recommendedLoaded = true
	m.busy = false
	if msg.err != nil {
		m.statusMsg = "load recommendations failed: " + msg.err.Error()
		return m, nil
	}
	if len(msg.items) == 0 {
		m.recommendedList.SetItems([]list.Item{
			noticeRow{text: "No recommendations yet — watch a few videos and sync again."},
		})
		m.statusMsg = "loaded recommendations"
		return m, nil
	}
	items := make([]list.Item, 0, len(msg.items))
	for _, it := range msg.items {
		items = append(items, recommendedRow{item: it, aiBadge: m.aiBadgeFor(it.Video.VideoID, 0)})
	}
	m.recommendedList.SetItems(items)
	m.statusMsg = "loaded recommendations"
	return m, nil
}
