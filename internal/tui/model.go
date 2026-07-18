// Package tui implements unspool's Bubble Tea interface. M2 adds Queue,
// Playlists, and Liked tabs alongside the Feed tab shipped in M1 —
// Recommended and History land with their own milestones.
package tui

import (
	"context"
	"fmt"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ali5ter/unspool/config"
	"github.com/ali5ter/unspool/internal/api"
	"github.com/ali5ter/unspool/internal/auth"
	"github.com/ali5ter/unspool/internal/feed"
	"github.com/ali5ter/unspool/internal/playback"
	"github.com/ali5ter/unspool/internal/store"
)

// Model is the top-level Bubble Tea model.
type Model struct {
	cfg   *config.Config
	store *store.Store
	keys  keyMap

	activeTab tab

	feedList          list.Model
	queueList         list.Model
	playlistsList     list.Model
	playlistItemsList list.Model
	likedList         list.Model

	spinner spinner.Model

	syncing     bool
	everSynced  bool // false only during the very first sync — shows the full splash
	quotaSpent  int
	quotaBudget int
	statusMsg   string

	width, height int

	// videoIndex resolves a video ID to its last-known feed metadata, used
	// to render Queue rows (which only persist video IDs).
	videoIndex map[string]feed.Item

	// Playlists tab drill-down state.
	viewingPlaylist   bool
	openPlaylistID    string
	openPlaylistTitle string
	playlistsLoaded   bool

	// "add to playlist" picker overlay.
	pickerActive  bool
	pickerVideo   store.Video
	pickerChannel string
	pickerPending bool // waiting on playlists to load before opening
	pickerList    list.Model

	// "create playlist" input overlay.
	creatingPlaylist bool
	newPlaylistInput textinput.Model

	likedLoaded bool
}

// New builds the initial (pre-sync) model.
func New(cfg *config.Config) Model {
	newListModel := func() list.Model {
		del := list.NewDefaultDelegate()
		del.Styles.SelectedTitle = styleSelected.Foreground(colorAccent)
		del.Styles.SelectedDesc = styleSelected.Foreground(colorMuted)
		del.Styles.NormalTitle = lipgloss.NewStyle().Padding(0, 0, 0, 2)
		del.Styles.NormalDesc = lipgloss.NewStyle().Padding(0, 0, 0, 2)
		l := list.New(nil, del, 0, 0)
		l.SetShowTitle(false)
		l.SetShowStatusBar(false)
		l.SetShowHelp(false)
		return l
	}

	sp := spinner.New()
	sp.Style = lipgloss.NewStyle().Foreground(colorAccent)

	ti := textinput.New()
	ti.Placeholder = "playlist title"

	playlistItemsList := newListModel()
	playlistItemsList.SetShowTitle(true)
	playlistItemsList.Styles.Title = styleDialogTitle.Background(colorBG)

	return Model{
		cfg:               cfg,
		store:             store.New(cfg.StoreDir),
		keys:              newKeyMap(),
		feedList:          newListModel(),
		queueList:         newListModel(),
		playlistsList:     newListModel(),
		playlistItemsList: playlistItemsList,
		likedList:         newListModel(),
		pickerList:        newListModel(),
		spinner:           sp,
		syncing:           true,
		quotaBudget:       api.DailyQuota,
		statusMsg:         "syncing…",
		videoIndex:        map[string]feed.Item{},
		newPlaylistInput:  ti,
	}
}

// newClient builds a fresh authenticated API client. Called per-action
// rather than cached on the model — the token itself is cached in the
// keychain, so this is a cheap local read plus lazy refresh, not a
// re-authentication.
func newClient(ctx context.Context, cfg *config.Config) (*api.Client, error) {
	hc, err := auth.Client(ctx, cfg.OAuthClientSecretFile)
	if err != nil {
		return nil, err
	}
	return api.NewClient(ctx, hc)
}

type syncDoneMsg struct {
	result *feed.Result
	err    error
}

// statusErrMsg carries a non-sync error (or nil for success) into the
// status bar without being mistaken for a sync failure.
type statusErrMsg struct {
	text string
	err  error
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, runSync(m.cfg))
}

