package tui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/flyingnobita/llml/internal/profiles"
)

func newFilterTextInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "Filter..."
	ti.CharLimit = FilterInputCharLimit
	ti.SetWidth(FilterTextInputWidth)
	ti.Blur()
	return ti
}

// openExportView populates the export view from model-params.json and opens it.
func (m Model) openExportView() Model {
	groups, err := profiles.AllToPortableGrouped()
	if err != nil {
		return m.addAlert(alertSeverityError, "Export", err.Error())
	}
	var items []exportProfileItem
	for _, g := range groups {
		modelDisplay := profiles.ModelHint(g.ModelKey)

		// Sort profiles within group by (backend, name).
		sorted := make([]profiles.PortableProfile, len(g.Profiles))
		copy(sorted, g.Profiles)
		sort.Slice(sorted, func(i, j int) bool {
			if sorted[i].Backend != sorted[j].Backend {
				return sorted[i].Backend < sorted[j].Backend
			}
			return sorted[i].Name < sorted[j].Name
		})

		items = append(items, exportProfileItem{
			kind:         exportItemHeader,
			modelKey:     g.ModelKey,
			modelDisplay: modelDisplay,
			checked:      false,
		})

		for _, pp := range sorted {
			backend := pp.Backend
			if backend == "" {
				backend = "llama"
			}
			items = append(items, exportProfileItem{
				kind:         exportItemProfile,
				modelKey:     g.ModelKey,
				modelDisplay: modelDisplay,
				backend:      backend,
				profileName:  pp.Name,
				checked:      false,
				pp:           pp,
			})
		}
	}

	dest := profiles.DefaultExportFilename()
	if !filepath.IsAbs(dest) {
		if cwd, err := filepath.Abs("."); err == nil {
			dest = filepath.Join(cwd, dest)
		}
	}

	// Position cursor on first profile item (skip initial header).
	startCursor := 0
	for startCursor < len(items) && items[startCursor].kind == exportItemHeader {
		startCursor++
	}
	if startCursor >= len(items) {
		startCursor = 0
	}

	m.export.filterInput.SetValue("")
	m.export.filteredIndices = nil
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = items
	m.export.cursor = startCursor
	m.export.scrollOffset = 0
	m.export.outputPath = dest
	m.export.pathInput.SetValue(dest)
	m.export.pathInput.CursorEnd()
	return m
}

// closeExportView closes the export modal and returns focus to the params panel.
func (m Model) closeExportView() Model {
	m.export.open = false
	m.export.items = nil
	m.export.cursor = 0
	m.export.scrollOffset = 0
	m.export.filterInput.SetValue("")
	m.export.filterInput.Blur()
	m.export.filteredIndices = nil
	return m
}

func (m Model) closeCollision() Model {
	m.collision.open = false
	m.collision.dest = ""
	m.collision.suffixPath = ""
	return m
}

func (m Model) exportSelectedCount() int {
	n := 0
	for _, it := range m.export.items {
		if it.kind == exportItemProfile && it.checked {
			n++
		}
	}
	return n
}

func (m Model) buildExportProfiles() []profiles.PortableProfile {
	var out []profiles.PortableProfile
	for _, it := range m.export.items {
		if it.kind == exportItemProfile && it.checked {
			out = append(out, it.pp)
		}
	}
	return out
}

// exportVisibleItems returns the items currently visible (all or filtered).
func (m Model) exportVisibleItems() []exportProfileItem {
	if m.export.filteredIndices == nil {
		return m.export.items
	}
	out := make([]exportProfileItem, len(m.export.filteredIndices))
	for i, idx := range m.export.filteredIndices {
		out[i] = m.export.items[idx]
	}
	return out
}

func (m Model) exportVisibleCount() int {
	if m.export.filteredIndices == nil {
		return len(m.export.items)
	}
	return len(m.export.filteredIndices)
}

func (m *Model) setExportPathWidth() {
	cw := m.paramPanelContentWidth()
	labelW := lipgloss.Width("Output: ")
	promptW := lipgloss.Width(m.export.pathInput.Prompt)
	// textinput.View() always adds 1 extra char for the cursor.
	inputW := cw - labelW - promptW - 1
	if inputW < 20 {
		inputW = 20
	}
	m.export.pathInput.SetWidth(inputW)
}

