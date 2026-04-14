package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// KeyMap holds key bindings. Add fields here as your TUI grows, and wire them in Update.
type KeyMap struct {
	Quit        key.Binding
	Refresh     key.Binding
	RunServer   key.Binding
	ScrollLeft  key.Binding
	ScrollRight key.Binding
	Nav         key.Binding
	CopyPath    key.Binding
	ConfigPort  key.Binding
	Parameters  key.Binding
	ToggleTheme key.Binding
}

// runServerKeyMode returns 1 for split-pane run (R), 2 for fullscreen [tea.ExecProcess] (ctrl+r),
// or 0 if the key is not a run-server binding.
//
// We cannot use "shift+R" for fullscreen: on common layouts, typing uppercase R sets ModShift, so shift+R
// would be indistinguishable from plain R. Fullscreen uses ctrl+r instead.
func runServerKeyMode(msg tea.KeyPressMsg) int {
	k := msg.Key()
	switch msg.String() {
	case "ctrl+r", "ctrl+R":
		return 2
	}
	if k.Mod.Contains(tea.ModCtrl) && (k.Code == 'r' || k.Code == 'R') {
		return 2
	}
	if k.Text == "R" {
		return 1
	}
	return 0
}

// DefaultKeyMap returns the default global shortcuts.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys(FooterKeyQuit),
			key.WithHelp(FooterKeyQuit, FooterDescQuit),
		),
		Refresh: key.NewBinding(
			key.WithKeys(FooterKeyRefresh),
			key.WithHelp(FooterKeyRefresh, FooterDescRefresh),
		),
		RunServer: key.NewBinding(
			key.WithKeys(FooterKeyRunSplit),
			key.WithHelp(FooterKeyRunSplit, FooterDescRunSplit),
		),
		ScrollLeft: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("h/l", "scroll"),
		),
		ScrollRight: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("h/l", "scroll"),
		),
		Nav: key.NewBinding(
			key.WithKeys("up", "down", "left", "right", "j", "k", "h", "l"),
			key.WithHelp(FooterKeyNav, FooterDescNav),
		),
		CopyPath: key.NewBinding(
			key.WithKeys(FooterKeyCopyPath),
			key.WithHelp(FooterKeyCopyPath, FooterDescCopyPath),
		),
		ConfigPort: key.NewBinding(
			key.WithKeys(FooterKeyConfigPort),
			key.WithHelp(FooterKeyConfigPort, FooterDescConfigPort),
		),
		Parameters: key.NewBinding(
			key.WithKeys(FooterKeyParameters),
			key.WithHelp(FooterKeyParameters, FooterDescParameters),
		),
		ToggleTheme: key.NewBinding(
			key.WithKeys(FooterKeyToggleTheme),
			key.WithHelp(FooterKeyToggleTheme, FooterDescToggleTheme),
		),
	}
}

// ShortHelp satisfies key.KeyMap (optional; use for help overlay later).
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Refresh, k.RunServer, k.ConfigPort, k.Parameters, k.ToggleTheme, k.Nav, k.Quit}
}

// FullHelp satisfies key.KeyMap.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Refresh, k.RunServer, k.ConfigPort, k.Parameters, k.ToggleTheme, k.Quit},
	}
}
