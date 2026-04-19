package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/flyingnobita/llml/internal/models"
)

type paramFocus int

const (
	paramFocusProfiles paramFocus = iota
	paramFocusEnv
	paramFocusArgs
)

type paramConfirm int

// paramConfirmDelete* values for Model.params.confirmDelete (0 = none).
const (
	paramConfirmNone paramConfirm = iota
	paramConfirmProfile
	paramConfirmEnvRow
	paramConfirmArgRow
)

type paramEditKind int

const (
	paramEditNone paramEditKind = iota
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
	key := strings.TrimSpace(s[:i])
	val := strings.TrimSpace(s[i+1:])
	val = models.ExpandTildePath(val)
	return EnvVar{Key: key, Value: val}
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
		cand := "Parameter Profile"
		if n > 1 {
			cand = fmt.Sprintf("Parameter Profile %d", n)
		}
		if !profileNameTaken(profiles, cand, -1) {
			return cand
		}
	}
	return "Parameter Profile"
}

// cloneProfileName picks a unique profile name derived from base (e.g. "foo copy", "foo copy 2").
func cloneProfileName(base string, profiles []ParameterProfile) string {
	b := strings.TrimSpace(base)
	if b == "" {
		return nextProfileName(profiles)
	}
	cand := b + " copy"
	if !profileNameTaken(profiles, cand, -1) {
		return cand
	}
	for n := 2; n < 1000; n++ {
		cand = fmt.Sprintf("%s copy %d", b, n)
		if !profileNameTaken(profiles, cand, -1) {
			return cand
		}
	}
	return nextProfileName(profiles)
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
		m = m.withLastRunError("Select a model row first.")
		return m, clearLastRunNoteAfterCmd()
	}
	m.params.open = true
	m = m.saveMainPaneFocusForModal()
	m.params.confirmDelete = paramConfirmNone
	m.params.modelPath = filepath.Clean(p)
	m.params.modelDisplayName = modelDisplayNameForPath(m)
	m = m.withLastRunCleared()
	m.params.editKind = paramEditNone
	m.params.editInput.Blur()
	m.params.editInput.SetValue("")

	ent, err := loadModelEntry(m.params.modelPath)
	var cmd tea.Cmd
	if err != nil {
		m = m.withLastRunError(err.Error())
		cmd = clearLastRunNoteAfterCmd()
		ent = modelEntry{
			Profiles:    []ParameterProfile{{Name: "default", Env: nil, Args: nil}},
			ActiveIndex: 0,
		}
	}
	m.params.profiles = copyProfiles(ent.Profiles)
	m.params.profileIndex = clampInt(ent.ActiveIndex, 0, max(0, len(m.params.profiles)-1))
	m.params.focus = paramFocusProfiles
	m.params.loadCurrentProfileIn()
	m.params.editInput.SetWidth(m.paramEditInnerWidth())
	return m, cmd
}

// paramEditInnerWidth is the textinput width for profile/env/argv line edits in the params modal.
func (m Model) paramEditInnerWidth() int {
	cw := m.paramPanelContentWidth()
	frame := m.ui.styles.paramSectionBox.GetHorizontalFrameSize()
	w := cw - frame
	if w < 32 {
		w = 32
	}
	return w
}

func (m Model) closeParamPanel() Model {
	m.params.open = false
	m.params.confirmDelete = paramConfirmNone
	m.params.editKind = paramEditNone
	m.params.editInput.Blur()
	m.params.editInput.SetValue("")
	m.params.env = nil
	m.params.args = nil
	m.params.profiles = nil
	m.params.modelPath = ""
	m.params.modelDisplayName = ""
	return m.restoreMainPaneFocusAfterModal()
}

// modelDisplayNameForPath returns the File Name column value for the row whose path is selected, or a basename fallback.
func modelDisplayNameForPath(m Model) string {
	p := m.SelectedPath()
	if p == "" {
		return ""
	}
	p = filepath.Clean(p)
	for i := range m.table.files {
		if filepath.Clean(m.table.files[i].Path) == p {
			if n := strings.TrimSpace(m.table.files[i].Name); n != "" {
				return n
			}
			break
		}
	}
	return filepath.Base(p)
}

func (m Model) focusParamEdit() (Model, tea.Cmd) {
	return m, m.params.editInput.Focus()
}

func (m Model) blurParamEdit() Model {
	m.params.editInput.Blur()
	return m
}

func (m Model) paramEnvLen() int { return len(m.params.env) }
func (m Model) paramArgsLen() int {
	return len(m.params.args)
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
