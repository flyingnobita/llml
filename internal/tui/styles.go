package tui

import (
	"github.com/charmbracelet/lipgloss"

	btable "github.com/flyingnobita/llm-launch/internal/tui/btable"
)

// DefaultTableStyles sets header/cell colors and spacing.
// Selected uses a bright cyan (ANSI 256 #51) so the active row stands out vs gray body text (252).
// PaddingRight(1) on Header, Cell, and Selected keeps columns aligned (Selected must match Cell).
// Using Padding(0,1) on Cell only would add a leading gap in the first column.
// Rendering uses internal/tui/btable so Selected applies per cell (reliable fg/bg across the row).
func DefaultTableStyles() btable.Styles {
	return btable.Styles{
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("252")).
			PaddingRight(1),
		Cell: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			PaddingRight(1),
		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("51")).
			PaddingRight(1),
	}
}

// Central place for Lip Gloss styles. Adjust palette as your design evolves.
var (
	app = lipgloss.NewStyle().Padding(1, appPaddingH)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("99")).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginBottom(1)

	bodyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			MarginTop(1)

	// errorStyle is used for load errors and server run notes (ANSI red #203).
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))

	// Bottom panel: detected llama-cli / llama-server paths (border separates from table body).
	runtimePanelStyle = lipgloss.NewStyle().
				BorderTop(true).
				BorderForeground(lipgloss.Color("240")).
				Foreground(lipgloss.Color("246")).
				Padding(1, 0).
				MarginTop(1)
)
