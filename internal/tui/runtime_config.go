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
	runtimeFieldVLLMPath
	runtimeFieldVLLMVenv
	runtimeFieldLlamaPort
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
	ti.SetWidth(56)
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
	m.runtimeConfigOpen = true
	m.launchPreviewFocused = false
	m = m.withLastRunCleared()
	m.runtimeInputs[runtimeFieldLlamaCppPath].SetValue(os.Getenv(models.EnvLlamaCppPath))
	m.runtimeInputs[runtimeFieldVLLMPath].SetValue(os.Getenv(models.EnvVLLMPath))
	m.runtimeInputs[runtimeFieldVLLMVenv].SetValue(os.Getenv(models.EnvVLLMVenv))
	m.runtimeInputs[runtimeFieldLlamaPort].SetValue(prefillPort(models.EnvLlamaServerPort, models.ListenPort()))
	m.runtimeInputs[runtimeFieldVLLMPort].SetValue(prefillPort(models.EnvVLLMServerPort, models.VLLMPort()))
	return m.focusRuntimeField(focus)
}

// maybeSetMissingRuntimeFooterNote sets [Model.lastRunNote] when the scan found models that need a
// backend binary, but [models.ResolveLlamaServerPath] or [models.ResolveVLLMPath] is empty.
// GGUF rows require llama-server; vLLM rows require vllm. Clears the footer line when neither applies.
func (m Model) maybeSetMissingRuntimeFooterNote() (Model, tea.Cmd) {
	var wantLlama, wantVLLM bool
	for _, f := range m.files {
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

func (m Model) closeRuntimeConfig() Model {
	m.runtimeConfigOpen = false
	for i := range m.runtimeInputs {
		(&m.runtimeInputs[i]).Blur()
		m.runtimeInputs[i].SetValue("")
	}
	return m
}

func (m Model) focusRuntimeField(i runtimeField) (Model, tea.Cmd) {
	if i < 0 || i >= runtimeFieldCount {
		i = 0
	}
	m.runtimeFocus = i
	var cmd tea.Cmd
	for j := range m.runtimeInputs {
		if runtimeField(j) == i {
			cmd = (&m.runtimeInputs[j]).Focus()
		} else {
			(&m.runtimeInputs[j]).Blur()
		}
	}
	return m, cmd
}

func (m Model) commitRuntimeConfig() (Model, tea.Cmd) {
	if err := validatePortCommit(m.runtimeInputs[runtimeFieldLlamaPort].Value()); err != nil {
		m = m.withLastRunError(fmt.Sprintf("%s: %v", models.EnvLlamaServerPort, err))
		return m, nil
	}
	if err := validatePortCommit(m.runtimeInputs[runtimeFieldVLLMPort].Value()); err != nil {
		m = m.withLastRunError(fmt.Sprintf("%s: %v", models.EnvVLLMServerPort, err))
		return m, nil
	}
	applyPathEnv(models.EnvLlamaCppPath, m.runtimeInputs[runtimeFieldLlamaCppPath].Value())
	applyPathEnv(models.EnvVLLMPath, m.runtimeInputs[runtimeFieldVLLMPath].Value())
	applyPathEnv(models.EnvVLLMVenv, m.runtimeInputs[runtimeFieldVLLMVenv].Value())
	if err := applyListenPortEnv(m.runtimeInputs[runtimeFieldLlamaPort].Value()); err != nil {
		m = m.withLastRunError(err.Error())
		return m, nil
	}
	if err := applyVLLMPortEnv(m.runtimeInputs[runtimeFieldVLLMPort].Value()); err != nil {
		m = m.withLastRunError(err.Error())
		return m, nil
	}
	m.runtime = models.DiscoverRuntime()
	if err := writeConfigFromModel(m); err != nil {
		m = m.withLastRunError("Could not save config: " + err.Error())
	} else {
		m = m.withLastRunCleared()
	}
	m = m.closeRuntimeConfig()
	return m, nil
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
		next := (m.runtimeFocus + 1) % runtimeFieldCount
		return m.focusRuntimeField(next)
	case "shift+tab":
		prev := (m.runtimeFocus + runtimeFieldCount - 1) % runtimeFieldCount
		return m.focusRuntimeField(prev)
	default:
		var cmd tea.Cmd
		m.runtimeInputs[m.runtimeFocus], cmd = m.runtimeInputs[m.runtimeFocus].Update(msg)
		return m, cmd
	}
}
