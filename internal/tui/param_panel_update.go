package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/flyingnobita/llml/internal/fsutil"
	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/profiles"
)

// isEditingNotes reports whether the Notes textarea is the active edit widget.
func (m Model) isEditingNotes() bool {
	return m.params.editKind == paramEditMetadataValue &&
		paramMetadataField(m.params.metadataCursor) == paramMetadataHardwareNotes
}

func (m Model) commitParamLineEdit() Model {
	var line string
	if m.isEditingNotes() {
		line = m.params.notesInput.Value()
	} else {
		line = m.params.editInput.Value()
	}
	kind := m.params.editKind

	switch kind {
	case paramEditEnvLine:
		if strings.TrimSpace(line) == "" {
			m = m.cancelParamLineEdit()
			cur := m.params.editor.envCursor
			if cur >= 0 && cur < m.paramEnvLen() {
				e := m.params.editor.env[cur]
				if strings.TrimSpace(e.Key) == "" && strings.TrimSpace(e.Value) == "" {
					m = m.deleteParamRow()
				}
			}
			return m
		}
	case paramEditArgLine:
		if strings.TrimSpace(line) == "" {
			m = m.cancelParamLineEdit()
			cur := m.params.editor.argsCursor
			if cur >= 0 && cur < m.paramArgsLen() &&
				strings.TrimSpace(m.params.editor.args[cur]) == "" {
				m = m.deleteParamRow()
			}
			return m
		}
	}

	m.params.editKind = paramEditNone
	m = m.blurParamEdit()
	switch kind {
	case paramEditProfileName:
		idx := m.params.editor.index
		if idx >= 0 && idx < len(m.params.editor.profiles) {
			name := strings.TrimSpace(line)
			if name == "" {
				name = fmt.Sprintf("Parameter Profile %d", idx+1)
			}
			if profiles.ProfileNameTaken(m.params.editor.profiles, name, idx) {
				name = profiles.NextProfileName(m.params.editor.profiles)
			}
			m.params.editor.SetProfileName(name)
		}
	case paramEditEnvLine:
		m.params.editor.SetEnvRow(m.params.editor.envCursor, parseEnvLine(line))
	case paramEditArgLine:
		m.params.editor.SetArgRow(m.params.editor.argsCursor, fsutil.ExpandTildePath(strings.TrimSpace(line)))
	case paramEditMetadataValue:
		m.params.editor.SetHardwareField(paramMetadataField(m.params.metadataCursor), line)
	}
	m.params.editInput.SetValue("")
	m.params.notesInput.Reset()
	m.params.notesInput.Blur()
	return m
}

func (m Model) cancelParamLineEdit() Model {
	m.params.editKind = paramEditNone
	m = m.blurParamEdit()
	m.params.editInput.SetValue("")
	m.params.notesInput.Reset()
	m.params.notesInput.Blur()
	return m
}

func (m Model) startParamLineEdit() (Model, tea.Cmd) {
	switch m.params.focus {
	case paramFocusEnv:
		if m.paramEnvLen() == 0 {
			return m, nil
		}
		m.params.editKind = paramEditEnvLine
		m.params.editInput.SetValue(formatEnvVar(m.params.editor.env[m.params.editor.envCursor]))
	case paramFocusArgs:
		if m.paramArgsLen() == 0 {
			return m, nil
		}
		m.params.editKind = paramEditArgLine
		m.params.editInput.SetValue(m.params.editor.args[m.params.editor.argsCursor])
	default:
		return m, nil
	}
	m.params.editInput.SetWidth(m.paramEditInnerWidth())
	return m.focusParamEdit()
}

func (m Model) startProfileNameEdit() (Model, tea.Cmd) {
	idx := m.params.editor.index
	if idx < 0 || idx >= len(m.params.editor.profiles) {
		return m, nil
	}
	m.params.editKind = paramEditProfileName
	m.params.editInput.SetValue(m.params.editor.profiles[idx].Name)
	m.params.editInput.SetWidth(m.paramEditInnerWidth())
	return m.focusParamEdit()
}

