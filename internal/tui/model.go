// Package tui implements unspool's Bubble Tea interface. M2 adds Queue,
// Playlists, and Liked tabs alongside the Feed tab shipped in M1 —
// Recommended and History land with their own milestones.
package tui

import (
	"context"
	"fmt"
	"image/color"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/paginator"
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

	// busy is true only while a real async operation is in flight that a
	// spinner.Tick chain was explicitly started for (sync, loading
	// playlists/liked/a playlist) — see every "…" statusMsg assignment
	// paired with m.spinner.Tick. Deliberately NOT true for "playing…":
	// mpv can run for a video's entire length (hours), and it's mpv doing
	// the work, not unspool, so there's nothing actively "busy" to animate.
	// Drives both whether the spinner keeps ticking (below) and whether
	// statusLine shows the animated glyph+sweep vs. a flat tint.
	busy bool

	// pulseTick advances once per spinner.TickMsg while busy — drives the
	// spinner glyph and the notice-text color sweep in statusLine, so a
	// busy state reads as alive rather than a static amber label.
	pulseTick int

	width, height int

	// focusedColumn indexes which of the active tab's columns currently
	// has keyboard focus — Left/Right (keys.FocusPrev/FocusNext) moves it,
	// Up/Down then acts on whichever column it points at (navigate a list,
	// or scroll the detail column — see focusedColumnKind). Feed/Queue/
	// Liked have up to 2 columns (list, detail); Playlists has up to 3
	// (playlists, its videos, detail). Reset to 0 on every tab switch.
	focusedColumn int

	// videoIndex resolves a video ID to its last-known feed metadata, used
	// to render Queue rows (which only persist video IDs).
	videoIndex map[string]feed.Item

	// openPlaylistID/openPlaylistTitle track which playlist the middle
	// column (playlistItemsList) currently shows — kept in sync with
	// whatever's highlighted in playlistsList by syncOpenPlaylistToSelection,
	// not by an explicit "open" keypress (there's no drill-down: all three
	// Playlists columns are visible at once).
	openPlaylistID    string
	openPlaylistTitle string
	playlistsLoaded   bool

	// "add to playlist" picker overlay. pickerMoveItemID/pickerMoveFromID
	// are set only when the picker was opened from the Playlists tab's
	// items/detail column ('p' while focusedColumn is 1 or 2) — see
	// openMovePickerForSelected — turning "add to" into "move to":
	// confirming also removes the item from the source playlist, and the
	// source itself is excluded from the picker's choices (moving a video
	// to the playlist it's already in is a confusing no-op, not worth
	// supporting).
	pickerActive     bool
	pickerVideo      store.Video
	pickerChannel    string
	pickerPending    bool // waiting on playlists to load before opening
	pickerList       list.Model
	pickerMoveItemID string
	pickerMoveFromID string

	// "create playlist" input overlay.
	creatingPlaylist bool
	newPlaylistInput textinput.Model

	// "delete playlist" confirm overlay.
	deletingPlaylist    bool
	deletePlaylistID    string
	deletePlaylistTitle string

	likedLoaded bool

	// Preview pane (PRD §7.1) — cached and only recomputed when the
	// selected item or width changes, since Glamour rendering isn't cheap
	// enough to redo on every View() call.
	previewVideoID   string
	previewContent   string
	previewWidthUsed int
	previewScroll    int // up/down while the detail column is focused — see handleGlobalKey

	// playingProcess is the currently-running mpv process, if any — tracked
	// so the Stop key can kill it even if its window never took focus.
	playingProcess *os.Process

	// dirtySeen holds video IDs marked seen in-memory (see markSeenIfNeeded)
	// that haven't been flushed to feed_state.json yet — see flushSeenCmd.
	dirtySeen map[string]bool
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
		// The built-in paginator's row was previously disabled here after
		// it silently pushed the status bar off-screen on long feeds — not
		// reproducible against the current bubbles/list version (confirmed
		// directly: list.View()'s line count matches SetSize()'s height
		// exactly with pagination on), so re-enabled. Dots is the library's
		// own default, which reads fine for a handful of pages but turns
		// into a long unreadable row for a 1000+ video feed at ~6 items/
		// page (close to 200 pages) — Arabic ("current/total") stays
		// compact regardless of feed size.
		l.SetShowPagination(true)
		l.Paginator.Type = paginator.Arabic
		return l
	}

	// MiniDot is the classic Braille-dot spinner (⠋⠙⠹⠸…) — spinner.New()'s
	// own default is Line ("|/-\"), which reads as a plain ASCII spinner,
	// not the Braille one this is meant to evoke.
	sp := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	sp.Style = lipgloss.NewStyle().Foreground(colorAccent)

	ti := textinput.New()
	ti.Placeholder = "playlist title"

	return Model{
		cfg:           cfg,
		store:         store.New(cfg.StoreDir),
		keys:          newKeyMap(),
		feedList:      newListModel(),
		queueList:     newListModel(),
		playlistsList: newListModel(),
		// No title (matches every other list): column 0 already shows
		// which playlist is highlighted, so repeating its name as a
		// header here would just be the same information twice.
		playlistItemsList: newListModel(),
		likedList:         newListModel(),
		pickerList:        newListModel(),
		spinner:           sp,
		syncing:           true,
		busy:              true,
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

// Update handles a message, marks the Feed tab's current selection seen,
// and refreshes the cached preview afterward, so View() never has to
// re-run Glamour rendering or seen-state bookkeeping itself.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.updateInner(msg)
	nm := next.(Model)
	nm.markSeenIfNeeded()
	nm.refreshPreview()
	return nm, cmd
}

