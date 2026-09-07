package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/flyingnobita/llml/internal/profiles"

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
		if m.importView.open {
			m.importView.picker, _ = m.importView.picker.Update(msg)
		}
		return m, nil

	case runtimeReadyMsg, startupCacheHitMsg, startupNeedFullScanMsg, scanDoneMsg,
		runtimeReloadErrMsg, modelsLoadedMsg, modelsErrMsg:
		return m.handleScanMsg(msg)

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

	case llamaServerExitedMsg, serverSplitReadyMsg, serverLogMsg, splitInterruptMsg:
		return m.handleServerLifecycleMsg(msg)

	case tea.MouseWheelMsg:
		return m.handleMouseWheel(msg)

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}

	return m.routeTextInputMsg(msg)
}

// handleScanMsg handles the messages produced by runtime probing and model
// discovery.
func (m Model) handleScanMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case runtimeReadyMsg:
		m.settings = msg.settings
		m.runtime = msg.runtime
		m.runtimeScanned = true
		return m, nil

	case startupCacheHitMsg:
		m = m.cancelInFlightScan()
		m.settings = msg.settings
		return m.applyScanResult(&msg.runtime, msg.files, msg.lastScan, msg.configPaths, msg.writeErr, true)

	case startupNeedFullScanMsg:
		return m.startScan(scanModeFull)

	case scanDoneMsg:
		m = m.cancelInFlightScan()
		res := msg.result
		m.settings = res.settings
		// A full pass also adopts the refreshed runtime; a models-only rescan
		// leaves whatever the last runtime probe found in place.
		var runtime *models.RuntimeInfo
		if msg.mode == scanModeFull {
			runtime = &res.runtime
		}
		m2, cmd := m.applyScanResult(runtime, res.files, res.lastScan, res.configPaths, res.writeErr, msg.mode == scanModeFull)
		return applyOllamaDiscoveryResult(m2, cmd, res.ollamaNote, res.ollamaWarn)

	case runtimeReloadErrMsg:
		m = m.addAlert(alertSeverityError, "Config", msg.err.Error())
		return m.flashError(msg.err.Error())

	case modelsLoadedMsg:
		return m.applyLoadedModels(msg.files)

	case modelsErrMsg:
		m = m.cancelInFlightScan()
		m.loading = false
		m.loadErr = msg.err
		m = m.addAlert(alertSeverityError, "Discovery", msg.err.Error())
		return m, nil
	}
	return m, nil
}

// applyLoadedModels installs a fresh model list and re-lays out around it.
func (m Model) applyLoadedModels(files []models.ModelFile) (tea.Model, tea.Cmd) {
	m.loading = false
	m.loadErr = nil
	m.table.files = files
	m = m.populateEffectiveBackends()
	sortModelFiles(m.table.files, m.table.sortCol, m.table.sortDesc)
	m = m.layoutTable()
	m.table.hscroll.SetXOffset(0)
	if len(m.table.files) > 0 {
		m.table.tbl.SetCursor(0)
		m = m.withLaunchPreviewSynced()
	}
	return m.maybeSetMissingRuntimeFooterNote()
}

// handleServerLifecycleMsg handles the split-pane server's start, output, and exit.
func (m Model) handleServerLifecycleMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case llamaServerExitedMsg:
		return m.handleServerExited(msg)

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
	}
	return m, nil
}

// handleServerExited reports a server exit, into the split log when the pane is
// open and into the footer otherwise.
func (m Model) handleServerExited(msg llamaServerExitedMsg) (tea.Model, tea.Cmd) {
	if !m.server.running {
		if msg.err != nil {
			m = m.addAlert(alertSeverityError, "System", msg.err.Error())
			m = m.withLastRunError(msg.err.Error())
			return m, clearLastRunNoteAfterCmd()
		}
		return m.withLastRunCleared(), nil
	}

	m.server.exited = true
	m.server.cmd = nil
	m.server.msgCh = nil
	if msg.err != nil {
		m = m.addAlert(alertSeverityError, "System", msg.err.Error())
		m = m.withLastRunError(msg.err.Error())
		m = m.appendServerLogLine(fmt.Sprintf("%s · %s", msg.err.Error(), splitPanePressEnterToClose))
		m = m.layoutTable()
		return m, clearLastRunNoteAfterCmd()
	}
	m = m.addAlert(alertSeverityInfo, "System", "Server stopped")
	m = m.withLastRunCleared()
	m = m.appendServerLogLine(splitServerStoppedWithHint)
	return m.layoutTable(), nil
}

// handleMouseWheel sends the wheel to whichever pane currently owns scrolling.
func (m Model) handleMouseWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch {
	case m.alerts.open:
		m.alerts.viewport, cmd = m.alerts.viewport.Update(msg)
	case m.server.running && m.server.splitFocused:
		m.server.viewport, cmd = m.server.viewport.Update(msg)
	case m.preview.focused && !m.server.running:
		m.preview.viewport, cmd = m.preview.viewport.Update(msg)
	default:
		m.table.tbl, cmd = m.table.tbl.Update(msg)
	}
	return m, cmd
}