func (m Model) addParamRow() (Model, tea.Cmd) {
	switch m.params.focus {
	case paramFocusEnv:
		m.params.editor.AddEnvRow()
		m.params.editKind = paramEditEnvLine
		m.params.editInput.SetValue("")
	case paramFocusArgs:
		m.params.editor.AddArgRow()
		m.params.editKind = paramEditArgLine
		m.params.editInput.SetValue("")
	default:
		return m, nil
	}
	m.params.editInput.SetWidth(m.paramEditInnerWidth())
	return m.focusParamEdit()
}

func (m Model) deleteParamRow() Model {
	switch m.params.focus {
	case paramFocusEnv:
		if m.paramEnvLen() == 0 {
			return m
		}
		m.params.editor.DeleteEnvRow()
	case paramFocusArgs:
		if m.paramArgsLen() == 0 {
			return m
		}
		m.params.editor.DeleteArgRow()
	default:
		return m
	}
	return m
}

func (m Model) addProfile() Model {
	nm := profiles.NextProfileName(m.params.editor.profiles)
	m.params.editor.AddProfile(nm)
	m.params.metadataCursor = 0
	return m
}

func (m Model) duplicateProfile() Model {
	idx := m.params.editor.index
	if idx < 0 || idx >= len(m.params.editor.profiles) {
		return m
	}
	nm := profiles.CloneProfileName(m.params.editor.profiles[idx].Name, m.params.editor.profiles)
	m.params.editor.DuplicateProfile(nm)
	m.params.metadataCursor = 0
	return m
}

func (m Model) deleteProfile() Model {
	if !m.params.editor.DeleteProfile() {
		return m
	}
	m.params.metadataCursor = 0
	return m
}

func (m Model) cycleParamFocus(delta int) Model {
	if delta >= 0 {
		switch m.params.focus {
		case paramFocusProfiles:
			m.params.focus = paramFocusMetadata
		case paramFocusMetadata:
			m.params.focus = paramFocusEnv
		case paramFocusEnv, paramFocusArgs:
			m.params.focus = paramFocusProfiles
		}
		return m
	}
	switch m.params.focus {
	case paramFocusProfiles:
		m.params.focus = paramFocusEnv
	case paramFocusMetadata:
		m.params.focus = paramFocusProfiles
	case paramFocusEnv, paramFocusArgs:
		m.params.focus = paramFocusMetadata
	}
	return m
}

func (m Model) moveProfile(delta int) Model {
	if !m.params.editor.MoveActive(delta) {
		return m
	}
	m.params.metadataCursor = 0
	m.params.backendCursor = m.backendCurrentIndex()
	m.params.hardwareClassCursor = m.hardwareClassCurrentIndex()
	return m
}

// persistParamPanel writes the current parameter profiles to disk without closing the panel.
// paramsModelIsGGUF reports whether the param panel's model is a GGUF row
// (the only kind where profile backend overrides matter).
func (m Model) paramsModelIsGGUF() bool {
	for _, f := range m.table.files {
		if f.Identity() == m.params.modelPath {
			return f.Backend == models.BackendLlama
		}
	}
	return false
}

func (m Model) persistParamPanelState() (Model, tea.Cmd, bool) {
	ent := m.params.editor.Entry()
	if !m.paramsModelIsGGUF() {
		hadBackend := false
		for i := range ent.Profiles {
			if ent.Profiles[i].Backend != "" {
				hadBackend = true
			}
			ent.Profiles[i].Backend = ""
		}
		for i := range m.params.editor.profiles {
			m.params.editor.profiles[i].Backend = ""
		}
		if hadBackend {
			m = m.withLastRunError("Backend override cleared: only GGUF models support profile backend selection")
		}
	}
	if err := profiles.SaveEntry(m.params.modelPath, ent); err != nil {
		m = m.withLastRunError(err.Error())
		return m, clearLastRunNoteAfterCmd(), true
	}
	m = m.withLastRunCleared()
	m = m.updateEffectiveBackendForPath(m.params.modelPath)
	m = m.refreshTableRows()
	m = m.withLaunchPreviewSynced()
	m, noteCmd := m.maybeSetMissingRuntimeFooterNote()
	return m, noteCmd, false
}

func (m Model) persistParamPanel() (Model, tea.Cmd) {
	m, cmd, _ := m.persistParamPanelState()
	return m, cmd
}

