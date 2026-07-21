// Package tui implements unspool's Bubble Tea interface. M2 adds Queue,
// Playlists, and Liked tabs alongside the Feed tab shipped in M1 —
// Recommended and History land with their own milestones.
package tui

import (
	"context"
	"fmt"
	"os"
	"time"

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

	// Preview pane (PRD §7.1) — cached and only recomputed when the
	// selected item or width changes, since Glamour rendering isn't cheap
	// enough to redo on every View() call.
	previewVideoID   string
	previewContent   string
	previewWidthUsed int

	// playingProcess is the currently-running mpv process, if any — tracked
	// so the Stop key can kill it even if its window never took focus.
	playingProcess *os.Process
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
		// bubbles/list's built-in filter (bound to "/") isn't part of
		// unspool's own key scheme and was never disabled — an errant "/"
		// dropped users into its filter UI unexpectedly (PRD's own "/"
		// filter action, when built, should be unspool's own, not this).
		l.SetFilteringEnabled(false)
		// The built-in paginator ("1/127") adds a row list.View() doesn't
		// account for in listHeight()'s budget, which was silently pushing
		// our own status bar off the bottom of the terminal on long feeds
		// (1000+ items). Scrolling/paging itself still works without the
		// indicator — this just hides the extra row.
		l.SetShowPagination(false)
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

// Update handles a message and refreshes the cached preview afterward, so
// View() never has to re-run Glamour rendering itself.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.updateInner(msg)
	nm := next.(Model)
	nm.refreshPreview()
	return nm, cmd
}

func (m Model) updateInner(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		h := listHeight(msg.Height)
		lw := listWidthFor(msg.Width)
		m.feedList.SetSize(lw, h)
		m.queueList.SetSize(lw, h)
		m.playlistsList.SetSize(lw, h)
		m.playlistItemsList.SetSize(lw, h)
		m.likedList.SetSize(lw, h)
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

	case playbackStartedMsg:
		m.playingProcess = msg.process
		m.statusMsg = "playing…"
		return m, nil

	case tea.KeyPressMsg:
		// Quit must always work, no matter what overlay or state is active
		// — no sub-handler below this point is allowed to swallow it. Also
		// stop any running mpv first: it's launched detached (fire-and-
		// forget) for responsiveness within a session, not to persist
		// invisibly once the user is done with unspool entirely — an
		// orphaned background mpv process with an unreachable window is
		// exactly the stuck-video problem the Stop key exists to solve.
		if key.Matches(msg, m.keys.Quit) {
			_ = playback.Stop(m.playingProcess)
			return m, tea.Quit
		}
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
	// The splash screen (logo + dialog) is a completely different content
	// shape from the main tabbed view — same class of stale-glyph bleed-
	// through as the modal transitions, so force a full repaint whenever
	// this sync is the one taking us out of it.
	var cmd tea.Cmd
	if !m.everSynced {
		cmd = clearScreenCmd()
	}

	m.syncing = false
	m.everSynced = true
	if msg.err != nil {
		m.statusMsg = "sync failed: " + msg.err.Error()
		return m, cmd
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
	return m, cmd
}

// handleGlobalKey handles keys valid regardless of the active tab. Returns
// handled=false to fall through to tab-specific handling. Quit is handled
// earlier, in updateInner, so every overlay/state sees it first.
func (m Model) handleGlobalKey(msg tea.KeyPressMsg) (bool, Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Stop):
		next, cmd := m.stopPlayback()
		return true, next.(Model), cmd
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

// listWidthFor returns the list pane's width given the total terminal
// width, accounting for the preview pane when it's wide enough to show.
func listWidthFor(totalWidth int) int {
	if totalWidth < previewMinWidth {
		return totalWidth
	}
	return totalWidth - previewWidth(totalWidth)
}

// clearScreenCmd forces a full repaint rather than a differential redraw.
// Used around modal open/close transitions and the splash-to-main handoff,
// where the content shape changes drastically frame-to-frame and the
// renderer's diffing can otherwise leave stale glyphs behind.
//
// Batches an immediate ClearScreen with one delayed by a beat: Cmds are
// dispatched to a worker goroutine and can execute either before or after
// the synchronous render that follows the same Update() call they're
// returned from, so the immediate one alone can occasionally lose that race
// and leave a torn frame on screen with nothing left to trigger a
// corrected repaint. The delayed one always lands as its own message (and
// thus its own render cycle), guaranteeing self-correction either way.
func clearScreenCmd() tea.Cmd {
	immediate := func() tea.Msg { return tea.ClearScreen() }
	delayed := tea.Tick(50*time.Millisecond, func(time.Time) tea.Msg {
		return tea.ClearScreen()
	})
	return tea.Batch(immediate, delayed)
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
		body := m.viewActiveTab()
		if m.width >= previewMinWidth {
			body = lipgloss.JoinHorizontal(lipgloss.Top, body, m.renderPreviewPane(listHeight(m.height)))
		}
		view = lipgloss.JoinVertical(lipgloss.Left, header, body, status)
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
	content := lipgloss.JoinVertical(lipgloss.Center, renderLogo(), "", dialog)
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
	if m.playingProcess != nil {
		hints = "S stop  " + hints
	}
	return fmt.Sprintf("%s   quota %d/%d   %s", hints, m.quotaSpent, m.quotaBudget, m.statusMsg)
}

func (m Model) overlayModal(dialog string) string {
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, dialog)
}

// playSelected launches mpv on the currently selected video, whichever tab
// it's selected from.
// playbackStartedMsg carries the spawned mpv process back to the model so
// it can be killed later via the Stop key — mpv's window frequently doesn't
// take focus when launched from a background process (a macOS quirk), and
// without this there'd be no way to stop a stuck, unreachable video short
// of quitting the whole terminal session.
type playbackStartedMsg struct {
	process *os.Process
}

func (m Model) playSelected(audioOnly bool) tea.Cmd {
	video, channel, ok := m.selectedVideo()
	if !ok {
		return nil
	}
	cfg, st := m.cfg, m.store
	launch := func() tea.Msg {
		process, err := playback.Play(cfg, st, video, channel, audioOnly)
		if err != nil {
			return statusErrMsg{err: err}
		}
		return playbackStartedMsg{process: process}
	}
	// mpv/yt-dlp startup (process spawn, stream resolution) isn't instant —
	// without this, the UI shows no change at all until launch finishes,
	// which reads as "nothing happened" even for a sub-second delay.
	immediate := func() tea.Msg { return statusErrMsg{text: "opening mpv…"} }
	return tea.Batch(immediate, launch)
}

func (m Model) stopPlayback() (tea.Model, tea.Cmd) {
	if m.playingProcess == nil {
		m.statusMsg = "nothing playing"
		return m, nil
	}
	err := playback.Stop(m.playingProcess)
	m.playingProcess = nil
	if err != nil {
		return m, func() tea.Msg { return statusErrMsg{err: err} }
	}
	m.statusMsg = "stopped"
	return m, nil
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