func (m Model) exportRealCursorIndex() int {
	if m.export.filteredIndices == nil {
		return m.export.cursor
	}
	if m.export.cursor < 0 || m.export.cursor >= len(m.export.filteredIndices) {
		return -1
	}
	return m.export.filteredIndices[m.export.cursor]
}

func matchesExportFilter(it exportProfileItem, filter string) bool {
	if it.kind == exportItemHeader {
		return strings.Contains(strings.ToLower(it.modelDisplay), filter)
	}
	return strings.Contains(strings.ToLower(it.profileName), filter) ||
		strings.Contains(strings.ToLower(it.modelDisplay), filter) ||
		strings.Contains(strings.ToLower(it.backend), filter)
}

func (m *Model) rebuildExportFilter() {
	filter := strings.ToLower(strings.TrimSpace(m.export.filterInput.Value()))
	if filter == "" {
		m.export.filteredIndices = nil
		return
	}

	indices := make([]int, 0)
	i := 0
	items := m.export.items
	for i < len(items) {
		if items[i].kind == exportItemHeader {
			headerIdx := i
			i++
			groupStart := i
			for i < len(items) && items[i].kind == exportItemProfile {
				i++
			}
			groupEnd := i

			headerMatches := matchesExportFilter(items[headerIdx], filter)
			anyProfileMatch := false
			for j := groupStart; j < groupEnd; j++ {
				if matchesExportFilter(items[j], filter) {
					anyProfileMatch = true
					break
				}
			}

			if headerMatches || anyProfileMatch {
				indices = append(indices, headerIdx)
				for j := groupStart; j < groupEnd; j++ {
					if matchesExportFilter(items[j], filter) {
						indices = append(indices, j)
					}
				}
			}
		} else {
			// Orphan profiles (no header).
			if matchesExportFilter(items[i], filter) {
				indices = append(indices, i)
			}
			i++
		}
	}

	m.export.filteredIndices = indices
	if len(indices) == 0 {
		m.export.cursor = 0
	} else if m.export.cursor >= len(indices) {
		m.export.cursor = len(indices) - 1
	}
	m.export.scrollOffset = 0
}

// exportMaxVisibleItems returns how many profile rows can fit in the terminal.
func (m Model) exportMaxVisibleItems() int {
	termH := m.layout.height
	if termH < 1 {
		termH = 24
	}
	fixedH := 13 // title + list section box + blanks + path + footer + borders + margin
	return max(termH-fixedH, 3)
}

// adjustExportScroll ensures the cursor is visible within the current scroll window.
func (m *Model) adjustExportScroll() {
	maxVis := m.exportMaxVisibleItems()
	if m.export.cursor < m.export.scrollOffset {
		m.export.scrollOffset = m.export.cursor
	}
	if m.export.cursor >= m.export.scrollOffset+maxVis {
		m.export.scrollOffset = m.export.cursor - maxVis + 1
	}
}

// syncHeaderStates updates each header's checked state based on visible profiles.
func (m *Model) syncHeaderStates() {
	items := m.exportVisibleItems()
	groupChecked := make(map[string]bool)
	groupAny := make(map[string]bool)
	for _, it := range items {
		if it.kind == exportItemProfile {
			groupAny[it.modelKey] = true
			if !it.checked {
				groupChecked[it.modelKey] = false
			} else if _, seen := groupChecked[it.modelKey]; !seen {
				groupChecked[it.modelKey] = true
			}
		}
	}
	for i := range m.export.items {
		if m.export.items[i].kind == exportItemHeader {
			key := m.export.items[i].modelKey
			all, ok := groupChecked[key]
			m.export.items[i].checked = ok && all && groupAny[key]
		}
	}
}

// toggleGroup sets all profiles in the same group as the header at cursor
// to the given checked state.
func (m *Model) toggleGroup(checked bool) {
	idx := m.exportRealCursorIndex()
	if idx < 0 || idx >= len(m.export.items) {
		return
	}
	header := m.export.items[idx]
	if header.kind != exportItemHeader {
		return
	}
	for i := range m.export.items {
		if m.export.items[i].kind == exportItemProfile && m.export.items[i].modelKey == header.modelKey {
			m.export.items[i].checked = checked
		}
	}
	m.syncHeaderStates()
}

