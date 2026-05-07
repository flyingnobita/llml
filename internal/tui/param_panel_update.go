package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/profiles"
)

func (ps *paramsState) syncCurrentProfileOut() {
	if ps.profileIndex < 0 || ps.profileIndex >= len(ps.profiles) {
		return
	}
	ps.profiles[ps.profileIndex].Env = append([]EnvVar(nil), ps.env...)
	ps.profiles[ps.profileIndex].Args = flattenArgLines(ps.args)
	ps.profiles[ps.profileIndex] = profiles.NormalizeProfile(ps.profiles[ps.profileIndex])
}

func (ps *paramsState) loadCurrentProfileIn() {
	if ps.profileIndex < 0 || ps.profileIndex >= len(ps.profiles) {
		return
	}
	p := ps.profiles[ps.profileIndex]
	ps.env = append([]EnvVar(nil), p.Env...)
	ps.args = pairFlagValueForShellDisplay(p.Args)
	ps.envCursor = 0
	ps.argsCursor = 0
}

func (m Model) commitParamLineEdit() Model {
	line := m.params.editInput.Value()
	kind := m.params.editKind

	switch kind {
	case paramEditEnvLine:
		if strings.TrimSpace(line) == "" {
			m = m.cancelParamLineEdit()
			if m.params.envCursor >= 0 && m.params.envCursor < m.paramEnvLen() {
				e := m.params.env[m.params.envCursor]
				if strings.TrimSpace(e.Key) == "" && strings.TrimSpace(e.Value) == "" {
					m = m.deleteParamRow()
				}
			}
			return m
		}
	case paramEditArgLine:
		if strings.TrimSpace(line) == "" {
			m = m.cancelParamLineEdit()
			if m.params.argsCursor >= 0 && m.params.argsCursor < m.paramArgsLen() &&
				strings.TrimSpace(m.params.args[m.params.argsCursor]) == "" {
				m = m.deleteParamRow()
			}
			return m
		}
	}

	m.params.editKind = paramEditNone
	m = m.blurParamEdit()
	switch kind {
	case paramEditProfileName:
		if m.params.profileIndex >= 0 && m.params.profileIndex < len(m.params.profiles) {
			name := strings.TrimSpace(line)
			if name == "" {
				name = fmt.Sprintf("Parameter Profile %d", m.params.profileIndex+1)
			}
			if profileNameTaken(m.params.profiles, name, m.params.profileIndex) {
				name = nextProfileName(m.params.profiles)
			}
			m.params.profiles[m.params.profileIndex].Name = name
		}
	case paramEditEnvLine:
		if m.params.envCursor >= 0 && m.params.envCursor < m.paramEnvLen() {
			m.params.env[m.params.envCursor] = parseEnvLine(line)
		}
	case paramEditArgLine:
		if m.params.argsCursor >= 0 && m.params.argsCursor < m.paramArgsLen() {
			m.params.args[m.params.argsCursor] = models.ExpandTildePath(strings.TrimSpace(line))
		}
	case paramEditMetadataValue:
		if m.params.profileIndex >= 0 && m.params.profileIndex < len(m.params.profiles) {
			p := m.params.profiles[m.params.profileIndex]
			switch paramMetadataField(m.params.metadataCursor) {
			case paramMetadataUseCaseTags:
				p.UseCase.Tags = profiles.NormalizeTagsCSV(line)
			case paramMetadataHardwareGPUCount:
				p.Hardware.GPUCount = profiles.ParseOptionalPositiveInt(line)
			case paramMetadataHardwareMinVRAM:
				p.Hardware.MinVRAMGB = profiles.ParseOptionalPositiveInt(line)
			case paramMetadataHardwareMaxVRAM:
				p.Hardware.MaxVRAMGB = profiles.ParseOptionalPositiveInt(line)
			case paramMetadataHardwareNotes:
				p.Hardware.Notes = strings.TrimSpace(line)
			}
			m.params.profiles[m.params.profileIndex] = profiles.NormalizeProfile(p)
		}
	}
	m.params.editInput.SetValue("")
	return m
}

func (m Model) cancelParamLineEdit() Model {
	m.params.editKind = paramEditNone
	m = m.blurParamEdit()
	m.params.editInput.SetValue("")
	return m
}

func (m Model) startParamLineEdit() (Model, tea.Cmd) {
	switch m.params.focus {
	case paramFocusEnv:
		if m.paramEnvLen() == 0 {
			return m, nil
		}
		m.params.editKind = paramEditEnvLine
		m.params.editInput.SetValue(formatEnvVar(m.params.env[m.params.envCursor]))
	case paramFocusArgs:
		if m.paramArgsLen() == 0 {
			return m, nil
		}
		m.params.editKind = paramEditArgLine
		m.params.editInput.SetValue(m.params.args[m.params.argsCursor])
	default:
		return m, nil
	}
	return m.focusParamEdit()
}

func (m Model) startProfileNameEdit() (Model, tea.Cmd) {
	if m.params.profileIndex < 0 || m.params.profileIndex >= len(m.params.profiles) {
		return m, nil
	}
	m.params.editKind = paramEditProfileName
	m.params.editInput.SetValue(m.params.profiles[m.params.profileIndex].Name)
	return m.focusParamEdit()
}

