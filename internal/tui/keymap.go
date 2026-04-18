package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// KeyMap holds key bindings. Add fields here as your TUI grows, and wire them in Update.
type KeyMap struct {
	Quit         key.Binding
	Refresh      key.Binding
	RunServer    key.Binding
	ScrollLeft   key.Binding
	ScrollRight  key.Binding
	Nav          key.Binding
	CopyPath     key.Binding
	ConfigPort   key.Binding
	Parameters   key.Binding
	ToggleTheme  key.Binding
	SortColumn   key.Binding
	SortReverse  key.Binding
	RescanModels key.Binding
	Help         key.Binding
	// LaunchPreviewScroll scroll the fixed-height launch command preview (idle main view).
	LaunchPreviewScrollUp   key.Binding
	LaunchPreviewScrollDown key.Binding
}

type runServerMode int

const (
	runServerModeNone       runServerMode = 0
	runServerModeSplit      runServerMode = 1
	runServerModeFullscreen runServerMode = 2
)

// runServerKeyMode returns [runServerModeSplit] for split-pane run (R),
// [runServerModeFullscreen] for fullscreen [tea.ExecProcess] (ctrl+r),
// or [runServerModeNone] if the key is not a run-server binding.
//
// We cannot use "shift+R" for fullscreen: on common layouts, typing uppercase R sets ModShift, so shift+R
// would be indistinguishable from plain R. Fullscreen uses ctrl+r instead.
func runServerKeyMode(msg tea.KeyPressMsg) runServerMode {
	k := msg.Key()
	switch msg.String() {
	case "ctrl+r", "ctrl+R":
		return runServerModeFullscreen
	}
	if k.Mod.Contains(tea.ModCtrl) && (k.Code == 'r' || k.Code == 'R') {
		return runServerModeFullscreen
	}
	if k.Text == "R" {
		return runServerModeSplit
	}
	return runServerModeNone
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
		RescanModels: key.NewBinding(
			key.WithKeys(FooterKeyRescan),
			key.WithHelp(FooterKeyRescan, FooterDescRescan),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
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
		SortColumn: key.NewBinding(
			key.WithKeys(FooterKeySortColumn),
			key.WithHelp(FooterKeySortColumn, FooterDescSortColumn),
		),
		SortReverse: key.NewBinding(
			key.WithKeys(FooterKeySortReverse),
			key.WithHelp(FooterKeySortReverse, FooterDescSortReverse),
		),
		LaunchPreviewScrollUp: key.NewBinding(
			key.WithKeys(FooterKeyLaunchPreviewScrollUp),
			key.WithHelp(FooterKeyLaunchPreviewScrollUp, FooterDescLaunchPreviewScroll),
		),
		LaunchPreviewScrollDown: key.NewBinding(
			key.WithKeys(FooterKeyLaunchPreviewScrollDown),
			key.WithHelp(FooterKeyLaunchPreviewScrollDown, FooterDescLaunchPreviewScroll),
		),
	}
}

// ShortHelp satisfies key.KeyMap (optional; use for help overlay later).
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Refresh, k.RescanModels, k.RunServer, k.ConfigPort, k.Parameters, k.SortColumn, k.SortReverse, k.ToggleTheme, k.Nav, k.Quit}
}

// FullHelp satisfies key.KeyMap.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Refresh, k.RescanModels, k.RunServer, k.ConfigPort, k.Parameters, k.SortColumn, k.SortReverse, k.ToggleTheme, k.Quit},
	}
}
