package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	paramFocusProfiles = iota
	paramFocusEnv
	paramFocusArgs
)

// paramConfirmDelete* values for Model.paramConfirmDelete (0 = none).
const (
	paramConfirmNone = iota
	paramConfirmProfile
	paramConfirmEnvRow
	paramConfirmArgRow
)

const (
	paramEditNone = iota
	paramEditEnvLine
	paramEditArgLine
	paramEditProfileName
)

func newParamLineTextInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.CharLimit = 4096
	ti.SetWidth(64)
	ti.Blur()
	return ti
}

func parseEnvLine(s string) EnvVar {
	s = strings.TrimSpace(s)
	if s == "" {
		return EnvVar{}
	}
	i := strings.IndexByte(s, '=')
	if i < 0 {
		return EnvVar{Key: s, Value: ""}
	}
	return EnvVar{Key: strings.TrimSpace(s[:i]), Value: s[i+1:]}
}

func formatEnvVar(e EnvVar) string {
	if e.Key == "" {
		return ""
	}
	return e.Key + "=" + e.Value
}

func profileNameTaken(profiles []ParameterProfile, name string, skip int) bool {
	n := strings.TrimSpace(name)
	for i, p := range profiles {
		if i == skip {
			continue
		}
		if strings.TrimSpace(p.Name) == n {
			return true
		}
	}
	return false
}

func nextProfileName(profiles []ParameterProfile) string {
	for n := 1; n < 1000; n++ {
		cand := "Parameter profile"
		if n > 1 {
			cand = fmt.Sprintf("Parameter profile %d", n)
		}
		if !profileNameTaken(profiles, cand, -1) {
			return cand
		}
	}
	return "Parameter profile"
}

func copyProfiles(in []ParameterProfile) []ParameterProfile {
	out := make([]ParameterProfile, len(in))
	for i := range in {
		out[i].Name = in[i].Name
		out[i].Env = append([]EnvVar(nil), in[i].Env...)
		out[i].Args = append([]string(nil), in[i].Args...)
	}
	return out
}

func (m Model) openParamPanel() (Model, tea.Cmd) {
	p := m.SelectedPath()
	if p == "" {
		m.lastRunNote = "Select a model row first."
		return m, nil
	}
	m.paramPanelOpen = true
	m.paramConfirmDelete = paramConfirmNone
	m.paramModelPath = filepath.Clean(p)
	m.paramModelDisplayName = modelDisplayNameForPath(m)
	m.lastRunNote = ""
	m.paramEditKind = paramEditNone
	m.paramEditInput.Blur()
	m.paramEditInput.SetValue("")

	ent, err := loadModelEntry(m.paramModelPath)
	if err != nil {
		m.lastRunNote = err.Error()
		ent = modelEntry{
			Profiles:    []ParameterProfile{{Name: "default", Env: nil, Args: nil}},
			ActiveIndex: 0,
		}
	}
	m.paramProfiles = copyProfiles(ent.Profiles)
	m.paramProfileIndex = clampInt(ent.ActiveIndex, 0, max(0, len(m.paramProfiles)-1))
	m.paramFocus = paramFocusProfiles
	m.loadCurrentProfileIn()
	m.paramEditInput.SetWidth(m.paramEditInnerWidth())
	return m, nil
}

// paramEditInnerWidth is the textinput width for profile/env/argv line edits in the params modal.
func (m Model) paramEditInnerWidth() int {
	cw := m.paramPanelContentWidth()
	frame := m.styles.paramSectionBox.GetHorizontalFrameSize()
	w := cw - frame
	if w < 32 {
		w = 32
	}
	return w
}

func (m Model) closeParamPanel() Model {
	m.paramPanelOpen = false
	m.paramConfirmDelete = paramConfirmNone
	m.paramEditKind = paramEditNone
	m.paramEditInput.Blur()
	m.paramEditInput.SetValue("")
	m.paramEnv = nil
	m.paramArgs = nil
	m.paramProfiles = nil
	m.paramModelPath = ""
	m.paramModelDisplayName = ""
	return m
}

// modelDisplayNameForPath returns the table display name for the row whose path is selected, or a basename fallback.
func modelDisplayNameForPath(m Model) string {
	p := m.SelectedPath()
	if p == "" {
		return ""
	}
	p = filepath.Clean(p)
	for i := range m.files {
		if filepath.Clean(m.files[i].Path) == p {
			if n := strings.TrimSpace(m.files[i].Name); n != "" {
				return n
			}
			break
		}
	}
	return filepath.Base(p)
}

func (m Model) focusParamEdit() (Model, tea.Cmd) {
	return m, m.paramEditInput.Focus()
}

func (m Model) blurParamEdit() Model {
	m.paramEditInput.Blur()
	return m
}

func (m Model) paramEnvLen() int { return len(m.paramEnv) }
func (m Model) paramArgsLen() int {
	return len(m.paramArgs)
}

func truncateParamLine(s string, maxW int) string {
	if maxW < 8 {
		return s
	}
	if lipgloss.Width(s) <= maxW {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r)) > maxW {
		r = r[:len(r)-1]
	}
	return string(r)
}
