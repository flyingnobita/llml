package tui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/settings"
)

type runtimeField int

const (
	runtimeFieldLlamaCppPath runtimeField = iota
	runtimeFieldLlamaPort
	runtimeFieldLlamaHost
	runtimeFieldOllamaPath
	runtimeFieldOllamaHost
	runtimeFieldNInferPath
	runtimeFieldNInferPort
	runtimeFieldNInferHost
	runtimeFieldSplashPath
	runtimeFieldSplashPort
	runtimeFieldSplashHost
	runtimeFieldVLLMPath
	runtimeFieldVLLMVenv
	runtimeFieldVLLMPort
	runtimeFieldVLLMHost
	runtimeFieldKoboldCppPath
	runtimeFieldKoboldCppPort
	runtimeFieldOMLXPath
	runtimeFieldOMLXPort
	runtimeFieldOMLXHost
	runtimeFieldCount
)

// runtimeFieldNone is the runtime panel's focus while the list, not a field,
// has the keyboard.
const runtimeFieldNone runtimeField = -1

// stepRuntimeField returns the focus after from in direction step (+1 or -1).
// Focus cycles through the list and then the selected Runtime's fields in
// visual order, so it never reaches a field the detail pane is not showing.
func (m Model) stepRuntimeField(from runtimeField, step int) runtimeField {
	order := []runtimeField{runtimeFieldNone}
	for _, d := range runtimeFor(m.rc.selected).fields {
		order = append(order, d.field)
	}
	i := max(slices.Index(order, from), 0)
	return order[(i+step+len(order))%len(order)]
}

// moveRuntimeCursor highlights the supported Runtime step rows away from the
// current one, stopping at either end. Group labels are not rows.
func (m Model) moveRuntimeCursor(step int) Model {
	rts := m.panelRuntimes()
	i := slices.IndexFunc(rts, func(rt runtimeDef) bool { return rt.backend == m.rc.selected })
	if i < 0 {
		i = 0
	} else {
		i = min(max(i+step, 0), len(rts)-1)
	}
	if len(rts) > 0 {
		m.rc.selected = rts[i].backend
	}
	return m
}

// parsePortField reads a port field. An empty field means "use defaultPort",
// which is what makes clearing the field restore the built-in behavior.
func parsePortField(raw string, defaultPort int) (int, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return defaultPort, nil
	}
	p, err := strconv.Atoi(v)
	if err != nil || p < 1 || p > 65535 {
		return 0, fmt.Errorf("port must be 1-65535 or empty for default %d", defaultPort)
	}
	return p, nil
}

// hostField returns the trimmed host, or defaultHost when the field is empty.
func hostField(raw, defaultHost string) string {
	if v := strings.TrimSpace(raw); v != "" {
		return v
	}
	return defaultHost
}

func validatePortInput(s string) error {
	for _, r := range s {
		if r < '0' || r > '9' {
			return errors.New("digits only")
		}
	}
	if len(s) > 5 {
		return errors.New("max 5 digits")
	}
	return nil
}

// validatePortCommit checks a port field before applying (empty = models default for that env).
func validatePortCommit(raw string) error {
	v := strings.TrimSpace(raw)
	if v == "" {
		return nil
	}
	p, err := strconv.Atoi(v)
	if err != nil || p < 1 || p > 65535 {
		return errors.New("port must be 1-65535 or empty to use the default")
	}
	return nil
}

func newPortTextInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.CharLimit = 5
	ti.SetWidth(8)
	ti.Validate = validatePortInput
	ti.Blur()
	return ti
}

func newPathTextInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.CharLimit = PathInputCharLimit
	ti.SetWidth(PathTextInputWidth)
	ti.Blur()
	return ti
}

// runtimeFieldValues returns the value each input should show for s, indexed by
// [runtimeField]. Prefill and dirty-checking share it so they cannot drift.
func runtimeFieldValues(s settings.Settings) [runtimeFieldCount]string {
	var v [runtimeFieldCount]string
	for _, rt := range runtimeTable {
		for _, d := range rt.fields {
			v[d.field] = d.value(s)
		}
	}
	return v
}

// runtimePanelPrefill returns what each input starts with: the value in use,
// except for a field the environment sets, which starts from the value
// config.toml (or the default) would give. Saving the panel then never copies
// an environment variable into config.toml.
func (m Model) runtimePanelPrefill() [runtimeFieldCount]string {
	v := runtimeFieldValues(m.settings)
	var saved *[runtimeFieldCount]string
	for _, rt := range runtimeTable {
		for _, d := range rt.fields {
			if m.settings.Source(d.setting).Origin != settings.OriginEnv {
				continue
			}
			if saved == nil {
				sv := runtimeFieldValues(m.savedRuntimeSettings())
				saved = &sv
			}
			v[d.field] = saved[d.field]
		}
	}
	return v
}

// savedRuntimeSettings resolves config.toml over the defaults, without the
// environment: the values llml would use if no variable were set.
func (m Model) savedRuntimeSettings() settings.Settings {
	cfg, err := m.svc.readConfig()
	if err != nil {
		return settings.Resolve(settings.Defaults())
	}
	return settings.Resolve(cfg.Layer(), settings.Defaults())
}

