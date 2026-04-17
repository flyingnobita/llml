package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/flyingnobita/llml/internal/models"
)

func (m *Model) syncCurrentProfileOut() {
	if m.paramProfileIndex < 0 || m.paramProfileIndex >= len(m.paramProfiles) {
		return
	}
	m.paramProfiles[m.paramProfileIndex].Env = append([]EnvVar(nil), m.paramEnv...)
	m.paramProfiles[m.paramProfileIndex].Args = flattenArgLines(m.paramArgs)
}

func (m *Model) loadCurrentProfileIn() {
	if m.paramProfileIndex < 0 || m.paramProfileIndex >= len(m.paramProfiles) {
		return
	}
	p := m.paramProfiles[m.paramProfileIndex]
	m.paramEnv = append([]EnvVar(nil), p.Env...)
	m.paramArgs = collapseArgsForDisplay(p.Args)
	m.paramEnvCursor = 0
	m.paramArgsCursor = 0
}

func (m Model) commitParamLineEdit() Model {
	line := m.paramEditInput.Value()
	kind := m.paramEditKind

	switch kind {
	case paramEditEnvLine:
		if strings.TrimSpace(line) == "" {
			m = m.cancelParamLineEdit()
			if m.paramEnvCursor >= 0 && m.paramEnvCursor < m.paramEnvLen() {
				e := m.paramEnv[m.paramEnvCursor]
				if strings.TrimSpace(e.Key) == "" && strings.TrimSpace(e.Value) == "" {
					m = m.deleteParamRow()
				}
			}
			return m
		}
	case paramEditArgLine:
		if strings.TrimSpace(line) == "" {
			m = m.cancelParamLineEdit()
			if m.paramArgsCursor >= 0 && m.paramArgsCursor < m.paramArgsLen() &&
				strings.TrimSpace(m.paramArgs[m.paramArgsCursor]) == "" {
				m = m.deleteParamRow()
			}
			return m
		}
	}

	m.paramEditKind = paramEditNone
	m = m.blurParamEdit()
	switch kind {
	case paramEditProfileName:
		if m.paramProfileIndex >= 0 && m.paramProfileIndex < len(m.paramProfiles) {
			name := strings.TrimSpace(line)
			if name == "" {
				name = fmt.Sprintf("parameter profile %d", m.paramProfileIndex+1)
			}
			if profileNameTaken(m.paramProfiles, name, m.paramProfileIndex) {
				name = nextProfileName(m.paramProfiles)
			}
			m.paramProfiles[m.paramProfileIndex].Name = name
		}
	case paramEditEnvLine:
		if m.paramEnvCursor >= 0 && m.paramEnvCursor < m.paramEnvLen() {
			m.paramEnv[m.paramEnvCursor] = parseEnvLine(line)
		}
	case paramEditArgLine:
		if m.paramArgsCursor >= 0 && m.paramArgsCursor < m.paramArgsLen() {
			m.paramArgs[m.paramArgsCursor] = models.ExpandTildePath(strings.TrimSpace(line))
		}
	}
	m.paramEditInput.SetValue("")
	return m
}

func (m Model) cancelParamLineEdit() Model {
	m.paramEditKind = paramEditNone
	m = m.blurParamEdit()
	m.paramEditInput.SetValue("")
	return m
}

func (m Model) startParamLineEdit() (Model, tea.Cmd) {
	switch m.paramFocus {
	case paramFocusEnv:
		if m.paramEnvLen() == 0 {
			return m, nil
		}
		m.paramEditKind = paramEditEnvLine
		m.paramEditInput.SetValue(formatEnvVar(m.paramEnv[m.paramEnvCursor]))
	case paramFocusArgs:
		if m.paramArgsLen() == 0 {
			return m, nil
		}
		m.paramEditKind = paramEditArgLine
		m.paramEditInput.SetValue(m.paramArgs[m.paramArgsCursor])
	default:
		return m, nil
	}
	return m.focusParamEdit()
}

func (m Model) startProfileNameEdit() (Model, tea.Cmd) {
	if m.paramProfileIndex < 0 || m.paramProfileIndex >= len(m.paramProfiles) {
		return m, nil
	}
	m.paramEditKind = paramEditProfileName
	m.paramEditInput.SetValue(m.paramProfiles[m.paramProfileIndex].Name)
	return m.focusParamEdit()
}

