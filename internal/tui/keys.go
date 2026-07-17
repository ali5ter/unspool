package tui

import "charm.land/bubbles/v2/key"

// keyMap centralises the M1 feed keybindings — a subset; Queue, playlist,
// mute, and inspect actions land with their own milestones.
type keyMap struct {
	Play      key.Binding
	AudioOnly key.Binding
	Sync      key.Binding
	Quit      key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Play:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "play")),
		AudioOnly: key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "audio-only")),
		Sync:      key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "sync")),
		Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}
