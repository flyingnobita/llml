package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/flyingnobita/llml/internal/models"
)

type runtimeField int

const (
	runtimeFieldLlamaCppPath runtimeField = iota
	runtimeFieldLlamaPort
	runtimeFieldVLLMPath
	runtimeFieldVLLMVenv
	runtimeFieldVLLMPort
	runtimeFieldCount
)

// applyListenPortEnv sets LLAMA_SERVER_PORT from user input, or unsets it when empty
// (default port 8080 via models.ListenPort). Input should be digits only or empty.
func applyListenPortEnv(raw string) error {
	v := strings.TrimSpace(raw)
	if v == "" {
		os.Unsetenv(models.EnvLlamaServerPort)
		return nil
	}
	p, err := strconv.Atoi(v)
	if err != nil || p < 1 || p > 65535 {
		return fmt.Errorf("port must be 1-65535 or empty for default 8080")
	}
	os.Setenv(models.EnvLlamaServerPort, v)
	return nil
}

// applyVLLMPortEnv sets VLLM_SERVER_PORT from user input, or unsets it when empty
// (default port 8000 via models.VLLMPort).
func applyVLLMPortEnv(raw string) error {
	v := strings.TrimSpace(raw)
	if v == "" {
		os.Unsetenv(models.EnvVLLMServerPort)
		return nil
	}
	p, err := strconv.Atoi(v)
	if err != nil || p < 1 || p > 65535 {
		return fmt.Errorf("port must be 1-65535 or empty for default 8000")
	}
	os.Setenv(models.EnvVLLMServerPort, v)
	return nil
}

// prefillPort shows the env value when set; otherwise the effective port (same as the footer / server commands).
func prefillPort(envKey string, effective int) string {
	if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
		return v
	}
	return strconv.Itoa(effective)
}

// applyPathEnv sets or unsets a path-style environment variable (trimmed; empty unsets).
// A leading "~" or "~/" is expanded to the user's home directory ([models.ExpandTildePath]).
func applyPathEnv(key, raw string) {
	v := strings.TrimSpace(raw)
	if v == "" {
		os.Unsetenv(key)
		return
	}
	v = filepath.Clean(models.ExpandTildePath(v))
	if v == "" || v == "." {
		os.Unsetenv(key)
		return
	}
	os.Setenv(key, v)
}