func (m Model) addParamRow() (Model, tea.Cmd) {
	(&m).syncCurrentProfileOut()
	switch m.paramFocus {
	case paramFocusEnv:
		m.paramEnv = append(m.paramEnv, EnvVar{})
		m.paramEnvCursor = m.paramEnvLen() - 1
		m.paramEditKind = paramEditEnvLine
		m.paramEditInput.SetValue("")
	case paramFocusArgs:
		m.paramArgs = append(m.paramArgs, "")
		m.paramArgsCursor = m.paramArgsLen() - 1
		m.paramEditKind = paramEditArgLine
		m.paramEditInput.SetValue("")
	default:
		return m, nil
	}
	return m.focusParamEdit()
}

func (m Model) deleteParamRow() Model {
	(&m).syncCurrentProfileOut()
	switch m.paramFocus {
	case paramFocusEnv:
		if m.paramEnvLen() == 0 || m.paramEnvCursor < 0 || m.paramEnvCursor >= m.paramEnvLen() {
			return m
		}
		m.paramEnv = append(m.paramEnv[:m.paramEnvCursor], m.paramEnv[m.paramEnvCursor+1:]...)
		if m.paramEnvCursor >= m.paramEnvLen() && m.paramEnvLen() > 0 {
			m.paramEnvCursor = m.paramEnvLen() - 1
		}
	case paramFocusArgs:
		if m.paramArgsLen() == 0 || m.paramArgsCursor < 0 || m.paramArgsCursor >= m.paramArgsLen() {
			return m
		}
		m.paramArgs = append(m.paramArgs[:m.paramArgsCursor], m.paramArgs[m.paramArgsCursor+1:]...)
		if m.paramArgsCursor >= m.paramArgsLen() && m.paramArgsLen() > 0 {
			m.paramArgsCursor = m.paramArgsLen() - 1
		}
	default:
		return m
	}
	return m
}

func (m Model) addProfile() Model {
	(&m).syncCurrentProfileOut()
	nm := nextProfileName(m.paramProfiles)
	m.paramProfiles = append(m.paramProfiles, ParameterProfile{Name: nm, Env: nil, Args: nil})
	m.paramProfileIndex = len(m.paramProfiles) - 1
	m.loadCurrentProfileIn()
	m.paramEnvCursor = 0
	m.paramArgsCursor = 0
	return m
}

func (m Model) deleteProfile() Model {
	if len(m.paramProfiles) <= 1 {
		return m
	}
	(&m).syncCurrentProfileOut()
	m.paramProfiles = append(m.paramProfiles[:m.paramProfileIndex], m.paramProfiles[m.paramProfileIndex+1:]...)
	if m.paramProfileIndex >= len(m.paramProfiles) {
		m.paramProfileIndex = len(m.paramProfiles) - 1
	}
	m.loadCurrentProfileIn()
	m.paramEnvCursor = 0
	m.paramArgsCursor = 0
	return m
}

func (m Model) cycleParamFocus(delta int) Model {
	(&m).syncCurrentProfileOut()
	m.paramFocus = paramFocus((int(m.paramFocus) + delta + 3) % 3)
	return m
}

func (m Model) moveProfile(delta int) Model {
	(&m).syncCurrentProfileOut()
	n := len(m.paramProfiles)
	if n == 0 {
		return m
	}
	next := m.paramProfileIndex + delta
	if next < 0 || next >= n {
		return m
	}
	m.paramProfileIndex = next
	m.loadCurrentProfileIn()
	m.paramEnvCursor = 0
	m.paramArgsCursor = 0
	return m
}

