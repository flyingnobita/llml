package tui

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/flyingnobita/llm-launch/internal/llamacpp"
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

func newPortTextInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.CharLimit = 5
	ti.Width = 8
	ti.Validate = validatePortInput
	ti.Blur()
	return ti
}

// openPortConfig shows the listen-port editor, prefilled from the environment when set.
func (m Model) openPortConfig() (Model, tea.Cmd) {
	m.portConfigOpen = true
	m.lastRunNote = ""
	m.portInput.SetValue(os.Getenv(llamacpp.EnvLlamaServerPort))
	return m, m.portInput.Focus()
}

func (m Model) closePortConfig() Model {
	m.portConfigOpen = false
	m.portInput.Blur()
	m.portInput.SetValue("")
	return m
}

func (m Model) commitPortConfig() (Model, tea.Cmd) {
	if err := applyListenPortEnv(m.portInput.Value()); err != nil {
		m.lastRunNote = err.Error()
		return m, nil
	}
	m.runtime.ProbePort = llamacpp.ListenPort()
	m.lastRunNote = ""
	m = m.closePortConfig()
	return m, nil
}

// updatePortConfigKey handles keys while the port editor is open. enter saves, esc cancels.
func (m Model) updatePortConfigKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	switch msg.String() {
	case "esc":
		m.lastRunNote = ""
		m = m.closePortConfig()
		return m, nil
	case "enter":
		return m.commitPortConfig()
	default:
		var cmd tea.Cmd
		m.portInput, cmd = m.portInput.Update(msg)
		return m, cmd
	}
}