func (m Model) updateExportKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.export.focus == exportFocusFilter {
		if isEscapeKey(msg) {
			m.export.filterInput.SetValue("")
			m.export.filteredIndices = nil
			m.export.cursor = 0
			m.export.scrollOffset = 0
			m.export.filterInput.Blur()
			m.export.focus = exportFocusList
			return m, nil
		}
		if isTabKey(msg) {
			m.export.filterInput.Blur()
			m.export.focus = exportFocusPath
			m.setExportPathWidth()
			m.export.pathInput.Focus()
			m.export.pathInput.SetValue(m.export.outputPath)
			m.export.pathInput.CursorEnd()
			return m, nil
		}
		var cmd tea.Cmd
		m.export.filterInput, cmd = m.export.filterInput.Update(msg)
		m.rebuildExportFilter()
		return m, cmd
	}

	if m.export.focus == exportFocusPath {
		if msg.String() == "/" {
			m.export.focus = exportFocusFilter
			m.export.pathInput.Blur()
			m.export.filterInput.Focus()
			m.export.filterInput.CursorEnd()
			m.rebuildExportFilter()
			return m, nil
		}
		if isEscapeKey(msg) {
			m.export.focus = exportFocusList
			return m, nil
		}
		if isTabKey(msg) {
			m.export.focus = exportFocusList
			return m, nil
		}
		if isEnterKey(msg) {
			return m.doExportAttempt()
		}
		var cmd tea.Cmd
		m.export.pathInput, cmd = m.export.pathInput.Update(msg)
		m.export.outputPath = m.export.pathInput.Value()
		return m, cmd
	}

	// Focus on profile list.
	if isEscapeKey(msg) {
		m = m.closeExportView()
		return m, nil
	}
	if isTabKey(msg) {
		m.export.focus = exportFocusPath
		m.setExportPathWidth()
		m.export.pathInput.Focus()
		m.export.pathInput.SetValue(m.export.outputPath)
		m.export.pathInput.CursorEnd()
		return m, nil
	}
	if isEnterKey(msg) {
		return m.doExportAttempt()
	}
	switch msg.String() {
	case "/":
		m.export.focus = exportFocusFilter
		m.export.filterInput.Focus()
		m.export.filterInput.CursorEnd()
		m.rebuildExportFilter()
		return m, nil
	case "space":
		idx := m.exportRealCursorIndex()
		if idx >= 0 && idx < len(m.export.items) {
			it := &m.export.items[idx]
			switch it.kind {
			case exportItemHeader:
				m.toggleGroup(!it.checked)
			case exportItemProfile:
				it.checked = !it.checked
				m.syncHeaderStates()
			}
		}
		return m, nil
	case "a":
		if m.export.filteredIndices != nil {
			for _, idx := range m.export.filteredIndices {
				if m.export.items[idx].kind == exportItemProfile {
					m.export.items[idx].checked = true
				}
			}
		} else {
			for i := range m.export.items {
				if m.export.items[i].kind == exportItemProfile {
					m.export.items[i].checked = true
				}
			}
		}
		m.syncHeaderStates()
		return m, nil
	case "A":
		if m.export.filteredIndices != nil {
			for _, idx := range m.export.filteredIndices {
				if m.export.items[idx].kind == exportItemProfile {
					m.export.items[idx].checked = false
				}
			}
		} else {
			for i := range m.export.items {
				if m.export.items[i].kind == exportItemProfile {
					m.export.items[i].checked = false
				}
			}
		}
		m.syncHeaderStates()
		return m, nil
	case "up", "k":
		if m.export.cursor > 0 {
			m.export.cursor--
			m.adjustExportScroll()
		}
		return m, nil
	case "down", "j":
		if m.export.cursor < m.exportVisibleCount()-1 {
			m.export.cursor++
			m.adjustExportScroll()
		}
		return m, nil
	case "ctrl+u":
		pageSize := m.exportMaxVisibleItems()
		m.export.cursor = max(m.export.cursor-pageSize, 0)
		m.adjustExportScroll()
		return m, nil
	case "ctrl+d":
		pageSize := m.exportMaxVisibleItems()
		m.export.cursor = min(m.export.cursor+pageSize, m.exportVisibleCount()-1)
		m.adjustExportScroll()
		return m, nil
	}
	return m, nil
}

