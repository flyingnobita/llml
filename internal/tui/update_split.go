package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

func isEscapeKey(msg tea.KeyPressMsg) bool {
	switch msg.String() {
	case "esc", "escape":
		return true
	}
	return msg.Key().Code == tea.KeyEscape
}

func isCtrlC(msg tea.KeyPressMsg) bool {
	s := strings.ToLower(strings.TrimSpace(msg.String()))
	if s == "ctrl+c" || s == "^c" {
		return true
	}
	k := msg.Key()
	return k.Mod.Contains(tea.ModCtrl) && (k.Code == 'c' || k.Text == "c" || k.Text == "C")
}

func isEnterKey(msg tea.KeyPressMsg) bool {
	if msg.String() == "enter" {
		return true
	}
	return msg.Key().Code == tea.KeyEnter
}

// cycleSplitPaneFocus shifts focus between the table, launch preview (if visible), and the server log.
func (m Model) cycleSplitPaneFocus() Model {
	if !m.server.splitFocused && !m.preview.focused {
		if m.launchPreviewVisible() {
			m.preview.focused = true
			m.table.tbl.Blur()
		} else {
			m.server.splitFocused = true
			m.table.tbl.Blur()
		}
	} else if m.preview.focused {
		m.preview.focused = false
		m.server.splitFocused = true
	} else {
		m.server.splitFocused = false
		m.table.tbl.Focus()
	}
	return m.applyMainPaneFocusStyles()
}

// updateServerSplitKeys handles input while a split-pane server is running.
// Tab switches focus between the model table, launch preview, and the log viewport.
func (m Model) updateServerSplitKeys(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if m.server.exited {
		switch {
		case isTabKey(msg):
			return m.cycleSplitPaneFocus(), nil
		case isEnterKey(msg), key.Matches(msg, m.keys.Quit), isEscapeKey(msg), isCtrlC(msg):
			m = m.dismissSplitServer()
			return m, nil
		}
		if m.preview.focused {
			if m2, cmd, handled := m.tableNavKeys(msg); handled {
				return m2, cmd
			}
			var cmd tea.Cmd
			m.preview.viewport, cmd = m.preview.viewport.Update(msg)
			return m, cmd
		}
		if m.server.splitFocused {
			var cmd tea.Cmd
			m.server.viewport, cmd = m.server.viewport.Update(msg)
			return m, cmd
		}
		return m.updateServerSplitTableKeys(msg)
	}
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m.stopSplitServer()
	case isEscapeKey(msg):
		return m.stopSplitServer()
	case isCtrlC(msg):
		return m.stopSplitServer()
	case isTabKey(msg):
		return m.cycleSplitPaneFocus(), nil
	}
	if m.preview.focused {
		if m2, cmd, handled := m.tableNavKeys(msg); handled {
			return m2, cmd
		}
		var cmd tea.Cmd
		m.preview.viewport, cmd = m.preview.viewport.Update(msg)
		return m, cmd
	}
	if m.server.splitFocused {
		var cmd tea.Cmd
		m.server.viewport, cmd = m.server.viewport.Update(msg)
		return m, cmd
	}
	return m.updateServerSplitTableKeys(msg)
}

func isTabKey(msg tea.KeyPressMsg) bool {
	if msg.String() == "tab" {
		return true
	}
	return msg.Key().Code == tea.KeyTab
}

func (m Model) stopSplitServer() (Model, tea.Cmd) {
	if m.server.exited {
		m = m.dismissSplitServer()
		return m, nil
	}
	_ = interruptServerProcess(m.server.cmd)
	return m, nil
}
