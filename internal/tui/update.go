package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/flyingnobita/llml/internal/models"
)

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case themeToastClearMsg:
		m.ui.themeToast = ""
		m = m.layoutTable()
		return m, nil

	case lastRunNoteClearMsg:
		m = m.withLastRunCleared()
		m = m.layoutTable()
		return m, nil

	case tea.WindowSizeMsg:
		m.layout.width = msg.Width
		m.layout.height = msg.Height
		m = m.layoutTable()
		if m.params.open {
			m.params.editInput.SetWidth(m.paramEditInnerWidth())
		}
		return m, nil

	case runtimeReadyMsg:
		m.runtime = msg.runtime
		m.runtimeScanned = true
		return m, nil

	case startupCacheHitMsg:
		return m.applyScanResult(&msg.runtime, msg.files, msg.lastScan, msg.configPaths, msg.writeErr, true)

	case startupNeedFullScanMsg:
		return m, applyAndFullScanCmd()

	case fullScanDoneMsg:
		m2, cmd := m.applyScanResult(&msg.runtime, msg.files, msg.lastScan, msg.configPaths, msg.writeErr, true)
		return applyOllamaDiscoveryResult(m2, cmd, msg.ollamaNote, msg.ollamaWarn)

	case modelRescanDoneMsg:
		m2, cmd := m.applyScanResult(nil, msg.files, msg.lastScan, msg.configPaths, msg.writeErr, false)
		return applyOllamaDiscoveryResult(m2, cmd, msg.ollamaNote, msg.ollamaWarn)

	case runtimeReloadErrMsg:
		m = m.addAlert(alertSeverityError, "Config", msg.err.Error())
		return m.flashError(msg.err.Error())

	case modelsLoadedMsg:
		m.loading = false
		m.loadErr = nil
		m.table.files = msg.files
		sortModelFiles(m.table.files, m.table.sortCol, m.table.sortDesc)
		m = m.layoutTable()
		m.table.hscroll.SetXOffset(0)
		if len(m.table.files) > 0 {
			m.table.tbl.SetCursor(0)
			m = m.withLaunchPreviewSynced()
		}
		return m.maybeSetMissingRuntimeFooterNote()

	case modelsErrMsg:
		m.loading = false
		m.loadErr = msg.err
		m = m.addAlert(alertSeverityError, "Discovery", msg.err.Error())
		return m, nil

	case runServerErrMsg:
		m = m.addAlert(alertSeverityError, "System", msg.err.Error())
		return m.flashError(msg.err.Error())

	case ollamaLaunchDoneMsg:
		if msg.err != nil {
			m = m.clearCurrentStatus()
			m = m.addAlert(alertSeverityError, "Ollama", msg.err.Error())
			return m.flashError(msg.err.Error())
		}
		m = m.clearCurrentStatus()
		if strings.TrimSpace(msg.note) == "" {
			return m, nil
		}
		m = m.addAlert(alertSeverityInfo, "Ollama", msg.note)
		return m.flashSuccess(msg.note)

	case ollamaLaunchStartedMsg:
		if strings.TrimSpace(msg.note) == "" {
			return m, nil
		}
		m = m.withLastRunCleared()
		m = m.setCurrentStatus("Ollama", msg.note)
		m = m.layoutTable()
		return m, nil

	case ollamaDiscoveryStartedMsg:
		if strings.TrimSpace(msg.note) == "" {
			return m, nil
		}
		m = m.withLastRunCleared()
		m = m.setCurrentStatus("Ollama", msg.note)
		m = m.layoutTable()
		return m, nil

	case llamaServerExitedMsg:
		if m.server.running {
			m.server.exited = true
			m.server.cmd = nil
			m.server.msgCh = nil
			if msg.err != nil {
				m = m.addAlert(alertSeverityError, "System", msg.err.Error())
				m = m.withLastRunError(msg.err.Error())
				m = m.appendServerLogLine(fmt.Sprintf("%s · %s", msg.err.Error(), splitPanePressEnterToClose))
			} else {
				m = m.addAlert(alertSeverityInfo, "System", "Server stopped")
				m = m.withLastRunCleared()
				m = m.appendServerLogLine(splitServerStoppedWithHint)
			}
			m = m.layoutTable()
			if msg.err != nil {
				return m, clearLastRunNoteAfterCmd()
			}
			return m, nil
		}
		if msg.err != nil {
			m = m.addAlert(alertSeverityError, "System", msg.err.Error())
			m = m.withLastRunError(msg.err.Error())
			return m, clearLastRunNoteAfterCmd()
		}
		m = m.withLastRunCleared()
		return m, nil

	case serverSplitReadyMsg:
		m.server.running = true
		m.server.exited = false
		m.server.splitFocused = false
		m.preview.focused = false
		m.server.cmd = msg.cmd
		m.server.msgCh = msg.ch
		m.table.tbl.Focus()
		m = m.layoutTable()
		return m, readNextServerMsg(msg.ch)

	case serverLogMsg:
		m = m.appendServerLogLine(msg.line)
		m = m.layoutTable()
		return m, readNextServerMsg(m.server.msgCh)

	case splitInterruptMsg:
		return m.stopSplitServer()

	case tea.MouseWheelMsg:
		if m.alerts.open {
			var cmd tea.Cmd
			m.alerts.viewport, cmd = m.alerts.viewport.Update(msg)
			return m, cmd
		}
		if m.server.running {
			var cmd tea.Cmd
			if m.server.splitFocused {
				m.server.viewport, cmd = m.server.viewport.Update(msg)
			} else {
				m.table.tbl, cmd = m.table.tbl.Update(msg)
			}
			return m, cmd
		}
		if m.preview.focused {
			var cmd tea.Cmd
			m.preview.viewport, cmd = m.preview.viewport.Update(msg)
			return m, cmd
		}
		var cmd tea.Cmd
		m.table.tbl, cmd = m.table.tbl.Update(msg)
		return m, cmd

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}

	var cmd tea.Cmd
	if m.params.open && m.params.editKind != paramEditNone {
		m.params.editInput, cmd = m.params.editInput.Update(msg)
		return m, cmd
	}
	if m.rc.open {
		m.rc.inputs[m.rc.focus], cmd = m.rc.inputs[m.rc.focus].Update(msg)
		return m, cmd
	}
	if m.discovery.open && m.discovery.editOpen {
		m.discovery.editInput, cmd = m.discovery.editInput.Update(msg)
		return m, cmd
	}
	m.table.tbl, cmd = m.table.tbl.Update(msg)
	return m, cmd
}

