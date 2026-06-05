package tui

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/flyingnobita/llml/internal/models"
	profilepkg "github.com/flyingnobita/llml/internal/profiles"
)

type paramFocus int

const (
	paramFocusProfiles paramFocus = iota
	paramFocusMetadata
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
	paramEditMetadataValue
)

type paramMetadataField int

const (
	paramMetadataBackend paramMetadataField = iota
	paramMetadataUseCasePrimary
	paramMetadataUseCaseTags
	paramMetadataHardwareClass
	paramMetadataHardwareGPUCount
	paramMetadataHardwareMinVRAM
	paramMetadataHardwareMaxVRAM
	paramMetadataHardwareNotes
	paramMetadataFieldCount
)

var paramMetadataFieldLabels = [...]string{
	"Backend",
	"Use Case Primary",
	"Tags",
	"Hardware Class",
	"Hardware GPU Count",
	"Hardware Min VRAM GB",
	"Hardware Max VRAM GB",
	"Hardware Notes",
}

var (
	paramBackendOptionsAll  = []string{"", "llama", "vllm", "ollama", "koboldcpp"}
	paramBackendOptionsGGUF = []string{"", "llama", "koboldcpp"}
)

// paramBackendOptionsForModel returns the valid backend options for the given
// model path. GGUF rows can choose llama or koboldcpp; non-GGUF rows are
// locked to their discovery backend (the profile override is ignored at
// launch time).
func (m Model) paramBackendOptionsForModel() []string {
	for _, f := range m.table.files {
		if f.Identity() == m.params.modelPath {
			if f.Backend == models.BackendLlama {
				return paramBackendOptionsGGUF
			}
			return nil // non-GGUF: no cycling
		}
	}
	return paramBackendOptionsAll // fallback (shouldn't happen)
}

var paramHardwareClassOptions = []profilepkg.HardwareClass{
	profilepkg.HardwareClassUnspecified,
	profilepkg.HardwareClassCPU,
	profilepkg.HardwareClassGPU,
	profilepkg.HardwareClassMixed,
}

func newParamLineTextInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.CharLimit = ParamEditCharLimit
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
	return profilepkg.ProfileNameTaken(profiles, name, skip)
}

func nextProfileName(profiles []ParameterProfile) string {
	return profilepkg.NextProfileName(profiles)
}

// cloneProfileName picks a unique profile name derived from base (e.g. "foo copy", "foo copy 2").
func cloneProfileName(base string, profiles []ParameterProfile) string {
	return profilepkg.CloneProfileName(base, profiles)
}