func (m Model) updateInner(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		ch := columnContentHeight(listHeight(msg.Height))

		lw := columnContentWidth(listWidthFor(msg.Width))
		m.feedList.SetSize(lw, ch)
		m.queueList.SetSize(lw, ch)
		m.likedList.SetSize(lw, ch)

		// Playlists' three columns split the width differently from
		// Feed/Queue/Liked's list+detail split — see playlistsColumnWidths.
		plWidths := playlistsColumnWidths(msg.Width)
		m.playlistsList.SetSize(columnContentWidth(plWidths[0]), ch)
		if len(plWidths) > 1 {
			m.playlistItemsList.SetSize(columnContentWidth(plWidths[1]), ch)
		} else {
			m.playlistItemsList.SetSize(columnContentWidth(plWidths[0]), ch)
		}

		m.pickerList.SetSize(modalListSize(msg.Width, msg.Height))

		if n := m.activeColumnCount(); m.focusedColumn >= n {
			m.focusedColumn = n - 1
		}
		if m.focusedColumn < 0 {
			m.focusedColumn = 0
		}
		return m, nil

	case spinner.TickMsg:
		if m.busy {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			m.pulseTick++
			return m, cmd
		}
		return m, nil

	case syncDoneMsg:
		return m.handleSyncDone(msg)

	case statusErrMsg:
		if msg.err != nil {
			m.statusMsg = firstLine(msg.err.Error())
		} else {
			m.statusMsg = msg.text
		}
		return m, nil

	case seenFlushedMsg:
		// Silent on success — marking videos seen shouldn't announce itself
		// over whatever the status bar is already showing.
		if msg.err != nil {
			m.statusMsg = "mark seen failed: " + msg.err.Error()
		}
		return m, nil

	case playlistsLoadedMsg:
		return m.handlePlaylistsLoaded(msg)

	case playlistItemsLoadedMsg:
		return m.handlePlaylistItemsLoaded(msg)

	case playlistCreatedMsg:
		return m.handlePlaylistCreated(msg)

	case playlistDeletedMsg:
		return m.handlePlaylistDeleted(msg)

	case likedLoadedMsg:
		return m.handleLikedLoaded(msg)

	case playbackStartedMsg:
		// m.busy stays false here, deliberately: unlike sync/loading,
		// "playing…" can sit for a video's entire runtime (hours), and mpv
		// — not unspool — is doing the work, so there's nothing actively
		// "busy" to animate. renderNotice gives it a flat amber tint
		// instead of the spinner+sweep treatment.
		m.playingProcess = msg.handle.Process()
		m.statusMsg = "playing…"
		return m, waitForExitCmd(msg.handle)

	case playbackExitedMsg:
		// Stale if the user has since stopped this video or started a
		// different one — nothing to report, whatever's showing now (a
		// new "playing…", a different error, whatever) is current and
		// this shouldn't stomp it. mpv can fail anywhere from ~2s to 30+
		// seconds after launch depending on why (confirmed live), so this
		// can arrive well after the user has moved on.
		if m.playingProcess == nil || m.playingProcess.Pid != msg.pid {
			return m, nil
		}
		m.playingProcess = nil
		if msg.err != nil {
			m.statusMsg = "play failed: " + firstLine(msg.err.Error())
		}
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
			if ids := m.drainDirtySeen(); len(ids) > 0 {
				_ = m.store.MarkVideosSeen(ids)
			}
			return m, tea.Quit
		}
		if m.creatingPlaylist {
			return m.updateCreatingPlaylist(msg)
		}
		if m.deletingPlaylist {
			return m.updateDeletingPlaylist(msg)
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
	m.busy = false
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

// markSeenIfNeeded marks the Feed tab's currently selected video as seen —
// PRD §5.1: "a video is 'new' until viewed in the feed". Runs after every
// Update() call (see the Update wrapper), including ordinary up/down
// navigation, so it updates the visible row immediately (the ● badge
// disappears right away) but only queues the change in m.dirtySeen for a
// later batched write — see flushSeenCmd for why persisting here directly
// would be a mistake. Scoped to the Feed tab: it's the only list that
// renders new/seen state.
func (m *Model) markSeenIfNeeded() {
	if m.activeTab != tabFeed {
		return
	}
	item, ok := m.feedList.SelectedItem().(feedItem)
	if !ok || item.State.Seen {
		return
	}
	item.State.Seen = true
	m.feedList.SetItem(m.feedList.Index(), item)
	if entry, ok := m.videoIndex[item.Video.VideoID]; ok {
		entry.State.Seen = true
		m.videoIndex[item.Video.VideoID] = entry
	}
	if m.dirtySeen == nil {
		m.dirtySeen = map[string]bool{}
	}
	m.dirtySeen[item.Video.VideoID] = true
}

// drainDirtySeen returns the pending seen-state video IDs and clears the
// set, so a caller can hand them to a write without double-flushing.
func (m *Model) drainDirtySeen() []string {
	if len(m.dirtySeen) == 0 {
		return nil
	}
	ids := make([]string, 0, len(m.dirtySeen))
	for id := range m.dirtySeen {
		ids = append(ids, id)
	}
	m.dirtySeen = nil
	return ids
}

// seenFlushedMsg carries the result of a batched seen-state write back to
// the model.
type seenFlushedMsg struct{ err error }

// flushSeenCmd persists a batch of seen video IDs in one write. Marking
// seen fires on every Feed-tab selection change, so writing feed_state.json
// per keystroke would either hammer disk during a fast scroll (it's a
// full-file atomic rewrite, and on a large feed not a cheap one) or, if
// dispatched as one async Cmd per keystroke, risk concurrent goroutines
// racing that file's read-modify-write and losing each other's updates.
// Batching at natural pause points (tab switch, quit) avoids both.
func flushSeenCmd(st *store.Store, ids []string) tea.Cmd {
	return func() tea.Msg {
		return seenFlushedMsg{err: st.MarkVideosSeen(ids)}
	}
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
		m.busy = true
		m.statusMsg = "syncing…"
		return true, m, tea.Batch(m.spinner.Tick, runSync(m.cfg))
	case key.Matches(msg, m.keys.NextTab):
		m.activeTab = m.activeTab.next()
		m.focusedColumn = 0
		return true, m, m.onTabChanged()
	case key.Matches(msg, m.keys.PrevTab):
		m.activeTab = m.activeTab.prev()
		m.focusedColumn = 0
		return true, m, m.onTabChanged()
	case key.Matches(msg, m.keys.FocusNext):
		if n := m.activeColumnCount(); m.focusedColumn < n-1 {
			m.focusedColumn++
		}
		return true, m, nil
	case key.Matches(msg, m.keys.FocusPrev):
		if m.focusedColumn > 0 {
			m.focusedColumn--
		}
		return true, m, nil
	case key.Matches(msg, m.keys.Up):
		// Only intercepted while the detail column has focus — otherwise
		// falls through (handled=false) so the tab handler forwards it to
		// whichever list currently has focus, same as before Left/Right
		// column focus existed.
		if m.focusedColumnKind() == columnDetail {
			m.previewScroll -= previewScrollStep
			if m.previewScroll < 0 {
				m.previewScroll = 0
			}
			return true, m, nil
		}
	case key.Matches(msg, m.keys.Down):
		if m.focusedColumnKind() == columnDetail {
			m.previewScroll += previewScrollStep
			maxScroll := previewScrollMax(m.previewContent, columnContentHeight(listHeight(m.height)))
			if m.previewScroll > maxScroll {
				m.previewScroll = maxScroll
			}
			return true, m, nil
		}
	}
	return false, m, nil
}

