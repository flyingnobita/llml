package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
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
	keys := DefaultKeyMap()
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
				{"tab", "Section"},
			},
		},
		{
			title: "Model Actions",
			entries: []helpEntry{
				{"R", "Run server (split pane)"},
				{"ctrl+R", "Run server (full terminal)"},
				{"enter", "Copy launch command"},
				{keys.ScrollPreviewUp.Help().Key, keys.ScrollPreviewUp.Help().Desc},
			},
		},
		{
			title: "Configuration",
			entries: []helpEntry{
				{"c", "Runtime Environment"},
				{"p", "Parameter Profiles"},
				{"E", "Export profiles"},
				{keys.Import.Help().Key, "Import profiles"},
				{"m", "Model Paths"},
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
				{"a", "Toggle alert history"},
				{"t", "Cycle theme"},
				{"?", "Keyboard shortcuts"},
				{"q", "Quit"},
			},
		},
		{
			title: "Split Server Pane",
			entries: []helpEntry{
				{"tab", "Switch table / log"},
				{"w", "Toggle word wrap"},
				{"s", "Stop server"},
				{"q", "Quit (warn if still running)"},
			},
		},
	}
}

// helpPanelModalBlock renders the keyboard shortcuts popup as a bordered modal.
// helpPanelModalBlock renders the keyboard shortcuts popup. The title and the
// footer (build identity, then key hints) stay put; the shortcut list between
// them scrolls when the terminal is too short to show it all.
func (m Model) helpPanelModalBlock() string {
	cw := m.paramPanelContentWidth()
	lines := m.helpBodyLines()
	height := m.helpBodyHeight()
	scrolls := len(lines) > height
	if scrolls {
		off := min(max(m.helpOffset, 0), len(lines)-height)
		lines = lines[off : off+height]
	}

	rows := make([]string, 0, len(lines)+4)
	rows = append(rows, m.modalTitleRow(cw, m.ui.styles.portConfigTitle, "Keyboard Shortcuts"))
	rows = append(rows, lines...)
	rows = append(rows, m.helpFooterRows(scrolls)...)

	block := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return m.ui.styles.portConfigBox.Render(block)
}

// helpBodyLines renders every shortcut section, one string per terminal line.
func (m Model) helpBodyLines() []string {
	sections := helpSections()
	maxKeyW := 0
	for _, s := range sections {
		for _, e := range s.entries {
			maxKeyW = max(maxKeyW, lipgloss.Width(e.key))
		}
	}
	// Only the key column's width depends on the content; the rest is theme.
	keyStyle := m.ui.styles.helpKey.Width(maxKeyW + 2)
	descStyle := m.ui.styles.helpDesc
	sectionTitleStyle := m.ui.styles.helpSectionTitle

	var rows []string
	for _, section := range sections {
		rows = append(rows, sectionTitleStyle.Render(section.title))
		for _, entry := range section.entries {
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top,
				keyStyle.Render(entry.key),
				descStyle.Render(entry.desc),
			))
		}
	}
	// Section titles carry a top margin, so one rendered row can span two lines.
	return strings.Split(lipgloss.JoinVertical(lipgloss.Left, rows...), "\n")
}

// helpFooterRows is the fixed bottom of the popup: a spacer, the running
// build's identity, and the key hints, which mention scrolling only when the
// list does not fit.
func (m Model) helpFooterRows(scrolls bool) []string {
	rows := []string{""}
	if m.buildID != "" {
		rows = append(rows, m.ui.styles.footer.Render(m.buildID))
	}
	hints := FooterParamHintBack
	if scrolls {
		hints = FooterHintHelpScroll + FooterHintSep + hints
	}
	return append(rows, m.renderFooterHints(hints))
}

// helpBodyHeight is how many shortcut lines fit between the popup's fixed title
// and footer in the current terminal.
func (m Model) helpBodyHeight() int {
	fixed := m.ui.styles.portConfigBox.GetVerticalFrameSize() + 1 + len(m.helpFooterRows(false))
	return max(1, m.layout.height-fixed)
}

// helpMaxOffset is the furthest the shortcut list can scroll.
func (m Model) helpMaxOffset() int {
	return max(0, len(m.helpBodyLines())-m.helpBodyHeight())
}

// openHelp shows the shortcuts popup scrolled to the top.
func (m Model) openHelp() Model {
	m.helpOpen = true
	m.helpOffset = 0
	return m
}

// updateHelpKey handles a key while the shortcuts popup is open: esc and ? close
// it, the scroll keys move the list, and every other key is swallowed.
func (m Model) updateHelpKey(msg tea.KeyPressMsg) Model {
	if isEscapeKey(msg) || key.Matches(msg, m.keys.Help) {
		m.helpOpen = false
		return m
	}
	page := m.helpBodyHeight()
	switch msg.String() {
	case "up", "k":
		m.helpOffset--
	case "down", "j":
		m.helpOffset++
	case "pgup":
		m.helpOffset -= page
	case "pgdown":
		m.helpOffset += page
	case "home":
		m.helpOffset = 0
	case "end":
		m.helpOffset = m.helpMaxOffset()
	}
	m.helpOffset = min(max(m.helpOffset, 0), m.helpMaxOffset())
	return m
}
