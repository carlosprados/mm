package tui

import "github.com/charmbracelet/bubbles/key"

// keyMap holds the app-level bindings. Within-pane navigation (j/k, filtering,
// viewport scrolling) is handled by the bubbles components themselves.
type keyMap struct {
	Tab      key.Binding
	Enter    key.Binding
	Back     key.Binding
	Refresh  key.Binding
	Send     key.Binding
	Schedule key.Binding
	Quit     key.Binding
}

func defaultKeys() keyMap {
	return keyMap{
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch pane"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "open channel"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back to list"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Send: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "send"),
		),
		Schedule: key.NewBinding(
			key.WithKeys("ctrl+t"),
			key.WithHelp("ctrl+t", "schedule"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}