// closeParamPanelWithPersist saves first; on error the panel stays open and lastRunNote is set.
func (m Model) closeParamPanelWithPersist() (Model, tea.Cmd) {
	m, cmd, failed := m.persistParamPanelState()
	if failed {
		return m, cmd
	}
	m = m.closeParamPanel()
	return m, cmd
}

// moveParamCursor moves the cursor by delta in the current focus section.
// Profile movement also persists; env/args movement returns no cmd.
func (m Model) moveParamCursor(delta int) (Model, tea.Cmd) {
	switch m.params.focus {
	case paramFocusProfiles:
		m = m.moveProfile(delta)
		return m.persistParamPanel()
	case paramFocusMetadata:
		next := clampIndex(m.params.metadataCursor+delta, int(paramMetadataFieldCount)-1)
		if next != m.params.metadataCursor {
			m.params.metadataCursor = next
			// Reset horizontal cursors when entering their interactive rows.
			switch paramMetadataField(next) {
			case paramMetadataUseCasePrimary:
				m.params.primaryCursor = 0
			case paramMetadataUseCaseTags:
				m.params.tagCursor = 0
			case paramMetadataBackend:
				m.params.backendCursor = m.backendCurrentIndex()
			case paramMetadataHardwareClass:
				m.params.hardwareClassCursor = m.hardwareClassCurrentIndex()
			}
		}
	case paramFocusEnv:
		n := m.paramEnvLen()
		if n == 0 {
			if delta > 0 && m.paramArgsLen() > 0 {
				m.params.focus = paramFocusArgs
				m.params.editor.argsCursor = 0
			}
			break
		}
		if delta > 0 && m.params.editor.envCursor >= n-1 && m.paramArgsLen() > 0 {
			m.params.focus = paramFocusArgs
			m.params.editor.argsCursor = 0
			break
		}
		m.params.editor.envCursor = clampIndex(m.params.editor.envCursor+delta, n-1)
	case paramFocusArgs:
		n := m.paramArgsLen()
		if n == 0 {
			if delta < 0 && m.paramEnvLen() > 0 {
				m.params.focus = paramFocusEnv
				m.params.editor.envCursor = m.paramEnvLen() - 1
			}
			break
		}
		if delta < 0 && m.params.editor.argsCursor <= 0 && m.paramEnvLen() > 0 {
			m.params.focus = paramFocusEnv
			m.params.editor.envCursor = m.paramEnvLen() - 1
			break
		}
		m.params.editor.argsCursor = clampIndex(m.params.editor.argsCursor+delta, n-1)
	}
	return m, nil
}

// handleConfirmKey handles y/n for pending delete confirmations.
func (m Model) handleConfirmKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		k := m.params.confirmDelete
		m.params.confirmDelete = paramConfirmNone
		switch k {
		case paramConfirmProfile:
			m = m.deleteProfile()
		case paramConfirmEnvRow, paramConfirmArgRow:
			m = m.deleteParamRow()
		}
		return m.persistParamPanel()
	case "n", "N":
		m.params.confirmDelete = paramConfirmNone
		return m, nil
	default:
		return m, nil
	}
}

// handleEditKey handles keys while a param line edit is active.
func (m Model) handleEditKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if isEscapeKey(msg) {
		m = m.cancelParamLineEdit()
		return m, nil
	}

	switch msg.String() {
	case "enter":
		m = m.commitParamLineEdit()
		return m.persistParamPanel()
	case "tab", "shift+tab":
		return m, nil
	default:
		var cmd tea.Cmd
		if m.isEditingNotes() {
			m.params.notesInput, cmd = m.params.notesInput.Update(msg)
		} else {
			m.params.editInput, cmd = m.params.editInput.Update(msg)
		}
		return m, cmd
	}
}

