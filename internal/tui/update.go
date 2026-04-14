package tui

import (
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/flyingnobita/llml/internal/llamacpp"
)

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.layoutTable()
		if m.paramPanelOpen {
			w := m.innerWidth() - 8
			if w < 32 {
				w = 32
			}
			m.paramEditInput.Width = w
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
		if m.paramPanelOpen {
			return m.updateParamPanelKey(msg)
		}
		if m.runtimeConfigOpen {
			return m.updateRuntimeConfigKey(msg)
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
			p, be := m.SelectedModel()
			if p == "" {
				m.lastRunNote = "Select a model row first."
				return m, nil
			}
			m.lastRunNote = ""
			params, _ := loadModelParamsForRun(p)
			if be == llamacpp.BackendVLLM {
				return m, runVLLMServerCmd(p, m.runtime, params)
			}
			return m, runLlamaServerCmd(p, m.runtime, params)
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