func (m Model) addParamRow() (Model, tea.Cmd) {
	m.params.syncCurrentProfileOut()
	switch m.params.focus {
	case paramFocusEnv:
		m.params.env = append(m.params.env, EnvVar{})
		m.params.envCursor = m.paramEnvLen() - 1
		m.params.editKind = paramEditEnvLine
		m.params.editInput.SetValue("")
	case paramFocusArgs:
		m.params.args = append(m.params.args, "")
		m.params.argsCursor = m.paramArgsLen() - 1
		m.params.editKind = paramEditArgLine
		m.params.editInput.SetValue("")
	default:
		return m, nil
	}
	return m.focusParamEdit()
}

func (m Model) deleteParamRow() Model {
	m.params.syncCurrentProfileOut()
	switch m.params.focus {
	case paramFocusEnv:
		if m.paramEnvLen() == 0 || m.params.envCursor < 0 || m.params.envCursor >= m.paramEnvLen() {
			return m
		}
		m.params.env = append(m.params.env[:m.params.envCursor], m.params.env[m.params.envCursor+1:]...)
		if m.params.envCursor >= m.paramEnvLen() && m.paramEnvLen() > 0 {
			m.params.envCursor = m.paramEnvLen() - 1
		}
	case paramFocusArgs:
		if m.paramArgsLen() == 0 || m.params.argsCursor < 0 || m.params.argsCursor >= m.paramArgsLen() {
			return m
		}
		m.params.args = append(m.params.args[:m.params.argsCursor], m.params.args[m.params.argsCursor+1:]...)
		if m.params.argsCursor >= m.paramArgsLen() && m.paramArgsLen() > 0 {
			m.params.argsCursor = m.paramArgsLen() - 1
		}
	default:
		return m
	}
	return m
}

func (m Model) addProfile() Model {
	m.params.syncCurrentProfileOut()
	nm := nextProfileName(m.params.profiles)
	m.params.profiles = append(m.params.profiles, ParameterProfile{Name: nm, Env: nil, Args: nil})
	m.params.profileIndex = len(m.params.profiles) - 1
	m.params.metadataCursor = 0
	m.params.loadCurrentProfileIn()
	m.params.envCursor = 0
	m.params.argsCursor = 0
	return m
}

func (m Model) duplicateProfile() Model {
	m.params.syncCurrentProfileOut()
	if m.params.profileIndex < 0 || m.params.profileIndex >= len(m.params.profiles) {
		return m
	}
	p := m.params.profiles[m.params.profileIndex]
	nm := cloneProfileName(p.Name, m.params.profiles)
	clone := profiles.CopyProfile(p)
	clone.Name = nm
	i := m.params.profileIndex
	m.params.profiles = append(m.params.profiles[:i+1], append([]ParameterProfile{clone}, m.params.profiles[i+1:]...)...)
	m.params.profileIndex = i + 1
	m.params.metadataCursor = 0
	m.params.loadCurrentProfileIn()
	m.params.envCursor = 0
	m.params.argsCursor = 0
	return m
}

func (m Model) deleteProfile() Model {
	if len(m.params.profiles) <= 1 {
		return m
	}
	m.params.syncCurrentProfileOut()
	m.params.profiles = append(m.params.profiles[:m.params.profileIndex], m.params.profiles[m.params.profileIndex+1:]...)
	if m.params.profileIndex >= len(m.params.profiles) {
		m.params.profileIndex = len(m.params.profiles) - 1
	}
	m.params.metadataCursor = 0
	m.params.loadCurrentProfileIn()
	m.params.envCursor = 0
	m.params.argsCursor = 0
	return m
}

