package tui

import (
	"context"
	"fmt"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/ali5ter/unspool/config"
	"github.com/ali5ter/unspool/internal/api"
	"github.com/ali5ter/unspool/internal/search"
	"github.com/ali5ter/unspool/internal/store"
)

// searchesPerDay is how many search.list calls the daily API quota affords
// (PRD §5.5: "the day's budget is ~100 searches total").
const searchesPerDay = api.DailyQuota / api.CostSearch

func (m Model) updateSearchInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		m.searchActive = false
		return m, clearScreenCmd()
	case key.Matches(msg, m.keys.Confirm):
		query := m.searchInput.Value()
		m.searchActive = false
		if query == "" {
			return m, clearScreenCmd()
		}
		m.searchQuery = query
		results, err := search.Local(m.store, query)
		if err != nil {
			m.statusMsg = "search failed: " + err.Error()
			return m, clearScreenCmd()
		}
		items := make([]list.Item, 0, len(results))
		for _, r := range results {
			items = append(items, searchResultRow{result: r, source: "local", aiBadge: m.aiBadgeFor(r.Video.VideoID, 0)})
		}
		m.searchResultsList.SetItems(items)
		m.searchResultsActive = true
		return m, clearScreenCmd()
	}
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

func (m Model) renderSearchInput() string {
	return renderDialog("Search", m.searchInput.View(), "↵ search   esc cancel")
}

func (m Model) updateSearchResults(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		m.searchResultsActive = false
		return m, clearScreenCmd()
	case key.Matches(msg, m.keys.SearchWeb):
		if m.searchesUsed >= searchesPerDay {
			m.statusMsg = "no YouTube searches left today"
			return m, nil
		}
		m.statusMsg = "searching YouTube…"
		m.busy = true
		return m, tea.Batch(searchYouTubeCmd(m.cfg, m.searchQuery), m.spinner.Tick)
	case key.Matches(msg, m.keys.Play):
		sel, ok := m.searchResultsList.SelectedItem().(searchResultRow)
		if !ok {
			return m, nil
		}
		if sel.result.Kind == "playlist" {
			return m.jumpToPlaylist(sel.result.PlaylistID)
		}
		return m, m.playSelected(false)
	case key.Matches(msg, m.keys.AudioOnly):
		return m, m.playSelected(true)
	case key.Matches(msg, m.keys.AddQueue):
		return m.addSelectedToQueue()
	case key.Matches(msg, m.keys.Like):
		return m, m.likeSelected()
	case key.Matches(msg, m.keys.AddToList):
		return m.openPickerForSelected()
	case key.Matches(msg, m.keys.Mute):
		return m.muteSelectedChannel()
	}
	var cmd tea.Cmd
	m.searchResultsList, cmd = m.searchResultsList.Update(msg)
	return m, cmd
}

// jumpToPlaylist closes the search-results overlay and switches to the
// Playlists tab with the matched playlist highlighted — a playlist-title
// match isn't individually actionable the way a video is (see
// searchResultRow.Description), so Enter on one just navigates to it.
//
// If playlists haven't been loaded yet this session (a fresh session that
// jumped here without ever visiting the Playlists tab or opening the
// add-to-playlist picker first), m.playlistsList is empty and there's
// nothing to select yet — confirmed live, this was a real bug, not
// theoretical: the naive "find and select by ID" loop silently found
// nothing and the jump did nothing useful. Deferred via
// pendingPlaylistJump, same shape as pickerPending, consumed once
// handlePlaylistsLoaded actually populates the list.
func (m Model) jumpToPlaylist(playlistID string) (tea.Model, tea.Cmd) {
	m.searchResultsActive = false
	m.activeTab = tabPlaylists
	m.focusedColumn = 0

	if !m.playlistsLoaded {
		m.pendingPlaylistJump = playlistID
		m.statusMsg = "loading playlists…"
		m.busy = true
		return m, tea.Batch(clearScreenCmd(), loadPlaylistsCmd(m.cfg), m.spinner.Tick)
	}

	m.selectPlaylistByID(playlistID)
	next, loadCmd := m.syncOpenPlaylistToSelection()
	return next, tea.Batch(clearScreenCmd(), loadCmd)
}

// selectPlaylistByID moves m.playlistsList's selection to playlistID, if
// present — a no-op if it isn't found (e.g. a stale/deleted playlist).
func (m *Model) selectPlaylistByID(playlistID string) {
	for i, it := range m.playlistsList.Items() {
		if p, ok := it.(playlistRow); ok && p.playlist.PlaylistID == playlistID {
			m.playlistsList.Select(i)
			return
		}
	}
}

func (m Model) renderSearchResults() string {
	remaining := searchesPerDay - m.searchesUsed
	hint := "↵ play   p playlist   a queue   l like   m mute   esc close"
	if remaining > 0 {
		hint += fmt.Sprintf("   y search YouTube (%d left today)", remaining)
	}
	return renderDialog("Search: \""+m.searchQuery+"\"", m.searchResultsList.View(), hint)
}

// searchYouTubeDoneMsg carries the result of searchYouTubeCmd.
type searchYouTubeDoneMsg struct {
	query   string
	results []api.SearchResult
	err     error
}

func searchYouTubeCmd(cfg *config.Config, query string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		client, err := newClient(ctx, cfg)
		if err != nil {
			return searchYouTubeDoneMsg{query: query, err: err}
		}
		results, err := client.Search(ctx, query, 25)
		if err != nil {
			return searchYouTubeDoneMsg{query: query, err: err}
		}
		return searchYouTubeDoneMsg{query: query, results: results}
	}
}

func (m Model) handleSearchYouTubeDone(msg searchYouTubeDoneMsg) (tea.Model, tea.Cmd) {
	m.busy = false
	if msg.query != m.searchQuery {
		return m, nil // stale — the query changed (or results were closed) while this was in flight
	}
	if msg.err != nil {
		m.statusMsg = "YouTube search failed: " + firstLine(msg.err.Error())
		return m, nil
	}
	m.searchesUsed++
	existing := m.searchResultsList.Items()
	seen := make(map[string]bool, len(existing))
	for _, it := range existing {
		if r, ok := it.(searchResultRow); ok {
			seen[r.result.Video.VideoID] = true
		}
	}
	items := existing
	for _, r := range msg.results {
		if seen[r.VideoID] {
			continue
		}
		items = append(items, searchResultRow{
			result: search.Result{
				Video: store.Video{
					VideoID:     r.VideoID,
					ChannelID:   r.ChannelID,
					Title:       r.Title,
					PublishedAt: r.PublishedAt,
				},
				Channel: r.ChannelTitle,
				Kind:    "video",
			},
			source:  "youtube",
			aiBadge: m.aiBadgeFor(r.VideoID, 0),
		})
	}
	m.searchResultsList.SetItems(items)
	m.statusMsg = "searched YouTube"
	return m, nil
}