// runtimeConfigDirty reports whether any input differs from what it held when
// the panel opened, or any Runtime was toggled.
func (m Model) runtimeConfigDirty() bool {
	if m.runtimeTogglesDirty() {
		return true
	}
	for i := range m.rc.prefill {
		if m.rc.inputs[i].Value() != m.rc.prefill[i] {
			return true
		}
	}
	return false
}

// updateRuntimeConfigDiscardConfirmKey handles y/n/esc in the discard-confirm overlay.
func (m Model) updateRuntimeConfigDiscardConfirmKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if isEscapeKey(msg) {
		m.rc.discardConfirm = false
		return m, nil
	}
	switch strings.ToLower(strings.TrimSpace(msg.String())) {
	case "y":
		m = m.withLastRunCleared()
		m = m.closeRuntimeConfig()
		return m, nil
	case "n":
		m.rc.discardConfirm = false
		return m, nil
	}
	return m, nil
}

// openRuntimeConfig opens the runtime panel with the list focused on the
// first supported Runtime, and clears any footer status line ([Model.lastRunNote]).
func (m Model) openRuntimeConfig() (Model, tea.Cmd) {
	m = m.saveMainPaneFocusForModal()
	m.rc.open = true
	m.rc.discardConfirm = false
	m = m.withLastRunCleared()
	m.rc.prefill = m.runtimePanelPrefill()
	m.rc.toggles = m.runtimeStates
	for i, v := range m.rc.prefill {
		m.rc.inputs[i].SetValue(v)
	}
	if rts := m.panelRuntimes(); len(rts) > 0 {
		m.rc.selected = rts[0].backend
	}
	return m.focusRuntimeField(runtimeFieldNone)
}

// missingRuntimeNotes pairs each Runtime with the footer note shown when a
// row needs it and its program cannot be found, in display order.
var missingRuntimeNotes = []struct {
	backend models.ModelBackend
	note    string
}{
	{models.BackendLlama, MissingLlamaServerFooterNote},
	{models.BackendVLLM, MissingVLLMFooterNote},
	{models.BackendOllama, MissingOllamaFooterNote},
	{models.BackendKobold, MissingKoboldCppFooterNote},
	{models.BackendNInfer, MissingNInferFooterNote},
	{models.BackendOMLX, MissingOMLXFooterNote},
	{models.BackendSplash, MissingSplashFooterNote},
}

// maybeSetMissingRuntimeFooterNote sets [Model.lastRunNote] when the scan found
// models whose Runtime's program cannot be found, and clears it otherwise. A
// GGUF row needs KoboldCpp too when its active profile launches with it. A
// Disabled Runtime is never reported: the user turned it off.
func (m Model) maybeSetMissingRuntimeFooterNote() (Model, tea.Cmd) {
	want := map[models.ModelBackend]bool{}
	for _, f := range m.table.files {
		want[f.Backend] = true
		if f.Backend == models.BackendLlama && m.table.effectiveBackends[f.Identity()] == models.BackendKobold {
			want[models.BackendKobold] = true
		}
	}
	var msgs []string
	for _, r := range missingRuntimeNotes {
		if want[r.backend] && m.runtimeEnabled(r.backend) && !m.runtimeAvailable(r.backend) {
			msgs = append(msgs, r.note)
		}
	}
	if len(msgs) > 0 {
		m = m.withLastRunError(strings.Join(msgs, "\n"))
	} else {
		m = m.withLastRunCleared()
	}
	return m, nil
}

// runtimeAvailable reports whether Runtime b can be used: its program was
// found. Ollama models launch through a running daemon, so for Ollama a
// daemon that answers is enough.
func (m Model) runtimeAvailable(b models.ModelBackend) bool {
	st := runtimeFor(b).status(m.runtime)
	if b == models.BackendOllama {
		return st.found || st.running
	}
	return st.found
}

// maybeSetMissingRuntimeFooterNoteBatch is like maybeSetMissingRuntimeFooterNote but batches with another command.
func (m Model) maybeSetMissingRuntimeFooterNoteBatch(cmd tea.Cmd) (Model, tea.Cmd) {
	m2, cmd2 := m.maybeSetMissingRuntimeFooterNote()
	return m2, tea.Batch(cmd, cmd2)
}

func (m Model) closeRuntimeConfig() Model {
	m.rc.open = false
	for i := range m.rc.inputs {
		(&m.rc.inputs[i]).Blur()
		m.rc.inputs[i].SetValue("")
	}
	return m.restoreMainPaneFocusAfterModal()
}

// focusRuntimeField gives field i the keyboard, or the list when i is
// [runtimeFieldNone].
func (m Model) focusRuntimeField(i runtimeField) (Model, tea.Cmd) {
	if i < runtimeFieldNone || i >= runtimeFieldCount {
		i = runtimeFieldNone
	}
	m.rc.focus = i
	var cmd tea.Cmd
	for j := range m.rc.inputs {
		if runtimeField(j) == i {
			cmd = (&m.rc.inputs[j]).Focus()
		} else {
			(&m.rc.inputs[j]).Blur()
		}
	}
	return m, cmd
}