// previewScrollStep is how many lines up/down moves the detail column's
// content per press while it's focused — small enough to feel like
// scrolling, not paging.
const previewScrollStep = 3

// onTabChanged lazily loads data the first time a tab is viewed, and
// flushes any seen-state accumulated while the Feed tab was active — see
// markSeenIfNeeded and flushSeenCmd.
func (m *Model) onTabChanged() tea.Cmd {
	var cmds []tea.Cmd
	if ids := m.drainDirtySeen(); len(ids) > 0 {
		cmds = append(cmds, flushSeenCmd(m.store, ids))
	}

	switch m.activeTab {
	case tabPlaylists:
		switch {
		case !m.playlistsLoaded:
			m.statusMsg = "loading playlists…"
			m.busy = true
			cmds = append(cmds, loadPlaylistsCmd(m.cfg), m.spinner.Tick)
		case m.openPlaylistID == "":
			// Playlists were loaded already (e.g. via the add-to-playlist
			// picker from another tab) but the middle column was never
			// populated — sync it now rather than showing an empty column
			// until the user happens to move the playlists selection.
			var loadCmd tea.Cmd
			*m, loadCmd = m.syncOpenPlaylistToSelection()
			cmds = append(cmds, loadCmd)
		}
	case tabLiked:
		if !m.likedLoaded {
			m.statusMsg = "loading liked videos…"
			m.busy = true
			cmds = append(cmds, loadLikedCmd(m.cfg), m.spinner.Tick)
		}
	}
	return tea.Batch(cmds...)
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

// footerHeight is the footer's total row count: statusLine's 2 content
// rows. Previously +1 for a rule above them (styleStatusBar's top
// border) — removed now that every column above renders in its own
// bordered box, whose bottom edge already marks this boundary.
const footerHeight = 2

func listHeight(totalHeight int) int {
	h := totalHeight - headerHeight - footerHeight
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
	case m.deletingPlaylist:
		view = m.overlayModal(m.renderDeletePlaylist())
	default:
		header := renderHeader(m.activeTab, m.width)
		status := styleStatusBar.Width(m.width).Render(m.statusLine())
		view = lipgloss.JoinVertical(lipgloss.Left, header, m.viewBody(), status)
	}

	v := tea.NewView(view)
	v.AltScreen = true
	return v
}

