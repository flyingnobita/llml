package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"

	"github.com/flyingnobita/llml/internal/llamacpp"
)

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case themeToastClearMsg:
		m.themeToast = ""
		m = m.layoutTable()
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.layoutTable()
		if m.paramPanelOpen {
			m.paramEditInput.SetWidth(m.paramEditInnerWidth())
		}
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
		if len(m.files) > 0 {
			m.tbl.SetCursor(0)
		}
		return m, nil

	case modelsErrMsg:
		m.loading = false
		m.loadErr = msg.err
		return m, nil

	case runServerErrMsg:
		m.lastRunNote = msg.err.Error()
		return m, nil

	case llamaServerExitedMsg:
		if m.serverRunning {
			m.serverRunning = false
			m.splitLogFocused = false
			m.serverCmd = nil
			m.serverMsgCh = nil
			m.serverLog = nil
			m.serverViewport.SetContent("")
			if msg.err != nil {
				m.lastRunNote = msg.err.Error()
			} else {
				m.lastRunNote = ""
			}
			m.tbl.Focus()
			m = m.layoutTable()
			return m, nil
		}
		if msg.err != nil {
			m.lastRunNote = msg.err.Error()
		} else {
			m.lastRunNote = ""
		}
		return m, nil

	case serverSplitReadyMsg:
		m.serverRunning = true
		m.splitLogFocused = false
		m.serverCmd = msg.cmd
		m.serverMsgCh = msg.ch
		m.tbl.Focus()
		m = m.layoutTable()
		return m, readNextServerMsg(msg.ch)

	case serverLogMsg:
		m = m.appendServerLogLine(msg.line)
		m = m.layoutTable()
		return m, readNextServerMsg(m.serverMsgCh)

	case splitInterruptMsg:
		return m.stopSplitServer()

	case tea.MouseWheelMsg:
		if m.serverRunning {
			var cmd tea.Cmd
			if m.splitLogFocused {
				m.serverViewport, cmd = m.serverViewport.Update(msg)
			} else {
				m.tbl, cmd = m.tbl.Update(msg)
			}
			return m, cmd
		}
		var cmd tea.Cmd
		m.tbl, cmd = m.tbl.Update(msg)
		return m, cmd

	case tea.KeyPressMsg:
		if m.paramPanelOpen {
			return m.updateParamPanelKey(msg)
		}
		if m.runtimeConfigOpen {
			return m.updateRuntimeConfigKey(msg)
		}
		if m.serverRunning {
			return m.updateServerSplitKeys(msg)
		}
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}
		if key.Matches(msg, m.keys.ConfigPort) {
			return m.openRuntimeConfig()
		}
		if key.Matches(msg, m.keys.Parameters) {
			if m.loading {
				m.lastRunNote = "Wait for the model scan to finish."
				return m, nil
			}
			return m.openParamPanel()
		}
		if key.Matches(msg, m.keys.ToggleTheme) {
			var cmd tea.Cmd
			m, cmd = m.cycleTheme()
			return m, cmd
		}
		if key.Matches(msg, m.keys.Refresh) {
			m.loading = true
			m.loadErr = nil
			m.lastRunNote = ""
			m.runtimeScanned = false
			return m, startupCmd()
		}
		mode := runServerKeyMode(msg)
		if mode != 0 {
			if m.loading {
				m.lastRunNote = "Wait for the model scan to finish."
				return m, nil
			}
			p, be := m.SelectedModel()
			if p == "" {
				m.lastRunNote = "Select a model row first."
				return m, nil
			}
			m.lastRunNote = ""
			params, _ := loadModelParamsForRun(p)
			if mode == 2 {
				if be == llamacpp.BackendVLLM {
					return m, runVLLMServerCmd(p, m.runtime, params)
				}
				return m, runLlamaServerCmd(p, m.runtime, params)
			}
			if be == llamacpp.BackendVLLM {
				return m, runVLLMServerSplitCmd(p, m.runtime, params)
			}
			return m, runLlamaServerSplitCmd(p, m.runtime, params)
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
	}

	var cmd tea.Cmd
	if m.paramPanelOpen && m.paramEditKind != paramEditNone {
		m.paramEditInput, cmd = m.paramEditInput.Update(msg)
		return m, cmd
	}
	if m.runtimeConfigOpen {
		m.runtimeInputs[m.runtimeFocus], cmd = m.runtimeInputs[m.runtimeFocus].Update(msg)
		return m, cmd
	}
	m.tbl, cmd = m.tbl.Update(msg)
	return m, cmd
}

// updateServerSplitTableKeys handles keys when the split-pane server is running
// and focus is on the model table (navigation, refresh, horizontal scroll, etc.).
func (m Model) updateServerSplitTableKeys(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Refresh) {
		m.loading = true
		m.loadErr = nil
		m.lastRunNote = ""
		m.runtimeScanned = false
		return m, startupCmd()
	}
	if key.Matches(msg, m.keys.ConfigPort) {
		return m.openRuntimeConfig()
	}
	if key.Matches(msg, m.keys.Parameters) {
		if m.loading {
			m.lastRunNote = "Wait for the model scan to finish."
			return m, nil
		}
		return m.openParamPanel()
	}
	if key.Matches(msg, m.keys.ToggleTheme) {
		var cmd tea.Cmd
		m, cmd = m.cycleTheme()
		return m, cmd
	}
	if runServerKeyMode(msg) != 0 {
		m.lastRunNote = "Stop the server (esc or q) before starting another."
		return m, nil
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
}