func runSync(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		result, err := feed.Sync(context.Background(), cfg)
		return syncDoneMsg{result: result, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		h := listHeight(msg.Height)
		m.feedList.SetSize(msg.Width, h)
		m.queueList.SetSize(msg.Width, h)
		m.playlistsList.SetSize(msg.Width, h)
		m.playlistItemsList.SetSize(msg.Width, h)
		m.likedList.SetSize(msg.Width, h)
		m.pickerList.SetSize(modalListSize(msg.Width, msg.Height))
		return m, nil

	case spinner.TickMsg:
		if m.syncing {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case syncDoneMsg:
		return m.handleSyncDone(msg)

	case statusErrMsg:
		if msg.err != nil {
			m.statusMsg = msg.err.Error()
		} else {
			m.statusMsg = msg.text
		}
		return m, nil

	case playlistsLoadedMsg:
		return m.handlePlaylistsLoaded(msg)

	case playlistItemsLoadedMsg:
		return m.handlePlaylistItemsLoaded(msg)

	case playlistCreatedMsg:
		return m.handlePlaylistCreated(msg)

	case likedLoadedMsg:
		return m.handleLikedLoaded(msg)

	case tea.KeyPressMsg:
		if m.creatingPlaylist {
			return m.updateCreatingPlaylist(msg)
		}
		if m.pickerActive {
			return m.updatePicker(msg)
		}
		if handled, next, cmd := m.handleGlobalKey(msg); handled {
			return next, cmd
		}
		return m.handleTabKey(msg)
	}

	return m.updateActiveList(msg)
}

func (m Model) handleSyncDone(msg syncDoneMsg) (tea.Model, tea.Cmd) {
	m.syncing = false
	m.everSynced = true
	if msg.err != nil {
		m.statusMsg = "sync failed: " + msg.err.Error()
		return m, nil
	}
	m.quotaSpent = msg.result.QuotaSpent

	items := make([]list.Item, 0, len(msg.result.Items))
	m.videoIndex = make(map[string]feed.Item, len(msg.result.Items))
	for _, it := range msg.result.Items {
		items = append(items, feedItem{it})
		m.videoIndex[it.Video.VideoID] = it
	}
	m.feedList.SetItems(items)
	m.refreshQueueList()

	switch {
	case msg.result.MirrorErr != nil:
		m.statusMsg = "synced (queue mirror failed: " + msg.result.MirrorErr.Error() + ")"
	case len(msg.result.SkippedChannels) > 0:
		m.statusMsg = fmt.Sprintf("synced (%d channels skipped)", len(msg.result.SkippedChannels))
	default:
		m.statusMsg = "synced"
	}
	return m, nil
}

// handleGlobalKey handles keys valid regardless of the active tab. Returns
// handled=false to fall through to tab-specific handling.
func (m Model) handleGlobalKey(msg tea.KeyPressMsg) (bool, Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return true, m, tea.Quit
	case key.Matches(msg, m.keys.Sync):
		m.syncing = true
		m.statusMsg = "syncing…"
		return true, m, tea.Batch(m.spinner.Tick, runSync(m.cfg))
	case key.Matches(msg, m.keys.NextTab):
		m.activeTab = m.activeTab.next()
		return true, m, m.onTabChanged()
	case key.Matches(msg, m.keys.PrevTab):
		m.activeTab = m.activeTab.prev()
		return true, m, m.onTabChanged()
	}
	return false, m, nil
}

// onTabChanged lazily loads data the first time a tab is viewed.
func (m *Model) onTabChanged() tea.Cmd {
	switch m.activeTab {
	case tabPlaylists:
		if !m.playlistsLoaded {
			m.statusMsg = "loading playlists…"
			return loadPlaylistsCmd(m.cfg)
		}
	case tabLiked:
		if !m.likedLoaded {
			m.statusMsg = "loading liked videos…"
			return loadLikedCmd(m.cfg)
		}
	}
	return nil
}

