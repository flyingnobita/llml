package tui

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/flyingnobita/llml/internal/llamacpp"
)

const (
	runtimeFieldLlamaCppPath = iota
	runtimeFieldVLLMPath
	runtimeFieldVLLMVenv
	runtimeFieldLlamaPort
	runtimeFieldVLLMPort
	runtimeFieldCount
)

// applyListenPortEnv sets LLAMA_SERVER_PORT from user input, or unsets it when empty
// (default port 8080 via llamacpp.ListenPort). Input should be digits only or empty.
func applyListenPortEnv(raw string) error {
	v := strings.TrimSpace(raw)
	if v == "" {
		os.Unsetenv(llamacpp.EnvLlamaServerPort)
		return nil
	}
	p, err := strconv.Atoi(v)
	if err != nil || p < 1 || p > 65535 {
		return fmt.Errorf("port must be 1-65535 or empty for default 8080")
	}
	os.Setenv(llamacpp.EnvLlamaServerPort, v)
	return nil
}

// applyVLLMPortEnv sets VLLM_SERVER_PORT from user input, or unsets it when empty
// (default port 8000 via llamacpp.VLLMPort).
func applyVLLMPortEnv(raw string) error {
	v := strings.TrimSpace(raw)
	if v == "" {
		os.Unsetenv(llamacpp.EnvVLLMServerPort)
		return nil
	}
	p, err := strconv.Atoi(v)
	if err != nil || p < 1 || p > 65535 {
		return fmt.Errorf("port must be 1-65535 or empty for default 8000")
	}
	os.Setenv(llamacpp.EnvVLLMServerPort, v)
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
func applyPathEnv(key, raw string) {
	v := strings.TrimSpace(raw)
	if v == "" {
		os.Unsetenv(key)
	} else {
		os.Setenv(key, v)
	}
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

// validatePortCommit checks a port field before applying (empty = llamacpp default for that env).
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
	ti.Width = 8
	ti.Validate = validatePortInput
	ti.Blur()
	return ti
}

func newPathTextInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.CharLimit = 2048
	ti.Width = 56
	ti.Blur()
	return ti
}

// openRuntimeConfig shows editors for the same env vars summarized in the runtimes footer.
func (m Model) openRuntimeConfig() (Model, tea.Cmd) {
	m.runtimeConfigOpen = true
	m.lastRunNote = ""
	m.runtimeInputs[runtimeFieldLlamaCppPath].SetValue(os.Getenv(llamacpp.EnvLlamaCppPath))
	m.runtimeInputs[runtimeFieldVLLMPath].SetValue(os.Getenv(llamacpp.EnvVLLMPath))
	m.runtimeInputs[runtimeFieldVLLMVenv].SetValue(os.Getenv(llamacpp.EnvVLLMVenv))
	m.runtimeInputs[runtimeFieldLlamaPort].SetValue(prefillPort(llamacpp.EnvLlamaServerPort, llamacpp.ListenPort()))
	m.runtimeInputs[runtimeFieldVLLMPort].SetValue(prefillPort(llamacpp.EnvVLLMServerPort, llamacpp.VLLMPort()))
	return m.focusRuntimeField(runtimeFieldLlamaCppPath)
}

func (m Model) closeRuntimeConfig() Model {
	m.runtimeConfigOpen = false
	for i := range m.runtimeInputs {
		(&m.runtimeInputs[i]).Blur()
		m.runtimeInputs[i].SetValue("")
	}
	return m
}

func (m Model) focusRuntimeField(i int) (Model, tea.Cmd) {
	if i < 0 || i >= runtimeFieldCount {
		i = 0
	}
	m.runtimeFocus = i
	var cmd tea.Cmd
	for j := range m.runtimeInputs {
		if j == i {
			cmd = (&m.runtimeInputs[j]).Focus()
		} else {
			(&m.runtimeInputs[j]).Blur()
		}
	}
	return m, cmd
}

func (m Model) commitRuntimeConfig() (Model, tea.Cmd) {
	if err := validatePortCommit(m.runtimeInputs[runtimeFieldLlamaPort].Value()); err != nil {
		m.lastRunNote = fmt.Sprintf("%s: %v", llamacpp.EnvLlamaServerPort, err)
		return m, nil
	}
	if err := validatePortCommit(m.runtimeInputs[runtimeFieldVLLMPort].Value()); err != nil {
		m.lastRunNote = fmt.Sprintf("%s: %v", llamacpp.EnvVLLMServerPort, err)
		return m, nil
	}
	applyPathEnv(llamacpp.EnvLlamaCppPath, m.runtimeInputs[runtimeFieldLlamaCppPath].Value())
	applyPathEnv(llamacpp.EnvVLLMPath, m.runtimeInputs[runtimeFieldVLLMPath].Value())
	applyPathEnv(llamacpp.EnvVLLMVenv, m.runtimeInputs[runtimeFieldVLLMVenv].Value())
	if err := applyListenPortEnv(m.runtimeInputs[runtimeFieldLlamaPort].Value()); err != nil {
		m.lastRunNote = err.Error()
		return m, nil
	}
	if err := applyVLLMPortEnv(m.runtimeInputs[runtimeFieldVLLMPort].Value()); err != nil {
		m.lastRunNote = err.Error()
		return m, nil
	}
	m.runtime = llamacpp.DiscoverRuntime()
	m.lastRunNote = ""
	m = m.closeRuntimeConfig()
	return m, nil
}

// updateRuntimeConfigKey handles keys while the runtime env editor is open.
func (m Model) updateRuntimeConfigKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	switch msg.String() {
	case "esc":
		m.lastRunNote = ""
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
