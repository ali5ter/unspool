package tui

import "charm.land/bubbles/v2/key"

// keyMap centralises the M2 keybindings — a subset of PRD §7.4; inspect,
// filter, sort, and Recommended-tab actions land with their own milestones.
type keyMap struct {
	Play      key.Binding
	AudioOnly key.Binding
	Stop      key.Binding
	Sync      key.Binding
	Quit      key.Binding
	NextTab   key.Binding
	PrevTab   key.Binding
	AddQueue  key.Binding
	Mute      key.Binding
	Like      key.Binding
	AddToList key.Binding
	NewList   key.Binding
	Remove    key.Binding
	Back      key.Binding
	Confirm   key.Binding
	FocusPrev key.Binding
	FocusNext key.Binding
	Up        key.Binding
	Down      key.Binding
	Inspect   key.Binding
	Search    key.Binding
	SearchWeb key.Binding
	Sweep     key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Play:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "play")),
		AudioOnly: key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "audio-only")),
		Stop:      key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "stop playback")),
		Sync:      key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "sync")),
		Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		NextTab:   key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next tab")),
		PrevTab:   key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("⇧tab", "prev tab")),
		AddQueue:  key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "queue")),
		Mute:      key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "mute channel")),
		Like:      key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "like")),
		AddToList: key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "add to playlist")),
		NewList:   key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new playlist")),
		Remove:    key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "remove")),
		Back:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Confirm:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "confirm")),
		FocusPrev: key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "focus prev column")),
		FocusNext: key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "focus next column")),
		Up:        key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "up")),
		Down:      key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "down")),
		Inspect:   key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "inspect")),
		Search:    key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		SearchWeb: key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "search YouTube")),
		// Sweep is a deliberate easter egg — not listed in footerHints(), so
		// it only rewards someone who goes looking. Bound to "L" for logo.
		Sweep: key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "sweep the logo")),
	}
}
