// Package tui implements unspool's Bubble Tea interface. M1 ships the Feed
// tab only — Queue, Playlists, Liked, History, and Recommended land with
// their own milestones (PRD §11).
package tui

import (
	"context"
	"fmt"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ali5ter/unspool/config"
	"github.com/ali5ter/unspool/internal/api"
	"github.com/ali5ter/unspool/internal/feed"
	"github.com/ali5ter/unspool/internal/playback"
	"github.com/ali5ter/unspool/internal/store"
)

// Model is the top-level Bubble Tea model.
type Model struct {
	cfg   *config.Config
	store *store.Store
	keys  keyMap

	list    list.Model
	spinner spinner.Model

	syncing     bool
	quotaSpent  int
	quotaBudget int
	statusMsg   string

	width, height int
}

// New builds the initial (pre-sync) model.
func New(cfg *config.Config) Model {
	del := list.NewDefaultDelegate()
	del.Styles.SelectedTitle = styleSelected.Foreground(colorAccent)
	del.Styles.SelectedDesc = styleSelected.Foreground(colorMuted)
	del.Styles.NormalTitle = lipgloss.NewStyle().Padding(0, 0, 0, 2)
	del.Styles.NormalDesc = lipgloss.NewStyle().Padding(0, 0, 0, 2)

	l := list.New(nil, del, 0, 0)
	l.Title = "unspool · feed"
	l.Styles.Title = styleHeader
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)

	sp := spinner.New()
	sp.Style = lipgloss.NewStyle().Foreground(colorAccent)

	return Model{
		cfg:         cfg,
		store:       store.New(cfg.StoreDir),
		keys:        newKeyMap(),
		list:        l,
		spinner:     sp,
		syncing:     true,
		quotaBudget: api.DailyQuota,
		statusMsg:   "syncing…",
	}
}

type syncDoneMsg struct {
	result *feed.Result
	err    error
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
		m.list.SetSize(msg.Width, listHeight(msg.Height))
		return m, nil

	case spinner.TickMsg:
		if m.syncing {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case syncDoneMsg:
		m.syncing = false
		if msg.err != nil {
			m.statusMsg = "sync failed: " + msg.err.Error()
			return m, nil
		}
		m.quotaSpent = msg.result.QuotaSpent
		items := make([]list.Item, 0, len(msg.result.Items))
		for _, it := range msg.result.Items {
			items = append(items, feedItem{it})
		}
		m.list.SetItems(items)
		if len(msg.result.SkippedChannels) > 0 {
			m.statusMsg = fmt.Sprintf("synced (%d channels skipped)", len(msg.result.SkippedChannels))
		} else {
			m.statusMsg = "synced"
		}
		return m, nil

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Sync):
			m.syncing = true
			m.statusMsg = "syncing…"
			return m, tea.Batch(m.spinner.Tick, runSync(m.cfg))
		case key.Matches(msg, m.keys.Play):
			return m, m.playSelected(false)
		case key.Matches(msg, m.keys.AudioOnly):
			return m, m.playSelected(true)
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) playSelected(audioOnly bool) tea.Cmd {
	sel, ok := m.list.SelectedItem().(feedItem)
	if !ok {
		return nil
	}
	return func() tea.Msg {
		err := playback.Play(m.cfg, m.store, sel.Video, sel.Channel, audioOnly)
		if err != nil {
			return syncDoneMsg{err: fmt.Errorf("play: %w", err)}
		}
		return nil
	}
}

func listHeight(totalHeight int) int {
	h := totalHeight - 2 // status bar
	if h < 0 {
		return 0
	}
	return h
}

func (m Model) View() tea.View {
	var body string
	if m.syncing {
		body = m.spinner.View() + " " + m.statusMsg
	} else {
		body = m.list.View()
	}

	status := styleStatusBar.Width(m.width).Render(
		fmt.Sprintf("↵ play  A audio-only  r sync  q quit   quota %d/%d   %s",
			m.quotaSpent, m.quotaBudget, m.statusMsg),
	)

	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, body, status))
	v.AltScreen = true
	return v
}
