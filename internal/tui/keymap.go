package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap holds key bindings. Add fields here as your TUI grows, and wire them in Update.
type KeyMap struct {
	Quit        key.Binding
	Refresh     key.Binding
	RunServer   key.Binding
	ScrollLeft  key.Binding
	ScrollRight key.Binding
	CopyPath    key.Binding
	ConfigPort  key.Binding
	Parameters  key.Binding
}

// DefaultKeyMap returns the default global shortcuts.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		RunServer: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "run server"),
		),
		ScrollLeft: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "scroll"),
		),
		ScrollRight: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "scroll"),
		),
		CopyPath: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "copy path"),
		),
		ConfigPort: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "runtime env"),
		),
		Parameters: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "param profiles"),
		),
	}
}

// ShortHelp satisfies key.KeyMap (optional; use for help overlay later).
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Refresh, k.RunServer, k.ConfigPort, k.Parameters, k.ScrollLeft, k.Quit}
}

// FullHelp satisfies key.KeyMap.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Refresh, k.RunServer, k.ConfigPort, k.Parameters, k.Quit},
	}
}
