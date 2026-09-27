package tui

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/flyingnobita/llml/internal/fsutil"
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

// runtimeFieldBackend returns the backend a field configures, so fields for a
// backend the platform cannot run are hidden and skipped by tab.
func runtimeFieldBackend(f runtimeField) models.ModelBackend {
	switch f {
	case runtimeFieldNInferPath, runtimeFieldNInferPort, runtimeFieldNInferHost:
		return models.BackendNInfer
	case runtimeFieldSplashPath, runtimeFieldSplashPort, runtimeFieldSplashHost:
		return models.BackendSplash
	case runtimeFieldOMLXPath, runtimeFieldOMLXPort, runtimeFieldOMLXHost:
		return models.BackendOMLX
	default:
		return models.BackendLlama
	}
}

// runtimeFieldVisible reports whether field f is shown on this platform.
func (m Model) runtimeFieldVisible(f runtimeField) bool {
	return m.runtime.Platform.Supports(runtimeFieldBackend(f))
}

// stepRuntimeField returns the next visible field after from in direction
// step (+1 or -1), wrapping around.
func (m Model) stepRuntimeField(from runtimeField, step int) runtimeField {
	f := from
	for range runtimeFieldCount {
		f = runtimeField((int(f) + step + int(runtimeFieldCount)) % int(runtimeFieldCount))
		if m.runtimeFieldVisible(f) {
			return f
		}
	}
	return from
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
	v[runtimeFieldLlamaCppPath] = s.LlamaCppPath
	v[runtimeFieldLlamaPort] = strconv.Itoa(s.LlamaServerPort)
	v[runtimeFieldLlamaHost] = s.LlamaServerHost
	v[runtimeFieldOllamaPath] = s.OllamaPath
	v[runtimeFieldOllamaHost] = s.OllamaHost
	v[runtimeFieldVLLMPath] = s.VLLMPath
	v[runtimeFieldVLLMVenv] = s.VLLMVenv
	v[runtimeFieldVLLMPort] = strconv.Itoa(s.VLLMServerPort)
	v[runtimeFieldVLLMHost] = s.VLLMServerHost
	v[runtimeFieldKoboldCppPath] = s.KoboldCppPath
	v[runtimeFieldKoboldCppPort] = strconv.Itoa(s.KoboldCppPort)
	v[runtimeFieldNInferPath] = s.NInferPath
	v[runtimeFieldNInferPort] = strconv.Itoa(s.NInferServerPort)
	v[runtimeFieldNInferHost] = s.NInferServerHost
	v[runtimeFieldOMLXPath] = s.OMLXPath
	v[runtimeFieldOMLXPort] = strconv.Itoa(s.OMLXPort)
	v[runtimeFieldOMLXHost] = s.OMLXHost
	v[runtimeFieldSplashPath] = s.SplashPath
	v[runtimeFieldSplashPort] = strconv.Itoa(s.SplashPort)
	v[runtimeFieldSplashHost] = s.SplashHost
	return v
}