func (m Model) doExportAttempt() (tea.Model, tea.Cmd) {
	sel := m.exportSelectedCount()
	if sel == 0 {
		return m, nil
	}

	dest := m.export.outputPath
	path, existed, err := profiles.NextAvailablePath(dest)
	if err != nil {
		return m.flashError(err.Error())
	}

	if !existed {
		return m.finishExport(dest, false)
	}

	// File exists — open collision sub-modal.
	m.collision.open = true
	m.collision.dest = dest
	m.collision.suffixPath = path
	return m, nil
}

func (m Model) finishExport(dest string, force bool) (tea.Model, tea.Cmd) {
	pps := m.buildExportProfiles()
	if err := profiles.WritePortable(dest, pps, force); err != nil {
		return m.flashError(err.Error())
	}
	m = m.addAlert(alertSeverityInfo, "Export", fmt.Sprintf("Exported %d profiles to %s", len(pps), dest))
	m = m.closeExportView()
	m = m.closeCollision()
	return m.flashSuccess(fmt.Sprintf("Exported %d profiles to %s", len(pps), dest))
}

func (m Model) updateCollisionKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case isEscapeKey(msg):
		m = m.closeCollision()
		return m, nil
	case strings.EqualFold(msg.String(), "o"):
		return m.finishExport(m.collision.dest, true)
	case strings.EqualFold(msg.String(), "n"):
		return m.finishExport(m.collision.suffixPath, false)
	}
	return m, nil
}

func (m Model) exportModalBlock() string {
	cw := m.paramPanelContentWidth()
	panelBox := m.ui.styles.paramPanelBox
	bodyStyle := m.ui.styles.body
	dimStyle := m.ui.styles.footer

	// List section border: bright when list or filter has focus.
	listBox := m.ui.styles.exportListBox
	if m.export.focus == exportFocusList || m.export.focus == exportFocusFilter {
		listBox = m.ui.styles.exportListBoxFocused
	}
	listFrameW := listBox.GetHorizontalFrameSize()
	listInnerW := cw - listFrameW

	var listRows []string
	listRows = append(listRows, m.renderExportFilterRow(bodyStyle, dimStyle))

	if len(m.export.items) == 0 {
		listRows = append(listRows, dimStyle.Render("  No profiles to export"))
	} else if m.export.filteredIndices != nil && len(m.export.filteredIndices) == 0 {
		listRows = append(listRows, dimStyle.Render("  "+FooterExportNoMatch))
	}

	items := m.exportVisibleItems()
	maxVis := m.exportMaxVisibleItems()

	if len(items) <= maxVis {
		m.export.scrollOffset = 0
		for i, it := range items {
			listRows = append(listRows, m.renderExportItemRow(i, it, bodyStyle))
		}
		for pad := len(items); pad < maxVis; pad++ {
			listRows = append(listRows, "")
		}
	} else {
		m.adjustExportScroll()
		contentW := listInnerW - 2 // 1 for scrollbar glyph + 1 spacer
		rowStyle := m.ui.styles.exportScrollRow.Width(contentW)

		for i := 0; i < maxVis; i++ {
			actualIdx := m.export.scrollOffset + i
			var content string
			if actualIdx < len(items) {
				content = m.renderExportItemRow(i, items[actualIdx], bodyStyle)
			}
			glyph := scrollbarGlyph(i, maxVis, len(items), m.export.scrollOffset)
			listRows = append(listRows, rowStyle.Render(content)+" "+glyph)
		}
	}

	listBlock := lipgloss.JoinVertical(lipgloss.Left, listRows...)
	listSection := listBox.Width(listInnerW + listFrameW).Render(listBlock)

	var rows []string
	rows = append(rows, m.modalTitleRow(cw, m.ui.styles.portConfigTitle, "Export Profiles"))
	rows = append(rows, listSection)
	rows = append(rows, "")

	// Output path.
	pathLabel := "Output: "
	if m.export.focus == exportFocusPath {
		m.setExportPathWidth()
		rows = append(rows, m.ui.styles.bodyBold.Render(pathLabel)+m.export.pathInput.View())
	} else {
		labelW := lipgloss.Width(bodyStyle.Render(pathLabel))
		pathW := cw - labelW
		dispPath := m.export.outputPath
		if lipgloss.Width(dispPath) > pathW && pathW > 0 {
			dispPath = dispPath[:max(pathW-1, 0)] + "…"
		}
		rows = append(rows, dimStyle.Render(pathLabel)+bodyStyle.Render(dispPath))
	}
	rows = append(rows, "")

	// Footer.
	sel := m.exportSelectedCount()
	footerHints := FooterExportHintsList
	switch m.export.focus {
	case exportFocusFilter:
		footerHints = FooterExportHintsFilter
	case exportFocusPath:
		footerHints = FooterExportHintsPath
	}
	footer := fmt.Sprintf("(%d selected)  %s", sel, footerHints)
	rows = append(rows, m.renderFooterHints(footer))

	block := lipgloss.JoinVertical(lipgloss.Left, rows...)
	if m.lastRunNote != "" {
		block = lipgloss.JoinVertical(lipgloss.Left, block, "", m.lastRunNoteView())
	}
	return panelBox.Width(cw + panelBox.GetHorizontalFrameSize()).Render(block)
}

