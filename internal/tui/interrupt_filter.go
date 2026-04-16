package tui

import tea "charm.land/bubbletea/v2"

// splitPaneInterruptFilter turns OS SIGINT into [splitInterruptMsg] while a split-pane server is running,
// so the default handler does not exit the program before Update can stop the child process.
func splitPaneInterruptFilter(m tea.Model, msg tea.Msg) tea.Msg {
	if _, ok := msg.(tea.InterruptMsg); !ok {
		return msg
	}
	mm, ok := m.(Model)
	if !ok || !mm.serverRunning {
		return msg
	}
	return splitInterruptMsg{}
}
