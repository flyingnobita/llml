package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	paramFocusProfiles = iota
	paramFocusEnv
	paramFocusArgs
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
	ti.Width = 64
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
	m.paramModelPath = filepath.Clean(p)
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
	w := m.innerWidth() - 8
	if w < 32 {
		w = 32
	}
	m.paramEditInput.Width = w
	return m, nil
}

func (m Model) closeParamPanel() Model {
	m.paramPanelOpen = false
	m.paramEditKind = paramEditNone
	m.paramEditInput.Blur()
	m.paramEditInput.SetValue("")
	m.paramEnv = nil
	m.paramArgs = nil
	m.paramProfiles = nil
	m.paramModelPath = ""
	return m
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

func (m Model) commitParamLineEdit() Model {
	line := m.paramEditInput.Value()
	kind := m.paramEditKind
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
			m.paramArgs[m.paramArgsCursor] = line
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
	m.paramFocus = (m.paramFocus + delta + 3) % 3
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

func (m Model) saveParamPanel() (Model, tea.Cmd) {
	(&m).syncCurrentProfileOut()
	ent := modelEntry{
		Profiles:    copyProfiles(m.paramProfiles),
		ActiveIndex: m.paramProfileIndex,
	}
	if err := saveModelEntry(m.paramModelPath, ent); err != nil {
		m.lastRunNote = err.Error()
		return m, nil
	}
	m.lastRunNote = ""
	m = m.closeParamPanel()
	return m, nil
}

// updateParamPanelKey handles keys while the parameters panel is open.
func (m Model) updateParamPanelKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if m.paramEditKind != paramEditNone {
		switch msg.String() {
		case "esc":
			m = m.cancelParamLineEdit()
			return m, nil
		case "enter":
			m = m.commitParamLineEdit()
			return m, nil
		case "tab":
			m = m.commitParamLineEdit()
			m = m.cycleParamFocus(1)
			return m, nil
		case "shift+tab":
			m = m.commitParamLineEdit()
			m = m.cycleParamFocus(-1)
			return m, nil
		default:
			var cmd tea.Cmd
			m.paramEditInput, cmd = m.paramEditInput.Update(msg)
			return m, cmd
		}
	}

	switch msg.String() {
	case "esc":
		m.lastRunNote = ""
		m = m.closeParamPanel()
		return m, nil
	case "s":
		return m.saveParamPanel()
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
		}
		return m, nil
	case "a":
		if m.paramFocus == paramFocusEnv || m.paramFocus == paramFocusArgs {
			return m.addParamRow()
		}
		return m, nil
	case "d":
		switch m.paramFocus {
		case paramFocusProfiles:
			m = m.deleteProfile()
		case paramFocusEnv, paramFocusArgs:
			m = m.deleteParamRow()
		}
		return m, nil
	case "enter":
		switch m.paramFocus {
		case paramFocusProfiles:
			return m.startProfileNameEdit()
		default:
			return m.startParamLineEdit()
		}
	default:
		return m, nil
	}
}

func (m Model) paramPanelView() string {
	iw := m.innerWidth()
	maxLine := iw - 8
	if maxLine < 24 {
		maxLine = 24
	}

	sectionPrefix := func(on bool) string {
		if on {
			return "› "
		}
		return "  "
	}

	title := portConfigTitleStyle.Render("Parameters — " + m.paramModelPath)
	rows := []string{title, ""}

	rows = append(rows, bodyStyle.Render(sectionPrefix(m.paramFocus == paramFocusProfiles)+"Parameter profiles (↑/↓ · n new · d delete · enter rename)"))
	rows = append(rows, "")
	for i := range m.paramProfiles {
		name := m.paramProfiles[i].Name
		if name == "" {
			name = "(unnamed)"
		}
		focused := m.paramFocus == paramFocusProfiles && i == m.paramProfileIndex
		switch {
		case focused && m.paramEditKind == paramEditProfileName:
			rows = append(rows, m.paramEditInput.View())
		default:
			prefix := "  "
			if focused {
				prefix = "› "
			}
			rows = append(rows, bodyStyle.Render(prefix+truncateParamLine(name, maxLine)))
		}
	}
	if len(m.paramProfiles) == 0 {
		rows = append(rows, bodyStyle.Render("  (none)"))
	}

	rows = append(rows, "")
	rows = append(rows, bodyStyle.Render(sectionPrefix(m.paramFocus == paramFocusEnv)+"Environment (KEY=value)"))
	rows = append(rows, "")
	if m.paramEnvLen() == 0 && !(m.paramFocus == paramFocusEnv && m.paramEditKind == paramEditEnvLine) {
		rows = append(rows, bodyStyle.Render("  (none) — a add"))
	}
	for i := range m.paramEnv {
		line := formatEnvVar(m.paramEnv[i])
		focused := m.paramFocus == paramFocusEnv && m.paramEnvCursor == i
		switch {
		case focused && m.paramEditKind == paramEditEnvLine:
			rows = append(rows, m.paramEditInput.View())
		default:
			prefix := "  "
			if focused {
				prefix = "› "
			}
			rows = append(rows, bodyStyle.Render(prefix+truncateParamLine(line, maxLine)))
		}
	}

	rows = append(rows, "")
	rows = append(rows, bodyStyle.Render(sectionPrefix(m.paramFocus == paramFocusArgs)+"Extra arguments (flag+value on one line when possible; or one token per line)"))
	rows = append(rows, "")
	if m.paramArgsLen() == 0 && !(m.paramFocus == paramFocusArgs && m.paramEditKind == paramEditArgLine) {
		rows = append(rows, bodyStyle.Render("  (none) — a add"))
	}
	for i := range m.paramArgs {
		line := m.paramArgs[i]
		focused := m.paramFocus == paramFocusArgs && m.paramArgsCursor == i
		switch {
		case focused && m.paramEditKind == paramEditArgLine:
			rows = append(rows, m.paramEditInput.View())
		default:
			prefix := "  "
			if focused {
				prefix = "› "
			}
			rows = append(rows, bodyStyle.Render(prefix+truncateParamLine(line, maxLine)))
		}
	}

	rows = append(rows, "",
		footerStyle.Render("tab sections · ↑/↓ · a add row · d delete · enter edit · n new parameter profile · s save · esc cancel · ctrl+c quit"),
	)
	block := lipgloss.JoinVertical(lipgloss.Left, rows...)
	if m.lastRunNote != "" {
		block = lipgloss.JoinVertical(lipgloss.Left, block, "", errorStyle.Render(m.lastRunNote))
	}
	framed := portConfigBoxStyle.Render(block)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, framed)
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
