// Package tui implements the Bubble Tea terminal UI for LLM Launcher (llml):
// model discovery, runtime detection, table rendering, and llama-server launch.
package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// Run starts the full-screen TUI. Alt-screen rendering is enabled on the root [tea.View]
// (see [Model.View]); do not use the removed [tea.WithAltScreen] program option.
func Run() error {
	p := tea.NewProgram(New())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}