// routeTextInputMsg forwards a message the switch above did not claim to
// whichever text input currently has focus, falling back to the table.
func (m Model) routeTextInputMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	if m.export.open && m.export.focus == exportFocusFilter {
		m.export.filterInput, cmd = m.export.filterInput.Update(msg)
		m = m.rebuildExportFilter()
		return m, cmd
	}
	if m.export.open && m.export.focus == exportFocusPath {
		m.export.pathInput, cmd = m.export.pathInput.Update(msg)
		m.export.outputPath = m.export.pathInput.Value()
		return m, cmd
	}
	if m.importView.open && m.importView.focus == importFocusPicker {
		m.importView.picker, cmd = m.importView.picker.Update(msg)
		if m.importView.picker.Path != "" && !isDir(m.importView.picker.Path) {
			m.importView.filePath = m.importView.picker.Path
			m.importView.pathInput.SetValue(m.importView.picker.Path)
			m.importView.focus = importFocusPath
			m.importView.pathInput.Focus()
			m2 := m.parseImportFile()
			return m2, cmd
		}
		return m, cmd
	}
	if m.importView.open && m.importView.focus == importFocusPath {
		m.importView.pathInput, cmd = m.importView.pathInput.Update(msg)
		m.importView.filePath = m.importView.pathInput.Value()
		return m, cmd
	}
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

// routeModalKey dispatches to whichever modal currently owns the keyboard.
// The order is the stacking order: the newest overlay wins.
func (m Model) routeModalKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	switch {
	case m.helpOpen:
		if isEscapeKey(msg) || key.Matches(msg, m.keys.Help) {
			m.helpOpen = false
		}
		// The help overlay swallows every other key.
		return m, nil, true
	case m.importView.open:
		m2, cmd := m.updateImportKey(msg)
		return m2, cmd, true
	case m.quit.open:
		m2, cmd := m.updateQuitConfirmKey(msg)
		return m2, cmd, true
	case m.collision.open:
		m2, cmd := m.updateCollisionKey(msg)
		return m2, cmd, true
	case m.export.open:
		m2, cmd := m.updateExportKey(msg)
		return m2, cmd, true
	case m.params.open:
		m2, cmd := m.updateParamPanelKey(msg)
		return m2, cmd, true
	case m.rc.open:
		m2, cmd := m.updateRuntimeConfigKey(msg)
		return m2, cmd, true
	case m.discovery.open:
		m2, cmd := m.updateDiscoveryPathsKey(msg)
		return m2, cmd, true
	}
	return m, nil, false
}

// handleRunServerKey launches the selected model, in the split pane or
// fullscreen according to mode.
func (m Model) handleRunServerKey(mode runServerMode) (tea.Model, tea.Cmd) {
	if m.loading {
		return m.flashError("Wait for the model scan to finish.")
	}
	p, _ := m.SelectedModel()
	if p == "" {
		return m.flashError("Select a model row first.")
	}
	m = m.withLastRunCleared()

	params, _ := profiles.LoadParamsForRun(p)
	be := m.resolveEffectiveBackend()
	spec, err := buildServerSpec(be, p, params, m.runtime, true)
	if err != nil {
		return m.flashError(err.Error())
	}
	m = m.warnAboutMMProj(spec, p)

	switch {
	case be == models.BackendOllama:
		return m, m.svc.runOllamaLaunchCmd(spec)
	case mode == runServerModeFullscreen:
		return m, runForegroundServerCmd(spec)
	default:
		return m, runSplitServerCmd(spec)
	}
}

// warnAboutMMProj records an alert when mmproj injection was requested but could
// not be resolved. The launch still proceeds, without multimodal support.
func (m Model) warnAboutMMProj(spec serverSpec, modelPath string) Model {
	if len(spec.mmprojCandidates) > 0 {
		m = m.addAlert(alertSeverityWarn, "mmproj",
			fmt.Sprintf("Multiple mmproj files found; none auto-selected. Add --mmproj <file> to the profile. Candidates: %s",
				strings.Join(spec.mmprojCandidates, ", ")))
	}
	if spec.mmprojMissing {
		m = m.addAlert(alertSeverityWarn, "mmproj",
			fmt.Sprintf("Profile has image/audio tag but no mmproj file found in %s — launching without multimodal support",
				filepath.Dir(modelPath)))
	}
	return m
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
	if m2, cmd, handled := m.routeModalKey(msg); handled {
		return m2, cmd
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
		return m.cancelInFlightScan(), tea.Quit
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
	if mode := runServerKeyMode(msg); mode != runServerModeNone {
		return m.handleRunServerKey(mode)
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
	case key.Matches(msg, m.keys.Export):
		return m.openExportView(), nil, true
	case key.Matches(msg, m.keys.Import):
		m2, cmd := m.openImportView()
		return m2, cmd, true
	case key.Matches(msg, m.keys.ModelPaths):
		if m.loading {
			m2, cmd := m.flashError("Wait for the model scan to finish.")
			return m2, cmd, true
		}
		return m.openDiscoveryPathsModal(), nil, true
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
	return m.startScan(scanModeModelsOnly)
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
	return m, m.svc.reloadRuntimeCmd()
}