func applyOllamaDiscoveryResult(m Model, cmd tea.Cmd, note, warn string) (tea.Model, tea.Cmd) {
	m = m.clearCurrentStatus()
	switch {
	case strings.TrimSpace(warn) != "":
		m = m.addAlert(alertSeverityWarn, "Ollama", warn)
		m2, clearCmd := m.flashError(warn)
		if cmd == nil {
			return m2, clearCmd
		}
		return m2, tea.Batch(cmd, clearCmd)
	case strings.TrimSpace(note) != "":
		m = m.addAlert(alertSeverityInfo, "Ollama", note)
		m2, clearCmd := m.flashSuccess(note)
		if cmd == nil {
			return m2, clearCmd
		}
		return m2, tea.Batch(cmd, clearCmd)
	default:
		return m, cmd
	}
}

// handleKey routes key presses in the idle (no server, no modal) state.
func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.helpOpen {
		switch {
		case isEscapeKey(msg), key.Matches(msg, m.keys.Help):
			m.helpOpen = false
			return m, nil
		}
		return m, nil
	}
	if m.params.open {
		return m.updateParamPanelKey(msg)
	}
	if m.rc.open {
		return m.updateRuntimeConfigKey(msg)
	}
	if m.discovery.open {
		return m.updateDiscoveryPathsKey(msg)
	}
	if m.server.running {
		if key.Matches(msg, m.keys.Alerts) {
			m = m.toggleAlerts()
			return m, nil
		}
		if key.Matches(msg, m.keys.Help) {
			m.helpOpen = true
			return m, nil
		}
		return m.updateServerSplitKeys(msg)
	}
	if key.Matches(msg, m.keys.Quit) {
		return m, tea.Quit
	}
	if key.Matches(msg, m.keys.Help) {
		m.helpOpen = true
		return m, nil
	}
	if key.Matches(msg, m.keys.Alerts) {
		m = m.toggleAlerts()
		return m, nil
	}
	if m.preview.focused && isTabKey(msg) {
		m.preview.focused = false
		m.table.tbl.Focus()
		m = m.applyMainPaneFocusStyles()
		return m, nil
	}
	if m2, cmd, handled := m.tableNavKeys(msg); handled {
		return m2, cmd
	}
	if key.Matches(msg, m.keys.RescanModels) {
		return m.tryRescan(false)
	}
	if key.Matches(msg, m.keys.Refresh) {
		return m.tryReloadRuntime(false)
	}
	mode := runServerKeyMode(msg)
	if mode != runServerModeNone {
		if m.loading {
			return m.flashError("Wait for the model scan to finish.")
		}
		p, be := m.SelectedModel()
		if p == "" {
			return m.flashError("Select a model row first.")
		}
		m = m.withLastRunCleared()
		params, _ := loadModelParamsForRun(p)
		spec, err := buildServerSpec(be, p, params, m.runtime, true)
		if err != nil {
			return m.flashError(err.Error())
		}
		if be == models.BackendOllama {
			return m, runOllamaLaunchCmd(spec)
		}
		if mode == runServerModeFullscreen {
			return m, runForegroundServerCmd(spec)
		}
		return m, runSplitServerCmd(spec)
	}
	if m.preview.focused {
		if m.alerts.open {
			var cmd tea.Cmd
			m.alerts.viewport, cmd = m.alerts.viewport.Update(msg)
			return m, cmd
		}
		var cmd tea.Cmd
		m.preview.viewport, cmd = m.preview.viewport.Update(msg)
		return m, cmd
	}
	if m.alerts.open {
		var cmd tea.Cmd
		m.alerts.viewport, cmd = m.alerts.viewport.Update(msg)
		return m, cmd
	}
	if m.launchPreviewVisible() && isTabKey(msg) {
		m.preview.focused = true
		m.table.tbl.Blur()
		m = m.applyMainPaneFocusStyles()
		return m, nil
	}
	var cmd tea.Cmd
	m.table.tbl, cmd = m.table.tbl.Update(msg)
	m = m.withLaunchPreviewSynced()
	return m, cmd
}