func copyProfiles(in []ParameterProfile) []ParameterProfile {
	return profilepkg.CopyProfiles(in)
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
	m.params.modelPath = modelParamsKey(p)
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
	m.params.metadataCursor = 0
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
	if w < MinParamEditInnerWidth {
		w = MinParamEditInnerWidth
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
	m.params.metadataCursor = 0
	return m.restoreMainPaneFocusAfterModal()
}

// modelDisplayNameForPath returns the File Name column value for the selected row, or an identity fallback.
func modelDisplayNameForPath(m Model) string {
	f, ok := m.SelectedModelFile()
	if !ok {
		return ""
	}
	if n := strings.TrimSpace(f.Name); n != "" {
		return n
	}
	p := f.Identity()
	if strings.HasPrefix(p, "ollama://") || strings.Contains(p, ":") && !strings.HasPrefix(p, "/") {
		return p
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

func formatOptionalInt(v *int) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%d", *v)
}

func (m Model) paramEnvLen() int { return len(m.params.env) }
func (m Model) paramArgsLen() int {
	return len(m.params.args)
}

func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}

func (m Model) metadataFieldValue(field paramMetadataField) string {
	if m.params.profileIndex < 0 || m.params.profileIndex >= len(m.params.profiles) {
		return ""
	}
	p := m.params.profiles[m.params.profileIndex]
	switch field {
	case paramMetadataBackend:
		return p.Backend
	case paramMetadataUseCasePrimary:
		strs := make([]string, len(p.UseCase.Primary))
		for i, v := range p.UseCase.Primary {
			strs[i] = string(v)
		}
		return strings.Join(strs, ", ")
	case paramMetadataUseCaseTags:
		return strings.Join(p.UseCase.Tags, ", ")
	case paramMetadataHardwareClass:
		return string(p.Hardware.Class)
	case paramMetadataHardwareGPUCount:
		return formatOptionalInt(p.Hardware.GPUCount)
	case paramMetadataHardwareMinVRAM:
		return formatOptionalInt(p.Hardware.MinVRAMGB)
	case paramMetadataHardwareMaxVRAM:
		return formatOptionalInt(p.Hardware.MaxVRAMGB)
	case paramMetadataHardwareNotes:
		return p.Hardware.Notes
	default:
		return ""
	}
}

func (m Model) startMetadataValueEdit() (Model, tea.Cmd) {
	if m.params.focus != paramFocusMetadata || m.params.profileIndex < 0 || m.params.profileIndex >= len(m.params.profiles) {
		return m, nil
	}
	switch paramMetadataField(m.params.metadataCursor) {
	case paramMetadataBackend, paramMetadataHardwareClass:
		return m.cycleMetadataEnum(1)
	case paramMetadataUseCasePrimary:
		return m.toggleCurrentPrimary()
	case paramMetadataUseCaseTags:
		return m.toggleCurrentTag()
	default:
		m.params.editKind = paramEditMetadataValue
		m.params.editInput.SetValue(m.metadataFieldValue(paramMetadataField(m.params.metadataCursor)))
		return m.focusParamEdit()
	}
}

func cycleOption[T comparable](options []T, current T, delta int) T {
	if len(options) == 0 {
		return current
	}
	cur := -1
	for i := range options {
		if options[i] == current {
			cur = i
			break
		}
	}
	if cur < 0 {
		cur = 0
	}
	return options[(cur+delta+len(options))%len(options)]
}

// toggleTag adds tag to tags if absent, or removes it (case-insensitive) if present.
func toggleTag(tags []string, tag string) []string {
	for i, t := range tags {
		if strings.EqualFold(t, tag) {
			return slices.Delete(tags, i, i+1)
		}
	}
	return append(tags, tag)
}

func (m Model) cycleMetadataEnum(delta int) (Model, tea.Cmd) {
	if m.params.focus != paramFocusMetadata || m.params.profileIndex < 0 || m.params.profileIndex >= len(m.params.profiles) {
		return m, nil
	}
	p := m.params.profiles[m.params.profileIndex]
	switch paramMetadataField(m.params.metadataCursor) {
	case paramMetadataBackend:
		opts := m.paramBackendOptionsForModel()
		if len(opts) == 0 {
			return m, nil
		}
		p.Backend = cycleOption(opts, p.Backend, delta)
	case paramMetadataHardwareClass:
		p.Hardware.Class = cycleOption(paramHardwareClassOptions, p.Hardware.Class, delta)
	default:
		return m, nil
	}
	m.params.profiles[m.params.profileIndex] = profilepkg.NormalizeProfile(p)
	return m.persistParamPanel()
}

// toggleCurrentPrimary toggles the canonical primary value at the current primaryCursor position.
func (m Model) toggleCurrentPrimary() (Model, tea.Cmd) {
	if m.params.profileIndex < 0 || m.params.profileIndex >= len(m.params.profiles) {
		return m, nil
	}
	if m.params.primaryCursor < 0 || m.params.primaryCursor >= len(profilepkg.CanonicalPrimaries) {
		return m, nil
	}
	primStr := string(profilepkg.CanonicalPrimaries[m.params.primaryCursor])
	p := m.params.profiles[m.params.profileIndex]
	// Convert to []string for toggleTag, then back to UseCasePrimaries.
	strs := make([]string, len(p.UseCase.Primary))
	for i, v := range p.UseCase.Primary {
		strs[i] = string(v)
	}
	strs = toggleTag(strs, primStr)
	newPrimary := make(profilepkg.UseCasePrimaries, len(strs))
	for i, s := range strs {
		newPrimary[i] = profilepkg.UseCasePrimary(s)
	}
	p.UseCase.Primary = newPrimary
	m.params.profiles[m.params.profileIndex] = profilepkg.NormalizeProfile(p)
	return m.persistParamPanel()
}

// toggleCurrentTag toggles the canonical tag at the current tagCursor position.
func (m Model) toggleCurrentTag() (Model, tea.Cmd) {
	if m.params.profileIndex < 0 || m.params.profileIndex >= len(m.params.profiles) {
		return m, nil
	}
	if m.params.tagCursor < 0 || m.params.tagCursor >= len(profilepkg.CanonicalTags) {
		return m, nil
	}
	tag := profilepkg.CanonicalTags[m.params.tagCursor]
	p := m.params.profiles[m.params.profileIndex]
	p.UseCase.Tags = toggleTag(p.UseCase.Tags, tag)
	m.params.profiles[m.params.profileIndex] = profilepkg.NormalizeProfile(p)
	return m.persistParamPanel()
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