func validatePortInput(s string) error {
	for _, r := range s {
		if r < '0' || r > '9' {
			return fmt.Errorf("digits only")
		}
	}
	if len(s) > 5 {
		return fmt.Errorf("max 5 digits")
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
		return fmt.Errorf("port must be 1-65535 or empty to use the default")
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
	ti.CharLimit = 2048
	ti.SetWidth(38)
	ti.Blur()
	return ti
}

// openRuntimeConfig shows editors for the same env vars summarized in the runtimes footer.
func (m Model) openRuntimeConfig() (Model, tea.Cmd) {
	return m.openRuntimeConfigFocused(runtimeFieldLlamaCppPath)
}

// openRuntimeConfigFocused opens the runtime editor with the given field focused and clears any
// footer status line ([Model.lastRunNote]).
func (m Model) openRuntimeConfigFocused(focus runtimeField) (Model, tea.Cmd) {
	m.rc.open = true
	m.preview.focused = false
	m = m.withLastRunCleared()
	m.rc.inputs[runtimeFieldLlamaCppPath].SetValue(os.Getenv(models.EnvLlamaCppPath))
	m.rc.inputs[runtimeFieldVLLMPath].SetValue(os.Getenv(models.EnvVLLMPath))
	m.rc.inputs[runtimeFieldVLLMVenv].SetValue(os.Getenv(models.EnvVLLMVenv))
	m.rc.inputs[runtimeFieldLlamaPort].SetValue(prefillPort(models.EnvLlamaServerPort, models.ListenPort()))
	m.rc.inputs[runtimeFieldVLLMPort].SetValue(prefillPort(models.EnvVLLMServerPort, models.VLLMPort()))
	return m.focusRuntimeField(focus)
}

// maybeSetMissingRuntimeFooterNote sets [Model.lastRunNote] when the scan found models that need a
// backend binary, but [models.ResolveLlamaServerPath] or [models.ResolveVLLMPath] is empty.
// GGUF rows require llama-server; vLLM rows require vllm. Clears the footer line when neither applies.
func (m Model) maybeSetMissingRuntimeFooterNote() (Model, tea.Cmd) {
	var wantLlama, wantVLLM bool
	for _, f := range m.table.files {
		switch f.Backend {
		case models.BackendLlama:
			wantLlama = true
		case models.BackendVLLM:
			wantVLLM = true
		}
	}
	haveLlama := models.ResolveLlamaServerPath(m.runtime) != ""
	haveVLLM := models.ResolveVLLMPath(m.runtime) != ""

	var msgs []string
	if wantLlama && !haveLlama {
		msgs = append(msgs, MissingLlamaServerFooterNote)
	}
	if wantVLLM && !haveVLLM {
		msgs = append(msgs, MissingVLLMFooterNote)
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
	return m
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

func (m Model) commitRuntimeConfig() (Model, tea.Cmd) {
	if err := validatePortCommit(m.rc.inputs[runtimeFieldLlamaPort].Value()); err != nil {
		m = m.withLastRunError(fmt.Sprintf("%s: %v", models.EnvLlamaServerPort, err))
		return m, clearLastRunNoteAfterCmd()
	}
	if err := validatePortCommit(m.rc.inputs[runtimeFieldVLLMPort].Value()); err != nil {
		m = m.withLastRunError(fmt.Sprintf("%s: %v", models.EnvVLLMServerPort, err))
		return m, clearLastRunNoteAfterCmd()
	}
	applyPathEnv(models.EnvLlamaCppPath, m.rc.inputs[runtimeFieldLlamaCppPath].Value())
	applyPathEnv(models.EnvVLLMPath, m.rc.inputs[runtimeFieldVLLMPath].Value())
	applyPathEnv(models.EnvVLLMVenv, m.rc.inputs[runtimeFieldVLLMVenv].Value())
	if err := applyListenPortEnv(m.rc.inputs[runtimeFieldLlamaPort].Value()); err != nil {
		m = m.withLastRunError(err.Error())
		return m, clearLastRunNoteAfterCmd()
	}
	if err := applyVLLMPortEnv(m.rc.inputs[runtimeFieldVLLMPort].Value()); err != nil {
		m = m.withLastRunError(err.Error())
		return m, clearLastRunNoteAfterCmd()
	}
	m.runtime = models.DiscoverRuntime()
	var cmd tea.Cmd
	if err := writeConfigFromModel(m); err != nil {
		m = m.withLastRunError("Could not save config: " + err.Error())
		cmd = clearLastRunNoteAfterCmd()
	} else {
		m = m.withLastRunCleared()
	}
	m = m.closeRuntimeConfig()
	m = m.withLaunchPreviewSynced()
	return m, cmd
}

// updateRuntimeConfigKey handles keys while the runtime env editor is open.
func (m Model) updateRuntimeConfigKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m = m.withLastRunCleared()
		m = m.closeRuntimeConfig()
		return m, nil
	case "enter":
		return m.commitRuntimeConfig()
	case "tab":
		next := (m.rc.focus + 1) % runtimeFieldCount
		return m.focusRuntimeField(next)
	case "shift+tab":
		prev := (m.rc.focus + runtimeFieldCount - 1) % runtimeFieldCount
		return m.focusRuntimeField(prev)
	default:
		var cmd tea.Cmd
		m.rc.inputs[m.rc.focus], cmd = m.rc.inputs[m.rc.focus].Update(msg)
		return m, cmd
	}
}