// handleNavKey handles navigation and action keys in the param panel idle state.
func (m Model) handleNavKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if isEscapeKey(msg) {
		return m.closeParamPanelWithPersist()
	}

	if isTabKey(msg) {
		m = m.cycleParamFocus(1)
		return m, nil
	}
	if isShiftTabKey(msg) {
		m = m.cycleParamFocus(-1)
		return m, nil
	}
	if isEnterKey(msg) {
		if m.params.focus == paramFocusMetadata {
			return m.startMetadataValueEdit()
		}
		if m.params.focus == paramFocusEnv || m.params.focus == paramFocusArgs {
			return m.startParamLineEdit()
		}
		return m, nil
	}

	switch msg.String() {
	case "E":
		m = m.openExportView()
		return m, nil
	case "t":
		return m.cycleTheme()
	case "up", "k":
		return m.moveParamCursor(-1)
	case "down", "j":
		return m.moveParamCursor(1)
	case "left", "h":
		return m.moveMetadataChip(-1), nil
	case "right", "l":
		return m.moveMetadataChip(1), nil
	case "space":
		return m.activateMetadataChip()
	case "c":
		if m.params.focus == paramFocusProfiles {
			m = m.duplicateProfile()
			return m.persistParamPanel()
		}
		return m, nil
	case "a":
		return m.handleParamAddKey()
	case "d":
		return m.handleParamDeleteKey()
	case "r", "R":
		if m.params.focus == paramFocusProfiles {
			return m.startProfileNameEdit()
		}
		return m, nil
	default:
		return m, nil
	}
}

// moveMetadataChip moves the horizontal cursor within the focused chip row
// (the checkbox and radio rows). It is a no-op on any other focus or field.
func (m Model) moveMetadataChip(delta int) Model {
	if m.params.focus != paramFocusMetadata {
		return m
	}
	switch paramMetadataField(m.params.metadataCursor) {
	case paramMetadataUseCasePrimary:
		m.params.primaryCursor = clampIndex(m.params.primaryCursor+delta, len(profiles.CanonicalPrimaries)-1)
	case paramMetadataUseCaseTags:
		m.params.tagCursor = clampIndex(m.params.tagCursor+delta, len(profiles.CanonicalTags)-1)
	case paramMetadataBackend:
		m.params.backendCursor = clampIndex(m.params.backendCursor+delta, len(m.paramBackendOptionsForModel())-1)
	case paramMetadataHardwareClass:
		m.params.hardwareClassCursor = clampIndex(m.params.hardwareClassCursor+delta, len(paramHardwareClassOptions)-1)
	}
	return m
}

// activateMetadataChip applies space to the focused chip row: toggling a
// checkbox, or selecting a radio option.
func (m Model) activateMetadataChip() (Model, tea.Cmd) {
	if m.params.focus != paramFocusMetadata {
		return m, nil
	}
	switch paramMetadataField(m.params.metadataCursor) {
	case paramMetadataUseCasePrimary:
		return m.toggleCurrentPrimary()
	case paramMetadataUseCaseTags:
		return m.toggleCurrentTag()
	case paramMetadataBackend:
		return m.selectBackend()
	case paramMetadataHardwareClass:
		return m.selectHardwareClass()
	}
	return m, nil
}

// handleParamAddKey adds a profile, env row, or arg row, depending on focus.
func (m Model) handleParamAddKey() (Model, tea.Cmd) {
	switch m.params.focus {
	case paramFocusProfiles:
		m = m.addProfile()
		return m.persistParamPanel()
	case paramFocusEnv, paramFocusArgs:
		m, cmd := m.addParamRow()
		m, pcmd := m.persistParamPanel()
		return m, tea.Batch(cmd, pcmd)
	}
	return m, nil
}

// handleParamDeleteKey opens the delete confirmation for the focused row, if
// there is anything that may be deleted.
func (m Model) handleParamDeleteKey() (Model, tea.Cmd) {
	switch m.params.focus {
	case paramFocusProfiles:
		// The last profile cannot be removed; a model always has one.
		if len(m.params.editor.profiles) > 1 {
			m.params.confirmDelete = paramConfirmProfile
		}
	case paramFocusEnv:
		if m.paramEnvLen() > 0 {
			m.params.confirmDelete = paramConfirmEnvRow
		}
	case paramFocusArgs:
		if m.paramArgsLen() > 0 {
			m.params.confirmDelete = paramConfirmArgRow
		}
	}
	return m, nil
}

// updateParamPanelKey handles keys while the parameters panel is open.
func (m Model) updateParamPanelKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if m.params.confirmDelete != paramConfirmNone {
		return m.handleConfirmKey(msg)
	}
	if m.params.editKind != paramEditNone {
		return m.handleEditKey(msg)
	}
	return m.handleNavKey(msg)
}