// viewBody renders the active tab's column(s), each wrapped in a
// focus-aware box (see columnBox) — Playlists has up to three (playlists /
// its videos / video detail), every other tab has up to two (list /
// detail).
func (m Model) viewBody() string {
	h := listHeight(m.height)
	if m.activeTab == tabPlaylists {
		return m.viewPlaylistsColumns(h)
	}

	cols := []string{columnBox(m.viewActiveTab(), listWidthFor(m.width), h, m.focusedColumn == 0)}
	if m.activeColumnCount() == 2 {
		detail := m.renderDetailContent(columnContentHeight(h))
		cols = append(cols, columnBox(detail, previewWidth(m.width), h, m.focusedColumn == 1))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cols...)
}

// viewSplash renders the startup screen shown only during the very first
// sync — the gradient logo above a dialog with a spinner, mirroring wwlog's
// splash/loading screens.
func (m Model) viewSplash() string {
	text := m.spinnerGlyph() + "  Syncing your subscriptions…"
	notice := sweepText(text, m.pulseTick, colorPanel)
	dialog := renderDialog("unspool", notice, "ctrl+c to quit")
	content := lipgloss.JoinVertical(lipgloss.Center, renderLogo(), "", dialog)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

// hint is one key/action pair in the status bar's key legend.
type hint struct {
	key, label string
}

// footerHints returns the key legend for the current tab/state — the same
// wording as before, just structured so statusLine can style the key
// distinctly from its label.
func (m Model) footerHints() []hint {
	var hints []hint
	switch {
	case m.activeTab == tabQueue:
		hints = []hint{{"↵", "play"}, {"d", "remove"}, {"tab", "switch"}, {"r", "sync"}, {"q", "quit"}}
	case m.activeTab == tabPlaylists:
		hints = m.playlistsFooterHints()
	default:
		hints = []hint{{"↵", "play"}, {"A", "audio"}, {"a", "queue"}, {"m", "mute"}, {"l", "like"}, {"p", "playlist"}, {"tab", "switch"}, {"r", "sync"}, {"q", "quit"}}
	}
	// Only relevant where there's more than one column to move focus
	// between — no point hinting it over a single-column narrow layout.
	if m.activeColumnCount() > 1 {
		hints = append(hints, hint{"←→", "focus"})
	}
	if m.playingProcess != nil {
		hints = append([]hint{{"S", "stop"}}, hints...)
	}
	return hints
}

// statusLine renders the footer: a key legend (key bolded, action label
// dim — previously rendered as one undifferentiated string, hard to scan),
// the quota meter, and the status notice, tinted by what kind of message it
// is (see statusToneColor) since a plain-grey "sync failed: ..." and a
// plain-grey "synced" previously looked identical from across the room.
// statusLine renders the footer's two rows: hints + quota on the first,
// the status notice alone on the second. Previously all three shared one
// row, which meant a long notice ("synced (3 channels skipped)") ran off
// the edge of the terminal and got silently cropped by the outer Width()
// wrap on anything narrower than roughly 130 columns — confirmed via a cast
// recording showing "synced (1" with the rest of the message gone. Giving
// the notice its own full-width row means it can never lose the race for
// space against the hints and quota meter.
func (m Model) statusLine() string {
	band := lipgloss.NewStyle().Background(colorPanel)
	keyStyle := band.Foreground(colorText).Bold(true)
	labelStyle := band.Foreground(colorMuted)

	parts := make([]string, 0, len(m.footerHints()))
	for _, h := range m.footerHints() {
		parts = append(parts, keyStyle.Render(h.key)+labelStyle.Render(" "+h.label))
	}
	sep := band.Render("  ")
	left := strings.Join(parts, sep)

	quota := labelStyle.Render(fmt.Sprintf("quota %d/%d", m.quotaSpent, m.quotaBudget))
	line1 := left + band.Render("   ") + quota

	return line1 + "\n" + m.renderNotice()
}

// renderNotice renders the status notice (statusLine's second row), tinted
// by what kind of message it is: failures consistently contain "failed";
// genuinely busy states (m.busy — sync, loading playlists/liked/a
// playlist) get the animated spinner glyph plus a color sweep travelling
// across the text (see sweepText); everything else, including "playing…"
// (which can sit for a video's entire runtime with mpv, not unspool, doing
// the work — nothing to animate), gets a flat tint.
func (m Model) renderNotice() string {
	padCell := lipgloss.NewStyle().Background(colorLine).Render(" ")
	flat := func(fg color.Color) string {
		return lipgloss.NewStyle().Background(colorLine).Foreground(fg).Padding(0, 1).Render(m.statusMsg)
	}
	switch {
	case strings.Contains(m.statusMsg, "failed"):
		return flat(colorAccent)
	case m.busy && strings.HasSuffix(m.statusMsg, "…"):
		text := m.spinnerGlyph() + " " + m.statusMsg
		return padCell + sweepText(text, m.pulseTick, colorLine) + padCell
	case strings.HasSuffix(m.statusMsg, "…"):
		return flat(colorAmber)
	default:
		return flat(colorTeal)
	}
}

// spinnerGlyph returns the spinner's current frame with no styling of its
// own. It's deliberately raw text, not m.spinner.View() — that method
// renders through m.spinner.Style, which emits its own ANSI reset at the
// end. Concatenating that into a larger string and then wrapping the whole
// thing in another lipgloss Style.Render() breaks: the inner reset fires
// partway through, so everything after the glyph (the notice text) loses
// the outer style entirely and falls back to the terminal's default color.
// Composing the raw glyph into the notice first and styling the combined
// string in one Render call (see statusLine, viewSplash) avoids this.
func (m Model) spinnerGlyph() string {
	sp := m.spinner
	sp.Style = lipgloss.NewStyle()
	return sp.View()
}

// firstLine collapses err to its first non-empty line, marking with "…" if
// there was more. The status notice is rendered as a single row (see
// statusLine) — some errors are naturally multi-line (mpv's own output on
// a failed stream load prints several lines to stdout, confirmed live),
// and without this a multi-line message would spill the notice across
// several rows instead of the one it's laid out for.
func firstLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	first := strings.TrimSpace(lines[0])
	if len(lines) > 1 {
		return first + " …"
	}
	return first
}