// tableNavKeys handles bindings that are identical in both the idle and split-pane table focus state:
// config, params, theme, scroll, copy, sort. Returns (model, cmd, handled).
func (m Model) tableNavKeys(msg tea.KeyPressMsg) (Model, tea.Cmd, bool) {
	switch {
	case key.Matches(msg, m.keys.ConfigPort):
		m2, cmd := m.openRuntimeConfig()
		return m2, cmd, true
	case key.Matches(msg, m.keys.Parameters):
		if m.loading {
			m2, cmd := m.flashError("Wait for the model scan to finish.")
			return m2, cmd, true
		}
		m2, cmd := m.openParamPanel()
		return m2, cmd, true
	case key.Matches(msg, m.keys.ModelPaths):
		if m.loading {
			m2, cmd := m.flashError("Wait for the model scan to finish.")
			return m2, cmd, true
		}
		m2, cmd := m.openDiscoveryPathsModal()
		return m2, cmd, true
	case key.Matches(msg, m.keys.ToggleTheme):
		m2, cmd := m.cycleTheme()
		return m2, cmd, true
	case key.Matches(msg, m.keys.ScrollLeft):
		m.table.hscroll.ScrollLeft(hScrollStep)
		return m, nil, true
	case key.Matches(msg, m.keys.ScrollRight):
		m.table.hscroll.ScrollRight(hScrollStep)
		return m, nil, true
	case key.Matches(msg, m.keys.CopyPath):
		if !m.preview.focused {
			return m, nil, false
		}
		m2, cmd := copyLaunchCommandToClipboard(m)
		return m2, cmd, true
	case key.Matches(msg, m.keys.SortColumn):
		if m.loading || len(m.table.files) == 0 {
			return m, nil, true
		}
		sel := m.SelectedPath()
		m.table.sortCol = (m.table.sortCol + 1) % tableSortColCount
		return m.applyTableSort(sel), nil, true
	case key.Matches(msg, m.keys.SortReverse):
		if m.loading || len(m.table.files) == 0 {
			return m, nil, true
		}
		sel := m.SelectedPath()
		m.table.sortDesc = !m.table.sortDesc
		return m.applyTableSort(sel), nil, true
	}
	return m, nil, false
}

// updateServerSplitTableKeys handles keys when the split-pane server is running
// and focus is on the model table (navigation, refresh, horizontal scroll, etc.).
func (m Model) updateServerSplitTableKeys(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if key.Matches(msg, m.keys.RescanModels) {
		return m.tryRescan(true)
	}
	if key.Matches(msg, m.keys.Refresh) {
		return m.tryReloadRuntime(true)
	}
	if runServerKeyMode(msg) != runServerModeNone {
		if m.server.exited {
			return m.flashError("Dismiss the log (enter, esc, or q) before starting another.")
		}
		return m.flashError("Stop the server (esc or q) before starting another.")
	}
	if m2, cmd, handled := m.tableNavKeys(msg); handled {
		return m2, cmd
	}
	var cmd tea.Cmd
	m.table.tbl, cmd = m.table.tbl.Update(msg)
	m = m.withLaunchPreviewSynced()
	return m, cmd
}

// serverBlocksAction returns true when a running server should prevent rescan/reload.
// When allowWhileExited is true, an exited-but-not-dismissed server does not block.
func serverBlocksAction(m Model, allowWhileExited bool) bool {
	if !m.server.running {
		return false
	}
	if allowWhileExited {
		return !m.server.exited
	}
	return true
}

// tryRescan initiates a model re-scan if preconditions allow.
// allowWhileExited controls whether an exited split-pane server blocks the scan.
func (m Model) tryRescan(allowWhileExited bool) (Model, tea.Cmd) {
	if m.loading {
		return m, nil
	}
	if serverBlocksAction(m, allowWhileExited) {
		return m.flashError("Stop the server before re-scanning models.")
	}
	m.loading = true
	m.loadErr = nil
	m = m.withLastRunCleared()
	return m, rescanModelsCmd()
}

// tryReloadRuntime initiates a runtime reload if preconditions allow.
// allowWhileExited controls whether an exited split-pane server blocks the reload.
func (m Model) tryReloadRuntime(allowWhileExited bool) (Model, tea.Cmd) {
	if m.loading {
		return m, nil
	}
	if serverBlocksAction(m, allowWhileExited) {
		return m.flashError("Stop the server before reloading runtime.")
	}
	m = m.withLastRunCleared()
	return m, reloadRuntimeCmd()
}