// clearScreenCmd forces a full repaint rather than a differential redraw.
// Used around modal open/close transitions, where the content shape changes
// drastically frame-to-frame and the renderer's diffing can otherwise leave
// stale glyphs behind.
func clearScreenCmd() tea.Cmd {
	return func() tea.Msg { return tea.ClearScreen() }
}

func listHeight(totalHeight int) int {
	h := totalHeight - 2 // header + status bar
	if h < 0 {
		return 0
	}
	return h
}

// modalListSize returns a sensible fixed-ish size for a list rendered
// inside a floating modal box, clamped to the terminal size.
func modalListSize(termWidth, termHeight int) (int, int) {
	w, h := 56, 10
	if w > termWidth-8 {
		w = termWidth - 8
	}
	if h > termHeight-8 {
		h = termHeight - 8
	}
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	return w, h
}

func (m Model) View() tea.View {
	var view string
	switch {
	case m.syncing && !m.everSynced:
		view = m.viewSplash()
	case m.pickerActive:
		view = m.overlayModal(m.renderPicker())
	case m.creatingPlaylist:
		view = m.overlayModal(m.renderCreatePlaylist())
	default:
		header := renderHeader(m.activeTab, m.width)
		status := styleStatusBar.Width(m.width).Render(m.statusLine())
		view = lipgloss.JoinVertical(lipgloss.Left, header, m.viewActiveTab(), status)
	}

	v := tea.NewView(view)
	v.AltScreen = true
	return v
}

// viewSplash renders the startup screen shown only during the very first
// sync — the gradient logo above a dialog with a spinner, mirroring wwlog's
// splash/loading screens.
func (m Model) viewSplash() string {
	dialog := renderDialog("unspool", styleSplashSub.Render(m.spinner.View()+"  Syncing your subscriptions…"), "ctrl+c to quit")
	content := lipgloss.JoinVertical(lipgloss.Center, renderGradientLogo(), "", dialog)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) statusLine() string {
	hints := "↵ play  A audio  a queue  m mute  l like  p playlist  tab switch  r sync  q quit"
	if m.activeTab == tabQueue {
		hints = "↵ play  d remove  tab switch  r sync  q quit"
	}
	if m.activeTab == tabPlaylists && m.viewingPlaylist {
		hints = "↵ play  d remove  esc back  tab switch  q quit"
	}
	if m.activeTab == tabPlaylists && !m.viewingPlaylist {
		hints = "↵ open  n new  tab switch  q quit"
	}
	return fmt.Sprintf("%s   quota %d/%d   %s", hints, m.quotaSpent, m.quotaBudget, m.statusMsg)
}

func (m Model) overlayModal(dialog string) string {
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, dialog)
}

// playSelected launches mpv on the currently selected video, whichever tab
// it's selected from.
func (m Model) playSelected(audioOnly bool) tea.Cmd {
	video, channel, ok := m.selectedVideo()
	if !ok {
		return nil
	}
	cfg, st := m.cfg, m.store
	return func() tea.Msg {
		if err := playback.Play(cfg, st, video, channel, audioOnly); err != nil {
			return statusErrMsg{err: err}
		}
		return statusErrMsg{text: "playing…"}
	}
}

// selectedVideo returns the video (and its channel title, where known)
// selected in the currently active tab's list.
func (m Model) selectedVideo() (store.Video, string, bool) {
	switch m.activeTab {
	case tabFeed:
		if sel, ok := m.feedList.SelectedItem().(feedItem); ok {
			return sel.Video, sel.Channel, true
		}
	case tabQueue:
		if sel, ok := m.queueList.SelectedItem().(queueRow); ok {
			return sel.video, sel.channel, true
		}
	case tabLiked:
		if sel, ok := m.likedList.SelectedItem().(likedRow); ok {
			return sel.video, sel.video.ChannelTitle, true
		}
	case tabPlaylists:
		if m.viewingPlaylist {
			if sel, ok := m.playlistItemsList.SelectedItem().(playlistItemRow); ok {
				return store.Video{VideoID: sel.ref.VideoID, Title: sel.ref.Title}, "", true
			}
		}
	}
	return store.Video{}, "", false
}