func (m Model) cycleParamFocus(delta int) Model {
	m.params.syncCurrentProfileOut()
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
	m.params.syncCurrentProfileOut()
	n := len(m.params.profiles)
	if n == 0 {
		return m
	}
	next := m.params.profileIndex + delta
	if next < 0 || next >= n {
		return m
	}
	m.params.profileIndex = next
	m.params.metadataCursor = 0
	m.params.loadCurrentProfileIn()
	m.params.envCursor = 0
	m.params.argsCursor = 0
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

func (m Model) persistParamPanel() (Model, tea.Cmd) {
	m.params.syncCurrentProfileOut()
	profiles := copyProfiles(m.params.profiles)
	if !m.paramsModelIsGGUF() {
		hadBackend := false
		for i := range profiles {
			if profiles[i].Backend != "" {
				hadBackend = true
			}
			profiles[i].Backend = ""
		}
		for i := range m.params.profiles {
			m.params.profiles[i].Backend = ""
		}
		if hadBackend {
			m = m.withLastRunError("Backend override cleared: only GGUF models support profile backend selection")
		}
	}
	ent := modelEntry{
		Profiles:    profiles,
		ActiveIndex: m.params.profileIndex,
	}
	if err := saveModelEntry(m.params.modelPath, ent); err != nil {
		m = m.withLastRunError(err.Error())
		return m, clearLastRunNoteAfterCmd()
	}
	m = m.withLastRunCleared()
	m = m.updateEffectiveBackendForPath(m.params.modelPath)
	m = m.refreshTableRows()
	m = m.withLaunchPreviewSynced()
	m, noteCmd := m.maybeSetMissingRuntimeFooterNote()
	return m, noteCmd
}

// closeParamPanelWithPersist saves first; on error the panel stays open and lastRunNote is set.
func (m Model) closeParamPanelWithPersist() (Model, tea.Cmd) {
	m, cmd := m.persistParamPanel()
	if m.lastRunNote != "" {
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
		m.params.metadataCursor = clampInt(m.params.metadataCursor+delta, 0, int(paramMetadataFieldCount)-1)
	case paramFocusEnv:
		n := m.paramEnvLen()
		if n == 0 {
			if delta > 0 && m.paramArgsLen() > 0 {
				m.params.focus = paramFocusArgs
				m.params.argsCursor = 0
			}
			break
		}
		if delta > 0 && m.params.envCursor >= n-1 && m.paramArgsLen() > 0 {
			m.params.focus = paramFocusArgs
			m.params.argsCursor = 0
			break
		}
		m.params.envCursor = clampInt(m.params.envCursor+delta, 0, n-1)
	case paramFocusArgs:
		n := m.paramArgsLen()
		if n == 0 {
			if delta < 0 && m.paramEnvLen() > 0 {
				m.params.focus = paramFocusEnv
				m.params.envCursor = m.paramEnvLen() - 1
			}
			break
		}
		if delta < 0 && m.params.argsCursor <= 0 && m.paramEnvLen() > 0 {
			m.params.focus = paramFocusEnv
			m.params.envCursor = m.paramEnvLen() - 1
			break
		}
		m.params.argsCursor = clampInt(m.params.argsCursor+delta, 0, n-1)
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
	switch msg.String() {
	case "esc":
		m = m.cancelParamLineEdit()
		return m, nil
	case "enter":
		m = m.commitParamLineEdit()
		return m.persistParamPanel()
	case "tab", "shift+tab":
		return m, nil
	default:
		var cmd tea.Cmd
		m.params.editInput, cmd = m.params.editInput.Update(msg)
		return m, cmd
	}
}

// handleNavKey handles navigation and action keys in the param panel idle state.
func (m Model) handleNavKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m.closeParamPanelWithPersist()
	case "E":
		m = m.openExportView()
		return m, nil
	case "t":
		return m.cycleTheme()
	case "tab":
		m = m.cycleParamFocus(1)
		return m, nil
	case "shift+tab":
		m = m.cycleParamFocus(-1)
		return m, nil
	case "up", "k":
		return m.moveParamCursor(-1)
	case "down", "j":
		return m.moveParamCursor(1)
	case "left", "h":
		if m.params.focus == paramFocusMetadata {
			switch paramMetadataField(m.params.metadataCursor) {
			case paramMetadataBackend, paramMetadataUseCasePrimary, paramMetadataHardwareClass:
				return m.cycleMetadataEnum(-1)
			}
		}
		return m, nil
	case "right", "l":
		if m.params.focus == paramFocusMetadata {
			switch paramMetadataField(m.params.metadataCursor) {
			case paramMetadataBackend, paramMetadataUseCasePrimary, paramMetadataHardwareClass:
				return m.cycleMetadataEnum(1)
			}
		}
		return m, nil
	case "c":
		if m.params.focus == paramFocusProfiles {
			m = m.duplicateProfile()
			return m.persistParamPanel()
		}
		return m, nil
	case "a":
		if m.params.focus == paramFocusProfiles {
			m = m.addProfile()
			return m.persistParamPanel()
		}
		if m.params.focus == paramFocusEnv || m.params.focus == paramFocusArgs {
			var cmd tea.Cmd
			m, cmd = m.addParamRow()
			m, pcmd := m.persistParamPanel()
			return m, tea.Batch(cmd, pcmd)
		}
		return m, nil
	case "d":
		switch m.params.focus {
		case paramFocusProfiles:
			if len(m.params.profiles) <= 1 {
				return m, nil
			}
			m.params.confirmDelete = paramConfirmProfile
			return m, nil
		case paramFocusEnv:
			if m.paramEnvLen() == 0 {
				return m, nil
			}
			m.params.confirmDelete = paramConfirmEnvRow
			return m, nil
		case paramFocusArgs:
			if m.paramArgsLen() == 0 {
				return m, nil
			}
			m.params.confirmDelete = paramConfirmArgRow
			return m, nil
		}
		return m, nil
	case "r", "R":
		if m.params.focus == paramFocusProfiles {
			return m.startProfileNameEdit()
		}
		return m, nil
	case "enter":
		if m.params.focus == paramFocusMetadata {
			return m.startMetadataValueEdit()
		}
		if m.params.focus == paramFocusEnv || m.params.focus == paramFocusArgs {
			return m.startParamLineEdit()
		}
		return m, nil
	default:
		return m, nil
	}
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