// settingsFromRuntimeInputs builds the settings the panel describes. Path fields
// are normalized, an empty port or host field falls back to the built-in default,
// and everything not editable in the panel (extra model roots, HF cache) is
// carried over from the current settings unchanged.
func (m Model) settingsFromRuntimeInputs() (settings.Settings, error) {
	s := m.settings
	for _, rt := range runtimeTable {
		for _, d := range rt.fields {
			if err := d.apply(&s, m.rc.inputs[d.field].Value()); err != nil {
				return m.settings, fmt.Errorf("%s: %w", d.env(), err)
			}
		}
	}
	return s, nil
}

// resolveSavedSettings returns the settings in use once saved holds what the
// panel wrote to config.toml: the environment still wins, and every value
// reports the source it now has. Values the panel does not edit (extra model
// roots, oMLX model dirs, the HF cache) are carried over unchanged.
func (m Model) resolveSavedSettings(saved settings.Settings) settings.Settings {
	carried := settings.Layer{
		ExtraModelPaths: saved.ExtraModelPaths,
		OMLXModelDirs:   saved.OMLXModelDirs,
		HFHubCache:      &saved.HFHubCache,
		HFHome:          &saved.HFHome,
	}
	return settings.Resolve(
		settings.FromEnv(m.svc.getenv),
		m.svc.runtimeConfig(saved).Layer(),
		carried,
		settings.Defaults(),
	)
}

func (m Model) commitRuntimeConfig() (Model, tea.Cmd) {
	for _, rt := range runtimeTable {
		for _, d := range rt.fields {
			if d.port == nil {
				continue
			}
			if err := validatePortCommit(m.rc.inputs[d.field].Value()); err != nil {
				m = m.withLastRunError(fmt.Sprintf("%s: %v", d.env(), err))
				return m, clearLastRunNoteAfterCmd()
			}
		}
	}
	next, err := m.settingsFromRuntimeInputs()
	if err != nil {
		m = m.withLastRunError(err.Error())
		return m, clearLastRunNoteAfterCmd()
	}
	m.settings = m.resolveSavedSettings(next)
	m = m.saveRuntimeToggles()
	// Re-probing is synchronous here because the panel must show the result of
	// the save immediately; the timeout keeps an unreachable backend from
	// freezing the UI. Toggling a Runtime needs only this, never a rescan.
	ctx, cancel := context.WithTimeout(context.Background(), runtimeProbeTimeout)
	defer cancel()
	m.runtime = m.svc.discoverRuntime(ctx, m.settings, m.runtimeStates.Disabled())
	var cmd tea.Cmd
	// config.toml gets the panel's values, not the settings in use, so a field
	// the environment overrides still saves what the panel holds.
	if err := m.svc.writeSettings(next); err != nil {
		m = m.withLastRunError("Could not save config: " + err.Error())
		m = m.addAlert(alertSeverityWarn, "Config", "Could not save config: "+err.Error())
		cmd = clearLastRunNoteAfterCmd()
	} else {
		m = m.withLastRunCleared()
	}
	m = m.closeRuntimeConfig()
	m = m.withLaunchPreviewSynced()
	return m, cmd
}

// updateRuntimeConfigKey handles keys while the runtime panel is open. Keys
// every focus shares are handled here; the rest go to the list or the field.
func (m Model) updateRuntimeConfigKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if m.rc.discardConfirm {
		return m.updateRuntimeConfigDiscardConfirmKey(msg)
	}
	switch {
	case isEscapeKey(msg):
		if m.runtimeConfigDirty() {
			m.rc.discardConfirm = true
			return m, nil
		}
		m = m.withLastRunCleared()
		m = m.closeRuntimeConfig()
		return m, nil
	case isEnterKey(msg):
		return m.commitRuntimeConfig()
	case isShiftTabKey(msg): // before isTabKey, which also matches shift+tab's key code
		return m.focusRuntimeField(m.stepRuntimeField(m.rc.focus, -1))
	case isTabKey(msg):
		return m.focusRuntimeField(m.stepRuntimeField(m.rc.focus, 1))
	}
	if m.rc.focus == runtimeFieldNone {
		return m.updateRuntimeListKey(msg)
	}
	var cmd tea.Cmd
	m.rc.inputs[m.rc.focus], cmd = m.rc.inputs[m.rc.focus].Update(msg)
	return m, cmd
}

// updateRuntimeListKey handles keys while the runtime list has focus: move the
// highlight, toggle the highlighted Runtime, or step right into its first
// field. Only here does space toggle, so a space typed in a field never does.
func (m Model) updateRuntimeListKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "space":
		return m.toggleSelectedRuntime(), nil
	case "up", "k":
		return m.moveRuntimeCursor(-1), nil
	case "down", "j":
		return m.moveRuntimeCursor(1), nil
	case "right":
		return m.focusRuntimeField(m.stepRuntimeField(runtimeFieldNone, 1))
	}
	return m, nil
}