func (m Model) overlayModal(dialog string) string {
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, dialog)
}

// playSelected launches mpv on the currently selected video, whichever tab
// it's selected from.
// playbackStartedMsg carries the launched mpv handle back to the model so
// it can be killed later via the Stop key — mpv's window frequently doesn't
// take focus when launched from a background process (a macOS quirk), and
// without this there'd be no way to stop a stuck, unreachable video short
// of quitting the whole terminal session. Also used to kick off
// waitForExitCmd — see playbackExitedMsg for why that's necessary at all.
type playbackStartedMsg struct {
	handle *playback.Handle
}

// playbackExitedMsg carries the eventual result of waiting on a launched
// mpv process. "Eventual" is doing real work in that sentence: confirmed
// live against a real account's videos, a failure can surface anywhere
// from ~2s to 30+ seconds after launch depending on why (extraction error
// vs. a slow region/availability check), so this has no fixed deadline —
// it simply reports whatever actually happened, whenever that turns out to
// be.
type playbackExitedMsg struct {
	pid int
	err error
}

// waitForExitCmd blocks (in its own goroutine, like every tea.Cmd) until
// the given handle's process exits, then reports what happened. Previously
// Play() itself waited up to a fixed 2s before reporting play as having
// succeeded, which was not a safe assumption — see playbackExitedMsg.
func waitForExitCmd(h *playback.Handle) tea.Cmd {
	return func() tea.Msg {
		return playbackExitedMsg{pid: h.Process().Pid, err: h.Wait()}
	}
}

func (m Model) playSelected(audioOnly bool) tea.Cmd {
	video, channel, ok := m.selectedVideo()
	if !ok {
		return nil
	}
	cfg, st := m.cfg, m.store
	launch := func() tea.Msg {
		handle, err := playback.Play(cfg, st, video, channel, audioOnly)
		if err != nil {
			return statusErrMsg{err: fmt.Errorf("play failed: %w", err)}
		}
		return playbackStartedMsg{handle: handle}
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
		if sel, ok := m.playlistItemsList.SelectedItem().(playlistItemRow); ok {
			video := sel.video
			video.VideoID = sel.ref.VideoID
			video.Title = sel.ref.Title
			return video, sel.channel, true
		}
	}
	return store.Video{}, "", false
}