// runtimeConfigDirty reports whether any input differs from the resolved settings.
func (m Model) runtimeConfigDirty() bool {
	want := runtimeFieldValues(m.settings)
	for i := range want {
		if m.rc.inputs[i].Value() != want[i] {
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

// openRuntimeConfig shows editors for the same env vars summarized in the runtimes footer.
func (m Model) openRuntimeConfig() (Model, tea.Cmd) {
	return m.openRuntimeConfigFocused(runtimeFieldLlamaCppPath)
}

// openRuntimeConfigFocused opens the runtime editor with the given field focused and clears any
// footer status line ([Model.lastRunNote]).
func (m Model) openRuntimeConfigFocused(focus runtimeField) (Model, tea.Cmd) {
	m = m.saveMainPaneFocusForModal()
	m.rc.open = true
	m.rc.discardConfirm = false
	m = m.withLastRunCleared()
	for i, v := range runtimeFieldValues(m.settings) {
		m.rc.inputs[i].SetValue(v)
	}
	return m.focusRuntimeField(focus)
}

// maybeSetMissingRuntimeFooterNote sets [Model.lastRunNote] when the scan found models that need a
// backend binary, but [models.ResolveLlamaServerPath] or [models.ResolveVLLMPath] is empty.
// GGUF rows require llama-server; vLLM rows require vllm. Clears the footer line when neither applies.
func (m Model) maybeSetMissingRuntimeFooterNote() (Model, tea.Cmd) {
	var wantLlama, wantVLLM, wantOllama, wantKobold, wantNInfer, wantOMLX, wantSplash bool
	for _, f := range m.table.files {
		switch f.Backend {
		case models.BackendLlama:
			wantLlama = true
			if m.table.effectiveBackends[f.Identity()] == models.BackendKobold {
				wantKobold = true
			}
		case models.BackendVLLM:
			wantVLLM = true
		case models.BackendOllama:
			wantOllama = true
		case models.BackendNInfer:
			wantNInfer = true
		case models.BackendOMLX:
			wantOMLX = true
		case models.BackendSplash:
			wantSplash = true
		}
	}
	haveLlama := models.ResolveLlamaServerPath(m.runtime) != ""
	haveVLLM := models.ResolveVLLMPath(m.runtime) != ""
	haveOllama := models.ResolveOllamaPath(m.runtime) != "" || m.runtime.OllamaRunning
	haveKobold := models.ResolveKoboldCppPath(m.runtime) != ""
	haveNInfer := models.ResolveNInferPath(m.runtime) != ""
	haveOMLX := models.ResolveOMLXPath(m.runtime) != ""
	haveSplash := models.ResolveSplashPath(m.runtime) != ""

	var msgs []string
	if wantLlama && !haveLlama {
		msgs = append(msgs, MissingLlamaServerFooterNote)
	}
	if wantVLLM && !haveVLLM {
		msgs = append(msgs, MissingVLLMFooterNote)
	}
	if wantOllama && !haveOllama {
		msgs = append(msgs, MissingOllamaFooterNote)
	}
	if wantKobold && !haveKobold {
		msgs = append(msgs, MissingKoboldCppFooterNote)
	}
	if wantNInfer && !haveNInfer {
		msgs = append(msgs, MissingNInferFooterNote)
	}
	if wantOMLX && !haveOMLX {
		msgs = append(msgs, MissingOMLXFooterNote)
	}
	if wantSplash && !haveSplash {
		msgs = append(msgs, MissingSplashFooterNote)
	}
	if len(msgs) > 0 {
		m = m.withLastRunError(strings.Join(msgs, "\n"))
	} else {
		m = m.withLastRunCleared()
	}
	return m, nil
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

func (m Model) focusRuntimeField(i runtimeField) (Model, tea.Cmd) {
	if i < 0 || i >= runtimeFieldCount {
		i = 0
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

	llamaPort, err := parsePortField(m.rc.inputs[runtimeFieldLlamaPort].Value(), settings.DefaultLlamaServerPort)
	if err != nil {
		return s, fmt.Errorf("%s: %w", settings.EnvLlamaServerPort, err)
	}
	vllmPort, err := parsePortField(m.rc.inputs[runtimeFieldVLLMPort].Value(), settings.DefaultVLLMServerPort)
	if err != nil {
		return s, fmt.Errorf("%s: %w", settings.EnvVLLMServerPort, err)
	}
	koboldPort, err := parsePortField(m.rc.inputs[runtimeFieldKoboldCppPort].Value(), settings.DefaultKoboldCppPort)
	if err != nil {
		return s, fmt.Errorf("%s: %w", settings.EnvKoboldCppPort, err)
	}
	ninferPort, err := parsePortField(m.rc.inputs[runtimeFieldNInferPort].Value(), settings.DefaultNInferServerPort)
	if err != nil {
		return s, fmt.Errorf("%s: %w", settings.EnvNInferServerPort, err)
	}
	omlxPort, err := parsePortField(m.rc.inputs[runtimeFieldOMLXPort].Value(), settings.DefaultOMLXPort)
	if err != nil {
		return s, fmt.Errorf("%s: %w", settings.EnvOMLXPort, err)
	}
	splashPort, err := parsePortField(m.rc.inputs[runtimeFieldSplashPort].Value(), settings.DefaultSplashPort)
	if err != nil {
		return s, fmt.Errorf("%s: %w", settings.EnvSplashPort, err)
	}

	s.LlamaCppPath = fsutil.NormalizePath(m.rc.inputs[runtimeFieldLlamaCppPath].Value())
	s.VLLMPath = fsutil.NormalizePath(m.rc.inputs[runtimeFieldVLLMPath].Value())
	s.VLLMVenv = fsutil.NormalizePath(m.rc.inputs[runtimeFieldVLLMVenv].Value())
	s.OllamaPath = fsutil.NormalizePath(m.rc.inputs[runtimeFieldOllamaPath].Value())
	s.KoboldCppPath = fsutil.NormalizePath(m.rc.inputs[runtimeFieldKoboldCppPath].Value())
	s.NInferPath = fsutil.NormalizePath(m.rc.inputs[runtimeFieldNInferPath].Value())
	s.OMLXPath = fsutil.NormalizePath(m.rc.inputs[runtimeFieldOMLXPath].Value())
	s.SplashPath = fsutil.NormalizePath(m.rc.inputs[runtimeFieldSplashPath].Value())

	s.LlamaServerPort = llamaPort
	s.VLLMServerPort = vllmPort
	s.KoboldCppPort = koboldPort
	s.NInferServerPort = ninferPort
	s.OMLXPort = omlxPort
	s.SplashPort = splashPort

	s.LlamaServerHost = hostField(m.rc.inputs[runtimeFieldLlamaHost].Value(), settings.DefaultLlamaServerHost)
	s.VLLMServerHost = hostField(m.rc.inputs[runtimeFieldVLLMHost].Value(), settings.DefaultVLLMServerHost)
	s.NInferServerHost = hostField(m.rc.inputs[runtimeFieldNInferHost].Value(), settings.DefaultNInferHost)
	s.OMLXHost = hostField(m.rc.inputs[runtimeFieldOMLXHost].Value(), settings.DefaultOMLXHost)
	s.SplashHost = hostField(m.rc.inputs[runtimeFieldSplashHost].Value(), settings.DefaultSplashHost)
	s.OllamaHost = hostField(
		settings.NormalizeOllamaHost(m.rc.inputs[runtimeFieldOllamaHost].Value()),
		settings.DefaultOllamaHost,
	)
	return s, nil
}

func (m Model) commitRuntimeConfig() (Model, tea.Cmd) {
	for _, f := range []runtimeField{runtimeFieldLlamaPort, runtimeFieldVLLMPort, runtimeFieldKoboldCppPort, runtimeFieldNInferPort, runtimeFieldOMLXPort, runtimeFieldSplashPort} {
		if err := validatePortCommit(m.rc.inputs[f].Value()); err != nil {
			m = m.withLastRunError(fmt.Sprintf("%s: %v", runtimePortEnvKey(f), err))
			return m, clearLastRunNoteAfterCmd()
		}
	}
	next, err := m.settingsFromRuntimeInputs()
	if err != nil {
		m = m.withLastRunError(err.Error())
		return m, clearLastRunNoteAfterCmd()
	}
	m.settings = next
	// Re-probing is synchronous here because the panel must show the result of
	// the save immediately; the timeout keeps an unreachable backend from
	// freezing the UI.
	ctx, cancel := context.WithTimeout(context.Background(), runtimeProbeTimeout)
	defer cancel()
	m.runtime = m.svc.discoverRuntime(ctx, m.settings)
	var cmd tea.Cmd
	if err := writeConfigFromModel(m); err != nil {
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

// runtimePortEnvKey names the environment variable a port field corresponds to,
// so validation errors point at something the user can also set from the shell.
func runtimePortEnvKey(f runtimeField) string {
	switch f {
	case runtimeFieldVLLMPort:
		return settings.EnvVLLMServerPort
	case runtimeFieldKoboldCppPort:
		return settings.EnvKoboldCppPort
	case runtimeFieldNInferPort:
		return settings.EnvNInferServerPort
	case runtimeFieldOMLXPort:
		return settings.EnvOMLXPort
	case runtimeFieldSplashPort:
		return settings.EnvSplashPort
	default:
		return settings.EnvLlamaServerPort
	}
}

// updateRuntimeConfigKey handles keys while the runtime env editor is open.
func (m Model) updateRuntimeConfigKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if m.rc.discardConfirm {
		return m.updateRuntimeConfigDiscardConfirmKey(msg)
	}
	if isEscapeKey(msg) {
		if m.runtimeConfigDirty() {
			m.rc.discardConfirm = true
			return m, nil
		}
		m = m.withLastRunCleared()
		m = m.closeRuntimeConfig()
		return m, nil
	}
	if isEnterKey(msg) {
		return m.commitRuntimeConfig()
	}
	if isTabKey(msg) {
		return m.focusRuntimeField(m.stepRuntimeField(m.rc.focus, 1))
	}
	if isShiftTabKey(msg) {
		return m.focusRuntimeField(m.stepRuntimeField(m.rc.focus, -1))
	}
	var cmd tea.Cmd
	m.rc.inputs[m.rc.focus], cmd = m.rc.inputs[m.rc.focus].Update(msg)
	return m, cmd
}