// persistParamPanel writes the current parameter profiles to disk without closing the panel.
func (m Model) persistParamPanel() (Model, tea.Cmd) {
	(&m).syncCurrentProfileOut()
	ent := modelEntry{
		Profiles:    copyProfiles(m.paramProfiles),
		ActiveIndex: m.paramProfileIndex,
	}
	if err := saveModelEntry(m.paramModelPath, ent); err != nil {
		m = m.withLastRunError(err.Error())
		return m, nil
	}
	m = m.withLastRunCleared()
	return m, nil
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

// updateParamPanelKey handles keys while the parameters panel is open.
func (m Model) updateParamPanelKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if m.paramConfirmDelete != paramConfirmNone {
		switch msg.String() {
		case "y", "Y":
			k := m.paramConfirmDelete
			m.paramConfirmDelete = paramConfirmNone
			switch k {
			case paramConfirmProfile:
				m = m.deleteProfile()
			case paramConfirmEnvRow, paramConfirmArgRow:
				m = m.deleteParamRow()
			}
			return m.persistParamPanel()
		case "n", "N":
			m.paramConfirmDelete = paramConfirmNone
			return m, nil
		default:
			return m, nil
		}
	}

	if m.paramEditKind != paramEditNone {
		switch msg.String() {
		case "esc":
			m = m.cancelParamLineEdit()
			return m, nil
		case "enter":
			m = m.commitParamLineEdit()
			return m.persistParamPanel()
		case "tab":
			m = m.commitParamLineEdit()
			m = m.cycleParamFocus(1)
			return m.persistParamPanel()
		case "shift+tab":
			m = m.commitParamLineEdit()
			m = m.cycleParamFocus(-1)
			return m.persistParamPanel()
		default:
			var cmd tea.Cmd
			m.paramEditInput, cmd = m.paramEditInput.Update(msg)
			return m, cmd
		}
	}

	switch msg.String() {
	case "esc", "q":
		return m.closeParamPanelWithPersist()
	case "t":
		var cmd tea.Cmd
		m, cmd = m.cycleTheme()
		return m, cmd
	case "tab":
		m = m.cycleParamFocus(1)
		return m, nil
	case "shift+tab":
		m = m.cycleParamFocus(-1)
		return m, nil
	case "up", "k":
		switch m.paramFocus {
		case paramFocusProfiles:
			m = m.moveProfile(-1)
			return m.persistParamPanel()
		case paramFocusEnv:
			if m.paramEnvCursor > 0 {
				m.paramEnvCursor--
			}
		case paramFocusArgs:
			if m.paramArgsCursor > 0 {
				m.paramArgsCursor--
			}
		}
		return m, nil
	case "down", "j":
		switch m.paramFocus {
		case paramFocusProfiles:
			m = m.moveProfile(1)
			return m.persistParamPanel()
		case paramFocusEnv:
			if m.paramEnvCursor < m.paramEnvLen()-1 {
				m.paramEnvCursor++
			}
		case paramFocusArgs:
			if m.paramArgsCursor < m.paramArgsLen()-1 {
				m.paramArgsCursor++
			}
		}
		return m, nil
	case "n":
		if m.paramFocus == paramFocusProfiles {
			m = m.addProfile()
			return m.persistParamPanel()
		}
		return m, nil
	case "a":
		if m.paramFocus == paramFocusEnv || m.paramFocus == paramFocusArgs {
			var cmd tea.Cmd
			m, cmd = m.addParamRow()
			m, pcmd := m.persistParamPanel()
			return m, tea.Batch(cmd, pcmd)
		}
		return m, nil
	case "d":
		switch m.paramFocus {
		case paramFocusProfiles:
			if len(m.paramProfiles) <= 1 {
				return m, nil
			}
			m.paramConfirmDelete = paramConfirmProfile
			return m, nil
		case paramFocusEnv:
			if m.paramEnvLen() == 0 {
				return m, nil
			}
			m.paramConfirmDelete = paramConfirmEnvRow
			return m, nil
		case paramFocusArgs:
			if m.paramArgsLen() == 0 {
				return m, nil
			}
			m.paramConfirmDelete = paramConfirmArgRow
			return m, nil
		}
		return m, nil
	case "r", "R":
		if m.paramFocus == paramFocusProfiles {
			return m.startProfileNameEdit()
		}
		return m, nil
	case "enter":
		if m.paramFocus == paramFocusEnv || m.paramFocus == paramFocusArgs {
			return m.startParamLineEdit()
		}
		return m, nil
	default:
		return m, nil
	}
}
