package tui

import (
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.layoutTable()
		return m, nil

	case runtimeReadyMsg:
		m.runtime = msg.runtime
		m.runtimeScanned = true
		return m, nil

	case modelsLoadedMsg:
		m.loading = false
		m.loadErr = nil
		m.files = msg.files
		m = m.layoutTable()
		m.hscroll.SetXOffset(0)
		return m, nil

	case modelsErrMsg:
		m.loading = false
		m.loadErr = msg.err
		return m, nil

	case runServerErrMsg:
		m.lastRunNote = msg.err.Error()
		return m, nil

	case llamaServerExitedMsg:
		if msg.err != nil {
			m.lastRunNote = msg.err.Error()
		} else {
			m.lastRunNote = ""
		}
		return m, nil

	case tea.KeyMsg:
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}
		if key.Matches(msg, m.keys.Refresh) {
			m.loading = true
			m.loadErr = nil
			m.lastRunNote = ""
			m.runtimeScanned = false
			return m, startupCmd()
		}
		if key.Matches(msg, m.keys.RunServer) {
			if m.loading {
				m.lastRunNote = "Wait for the model scan to finish."
				return m, nil
			}
			p := m.SelectedPath()
			if p == "" {
				m.lastRunNote = "Select a model row first."
				return m, nil
			}
			m.lastRunNote = ""
			return m, runLlamaServerCmd(p, m.runtime)
		}
		if key.Matches(msg, m.keys.ScrollLeft) {
			m.hscroll.ScrollLeft(hScrollStep)
			return m, nil
		}
		if key.Matches(msg, m.keys.ScrollRight) {
			m.hscroll.ScrollRight(hScrollStep)
			return m, nil
		}
		if key.Matches(msg, m.keys.CopyPath) {
			if p := m.SelectedPath(); p != "" {
				_ = clipboard.WriteAll(p)
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.tbl, cmd = m.tbl.Update(msg)
		return m, cmd

	case tea.MouseMsg:
		// Vertical wheel moves the table cursor (row selection). Shift+vertical
		// wheel and horizontal wheel pan the outer viewport (same as
		// bubbles/viewport defaults).
		me := tea.MouseEvent(msg)
		if me.IsWheel() {
			switch {
			case me.Button == tea.MouseButtonWheelLeft,
				me.Button == tea.MouseButtonWheelRight,
				me.Shift && (me.Button == tea.MouseButtonWheelUp || me.Button == tea.MouseButtonWheelDown):
				var cmd tea.Cmd
				m.hscroll, cmd = m.hscroll.Update(msg)
				return m, cmd
			case me.Button == tea.MouseButtonWheelUp:
				m.tbl.MoveUp(1)
				return m, nil
			case me.Button == tea.MouseButtonWheelDown:
				m.tbl.MoveDown(1)
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.hscroll, cmd = m.hscroll.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.tbl, cmd = m.tbl.Update(msg)
	return m, cmd
}