func (m Model) renderExportFilterRow(bodyStyle, dimStyle lipgloss.Style) string {
	prefix := "/"
	if m.export.focus == exportFocusFilter {
		return bodyStyle.Render(prefix) + m.export.filterInput.View()
	}
	filterVal := m.export.filterInput.Value()
	if filterVal != "" {
		return bodyStyle.Render(prefix + filterVal)
	}
	return dimStyle.Render(prefix + "filter...")
}

func (m Model) renderExportItemRow(i int, it exportProfileItem, bodyStyle lipgloss.Style) string {
	cursor := "  "
	if i == m.export.cursor && m.export.focus == exportFocusList {
		cursor = "▸ "
	}

	if it.kind == exportItemHeader {
		check := "☐"
		if it.checked {
			check = "☑"
		}
		line := fmt.Sprintf("%s%s %s", cursor, check, it.modelDisplay)
		return m.ui.styles.bodyBold.Render(line) // headers always bold
	}

	check := "☐"
	if it.checked {
		check = "☑"
	}
	label := fmt.Sprintf("  %s · %s", it.backend, it.profileName)
	line := fmt.Sprintf("%s%s %s", cursor, check, label)

	if i == m.export.cursor && m.export.focus == exportFocusList {
		line = m.ui.styles.bodyBold.Render(line)
	} else {
		line = bodyStyle.Render(line)
	}
	return line
}

// scrollbarGlyph returns the scrollbar character for a given visible row.
func scrollbarGlyph(row, maxVis, totalItems, scrollOffset int) string {
	if totalItems <= maxVis {
		return ""
	}

	thumbSize := max(1, maxVis*maxVis/totalItems)
	thumbStart := scrollOffset * maxVis / totalItems

	if row == 0 && scrollOffset > 0 {
		return "▴"
	}
	if row == maxVis-1 && scrollOffset+maxVis < totalItems {
		return "▾"
	}
	if row >= thumbStart && row < thumbStart+thumbSize {
		return "█"
	}
	actualIdx := scrollOffset + row
	if actualIdx < totalItems {
		return "┃"
	}
	return " "
}

func (m Model) collisionModalBlock() string {
	cw := min(m.paramPanelContentWidth(), 60)
	panelBox := m.ui.styles.paramPanelBox
	bodyStyle := m.ui.styles.body

	destName := filepath.Base(m.collision.dest)
	suffixName := filepath.Base(m.collision.suffixPath)

	rows := []string{
		m.modalTitleRow(cw, m.ui.styles.portConfigTitle, "File exists"),
		"",
		bodyStyle.Render(fmt.Sprintf("%s already exists.", destName)),
		"",
		m.ui.styles.bodyBold.Render("[O] Overwrite"),
		bodyStyle.Render(fmt.Sprintf("[N] Save as %s", suffixName)),
		"",
		m.renderFooterHints(FooterCollisionHints),
	}

	block := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return panelBox.Width(cw + panelBox.GetHorizontalFrameSize()).Render(block)
}
