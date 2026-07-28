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
	Attach   key.Binding
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
		// ctrl+o ("open a file"): unlike ctrl+u/ctrl+k it is not bound by the
		// composer textarea, so no editing shortcut is shadowed.
		Attach: key.NewBinding(
			key.WithKeys("ctrl+o"),
			key.WithHelp("ctrl+o", "attach file"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}
