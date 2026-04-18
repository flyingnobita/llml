package tui

import (
	"charm.land/lipgloss/v2"
)

// helpEntry is a single row in the keyboard shortcuts popup.
type helpEntry struct {
	key  string
	desc string
}

// helpSections returns sections of keyboard shortcuts for the help popup.
func helpSections() []struct {
	title   string
	entries []helpEntry
} {
	return []struct {
		title   string
		entries []helpEntry
	}{
		{
			title: "Navigation",
			entries: []helpEntry{
				{"↑/k", "Move up"},
				{"↓/j", "Move down"},
				{"←/h", "Scroll left"},
				{"→/l", "Scroll right"},
				{"tab", "Switch sections"},
			},
		},
		{
			title: "Model Actions",
			entries: []helpEntry{
				{"R", "Run server (split pane)"},
				{"ctrl+R", "Run server (full terminal)"},
				{"enter", "Copy launch command"},
				{"[/]", "Scroll launch preview"},
			},
		},
		{
			title: "Configuration",
			entries: []helpEntry{
				{"c", "Runtime environment"},
				{"p", "Parameter profiles"},
				{"r", "Reload runtime"},
				{"S", "Rescan models"},
			},
		},
		{
			title: "Table",
			entries: []helpEntry{
				{",", "Cycle sort column"},
				{".", "Reverse sort order"},
			},
		},
		{
			title: "General",
			entries: []helpEntry{
				{"t", "Cycle theme"},
				{"?", "Keyboard shortcuts"},
				{"q", "Quit"},
			},
		},
		{
			title: "Split Server Pane",
			entries: []helpEntry{
				{"tab", "Switch table / log"},
				{"esc/q", "Stop server"},
			},
		},
	}
}

// helpPanelModalBlock renders the keyboard shortcuts popup as a bordered modal.
func (m Model) helpPanelModalBlock() string {
	cw := m.paramPanelContentWidth()

	// Determine column widths: key column is the widest key, rest goes to description.
	sections := helpSections()
	maxKeyW := 0
	for _, s := range sections {
		for _, e := range s.entries {
			if len(e.key) > maxKeyW {
				maxKeyW = len(e.key)
			}
		}
	}
	keyColW := maxKeyW + 2 // padding

	keyStyle := lipgloss.NewStyle().
		Foreground(m.ui.theme.Title).
		Bold(true).
		Width(keyColW).
		Align(lipgloss.Right)

	descStyle := lipgloss.NewStyle().
		Foreground(m.ui.theme.Body).
		PaddingLeft(2)

	sectionTitleStyle := lipgloss.NewStyle().
		Foreground(m.ui.theme.ParamSectionHeading).
		Bold(true).
		MarginTop(1)

	var rows []string
	rows = append(rows, m.modalTitleRow(cw, m.ui.styles.portConfigTitle, "Keyboard Shortcuts"))

	for _, section := range sections {
		rows = append(rows, sectionTitleStyle.Render(section.title))
		for _, entry := range section.entries {
			line := lipgloss.JoinHorizontal(lipgloss.Top,
				keyStyle.Render(entry.key),
				descStyle.Render(entry.desc),
			)
			rows = append(rows, line)
		}
	}
	rows = append(rows, "")
	rows = append(rows, m.ui.styles.footer.Render("esc: close"))

	block := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return m.ui.styles.portConfigBox.Render(block)
}
